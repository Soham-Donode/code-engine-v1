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
	pgConnStr := "postgres://engine_user:engine_password@localhost:5432/code_engine?sslmode=disable"
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