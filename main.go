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

	// 1. Initialize Storage Infrastructure Connections
	fmt.Println("Connecting to Databases...")
	dataStore, err := store.NewStore()
	if err != nil {
		panic(err)
	}
	defer dataStore.DB.Close()
	defer dataStore.Redis.Close()
	fmt.Println("Successfully connected to Postgres and Redis!")

	// 2. Mock a payload coming from a user submission
	userCode := `print("Hello from database-backed worker system!")`
	
	// 3. Persist the state into Postgres as "queued"
	submissionID, err := dataStore.CreateSubmission(ctx, "python", userCode)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Saved submission to DB. Assigned UUID: %s\n", submissionID)

	// 4. Run the engine container execution phase
	fmt.Println("Executing code payload in Sandbox container...")
	res, err := runner.ExecutePython(userCode, 5*time.Second)
	if err != nil {
		panic(err)
	}

	// 5. Compute statuses and write execution data back to Postgres
	finalStatus := "completed"
	if res.TimedOut {
		finalStatus = "timeout"
	} else if res.Stderr != "" {
		finalStatus = "error"
	}

	err = dataStore.UpdateSubmissionStatus(
		ctx, 
		submissionID, 
		finalStatus, 
		res.Stdout, 
		res.Stderr, 
		int(res.Duration.Milliseconds()),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Pipeline transaction finished cleanly. Execution states saved permanently!")
}