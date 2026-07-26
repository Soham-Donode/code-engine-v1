package runner

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// setupTestRuntime configures SANDBOX_RUNTIME for tests, falling back to runc if runsc is missing
func setupTestRuntime(t *testing.T) string {
	runtime := os.Getenv("SANDBOX_RUNTIME")
	if runtime == "" {
		runtime = "runsc"
		// Check if runsc is registered with docker daemon
		cmd := exec.Command("docker", "info", "--format", "{{json .Runtimes}}")
		out, err := cmd.Output()
		if err != nil || !strings.Contains(string(out), "runsc") {
			t.Log("gVisor ('runsc') not found in docker info runtimes. Falling back to 'runc' for local dev test.")
			runtime = "runc"
			t.Setenv("SANDBOX_RUNTIME", "runc")
		}
	}
	return runtime
}

func TestExecuteCode_Python(t *testing.T) {
	runtime := setupTestRuntime(t)
	t.Logf("Running Python test under sandbox runtime: %s", runtime)

	code := `print("Hello from Python gVisor Sandbox")`
	res, err := ExecuteCode("python", code, "", 7*time.Second)
	if err != nil {
		t.Fatalf("ExecuteCode python failed: %v, stderr: %s", err, res.Stderr)
	}

	if !strings.Contains(res.Stdout, "Hello from Python gVisor Sandbox") {
		t.Errorf("Unexpected stdout: %q", res.Stdout)
	}
	if res.TimedOut {
		t.Errorf("Execution timed out unexpectedly")
	}
}

func TestExecuteCode_NodeJS(t *testing.T) {
	runtime := setupTestRuntime(t)
	t.Logf("Running Node.js test under sandbox runtime: %s", runtime)

	code := `console.log("Hello from Node.js gVisor Sandbox");`
	res, err := ExecuteCode("node", code, "", 7*time.Second)
	if err != nil {
		t.Fatalf("ExecuteCode node failed: %v, stderr: %s", err, res.Stderr)
	}

	if !strings.Contains(res.Stdout, "Hello from Node.js gVisor Sandbox") {
		t.Errorf("Unexpected stdout: %q", res.Stdout)
	}
	if res.TimedOut {
		t.Errorf("Execution timed out unexpectedly")
	}
}

func TestExecuteCode_CPP(t *testing.T) {
	runtime := setupTestRuntime(t)
	t.Logf("Running C++ compilation and execution test under sandbox runtime: %s", runtime)

	code := `#include <iostream>
int main() {
    std::cout << "Hello from C++ gVisor Sandbox" << std::endl;
    return 0;
}`
	res, err := ExecuteCode("cpp", code, "", 10*time.Second)
	if err != nil {
		t.Fatalf("ExecuteCode cpp failed: %v, stderr: %s", err, res.Stderr)
	}

	if !strings.Contains(res.Stdout, "Hello from C++ gVisor Sandbox") {
		t.Errorf("Unexpected stdout: %q", res.Stdout)
	}
	if res.TimedOut {
		t.Errorf("Execution timed out unexpectedly")
	}
}
