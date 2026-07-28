package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashRunnerPropagatesMigrationLockTimeout(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	content, err := os.ReadFile(
		filepath.Join(workingDirectory, "test-db.sh"),
	)
	if err != nil {
		t.Fatalf("read Bash runner: %v", err)
	}
	script := string(content)

	for _, expected := range []string{
		`export DB_MIGRATION_LOCK_TIMEOUT="2m"`,
		"DB_MIGRATION_LOCK_TIMEOUT=2m",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("Bash runner is missing %q", expected)
		}
	}
}

func TestBashRunnerStopsDatabaseAfterFailedStart(t *testing.T) {
	bash := findBash(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")

	fakeDocker := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$TEST_DB_DOCKER_LOG"
case " $* " in
  *" ps --status running --services "*) exit 0 ;;
  *" up -d --wait mariadb "*) exit 17 ;;
  *" stop mariadb "*) exit 0 ;;
  *) exit 99 ;;
esac
`
	writeExecutable(t, filepath.Join(fakeBin, "docker"), fakeDocker)

	command := bashRunner(bash, repositoryRoot, "mariadb")
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner unexpectedly succeeded:\n%s", output)
	}
	assertStopFollowsFailedStart(t, logPath, "mariadb", string(output))
}

func TestBashRunnerAwaitsPreExistingDatabase(t *testing.T) {
	bash := findBash(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")

	fakeDocker := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"$TEST_DB_DOCKER_LOG"
case " $* " in
  *" ps --status running --services "*) printf 'mariadb\n'; exit 0 ;;
  *" up -d --wait mariadb "*) exit 17 ;;
  *" stop mariadb "*) exit 0 ;;
  *) exit 99 ;;
esac
`
	writeExecutable(t, filepath.Join(fakeBin, "docker"), fakeDocker)

	command := bashRunner(bash, repositoryRoot, "mariadb")
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner unexpectedly succeeded:\n%s", output)
	}
	assertAwaitedWithoutStop(t, logPath, "mariadb", string(output))
}

func TestBashRunnerRejectsWindowsPackageTraversal(t *testing.T) {
	bash := findBash(t)
	repositoryRoot := runnerSandbox(t)
	command := bashRunner(
		bash,
		repositoryRoot,
		"mariadb",
		"--package",
		`./..\...`,
		"--dry-run",
	)

	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unsafe package unexpectedly succeeded:\n%s", output)
	}
}

func TestBashRunnerRejectsConcurrentProfile(t *testing.T) {
	bash := findBash(t)
	repositoryRoot := runnerSandbox(t)
	fakeBin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "docker.log")
	lockPath := filepath.Join(
		repositoryRoot,
		".go-api-starter-test-db-mariadb.lock",
	)
	writeLockFile(t, lockPath)

	writeExecutable(
		t,
		filepath.Join(fakeBin, "docker"),
		"#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >>\"$TEST_DB_DOCKER_LOG\"\nexit 99\n",
	)
	command := bashRunner(bash, repositoryRoot, "mariadb")
	command.Env = fakeDockerEnvironment(fakeBin, logPath)

	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("concurrent runner unexpectedly succeeded:\n%s", output)
	}
	assertConcurrentProfileRejected(t, output, logPath, "mariadb")
}

func bashRunner(
	bash string,
	repositoryRoot string,
	profile string,
	extraArguments ...string,
) *exec.Cmd {
	arguments := []string{
		"./scripts/test-db.sh",
		"--profile",
		profile,
	}
	arguments = append(arguments, extraArguments...)
	command := exec.Command(bash, arguments...)
	command.Dir = repositoryRoot
	return command
}
