package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func assertStopFollowsFailedStart(
	t *testing.T,
	logPath string,
	service string,
	runnerOutput string,
) {
	t.Helper()

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf(
			"read fake Docker log: %v\nrunner output:\n%s",
			err,
			runnerOutput,
		)
	}
	logOutput := string(logBytes)
	upIndex := strings.Index(logOutput, " up -d --wait "+service)
	if upIndex == -1 {
		t.Fatalf("start command was not observed:\n%s", logOutput)
	}
	stopIndex := strings.Index(logOutput, " stop "+service)
	if stopIndex == -1 {
		t.Fatalf("stop command was not invoked after failed start:\n%s", logOutput)
	}
	if stopIndex < upIndex {
		t.Fatalf("stop command preceded failed start:\n%s", logOutput)
	}
}

func assertAwaitedWithoutStop(
	t *testing.T,
	logPath string,
	service string,
	runnerOutput string,
) {
	t.Helper()

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf(
			"read fake Docker log: %v\nrunner output:\n%s",
			err,
			runnerOutput,
		)
	}
	logOutput := string(logBytes)
	if !strings.Contains(logOutput, " up -d --wait "+service) {
		t.Fatalf("pre-existing database was not health-awaited:\n%s", logOutput)
	}
	if strings.Contains(logOutput, " stop "+service) {
		t.Fatalf("pre-existing database was stopped:\n%s", logOutput)
	}
}

func assertConcurrentProfileRejected(
	t *testing.T,
	output []byte,
	logPath string,
	profile string,
) {
	t.Helper()

	expected := "Another database test runner already owns profile " + profile + "."
	if !strings.Contains(string(output), expected) {
		t.Fatalf("missing concurrent-run error %q:\n%s", expected, output)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("Docker was called before concurrent run rejection")
	}
}

func fakeDockerEnvironment(fakeBin string, logPath string) []string {
	pathValue := fakeBin + string(os.PathListSeparator) + os.Getenv("PATH")
	environment := append([]string(nil), os.Environ()...)
	environment = append(environment, "PATH="+pathValue)
	environment = append(environment, "TEST_DB_DOCKER_LOG="+logPath)
	return environment
}

func writeLockFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("existing runner\n"), 0o600); err != nil {
		t.Fatalf("write profile lock: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove profile lock: %v", err)
		}
	})
}

func findPowerShell(t *testing.T) string {
	t.Helper()

	for _, candidate := range []string{"pwsh", "powershell.exe", "powershell"} {
		if executable, err := exec.LookPath(candidate); err == nil {
			return executable
		}
	}
	t.Skip("PowerShell is not available")
	return ""
}

func findBash(t *testing.T) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		candidate := filepath.Join(
			os.Getenv("ProgramFiles"),
			"Git",
			"bin",
			"bash.exe",
		)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if executable, err := exec.LookPath("bash"); err == nil {
		return executable
	}
	t.Skip("Bash is not available")
	return ""
}

func runnerSandbox(t *testing.T) string {
	t.Helper()

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	sourceRoot := filepath.Dir(workingDirectory)
	sandboxRoot := t.TempDir()
	sandboxScripts := filepath.Join(sandboxRoot, "scripts")
	if err := os.Mkdir(sandboxScripts, 0o700); err != nil {
		t.Fatalf("create runner sandbox: %v", err)
	}
	for _, name := range []string{"test-db.ps1", "test-db.sh"} {
		sourcePath := filepath.Join(sourceRoot, "scripts", name)
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		targetPath := filepath.Join(sandboxScripts, name)
		if err := os.WriteFile(targetPath, content, 0o700); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return sandboxRoot
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake executable: %v", err)
	}
}
