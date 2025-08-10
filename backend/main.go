package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

func main() {
	var err error
	godotenv.Load()
	db, err = sql.Open("postgres", mustGetenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	initDB()

	minioClient, err = minio.New(mustGetenv("MINIO_ENDPOINT"), &minio.Options{
		Creds:  credentials.NewStaticV4(mustGetenv("MINIO_ACCESS_KEY"), mustGetenv("MINIO_SECRET_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal(err)
	}
	bucketName = mustGetenv("MINIO_BUCKET")
	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err == nil && !exists {
		if err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{}); err != nil {
			log.Fatal(err)
		}
	}
	jwtSecret = []byte(mustGetenv("JWT_SECRET"))

	redisClient = redis.NewClient(&redis.Options{
		Addr:     mustGetenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal(err)
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

func initDB() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        email TEXT UNIQUE NOT NULL,
        password TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS files (
        id SERIAL PRIMARY KEY,
        user_id INT REFERENCES users(id),
        name TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS shares (
        id SERIAL PRIMARY KEY,
        file_id INT REFERENCES files(id),
        token TEXT UNIQUE NOT NULL,
        expires TIMESTAMPTZ NOT NULL
    );`)
	if err != nil {
		log.Fatal(err)
	}
}

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
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	_, err := db.Exec("INSERT INTO users(email,password) VALUES($1,$2)", in.Email, string(hash))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email exists"})
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

type fileMeta struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func presign(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}
	url, err := minioClient.PresignedPutObject(context.Background(), bucketName, req.Name, time.Minute*10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presign failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url.String()})
}

func createFile(c *gin.Context) {
	userID := c.GetInt("userID")
	var meta fileMeta
	if err := c.BindJSON(&meta); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid"})
		return
	}
	err := db.QueryRow("INSERT INTO files(user_id,name) VALUES($1,$2) RETURNING id", userID, meta.Name).Scan(&meta.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, meta)
}

func listFiles(c *gin.Context) {
	userID := c.GetInt("userID")
	rows, err := db.Query("SELECT id,name FROM files WHERE user_id=$1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	var files []fileMeta
	for rows.Next() {
		var f fileMeta
		if err := rows.Scan(&f.ID, &f.Name); err == nil {
			files = append(files, f)
		}
	}
	c.JSON(http.StatusOK, files)
}

func createShare(c *gin.Context) {
	userID := c.GetInt("userID")
	fileID := c.Param("id")
	var owner int
	var name string
	err := db.QueryRow("SELECT user_id,name FROM files WHERE id=$1", fileID).Scan(&owner, &name)
	if err != nil || owner != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	token := generateToken()
	expires := time.Now().Add(10 * time.Minute)
	_, err = db.Exec("INSERT INTO shares(file_id,token,expires) VALUES($1,$2,$3)", fileID, token, expires)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "share failed"})
		return
	}
	redisClient.Set(context.Background(), token, name, time.Until(expires))
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func downloadShare(c *gin.Context) {
	token := c.Param("token")
	ctx := context.Background()
	name, err := redisClient.Get(ctx, token).Result()
	if err == redis.Nil {
		var fileID int
		var expires time.Time
		err = db.QueryRow("SELECT file_id,expires FROM shares WHERE token=$1", token).Scan(&fileID, &expires)
		if err != nil || time.Now().After(expires) {
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid"})
			return
		}
		err = db.QueryRow("SELECT name FROM files WHERE id=$1", fileID).Scan(&name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		redisClient.Set(ctx, token, name, time.Until(expires))
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache failed"})
		return
	}
	url, err := minioClient.PresignedGetObject(ctx, bucketName, name, time.Minute*10, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presign failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url.String()})
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

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", b)
}

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s not set", key)
	}
	return v
}
