package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPowerShellRunnerPropagatesMigrationLockTimeout(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	content, err := os.ReadFile(
		filepath.Join(workingDirectory, "test-db.ps1"),
	)
	if err != nil {
		t.Fatalf("read PowerShell runner: %v", err)
	}
	script := string(content)

	for _, expected := range []string{
		"DB_MIGRATION_LOCK_TIMEOUT = '2m'",
		"'DB_MIGRATION_LOCK_TIMEOUT=2m'",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("PowerShell runner is missing %q", expected)
		}
	}
}

func TestPowerShellRunnerSerializesDatabasePackages(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	content, err := os.ReadFile(
		filepath.Join(workingDirectory, "test-db.ps1"),
	)
	if err != nil {
		t.Fatalf("read PowerShell runner: %v", err)
	}
	if !strings.Contains(string(content), "@('test', '-p', '1', $Package, '-count=1')") {
		t.Fatal("PowerShell runner must serialize database test packages")
	}
}

func TestPowerShellRunnerStopsDatabaseAfterFailedStart(t *testing.T) {
	powerShell := findPowerShell(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")

	fakeDocker := `@echo off
echo %*>>"%TEST_DB_DOCKER_LOG%"
echo %*|%SystemRoot%\System32\findstr.exe /C:" ps --status running --services" >nul && exit /b 0
echo %*|%SystemRoot%\System32\findstr.exe /C:" up -d --wait postgres" >nul && exit /b 17
echo %*|%SystemRoot%\System32\findstr.exe /C:" stop postgres" >nul && exit /b 0
exit /b 99
`
	writeExecutable(t, filepath.Join(fakeBin, "docker.cmd"), fakeDocker)

	command := powerShellRunner(
		powerShell,
		repositoryRoot,
		"postgres",
	)
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner unexpectedly succeeded:\n%s", output)
	}
	assertStopFollowsFailedStart(t, logPath, "postgres", string(output))
}

func TestPowerShellRunnerAlreadyStopsDatabaseAfterLaterFailure(t *testing.T) {
	powerShell := findPowerShell(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")

	fakeDocker := `@echo off
echo %*>>"%TEST_DB_DOCKER_LOG%"
echo %*|%SystemRoot%\System32\findstr.exe /C:" ps --status running --services" >nul && exit /b 0
echo %*|%SystemRoot%\System32\findstr.exe /C:" up -d --wait postgres" >nul && exit /b 0
echo %*|%SystemRoot%\System32\findstr.exe /C:" run --rm -T " >nul && exit /b 17
echo %*|%SystemRoot%\System32\findstr.exe /C:" stop postgres" >nul && exit /b 0
exit /b 99
`
	writeExecutable(t, filepath.Join(fakeBin, "docker.cmd"), fakeDocker)

	command := powerShellRunner(
		powerShell,
		repositoryRoot,
		"postgres",
	)
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner unexpectedly succeeded:\n%s", output)
	}
	assertStopFollowsFailedStart(t, logPath, "postgres", string(output))
}

func TestPowerShellRunnerAwaitsPreExistingDatabase(t *testing.T) {
	powerShell := findPowerShell(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")

	fakeDocker := `@echo off
echo %*>>"%TEST_DB_DOCKER_LOG%"
echo %*|%SystemRoot%\System32\findstr.exe /C:" ps --status running --services" >nul
if not errorlevel 1 (
  echo postgres
  exit /b 0
)
echo %*|%SystemRoot%\System32\findstr.exe /C:" up -d --wait postgres" >nul && exit /b 17
echo %*|%SystemRoot%\System32\findstr.exe /C:" stop postgres" >nul && exit /b 0
exit /b 99
`
	writeExecutable(t, filepath.Join(fakeBin, "docker.cmd"), fakeDocker)

	command := powerShellRunner(
		powerShell,
		repositoryRoot,
		"postgres",
	)
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner unexpectedly succeeded:\n%s", output)
	}
	assertAwaitedWithoutStop(t, logPath, "postgres", string(output))
}

func TestPowerShellRunnerRejectsWindowsPackageTraversal(t *testing.T) {
	powerShell := findPowerShell(t)
	repositoryRoot := runnerSandbox(t)
	command := powerShellRunner(
		powerShell,
		repositoryRoot,
		"postgres",
		"-Package",
		`./..\...`,
		"-DryRun",
	)

	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unsafe package unexpectedly succeeded:\n%s", output)
	}
}

func TestPowerShellRunnerRejectsConcurrentProfile(t *testing.T) {
	powerShell := findPowerShell(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")
	lockPath := filepath.Join(
		repositoryRoot,
		".go-api-starter-test-db-postgres.lock",
	)
	writeLockFile(t, lockPath)

	writeExecutable(
		t,
		filepath.Join(fakeBin, "docker.cmd"),
		"@echo off\r\necho %*>>\"%TEST_DB_DOCKER_LOG%\"\r\nexit /b 99\r\n",
	)
	command := powerShellRunner(
		powerShell,
		repositoryRoot,
		"postgres",
	)
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("concurrent runner unexpectedly succeeded:\n%s", output)
	}
	assertConcurrentProfileRejected(t, output, logPath, "postgres")
}

func powerShellRunner(
	powerShell string,
	repositoryRoot string,
	profile string,
	extraArguments ...string,
) *exec.Cmd {
	arguments := []string{
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-File",
		filepath.Join(repositoryRoot, "scripts", "test-db.ps1"),
		"-Profile",
		profile,
	}
	arguments = append(arguments, extraArguments...)
	command := exec.Command(powerShell, arguments...)
	command.Dir = repositoryRoot
	return command
}
