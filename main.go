package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"code-engine/runner"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	// Import your database package here
)

// StatusCache defines the lightweight payload stored in Redis RAM
type StatusCache struct {
	Status          string  `json:"status"`
	Stdout          *string `json:"stdout"`
	Stderr          *string `json:"stderr"`
	ExecutionTimeMs *int    `json:"execution_time_ms"`
}

var redisClient *redis.Client

func main() {
	// 1. Initialize Redis Connection
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for local dev
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 2. Start the Background Worker
	go StartWorker(context.Background(), redisClient)

	// 3. Start Gin Router
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		// 1. Allow the frontend to connect
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// 2. Explicitly allow the HTTP methods your app needs
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		// 3. Allow custom headers like Content-Type
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// 4. CRITICAL FIX: If the browser is just sending a Preflight OPTIONS request,
		// tell it "Yes, you are allowed!" and instantly stop processing the route.
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// --- ROUTE 1: INGESTION ---
	r.POST("/submit", func(c *gin.Context) {
		var req struct {
			Language string `json:"language"`
			Code     string `json:"code"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		// Save to Postgres to generate ID (Mocked here)
		subID := fmt.Sprintf("%d", time.Now().UnixNano())

		queuedState, _ := json.Marshal(StatusCache{Status: "queued"})
		redisClient.Set(c.Request.Context(), "status:"+subID, queuedState, 1*time.Hour)

		// Push to Redis Stream Queue
		redisClient.XAdd(c.Request.Context(), &redis.XAddArgs{
			Stream: "code_queue",
			Values: map[string]interface{}{
				"id":       subID,
				"language": req.Language,
				"code":     req.Code,
			},
		})

		c.JSON(http.StatusOK, gin.H{"submission_id": subID})
	})

	// --- ROUTE 2: REAL-TIME REDIS CACHE STREAM ---
	r.GET("/stream/:id", func(c *gin.Context) {
		subID := c.Param("id")

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")

		c.Stream(func(w io.Writer) bool {
			// Read directly from Redis RAM (Lightning Fast)
			val, err := redisClient.Get(c.Request.Context(), "status:"+subID).Result()

			var cacheData StatusCache

			if err == redis.Nil {
				// Key not found = Worker hasn't processed it yet
				cacheData = StatusCache{Status: "queued"}
			} else if err != nil {
				fmt.Printf("[Stream Error] Redis read failed: %v\n", err)
				return false
			} else {
				// Parse JSON string from Redis back to struct
				json.Unmarshal([]byte(val), &cacheData)
			}

			c.SSEvent("message", gin.H{
				"status":            cacheData.Status,
				"stdout":            cacheData.Stdout,
				"stderr":            cacheData.Stderr,
				"execution_time_ms": cacheData.ExecutionTimeMs,
			})

			// Close stream gracefully on terminal states
			if cacheData.Status == "completed" || cacheData.Status == "error" || cacheData.Status == "timeout" {
				return false
			}

			time.Sleep(500 * time.Millisecond)
			return true
		})
	})

	fmt.Println("API running on :8080")
	r.Run(":8080")
}

func StartWorker(ctx context.Context, redisClient *redis.Client) {
	fmt.Println("Worker listening to Redis Stream: code_queue...")

	if err := redisClient.XGroupCreateMkStream(ctx, "code_queue", "worker_group", "$").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		fmt.Printf("[Worker Error] Failed creating group: %v\n", err)
	}

	for {
		// 1. Pull next job from Redis Stream
		streams, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "worker_group",
			Consumer: "worker_1",
			Streams:  []string{"code_queue", ">"},
			Count:    1,
			Block:    2 * time.Second,
		}).Result()

		if err == redis.Nil {
			continue // No new jobs
		} else if err != nil {
			if strings.Contains(err.Error(), "NOGROUP") {
				_ = redisClient.XGroupCreateMkStream(ctx, "code_queue", "worker_group", "$").Err()
			}
			time.Sleep(1 * time.Second)
			continue
		}

		message := streams[0].Messages[0]
		subID := message.Values["id"].(string)
		language := message.Values["language"].(string)
		code := message.Values["code"].(string)

		// --- A. SET RUNNING STATE ---

		// WRITE TO REDIS CACHE
		runningState, _ := json.Marshal(StatusCache{Status: "running"})
		redisClient.Set(ctx, "status:"+subID, runningState, 1*time.Hour)

		// --- B. EXECUTE CODE ---

		startTime := time.Now()
		res, execErr := runner.ExecuteCode(language, code, 7*time.Second)
		duration := int(time.Since(startTime).Milliseconds())

		status := "completed"
		var stdoutPtr *string
		var stderrPtr *string
		execTimePtr := &duration

		if execErr != nil {
			status = "error"
			if res != nil && res.Stderr != "" {
				stderrPtr = &res.Stderr
			} else {
				errMsg := execErr.Error()
				stderrPtr = &errMsg
			}
		} else if res != nil {
			if res.TimedOut {
				status = "timeout"
			} else if res.Stderr != "" {
				status = "error"
			}
			if res.Stdout != "" {
				stdoutPtr = &res.Stdout
			}
			if res.Stderr != "" {
				stderrPtr = &res.Stderr
			}
			execTimePtr = func() *int {
				ms := int(res.Duration.Milliseconds())
				return &ms
			}()
		}

		// --- C. SET COMPLETED STATE ---

		// WRITE FINAL RESULTS TO REDIS CACHE
		completedState, _ := json.Marshal(StatusCache{
			Status:          status,
			Stdout:          stdoutPtr,
			Stderr:          stderrPtr,
			ExecutionTimeMs: execTimePtr,
		})
		redisClient.Set(ctx, "status:"+subID, completedState, 1*time.Hour)

		// Acknowledge task completion in the Stream
		redisClient.XAck(ctx, "code_queue", "worker_group", message.ID)

		fmt.Printf("Processed submission %s in %d ms\n", subID, duration)
	}
}
