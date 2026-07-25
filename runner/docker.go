package runner

import (
	"context"
	"fmt"
	"os"
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
func ExecuteCode(language, code, input string, timeout time.Duration) (*ExecutionResult, error) {
	var image string
	var cmdArgs []string

	// Configuration switch layer based on user payload language selection
	switch strings.ToLower(language) {
	case "python":
		image = "python:3.10-alpine"
		// "-c" runs string as inline script
		cmdArgs = []string{"python", "-c", code}

	case "node", "javascript", "js":
		image = "node:20-alpine"
		cmdArgs = []string{"node", "-e", code}

	case "cpp", "c++":
		image = "frolvlad/alpine-gxx"
		// echo the code into main.cpp using bash, compile it, and run it
		runCommand := fmt.Sprintf(`cat << 'EOF' > /tmp/main.cpp
%s
EOF
g++ -O0 /tmp/main.cpp -o /tmp/main && /tmp/main`, code)
		cmdArgs = []string{"sh", "-c", runCommand}

	default:
		return nil, fmt.Errorf("unsupported language engine: %s", language)
	}

	// Make sandbox runtime configurable via SANDBOX_RUNTIME env var (defaults to "runsc" for gVisor)
	sandboxRuntime := os.Getenv("SANDBOX_RUNTIME")
	if sandboxRuntime == "" {
		sandboxRuntime = "runsc"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()

	// Base Docker sandbox architecture command array construction
	dockerArgs := []string{
		"run", "-i", "--rm", // -i is crucial to keep STDIN open
		fmt.Sprintf("--runtime=%s", sandboxRuntime),
		"--platform", "linux/arm64", // Native Apple Silicon support
		"--network", "none",
		"-m", "256m", // Increased to 256m since g++ requires slightly more memory during compilation
		"--pids-limit", "64",
		"--cap-drop", "ALL",
		image,
	}
	dockerArgs = append(dockerArgs, cmdArgs...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Pipe the actual user input heavily into stdin for the docker container
	cmd.Stdin = strings.NewReader(input)

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

	if err != nil {
		stderrStr := stderrBuf.String()
		combinedErrStr := stderrStr + " " + err.Error()
		if strings.Contains(combinedErrStr, "Unknown runtime") ||
			strings.Contains(combinedErrStr, "unknown runtime") ||
			strings.Contains(combinedErrStr, "daemon without runsc") ||
			strings.Contains(combinedErrStr, "runtime name") {
			err = fmt.Errorf("gVisor sandbox runtime '%s' is not registered with Docker daemon. Install gVisor (runsc) or set SANDBOX_RUNTIME=runc for local development: %w", sandboxRuntime, err)
		}
	}

	return &ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		TimedOut: timedOut,
	}, err
}
