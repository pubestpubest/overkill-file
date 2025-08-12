# Database Migrations

This directory contains the database migration system for FileBox. The migration system provides versioned schema management with automatic rollback protection and comprehensive logging.

## Migration Files

Migration files follow the naming convention: `{version}_{description}.sql`

- `001_initial_schema.sql` - Creates the complete database schema with tables, indexes, and constraints

## Database Schema

### Tables

#### users
- `id` (SERIAL PRIMARY KEY) - Unique user identifier
- `email` (TEXT UNIQUE NOT NULL) - User email address (used for authentication)
- `password` (TEXT NOT NULL) - Bcrypt hashed password
- `created_at` (TIMESTAMPTZ DEFAULT NOW()) - Account creation timestamp

#### files
- `id` (SERIAL PRIMARY KEY) - Unique file identifier
- `user_id` (INT NOT NULL) - Foreign key to users table with CASCADE DELETE
- `name` (TEXT NOT NULL) - Original filename
- `size` (BIGINT) - File size in bytes
- `content_type` (TEXT) - MIME type of the file
- `tags` (TEXT[]) - Array of user-defined tags for categorization
- `created_at` (TIMESTAMPTZ DEFAULT NOW()) - Upload timestamp
- `updated_at` (TIMESTAMPTZ DEFAULT NOW()) - Last modification timestamp

#### shares
- `id` (SERIAL PRIMARY KEY) - Unique share identifier
- `file_id` (INT NOT NULL) - Foreign key to files table with CASCADE DELETE
- `token` (TEXT UNIQUE NOT NULL) - Cryptographically secure share token
- `expires` (TIMESTAMPTZ NOT NULL) - Share expiration timestamp
- `created_at` (TIMESTAMPTZ DEFAULT NOW()) - Share creation timestamp

#### activities
- `id` (SERIAL PRIMARY KEY) - Unique activity identifier
- `user_id` (INT) - Foreign key to users table with SET NULL on delete
- `action` (TEXT NOT NULL) - Action type (e.g., 'file_upload', 'share_create')
- `resource_type` (TEXT) - Type of resource affected
- `resource_id` (INT) - ID of the affected resource
- `metadata` (JSONB) - Additional context data in JSON format
- `created_at` (TIMESTAMPTZ DEFAULT NOW()) - Activity timestamp

#### schema_migrations
- `version` (INT PRIMARY KEY) - Migration version number
- `applied_at` (TIMESTAMPTZ DEFAULT NOW()) - Migration application timestamp

### Indexes

Performance indexes are created for common query patterns:

- `idx_files_user_id` - Fast file lookups by user
- `idx_files_tags` - GIN index for tag-based searches
- `idx_files_created_at` - Chronological file ordering
- `idx_shares_token` - Fast share token lookups
- `idx_shares_expires` - Efficient expired share cleanup
- `idx_shares_file_id` - Share lookups by file
- `idx_activities_user_id` - User activity history
- `idx_activities_created_at` - Chronological activity ordering
- `idx_activities_action` - Activity filtering by action type

### Constraints and Relationships

- **Foreign Key Constraints**: All relationships use proper foreign keys with appropriate cascade behavior
- **Unique Constraints**: Email addresses and share tokens are unique
- **NOT NULL Constraints**: Critical fields are marked as required
- **Cascade Deletes**: File deletion removes associated shares; user deletion removes files and shares
- **Automatic Timestamps**: `updated_at` is automatically maintained via triggers

### Triggers

- `update_files_updated_at` - Automatically updates the `updated_at` timestamp when files are modified

## Migration System Usage

### Running Migrations

Migrations are automatically executed when the application starts:

```go
migrator := migrations.NewMigrator(db)
if err := migrator.RunMigrations(); err != nil {
    log.Fatalf("Failed to run migrations: %v", err)
}
```

### Creating New Migrations

1. Create a new SQL file with the next version number: `002_add_new_feature.sql`
2. Write the migration SQL (CREATE, ALTER, INSERT statements)
3. The migration system will automatically detect and apply it on next startup

### Migration Features

- **Idempotent**: Safe to run multiple times
- **Transactional**: Each migration runs in a transaction with automatic rollback on failure
- **Versioned**: Tracks applied migrations in `schema_migrations` table
- **Ordered**: Migrations are applied in version order
- **Logged**: Comprehensive logging of migration execution

### Testing Migrations

Run migration tests with a test database:

```bash
TEST_DATABASE_URL="postgres://user:pass@localhost/test_db" go test ./migrations -v
```

## Security Considerations

- **Password Hashing**: Uses bcrypt with cost factor 12
- **Token Generation**: Share tokens use cryptographically secure random generation (32 bytes)
- **Input Validation**: All user inputs are validated before database operations
- **SQL Injection Protection**: All queries use parameterized statements
- **Cascade Deletes**: Proper cleanup of related data on user/file deletion

## Performance Considerations

- **Connection Pooling**: Database connections are pooled at the application level
- **Indexed Queries**: All common query patterns have supporting indexes
- **Tag Searches**: GIN indexes enable fast tag-based file searches
- **Pagination Ready**: Schema supports efficient pagination with created_at ordering
- **Cache Integration**: Share tokens are cached in Redis for performance

## Monitoring and Observability

- **Activity Logging**: All user actions are logged in the activities table
- **Audit Trail**: Complete history of file operations and share creation
- **Metadata Storage**: JSONB fields store additional context for debugging
- **Timestamp Tracking**: All operations include precise timestamps
- **Error Context**: Failed operations include correlation IDs for troubleshooting