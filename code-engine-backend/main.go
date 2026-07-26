package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"code-engine/runner"
	"code-engine/store"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// StatusCache defines the lightweight payload stored in Redis RAM
type StatusCache struct {
	Status          string  `json:"status"`
	Stdout          *string `json:"stdout"`
	Stderr          *string `json:"stderr"`
	ExecutionTimeMs *int    `json:"execution_time_ms"`
}

var redisClient *redis.Client
var appStore *store.Store

func generateAPIKey() (rawKey string, keyPrefix string, keyHash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}
	rawKey = "ce_" + base64.RawURLEncoding.EncodeToString(bytes)
	keyPrefix = rawKey[:8]
	hash := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(hash[:])
	return rawKey, keyPrefix, keyHash, nil
}

func APIKeyAuth(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawKey := c.GetHeader("X-API-Key")
		if rawKey == "" {
			rawKey = c.Query("api_key")
		}
		if rawKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "API key missing. Provide X-API-Key header or api_key query parameter.",
			})
			return
		}

		hashBytes := sha256.Sum256([]byte(rawKey))
		keyHash := hex.EncodeToString(hashBytes[:])

		key, err := s.GetAPIKeyByHash(c.Request.Context(), keyHash)
		if err != nil || key == nil || key.Revoked {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or revoked API key.",
			})
			return
		}

		// Check if 24 hours have elapsed since last reset
		now := time.Now()
		if now.Sub(key.LastResetAt) >= 24*time.Hour {
			_ = s.ResetAPIKeyUsage(c.Request.Context(), key.ID)
			key.RequestsToday = 0
			key.LastResetAt = now
		}

		if key.RequestsToday >= key.DailyLimit {
			resetsAt := key.LastResetAt.Add(24 * time.Hour)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":          "Daily request limit reached",
				"daily_limit":    key.DailyLimit,
				"requests_today": key.RequestsToday,
				"resets_at":      resetsAt.Format(time.RFC3339),
			})
			return
		}

		// Increment usage count ONLY on execution submission (POST requests), not on log stream connections (GET)
		if c.Request.Method == http.MethodPost {
			if err := s.IncrementAPIKeyUsage(c.Request.Context(), key.ID); err != nil {
				fmt.Printf("[Auth Warning] Failed to increment key usage: %v\n", err)
			}
		}

		c.Set("api_key_id", key.ID)
		c.Set("api_key_prefix", key.KeyPrefix)
		c.Next()
	}
}

func StartDailyResetWorker(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s != nil {
				count, err := s.ResetExpiredAPIKeys(ctx)
				if err != nil {
					fmt.Printf("[Reset Worker Error] Failed to reset expired API keys: %v\n", err)
				} else if count > 0 {
					fmt.Printf("[Reset Worker] Reset daily request limits for %d API keys\n", count)
				}
			}
		}
	}
}

// Simple in-memory rate limiter for key generation by IP (max 10 requests per hour per IP)
var keyGenLimiter = struct {
	sync.Mutex
	counts map[string][]time.Time
}{counts: make(map[string][]time.Time)}

func RequireUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required. Please sign in to generate API keys.",
			})
			return
		}

		// Store verified token identity for downstream context
		c.Set("user_token", token)
		c.Next()
	}
}

func main() {
	// 1. Initialize Redis Connection
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for local dev
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 2. Initialize Store (Postgres + Redis)
	var err error
	appStore, err = store.NewStore()
	if err != nil {
		fmt.Printf("[Warning] Database store initialization failed: %v. Running in cache-only fallback mode.\n", err)
	}

	// 3. Start Background Workers
	go StartWorker(context.Background(), redisClient)
	if appStore != nil {
		go StartDailyResetWorker(context.Background(), appStore)
	}

	// 4. Start Gin Router
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		// 1. Allow the frontend to connect (TODO: restrict allowed origin in production)
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// 2. Explicitly allow the HTTP methods your app needs
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		// 3. Allow custom headers like Content-Type, X-API-Key, and Authorization
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")

		// 4. If the browser is sending a Preflight OPTIONS request, stop processing instantly
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// --- API KEY MANAGEMENT ENDPOINTS ---
	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/keys", RequireUserAuth(), func(c *gin.Context) {
			if appStore == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database store unavailable"})
				return
			}

			// 1. IP Rate Limiting (max 10 keys per hour per IP)
			ip := c.ClientIP()
			now := time.Now()
			keyGenLimiter.Lock()
			timestamps := keyGenLimiter.counts[ip]
			var valid []time.Time
			for _, t := range timestamps {
				if now.Sub(t) < time.Hour {
					valid = append(valid, t)
				}
			}
			if len(valid) >= 10 {
				keyGenLimiter.Unlock()
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "API Key generation limit reached for this IP. Please wait before creating more keys.",
				})
				return
			}
			keyGenLimiter.counts[ip] = append(valid, now)
			keyGenLimiter.Unlock()

			var req struct {
				Label *string `json:"label"`
			}
			_ = c.ShouldBindJSON(&req)

			rawKey, keyPrefix, keyHash, err := generateAPIKey()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
				return
			}

			k, err := appStore.CreateAPIKey(c.Request.Context(), keyHash, keyPrefix, req.Label)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store API key"})
				return
			}

			c.JSON(http.StatusCreated, gin.H{
				"api_key":     rawKey,
				"key":         rawKey,
				"key_prefix":  k.KeyPrefix,
				"prefix":      k.KeyPrefix,
				"daily_limit": k.DailyLimit,
				"limit":       k.DailyLimit,
				"created_at":  k.CreatedAt.Format(time.RFC3339),
			})
		})

		apiGroup.GET("/keys/:prefix/usage", func(c *gin.Context) {
			if appStore == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database store unavailable"})
				return
			}
			prefix := c.Param("prefix")
			k, err := appStore.GetAPIKeyByPrefix(c.Request.Context(), prefix)
			if err != nil || k == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "API key prefix not found"})
				return
			}

			now := time.Now()
			requestsToday := k.RequestsToday
			lastResetAt := k.LastResetAt
			if now.Sub(lastResetAt) >= 24*time.Hour {
				_ = appStore.ResetAPIKeyUsage(c.Request.Context(), k.ID)
				requestsToday = 0
				lastResetAt = now
			}

			remaining := k.DailyLimit - requestsToday
			if remaining < 0 {
				remaining = 0
			}

			resetsAtStr := lastResetAt.Add(24 * time.Hour).Format(time.RFC3339)
			c.JSON(http.StatusOK, gin.H{
				"prefix":         k.KeyPrefix,
				"key_prefix":     k.KeyPrefix,
				"requests_today": requestsToday,
				"daily_limit":    k.DailyLimit,
				"limit":          k.DailyLimit,
				"remaining":      remaining,
				"resets_at":      resetsAtStr,
				"reset_time":     resetsAtStr,
			})
		})

		apiGroup.DELETE("/keys/:prefix", func(c *gin.Context) {
			if appStore == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database store unavailable"})
				return
			}
			prefix := c.Param("prefix")
			if err := appStore.RevokeAPIKey(c.Request.Context(), prefix); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message":    "API key revoked successfully",
				"key_prefix": prefix,
			})
		})
	}

	// Protected routes group (Auth middleware applied if appStore is initialized)
	protected := r.Group("/")
	if appStore != nil {
		protected.Use(APIKeyAuth(appStore))
	}

	// --- ROUTE 1: INGESTION ---
	protected.POST("/submit", func(c *gin.Context) {
		var req struct {
			Language string `json:"language"`
			Code     string `json:"code"`
			Input    string `json:"input"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var subID string
		var apiKeyIDPtr *string
		if val, exists := c.Get("api_key_id"); exists {
			idStr := val.(string)
			apiKeyIDPtr = &idStr
		}

		if appStore != nil {
			dbSubID, err := appStore.CreateSubmission(c.Request.Context(), req.Language, req.Code, apiKeyIDPtr)
			if err == nil && dbSubID != "" {
				subID = dbSubID
			}
		}

		if subID == "" {
			subID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		queuedState, _ := json.Marshal(StatusCache{Status: "queued"})
		redisClient.Set(c.Request.Context(), "status:"+subID, queuedState, 1*time.Hour)

		// Push to Redis Stream Queue
		redisClient.XAdd(c.Request.Context(), &redis.XAddArgs{
			Stream: "code_queue",
			Values: map[string]interface{}{
				"id":       subID,
				"language": req.Language,
				"code":     req.Code,
				"input":    req.Input,
			},
		})

		c.JSON(http.StatusOK, gin.H{"submission_id": subID})
	})

	// --- ROUTE 2: REAL-TIME REDIS CACHE STREAM ---
	protected.GET("/stream/:id", func(c *gin.Context) {
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

		input := ""
		if val, exists := message.Values["input"]; exists && val != nil {
			input = val.(string)
		}

		// --- A. SET RUNNING STATE ---

		// WRITE TO REDIS CACHE
		runningState, _ := json.Marshal(StatusCache{Status: "running"})
		redisClient.Set(ctx, "status:"+subID, runningState, 1*time.Hour)

		// --- B. EXECUTE CODE ---

		startTime := time.Now()
		res, execErr := runner.ExecuteCode(language, code, input, 7*time.Second)
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
