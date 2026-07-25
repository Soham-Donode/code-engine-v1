package store

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

type APIKey struct {
	ID            string    `json:"id"`
	KeyHash       string    `json:"key_hash"`
	KeyPrefix     string    `json:"key_prefix"`
	Label         *string   `json:"label"`
	RequestsToday int       `json:"requests_today"`
	DailyLimit    int       `json:"daily_limit"`
	CreatedAt     time.Time `json:"created_at"`
	LastResetAt   time.Time `json:"last_reset_at"`
	Revoked       bool      `json:"revoked"`
}

// NewStore initializes connections to Postgres and Redis
func NewStore() (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Connect to PostgreSQL using a connection pool (production grade)
	pgConnStr := os.Getenv("DATABASE_URL")
	if pgConnStr == "" {
		pgConnStr = "postgres://engine_user:engine_password@postgres:5432/code_engine?sslmode=disable"
	}
	dbPool, err := pgxpool.New(ctx, pgConnStr)
	if err != nil || dbPool.Ping(ctx) != nil {
		// Fallback for local dev outside docker
		fallbackConn := "postgres://engine_user:engine_password@localhost:5435/code_engine?sslmode=disable"
		if altPool, altErr := pgxpool.New(ctx, fallbackConn); altErr == nil && altPool.Ping(ctx) == nil {
			dbPool = altPool
		} else if err != nil {
			return nil, fmt.Errorf("unable to connect to postgres: %w", err)
		}
	}

	// 2. Connect to Redis
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Store{
		DB:    dbPool,
		Redis: rdb,
	}, nil
}

// CreateAPIKey persists a new hashed API key record to Postgres
func (s *Store) CreateAPIKey(ctx context.Context, keyHash, keyPrefix string, label *string) (*APIKey, error) {
	query := `
		INSERT INTO api_keys (key_hash, key_prefix, label, requests_today, daily_limit, revoked)
		VALUES ($1, $2, $3, 0, 100, false)
		RETURNING id, key_hash, key_prefix, label, requests_today, daily_limit, created_at, last_reset_at, revoked;
	`
	var k APIKey
	err := s.DB.QueryRow(ctx, query, keyHash, keyPrefix, label).Scan(
		&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Label, &k.RequestsToday, &k.DailyLimit, &k.CreatedAt, &k.LastResetAt, &k.Revoked,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetAPIKeyByHash retrieves an API key by SHA-256 hash string
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `
		SELECT id, key_hash, key_prefix, label, requests_today, daily_limit, created_at, last_reset_at, revoked
		FROM api_keys
		WHERE key_hash = $1;
	`
	var k APIKey
	err := s.DB.QueryRow(ctx, query, keyHash).Scan(
		&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Label, &k.RequestsToday, &k.DailyLimit, &k.CreatedAt, &k.LastResetAt, &k.Revoked,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetAPIKeyByPrefix retrieves an API key usage record by prefix string
func (s *Store) GetAPIKeyByPrefix(ctx context.Context, keyPrefix string) (*APIKey, error) {
	query := `
		SELECT id, key_hash, key_prefix, label, requests_today, daily_limit, created_at, last_reset_at, revoked
		FROM api_keys
		WHERE key_prefix = $1;
	`
	var k APIKey
	err := s.DB.QueryRow(ctx, query, keyPrefix).Scan(
		&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Label, &k.RequestsToday, &k.DailyLimit, &k.CreatedAt, &k.LastResetAt, &k.Revoked,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// RevokeAPIKey flags a key as revoked by key_prefix
func (s *Store) RevokeAPIKey(ctx context.Context, keyPrefix string) error {
	query := `UPDATE api_keys SET revoked = true WHERE key_prefix = $1;`
	res, err := s.DB.Exec(ctx, query, keyPrefix)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("api key with prefix %s not found", keyPrefix)
	}
	return nil
}

// IncrementAPIKeyUsage increments the daily request count for a given API key ID
func (s *Store) IncrementAPIKeyUsage(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET requests_today = requests_today + 1 WHERE id = $1;`
	_, err := s.DB.Exec(ctx, query, id)
	return err
}

// ResetAPIKeyUsage resets request count to 0 and updates last_reset_at for a single key
func (s *Store) ResetAPIKeyUsage(ctx context.Context, id string) error {
	query := `UPDATE api_keys SET requests_today = 0, last_reset_at = NOW() WHERE id = $1;`
	_, err := s.DB.Exec(ctx, query, id)
	return err
}

// ResetExpiredAPIKeys bulk resets request counts for all keys whose last_reset_at is older than 24h
func (s *Store) ResetExpiredAPIKeys(ctx context.Context) (int64, error) {
	query := `
		UPDATE api_keys
		SET requests_today = 0, last_reset_at = NOW()
		WHERE last_reset_at <= NOW() - INTERVAL '24 hours' AND revoked = false;
	`
	res, err := s.DB.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// CreateSubmission saves the initial execution task to Postgres with optional api_key_id
func (s *Store) CreateSubmission(ctx context.Context, language, code string, apiKeyID *string) (string, error) {
	query := `
		INSERT INTO submissions (language, code, status, api_key_id) 
		VALUES ($1, $2, 'queued', $3) 
		RETURNING id;
	`
	var id string
	err := s.DB.QueryRow(ctx, query, language, code, apiKeyID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpdateSubmissionStatus changes states and records logs when execution terminates
func (s *Store) UpdateSubmissionStatus(ctx context.Context, id, status, stdout, stderr string, durationMs int) error {
	query := `
		UPDATE submissions 
		SET status = $1, stdout = $2, stderr = $3, execution_time_ms = $4, completed_at = NOW()
		WHERE id = $5;
	`
	_, err := s.DB.Exec(ctx, query, status, stdout, stderr, durationMs, id)
	return err
}

// EnqueueSubmission pushes a job task containing the submission ID into a Redis Stream
func (s *Store) EnqueueSubmission(ctx context.Context, submissionID string) error {
	streamName := "submission_stream"
	
	// Create the job payload data map
	jobData := map[string]interface{}{
		"submission_id": submissionID,
	}

	// XAdd appends the message to the stream. "*" instructs Redis to auto-generate a unique ID
	err := s.Redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: jobData,
	}).Err()

	return err
}

// DequeueSubmission blocks and waits for a new job to arrive in the Redis Stream
func (s *Store) DequeueSubmission(ctx context.Context) (string, error) {
	streamName := "submission_stream"

	// XRead blocks and listens for new messages.
	// "0" means listen indefinitely until a message arrives ($ represents new messages)
	streams, err := s.Redis.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamName, "$"},
		Block:   0, // 0 means block forever until a message is available
		Count:   1, // Fetch 1 job at a time to distribute weight evenly
	}).Result()

	if err != nil {
		return "", err
	}

	// Parse out the nested submission_id string from the Redis stream structure
	if len(streams) > 0 && len(streams[0].Messages) > 0 {
		msg := streams[0].Messages[0]
		if subID, ok := msg.Values["submission_id"].(string); ok {
			return subID, nil
		}
	}

	return "", fmt.Errorf("received empty or invalid message payload structure")
}

// GetSubmissionDetails pulls the raw code out of Postgres when a worker starts processing it
func (s *Store) GetSubmissionDetails(ctx context.Context, id string) (string, string, error) {
	var code, language string
	query := "SELECT code, language FROM submissions WHERE id = $1"
	err := s.DB.QueryRow(ctx, query, id).Scan(&code, &language)
	return code, language, err
}

// UpdateSubmissionToRunning updates the state to 'running' right before container execution starts
func (s *Store) UpdateSubmissionToRunning(ctx context.Context, id string) error {
	query := "UPDATE submissions SET status = 'running', started_at = NOW() WHERE id = $1"
	_, err := s.DB.Exec(ctx, query, id)
	return err
}