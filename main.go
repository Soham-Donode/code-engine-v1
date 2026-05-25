package main

import (
	"context"
	"fmt"
	"time"

	"code-engine/runner"
	"code-engine/store"
)

func main() {
	ctx := context.Background()

	// 1. Initialize Storage Connections
	fmt.Println("[System] Connecting to Infrastructure Services...")
	dataStore, err := store.NewStore()
	if err != nil {
		panic(err)
	}
	defer dataStore.DB.Close()
	defer dataStore.Redis.Close()

	// 2. Start the Background Worker Thread (Asynchronous Consumer)
	go startBackgroundWorker(dataStore)

	// 3. Simulate Incoming Client Traffic (The Producer)
	// We will simulate 3 fast sequential submissions to prove the queue holds and handles them sequentially
	mockSubmissions := []string{
		`print("Execution Task #1 Dynamic Safe Run!")`,
		`print("Execution Task #2 Processing...")`,
		`import time; time.sleep(3); print("Execution Task #3 Finished heavy calculation!")`,
	}

	fmt.Println("\n[API Gateway] Simulating continuous inbound client requests...")
	for i, codePayload := range mockSubmissions {
		// Store the submission initial tracking record in PostgreSQL
		subID, err := dataStore.CreateSubmission(ctx, "python", codePayload)
		if err != nil {
			fmt.Printf("[API Error] Failed creating entry: %v\n", err)
			continue
		}
		fmt.Printf("[API Server] Received request #%d. Saved to DB. UUID: %s\n", i+1, subID)

		// Drop the job into the Redis stream queue and immediately free up the thread
		err = dataStore.EnqueueSubmission(ctx, subID)
		if err != nil {
			fmt.Printf("[API Error] Queue buffering failed: %v\n", err)
			continue
		}
		fmt.Printf("[API Server] Job %s successfully buffered into Redis Stream. Connection closed.\n", subID)
	}

	// Keep the main system engine awake for a few seconds to let the concurrent background worker finish running the tasks
	time.Sleep(12 * time.Second)
	fmt.Println("\n[System] Orchestration pipeline simulation complete.")
}

// Background worker loops indefinitely, waiting to execute items from the Redis Queue
func startBackgroundWorker(s *store.Store) {
	ctx := context.Background()
	fmt.Println("[Worker Thread] Background consumer loop initialized. Awaiting tasks...")

	for {
		// This blocks entirely until an entry is pushed into the stream queue
		subID, err := s.DequeueSubmission(ctx)
		if err != nil {
			// Catch connection drop sleep timeouts
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Printf("\n[Worker Process] Found new task! Claiming Submission UUID: %s\n", subID)

		// 1. Mark status as 'running' in Postgres
		if err := s.UpdateSubmissionToRunning(ctx, subID); err != nil {
			fmt.Printf("[Worker Error] Failed updating state to running: %v\n", err)
			continue
		}

		// 2. Fetch the raw payload code out of Postgres using the ID
		code, _, err := s.GetSubmissionDetails(ctx, subID)
		if err != nil {
			fmt.Printf("[Worker Error] Failed retrieving metadata code: %v\n", err)
			continue
		}

		// 3. Spin up the isolated sandbox container and track parameters
		fmt.Printf("[Worker Process] Mounting sandbox image and executing payload %s...\n", subID)
		res, err := runner.ExecutePython(code, 5*time.Second)
		if err != nil {
			fmt.Printf("[Worker Error] Container sandbox runtime error: %v\n", err)
			continue
		}

		// 4. Calculate final execution statuses
		finalStatus := "completed"
		if res.TimedOut {
			finalStatus = "timeout"
		} else if res.Stderr != "" {
			finalStatus = "error"
		}

		// 5. Write the final outputs back to PostgreSQL permanently
		err = s.UpdateSubmissionStatus(
			ctx,
			subID,
			finalStatus,
			res.Stdout,
			res.Stderr,
			int(res.Duration.Milliseconds()),
		)
		if err != nil {
			fmt.Printf("[Worker Error] Failed writing final results to Postgres: %v\n", err)
			continue
		}

		fmt.Printf("[Worker Process] Successfully closed sandbox container. Job %s saved with status: '%s' (Took %dms)\n", 
			subID, finalStatus, res.Duration.Milliseconds())
	}
}