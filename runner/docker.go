package runner

import (
	"context"
	"fmt"
	"os/exec"
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
	var image string
	var cmdArgs []string

	// Configuration switch layer based on user payload language selection
	switch strings.ToLower(language) {
	case "python":
		image = "python:3.10-alpine"
		// "-" tells python to read script from stdin
		cmdArgs = []string{"python", "-"}

	case "cpp", "c++":
		image = "gcc:13.2.0"
		// read stdin to a file, compile, and run it
		runCommand := "cat > /tmp/main.cpp && g++ -O0 /tmp/main.cpp -o /tmp/main && /tmp/main"
		cmdArgs = []string{"sh", "-c", runCommand}

	default:
		return nil, fmt.Errorf("unsupported language engine: %s", language)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()

	// Base Docker sandbox architecture command array construction
	dockerArgs := []string{
		"run", "-i", "--rm", // -i is crucial to keep STDIN open
		"--platform", "linux/arm64", // Native Apple Silicon support
		"--network", "none",
		"-m", "256m", // Increased to 256m since g++ requires slightly more memory during compilation
		image,
	}
	dockerArgs = append(dockerArgs, cmdArgs...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Pipe the actual code payload heavily into stdin for the docker container
	cmd.Stdin = strings.NewReader(code)

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	err := cmd.Run()
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
	}, err
}
