package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"filebox/backend/migrations"
	"filebox/backend/models"
	"filebox/backend/validation"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var minioClient *minio.Client
var redisClient *redis.Client
var jwtSecret []byte
var bucketName string

// connectToDatabase attempts to connect to the database with retry logic
func connectToDatabase(databaseURL string) (*sql.DB, error) {
	maxRetries := 30
	retryInterval := 2 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			log.Printf("Failed to open database connection (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}
		
		if err = db.Ping(); err != nil {
			log.Printf("Failed to ping database (attempt %d/%d): %v", i+1, maxRetries, err)
			db.Close()
			time.Sleep(retryInterval)
			continue
		}
		
		log.Printf("Successfully connected to database on attempt %d", i+1)
		return db, nil
	}
	
	return nil, fmt.Errorf("failed to connect to database after %d attempts", maxRetries)
}

// connectToMinio attempts to connect to MinIO with retry logic
func connectToMinio() (*minio.Client, error) {
	maxRetries := 30
	retryInterval := 2 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		client, err := minio.New(mustGetenv("MINIO_ENDPOINT"), &minio.Options{
			Creds:  credentials.NewStaticV4(mustGetenv("MINIO_ACCESS_KEY"), mustGetenv("MINIO_SECRET_KEY"), ""),
			Secure: false,
		})
		if err != nil {
			log.Printf("Failed to create MinIO client (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}
		
		// Test connection by listing buckets
		_, err = client.ListBuckets(context.Background())
		if err != nil {
			log.Printf("Failed to connect to MinIO (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}
		
		log.Printf("Successfully connected to MinIO on attempt %d", i+1)
		return client, nil
	}
	
	return nil, fmt.Errorf("failed to connect to MinIO after %d attempts", maxRetries)
}

// connectToRedis attempts to connect to Redis with retry logic
func connectToRedis() (*redis.Client, error) {
	maxRetries := 30
	retryInterval := 2 * time.Second
	
	for i := 0; i < maxRetries; i++ {
		client := redis.NewClient(&redis.Options{
			Addr:     mustGetenv("REDIS_ADDR"),
			Password: os.Getenv("REDIS_PASSWORD"),
		})
		
		if err := client.Ping(context.Background()).Err(); err != nil {
			log.Printf("Failed to connect to Redis (attempt %d/%d): %v", i+1, maxRetries, err)
			client.Close()
			time.Sleep(retryInterval)
			continue
		}
		
		log.Printf("Successfully connected to Redis on attempt %d", i+1)
		return client, nil
	}
	
	return nil, fmt.Errorf("failed to connect to Redis after %d attempts", maxRetries)
}

func main() {
	var err error
	godotenv.Load()
	
	// Connect to database with retry logic
	db, err = connectToDatabase(mustGetenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to database after retries: %v", err)
	}
	
	// Run database migrations
	migrator := migrations.NewMigrator(db)
	if err := migrator.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Connect to MinIO with retry logic
	minioClient, err = connectToMinio()
	if err != nil {
		log.Fatalf("Failed to connect to MinIO after retries: %v", err)
	}
	
	// Setup bucket
	bucketName = mustGetenv("MINIO_BUCKET")
	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err == nil && !exists {
		if err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{}); err != nil {
			log.Fatalf("Failed to create MinIO bucket: %v", err)
		}
	}
	
	jwtSecret = []byte(mustGetenv("JWT_SECRET"))

	// Connect to Redis with retry logic
	redisClient, err = connectToRedis()
	if err != nil {
		log.Fatalf("Failed to connect to Redis after retries: %v", err)
	}

	r := gin.Default()
	r.POST("/register", register)
	r.POST("/login", login)
	auth := r.Group("/", authMiddleware)
	auth.POST("/files/presign", presign)
	auth.POST("/files", createFile)
	auth.GET("/files", listFiles)
	auth.POST("/shares/:id", createShare)
	r.GET("/shares/:token", downloadShare)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

// initDB function is no longer needed as migrations handle schema creation

type credentialsInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func register(c *gin.Context) {
	var in credentialsInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	
	// Validate email format
	if err := validation.ValidateEmail(in.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate password strength
	if err := validation.ValidatePassword(in.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Hash password with higher cost for better security
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hashing failed"})
		return
	}
	
	// Insert user with created_at timestamp
	_, err = db.Exec("INSERT INTO users(email, password, created_at) VALUES($1, $2, NOW())", in.Email, string(hash))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
		return
	}
	
	c.Status(http.StatusCreated)
}

func login(c *gin.Context) {
	var in credentialsInput
	if err := c.BindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	var id int
	var hash string
	err := db.QueryRow("SELECT id,password FROM users WHERE email=$1", in.Email).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id,
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	})
	s, _ := token.SignedString(jwtSecret)
	c.JSON(http.StatusOK, gin.H{"token": s})
}

type fileRequest struct {
	Name        string        `json:"name"`
	Size        *int64        `json:"size,omitempty"`
	ContentType *string       `json:"content_type,omitempty"`
	Tags        models.Tags   `json:"tags,omitempty"`
}

func presign(c *gin.Context) {
	var req fileRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	
	// Validate file name
	if err := validation.ValidateFileName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Validate file size if provided
	if req.Size != nil {
		if err := validation.ValidateFileSize(*req.Size); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	
	// Validate content type if provided
	if req.ContentType != nil {
		if err := validation.ValidateContentType(*req.ContentType); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	
	// Generate pre-signed URL with 10-minute expiration
	url, err := minioClient.PresignedPutObject(context.Background(), bucketName, req.Name, time.Minute*10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presign failed"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"url": url.String()})
}

func createFile(c *gin.Context) {
	userID := c.GetInt("userID")
	var req fileRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	
	// Create file model for validation
	file := &models.File{
		UserID:      userID,
		Name:        req.Name,
		Size:        req.Size,
		ContentType: req.ContentType,
		Tags:        req.Tags,
	}
	
	// Validate file data
	if errors := validation.ValidateFile(file); errors.HasErrors() {
		c.JSON(http.StatusBadRequest, gin.H{"error": errors.Error()})
		return
	}
	
	// Insert file with comprehensive metadata
	var fileID int
	err := db.QueryRow(`
		INSERT INTO files(user_id, name, size, content_type, tags, created_at, updated_at) 
		VALUES($1, $2, $3, $4, $5, NOW(), NOW()) 
		RETURNING id`, 
		userID, req.Name, req.Size, req.ContentType, req.Tags).Scan(&fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file metadata"})
		return
	}
	
	// Log activity
	logActivity(&userID, "file_upload", "file", &fileID, map[string]interface{}{
		"file_name": req.Name,
		"file_size": req.Size,
	})
	
	// Return created file metadata
	file.ID = fileID
	c.JSON(http.StatusOK, file)
}

func listFiles(c *gin.Context) {
	userID := c.GetInt("userID")
	
	// Query with comprehensive file metadata
	rows, err := db.Query(`
		SELECT id, user_id, name, size, content_type, tags, created_at, updated_at 
		FROM files 
		WHERE user_id = $1 
		ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	
	var files []models.File
	for rows.Next() {
		var f models.File
		err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.Size, &f.ContentType, &f.Tags, &f.CreatedAt, &f.UpdatedAt)
		if err == nil {
			files = append(files, f)
		}
	}
	
	// Return empty array instead of null if no files
	if files == nil {
		files = []models.File{}
	}
	
	c.JSON(http.StatusOK, files)
}

func createShare(c *gin.Context) {
	userID := c.GetInt("userID")
	fileID := c.Param("id")
	
	// Verify file ownership and get file details
	var owner int
	var name string
	err := db.QueryRow("SELECT user_id, name FROM files WHERE id = $1", fileID).Scan(&owner, &name)
	if err != nil || owner != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	
	// Generate cryptographically secure token
	token := generateSecureToken()
	expires := time.Now().Add(10 * time.Minute)
	
	// Insert share record with proper timestamps
	var shareID int
	err = db.QueryRow(`
		INSERT INTO shares(file_id, token, expires, created_at) 
		VALUES($1, $2, $3, NOW()) 
		RETURNING id`, fileID, token, expires).Scan(&shareID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create share"})
		return
	}
	
	// Cache share token in Redis with TTL
	ctx := context.Background()
	redisClient.Set(ctx, token, name, time.Until(expires))
	
	// Log activity
	logActivity(&userID, "share_create", "share", &shareID, map[string]interface{}{
		"file_id":   fileID,
		"file_name": name,
		"expires":   expires,
	})
	
	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"expires": expires,
	})
}

func downloadShare(c *gin.Context) {
	token := c.Param("token")
	ctx := context.Background()
	
	// Try to get file name from Redis cache first
	name, err := redisClient.Get(ctx, token).Result()
	if err == redis.Nil {
		// Cache miss - query database
		var fileID int
		var expires time.Time
		err = db.QueryRow("SELECT file_id, expires FROM shares WHERE token = $1", token).Scan(&fileID, &expires)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "share token not found"})
			return
		}
		
		// Check if share has expired
		if time.Now().After(expires) {
			c.JSON(http.StatusNotFound, gin.H{"error": "share token expired"})
			return
		}
		
		// Get file name
		err = db.QueryRow("SELECT name FROM files WHERE id = $1", fileID).Scan(&name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		
		// Cache the result with remaining TTL
		redisClient.Set(ctx, token, name, time.Until(expires))
		
		// Log activity (anonymous access)
		logActivity(nil, "share_access", "share", nil, map[string]interface{}{
			"token":     token,
			"file_id":   fileID,
			"file_name": name,
		})
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache error"})
		return
	}
	
	// Generate pre-signed download URL
	url, err := minioClient.PresignedGetObject(ctx, bucketName, name, time.Minute*10, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate download URL"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"url":       url.String(),
		"file_name": name,
	})
}

func authMiddleware(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := jwt.Parse(header[7:], func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	c.Set("userID", int(claims["sub"].(float64)))
	c.Next()
}

func generateSecureToken() string {
	b := make([]byte, 32) // Increased to 32 bytes for better security
	if _, err := rand.Read(b); err != nil {
		log.Printf("Failed to generate secure token: %v", err)
		return ""
	}
	return fmt.Sprintf("%x", b)
}

// logActivity logs user activities for audit trail
func logActivity(userID *int, action, resourceType string, resourceID *int, metadata map[string]interface{}) {
	metadataJSON, _ := json.Marshal(metadata)
	
	_, err := db.Exec(`
		INSERT INTO activities(user_id, action, resource_type, resource_id, metadata, created_at) 
		VALUES($1, $2, $3, $4, $5, NOW())`,
		userID, action, resourceType, resourceID, metadataJSON)
	
	if err != nil {
		log.Printf("Failed to log activity: %v", err)
	}
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s not set", key)
	}
	return v
}
