#!/usr/bin/env bash

set -uo pipefail

usage() {
  cat <<'EOF'
Usage:
  test-db.sh --profile <postgres|mariadb|mariadb-xampp> [options]

Options:
  --package <go-package>       Relative package (default: ./...)
  --run <test-name>            Exact Go test identifier
  --dry-run                    Validate and print redacted actions only
  --xampp-host <host>          Must be 127.0.0.1
  --xampp-port <port>          Default: 3306
  --xampp-database <name>      Dedicated go_lang_starter_test_ database
  --xampp-user <user>          Dedicated XAMPP test user
  --xampp-password-env <name>  Environment variable containing the password
  --allow-test-database-reset  Required for mariadb-xampp
EOF
}

profile=""
package="./..."
run_name=""
dry_run=false
xampp_host="127.0.0.1"
xampp_port="3306"
xampp_database=""
xampp_user=""
xampp_password_env=""
allow_test_database_reset=false

while (($# > 0)); do
  case "$1" in
    --profile|--package|--run|--xampp-host|--xampp-port|--xampp-database|--xampp-user|--xampp-password-env)
      if (($# < 2)); then
        echo "Missing value for $1" >&2
        exit 2
      fi
      option="$1"
      value="$2"
      shift 2
      case "$option" in
        --profile) profile="$value" ;;
        --package) package="$value" ;;
        --run) run_name="$value" ;;
        --xampp-host) xampp_host="$value" ;;
        --xampp-port) xampp_port="$value" ;;
        --xampp-database) xampp_database="$value" ;;
        --xampp-user) xampp_user="$value" ;;
        --xampp-password-env) xampp_password_env="$value" ;;
      esac
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    --allow-test-database-reset)
      allow_test_database_reset=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$profile" in
  postgres|mariadb|mariadb-xampp) ;;
  *)
    echo "Unknown profile: $profile" >&2
    exit 2
    ;;
esac

if [[ "$package" != ./* ]] ||
  [[ "$package" =~ [[:space:]] ]] ||
  [[ "$package" =~ (^|[\\/])\.\.($|[\\/]) ]]; then
  echo "Package must be a relative Go package below the repository root." >&2
  exit 2
fi
if [[ -n "$run_name" ]] &&
  [[ ! "$run_name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo "Run must be one exact Go test identifier." >&2
  exit 2
fi

script_directory="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repository_root="$(cd -- "$script_directory/.." && pwd -P)"
compose_file="$repository_root/compose.dev.yml"
is_xampp=false

if [[ "$profile" == "mariadb-xampp" ]]; then
  is_xampp=true
  if [[ "$xampp_host" != "127.0.0.1" ]]; then
    echo "XAMPP test host must be 127.0.0.1." >&2
    exit 2
  fi
  if [[ ! "$xampp_port" =~ ^[0-9]+$ ]] ||
    ((xampp_port < 1 || xampp_port > 65535)); then
    echo "XAMPP port must be between 1 and 65535." >&2
    exit 2
  fi
  if [[ "$xampp_database" != go_lang_starter_test_* ]]; then
    echo "XAMPP database must use the go_lang_starter_test_ prefix." >&2
    exit 2
  fi
  if [[ -z "$xampp_user" ]]; then
    echo "XAMPP test user is required." >&2
    exit 2
  fi
  if [[ ! "$xampp_password_env" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
    echo "XAMPP password environment variable name is invalid." >&2
    exit 2
  fi
  if [[ -z "${!xampp_password_env-}" ]]; then
    echo "XAMPP password environment variable is missing or empty." >&2
    exit 2
  fi
  if [[ "$allow_test_database_reset" != true ]]; then
    echo "XAMPP tests require --allow-test-database-reset." >&2
    exit 2
  fi
fi

case "$profile" in
  postgres) database_name="go_lang_starter_test_postgres" ;;
  mariadb) database_name="go_lang_starter_test_mariadb" ;;
  mariadb-xampp) database_name="$xampp_database" ;;
esac

if [[ "$dry_run" == true ]]; then
  printf 'profile=%s\n' "$profile"
  printf 'package=%s\n' "$package"
  printf 'run=%s\n' "$run_name"
  printf 'database=%s\n' "$database_name"
  printf 'password=[REDACTED]\n'
  if [[ "$is_xampp" == true ]]; then
    printf 'actions=guard,native-go-test\n'
  else
    printf 'actions=compose-up,guard,container-go-test,compose-stop\n'
  fi
  exit 0
fi

test_arguments=(test -p 1 "$package" -count=1)
if [[ -n "$run_name" ]]; then
  test_arguments+=(-run "^${run_name}$")
fi

lock_file="$repository_root/.go-api-starter-test-db-${profile}.lock"
lock_owned=false

acquire_profile_lock() {
  if (set -o noclobber; : >"$lock_file") 2>/dev/null; then
    lock_owned=true
    return 0
  fi
  echo "Another database test runner already owns profile $profile. If no runner is active, remove $lock_file." >&2
  return 1
}

release_profile_lock() {
  if [[ "$lock_owned" == true ]]; then
    rm -f -- "$lock_file"
    lock_owned=false
  fi
}

if [[ "$is_xampp" == true ]]; then
  if command -v go >/dev/null 2>&1; then
    go_executable="$(command -v go)"
  elif [[ -x "/c/Program Files/Go/bin/go.exe" ]]; then
    go_executable="/c/Program Files/Go/bin/go.exe"
  else
    echo "Go executable was not found." >&2
    exit 1
  fi

  xampp_password="${!xampp_password_env}"
  acquire_profile_lock || exit 1
  trap 'release_profile_lock' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  (
    export APP_ENV="test"
    export ALLOW_TEST_DATABASE_RESET="1"
    export DB_DRIVER="mysql"
    export DB_FLAVOR="mariadb"
    export DB_HOST="$xampp_host"
    export DB_PORT="$xampp_port"
    export DB_NAME="$xampp_database"
    export DB_USER="$xampp_user"
    export DB_PASSWORD="$xampp_password"
    export DB_TLS_MODE="disable"
    export DB_TIME_ZONE="UTC"
    export DB_CONNECT_TIMEOUT="5s"
    export DB_MIGRATION_LOCK_TIMEOUT="2m"
    export DB_MAX_OPEN_CONNECTIONS="10"
    export DB_MAX_IDLE_CONNECTIONS="5"
    export DB_CONNECTION_MAX_LIFETIME="5m"
    export RUN_DATABASE_INTEGRATION_TESTS="1"

    cd -- "$repository_root" || exit 1
    "$go_executable" run ./tests/testsupport/cmd/database-guard || exit $?
    "$go_executable" "${test_arguments[@]}"
  )
  result_code=$?
  release_profile_lock
  trap - EXIT INT TERM
  exit "$result_code"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker executable was not found." >&2
  exit 1
fi
docker_executable="$(command -v docker)"
service="$profile"
if [[ "$profile" == "postgres" ]]; then
  driver="postgres"
  flavor=""
  host_name="postgres"
  port="5432"
else
  driver="mysql"
  flavor="mariadb"
  host_name="mariadb"
  port="3306"
fi

compose=(
  "$docker_executable"
  compose
  -f "$compose_file"
  --profile go-test
  --profile "$profile"
)
runner_environment=(
  -e APP_ENV=test
  -e ALLOW_TEST_DATABASE_RESET=1
  -e "DB_DRIVER=$driver"
  -e "DB_FLAVOR=$flavor"
  -e "DB_HOST=$host_name"
  -e "DB_PORT=$port"
  -e "DB_NAME=$database_name"
  -e DB_USER=go_lang_starter_test
  -e DB_PASSWORD=development-only-test-database-password
  -e DB_TLS_MODE=disable
  -e DB_TIME_ZONE=UTC
  -e DB_CONNECT_TIMEOUT=5s
  -e DB_MIGRATION_LOCK_TIMEOUT=2m
  -e DB_MAX_OPEN_CONNECTIONS=10
  -e DB_MAX_IDLE_CONNECTIONS=5
  -e DB_CONNECTION_MAX_LIFETIME=5m
  -e RUN_DATABASE_INTEGRATION_TESTS=1
)

started_by_script=false
acquire_profile_lock || exit 1
trap 'if [[ "$started_by_script" == true ]]; then "${compose[@]}" stop "$service" >/dev/null; fi; release_profile_lock' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if ! running_services="$("${compose[@]}" ps --status running --services)"; then
  echo "Unable to inspect disposable database service state." >&2
  exit 1
fi
was_running=false
while IFS= read -r running_service; do
  if [[ "$running_service" == "$service" ]]; then
    was_running=true
  fi
done <<<"$running_services"

if [[ "$was_running" != true ]]; then
  started_by_script=true
fi
if ! "${compose[@]}" up -d --wait "$service"; then
  echo "Unable to start or await disposable $profile database." >&2
  exit 1
fi

"${compose[@]}" run --rm -T \
  "${runner_environment[@]}" \
  go-test run ./tests/testsupport/cmd/database-guard
result_code=$?

if ((result_code == 0)); then
  "${compose[@]}" run --rm -T \
    "${runner_environment[@]}" \
    go-test "${test_arguments[@]}"
  result_code=$?
fi

if [[ "$started_by_script" == true ]]; then
  "${compose[@]}" stop "$service"
  cleanup_code=$?
  started_by_script=false
  if ((result_code == 0 && cleanup_code != 0)); then
    result_code=$cleanup_code
  fi
fi

release_profile_lock
trap - EXIT INT TERM
exit "$result_code"
