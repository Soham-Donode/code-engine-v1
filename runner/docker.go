package runner

import (
	"context"
	"fmt"
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

// ExecuteCode dynamically handles different language engines inside dedicated sandbox images
func ExecuteCode(language, code string, timeout time.Duration) (*ExecutionResult, error) {
	tmpDir, err := os.MkdirTemp("", "code-run-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	var filename string
	var image string
	var cmdArgs []string

	// Configuration switch layer based on user payload language selection
	switch strings.ToLower(language) {
	case "python":
		filename = "script.py"
		image = "python:3.10-alpine"
		cmdArgs = []string{"python", "/app/" + filename}

	case "cpp", "c++":
		filename = "main.cpp"
		image = "gcc:13.2.0"
		runCommand := "sleep 0.1 && g++ -O0 /app/main.cpp -o /tmp/main && /tmp/main"
		cmdArgs = []string{"sh", "-c", runCommand}

	default:
		return nil, fmt.Errorf("unsupported language engine: %s", language)
	}

	// Write the code to our isolated file path
	tmpFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()

	// Base Docker sandbox architecture command array construction
	dockerArgs := []string{
		"run", "--rm",
		"--platform", "linux/arm64", // Native Apple Silicon support
		"--network", "none",
		"-m", "256m", // Increased to 256m since g++ requires slightly more memory during compilation
		"-v", tmpDir + ":/app:ro",
		image,
	}
	dockerArgs = append(dockerArgs, cmdArgs...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	var timedOut bool
	if duration >= timeout || (ctx.Err() != nil && ctx.Err() == context.DeadlineExceeded) {
		timedOut = true
	}

	return &ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		TimedOut: timedOut,
	}, nil
}
