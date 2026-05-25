package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"code-engine/runner"
	"code-engine/store"

	"github.com/gin-gonic/gin"
)

// Define the incoming request structure
type SubmitRequest struct {
	Language string `json:"language" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

func main() {
	// 1. Initialize Storage Connections
	fmt.Println("[System] Connecting to Infrastructure Services...")
	dataStore, err := store.NewStore()
	if err != nil {
		panic(err)
	}
	defer dataStore.DB.Close()
	defer dataStore.Redis.Close()

	// 2. Fire up our background worker execution loop in an isolated goroutine
	go startBackgroundWorker(dataStore)

	// 3. Initialize the HTTP Gin Server
	r := gin.Default()

	// Apply the foolproof, bulletproof production CORS policy:
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		// CRITICAL LINE: Added 'content-type' explicitly in lowercase as well
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, content-type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Route 1: Code Ingestion Endpoint
	r.POST("/submit", func(c *gin.Context) {
		var req SubmitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Store initially in Postgres
		subID, err := dataStore.CreateSubmission(c.Request.Context(), req.Language, req.Code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database write failure"})
			return
		}

		// Push to Redis Queue Stream
		if err := dataStore.EnqueueSubmission(c.Request.Context(), subID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Queue streaming failure"})
			return
		}

		// Return tracking ID immediately
		c.JSON(http.StatusAccepted, gin.H{
			"submission_id": subID,
			"status":        "queued",
		})
	})

	// Route 2: Status Checking & Result Retrieval Endpoint
	r.GET("/status/:id", func(c *gin.Context) {
		id := c.Param("id")

		var status, stdout, stderr string
		var executionTime *int

		// Query current state directly from PostgreSQL
		query := "SELECT status, stdout, stderr, execution_time_ms FROM submissions WHERE id = $1"
		err := dataStore.DB.QueryRow(c.Request.Context(), query, id).Scan(&status, &stdout, &stderr, &executionTime)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Submission tracking record not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"submission_id":     id,
			"status":            status,
			"stdout":            stdout,
			"stderr":            stderr,
			"execution_time_ms": executionTime,
		})
	})

	// Real-Time Server-Sent Events (SSE) Endpoint
	r.GET("/stream/:id", func(c *gin.Context) {
		subID := c.Param("id")

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		c.Stream(func(w io.Writer) bool {
			var status string
			// CHANGED: Use pointers so Go can safely absorb Postgres NULL values
			var stdout, stderr *string
			var execTime *int

			query := "SELECT status, stdout, stderr, execution_time_ms FROM submissions WHERE id = $1"
			err := dataStore.DB.QueryRow(c.Request.Context(), query, subID).Scan(&status, &stdout, &stderr, &execTime)

			if err != nil {
				// Pro-tip: Print the error to your terminal so you aren't debugging blindly if it fails
				fmt.Printf("[Stream Error] Failed to fetch sub %s: %v\n", subID, err)
				return false
			}

			c.SSEvent("message", gin.H{
				"status":            status,
				"stdout":            stdout,
				"stderr":            stderr,
				"execution_time_ms": execTime,
			})

			if status == "completed" || status == "error" || status == "timeout" {
				return false
			}

			time.Sleep(500 * time.Millisecond)

			return true
		})
	})

	fmt.Println("[System] Real-time Remote Code Execution Engine listening on port :8080...")
	r.Run(":8080")
}

// Background worker loops indefinitely, waiting to execute items from the Redis Queue
func startBackgroundWorker(s *store.Store) {
	ctx := context.Background()
	fmt.Println("[Worker Thread] Background consumer loop initialized. Awaiting tasks...")

	for {
		subID, err := s.DequeueSubmission(ctx)
		if err != nil {
			continue
		}

		fmt.Printf("\n[Worker Process] Claimed Task ID: %s\n", subID)

		if err := s.UpdateSubmissionToRunning(ctx, subID); err != nil {
			continue
		}

		code, language, err := s.GetSubmissionDetails(ctx, subID)
		if err != nil {
			continue
		}

		// Sandbox executes with a hard limit timeout
		res, err := runner.ExecuteCode(language, code, 7*time.Second)
		if err != nil {
			continue
		}

		finalStatus := "completed"
		if res.TimedOut {
			finalStatus = "timeout"
		} else if res.Stderr != "" {
			finalStatus = "error"
		}

		s.UpdateSubmissionStatus(
			ctx,
			subID,
			finalStatus,
			strings.TrimSpace(res.Stdout), // <-- Update this to TrimSpace
			strings.TrimSpace(res.Stderr), // <-- Update this to TrimSpace
			int(res.Duration.Milliseconds()),
		)
		fmt.Printf("[Worker Process] Finished Job %s -> State: %s\n", subID, finalStatus)
	}
}
