package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

// NewStore initializes connections to Postgres and Redis
func NewStore() (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Connect to PostgreSQL using a connection pool (production grade)
	pgConnStr := "postgres://engine_user:engine_password@localhost:5435/code_engine?sslmode=disable"
	dbPool, err := pgxpool.New(ctx, pgConnStr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to postgres: %w", err)
	}

	// Ping Postgres to ensure communication channel is actually open
	if err := dbPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Store{
		DB:    dbPool,
		Redis: rdb,
	}, nil
}

// CreateSubmission saves the initial execution task to Postgres
func (s *Store) CreateSubmission(ctx context.Context, language, code string) (string, error) {
	query := `
		INSERT INTO submissions (language, code, status) 
		VALUES ($1, $2, 'queued') 
		RETURNING id;
	`
	var id string
	err := s.DB.QueryRow(ctx, query, language, code).Scan(&id)
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