package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ExecutionResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	TimedOut bool
}

// ExecutePython runs a string of Python code inside a temporary Docker container using native CLI
func ExecutePython(code string, timeout time.Duration) (*ExecutionResult, error) {
	// 1. Create a isolated temporary directory on your Mac to hold the code
	tmpDir, err := os.MkdirTemp("", "code-run-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		return nil, err
	}

	// 2. Set up OS context execution limit to prevent infinite runaways
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 3. Construct the clean docker command line string
	// --rm: auto deletes container when done
	// --network none: fully isolates the container from the internet
	// -m 128m: caps memory usage tightly
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--network", "none",
		"-m", "128m",
		"-v", tmpDir+":/app:ro",
		"python:3.10-alpine",
		"python", "/app/script.py",
	)

	// Create distinct memory buffers to cleanly demux output channels
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	// 4. Evaluate execution states precisely
	var timedOut bool
	if ctx.Err() == context.DeadlineExceeded {
		timedOut = true
	}

	return &ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		TimedOut: timedOut,
	}, nil
}