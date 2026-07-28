[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('postgres', 'mariadb', 'mariadb-xampp')]
    [string]$Profile,

    [string]$Package = './...',
    [string]$Run = '',
    [switch]$DryRun,

    [string]$XamppHost = '127.0.0.1',
    [ValidateRange(1, 65535)]
    [int]$XamppPort = 3306,
    [string]$XamppDatabase = '',
    [string]$XamppUser = '',
    [string]$XamppPasswordEnv = '',
    [switch]$AllowTestDatabaseReset
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-TestSelector {
    param(
        [string]$PackageValue,
        [string]$RunValue
    )

    if (
        -not $PackageValue.StartsWith('./') -or
        $PackageValue -match '\s' -or
        $PackageValue -match '(^|[\\/])\.\.($|[\\/])'
    ) {
        throw 'Package must be a relative Go package below the repository root.'
    }
    if ($RunValue -ne '' -and $RunValue -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        throw 'Run must be one exact Go test identifier.'
    }
}

function Resolve-GoExecutable {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if ($null -ne $goCommand) {
        return $goCommand.Source
    }

    $fallback = Join-Path $env:ProgramFiles 'Go\bin\go.exe'
    if (Test-Path -LiteralPath $fallback -PathType Leaf) {
        return $fallback
    }
    throw 'Go executable was not found.'
}

function Set-ProcessEnvironment {
    param([System.Collections.IDictionary]$Values)

    $previous = @{}
    foreach ($entry in $Values.GetEnumerator()) {
        $previous[$entry.Key] = [Environment]::GetEnvironmentVariable(
            $entry.Key,
            'Process'
        )
        [Environment]::SetEnvironmentVariable(
            $entry.Key,
            [string]$entry.Value,
            'Process'
        )
    }
    return $previous
}

function Restore-ProcessEnvironment {
    param([System.Collections.IDictionary]$Previous)

    foreach ($entry in $Previous.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable(
            $entry.Key,
            $entry.Value,
            'Process'
        )
    }
}

function New-TestRunLock {
    param(
        [string]$LockPath,
        [string]$ProfileName
    )

    try {
        return [System.IO.File]::Open(
            $LockPath,
            [System.IO.FileMode]::CreateNew,
            [System.IO.FileAccess]::Write,
            [System.IO.FileShare]::None
        )
    } catch [System.IO.IOException] {
        if ([System.IO.File]::Exists($LockPath)) {
            throw "Another database test runner already owns profile $ProfileName. " +
                "If no runner is active, remove $LockPath."
        }
        throw 'Unable to acquire the database test runner lock.'
    }
}

function Remove-TestRunLock {
    param(
        [System.IO.FileStream]$LockHandle,
        [string]$LockPath
    )

    $LockHandle.Dispose()
    [System.IO.File]::Delete($LockPath)
}

Assert-TestSelector -PackageValue $Package -RunValue $Run

$repositoryRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$composeFile = Join-Path $repositoryRoot 'compose.dev.yml'
$isXampp = $Profile -eq 'mariadb-xampp'

if ($isXampp) {
    if ($XamppHost -ne '127.0.0.1') {
        throw 'XAMPP test host must be 127.0.0.1.'
    }
    if (
        $XamppDatabase -eq '' -or
        -not $XamppDatabase.StartsWith('go_lang_starter_test_')
    ) {
        throw 'XAMPP database must use the go_lang_starter_test_ prefix.'
    }
    if ($XamppUser -eq '') {
        throw 'XAMPP test user is required.'
    }
    if ($XamppPasswordEnv -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        throw 'XAMPP password environment variable name is invalid.'
    }
    $xamppPassword = [Environment]::GetEnvironmentVariable(
        $XamppPasswordEnv,
        'Process'
    )
    if ([string]::IsNullOrEmpty($xamppPassword)) {
        throw 'XAMPP password environment variable is missing or empty.'
    }
    if (-not $AllowTestDatabaseReset) {
        throw 'XAMPP tests require -AllowTestDatabaseReset.'
    }
}

$databaseName = switch ($Profile) {
    'postgres' { 'go_lang_starter_test_postgres' }
    'mariadb' { 'go_lang_starter_test_mariadb' }
    'mariadb-xampp' { $XamppDatabase }
}

if ($DryRun) {
    Write-Output "profile=$Profile"
    Write-Output "package=$Package"
    Write-Output "run=$Run"
    Write-Output "database=$databaseName"
    Write-Output 'password=[REDACTED]'
    if ($isXampp) {
        Write-Output 'actions=guard,native-go-test'
    } else {
        Write-Output 'actions=compose-up,guard,container-go-test,compose-stop'
    }
    exit 0
}

$testArguments = @('test', $Package, '-count=1')
if ($Run -ne '') {
    $testArguments += @('-run', "^$Run`$")
}

if ($isXampp) {
    $goExecutable = Resolve-GoExecutable
    $environment = [ordered]@{
        APP_ENV = 'test'
        ALLOW_TEST_DATABASE_RESET = '1'
        DB_DRIVER = 'mysql'
        DB_FLAVOR = 'mariadb'
        DB_HOST = $XamppHost
        DB_PORT = [string]$XamppPort
        DB_NAME = $XamppDatabase
        DB_USER = $XamppUser
        DB_PASSWORD = $xamppPassword
        DB_TLS_MODE = 'disable'
        DB_TIME_ZONE = 'UTC'
        DB_MAX_OPEN_CONNECTIONS = '10'
        DB_MAX_IDLE_CONNECTIONS = '5'
        DB_CONNECTION_MAX_LIFETIME = '5m'
    }
    $lockPath = Join-Path $repositoryRoot ".go-api-starter-test-db-$Profile.lock"
    $lockHandle = New-TestRunLock -LockPath $lockPath -ProfileName $Profile
    $previousEnvironment = $null
    $locationPushed = $false
    $resultCode = 1
    try {
        $previousEnvironment = Set-ProcessEnvironment -Values $environment
        Push-Location $repositoryRoot
        $locationPushed = $true

        & $goExecutable run ./tests/testsupport/cmd/database-guard
        $resultCode = $LASTEXITCODE
        if ($resultCode -eq 0) {
            & $goExecutable @testArguments
            $resultCode = $LASTEXITCODE
        }
    } finally {
        try {
            if ($locationPushed) {
                Pop-Location
            }
        } finally {
            try {
                if ($null -ne $previousEnvironment) {
                    Restore-ProcessEnvironment -Previous $previousEnvironment
                }
            } finally {
                Remove-TestRunLock -LockHandle $lockHandle -LockPath $lockPath
            }
        }
    }
    exit $resultCode
}

$dockerCommand = Get-Command docker -ErrorAction Stop
$dockerExecutable = $dockerCommand.Source
$service = $Profile
$driver = if ($Profile -eq 'postgres') { 'postgres' } else { 'mysql' }
$flavor = if ($Profile -eq 'mariadb') { 'mariadb' } else { '' }
$hostName = $service
$port = if ($Profile -eq 'postgres') { '5432' } else { '3306' }
$user = 'go_lang_starter_test'
$password = 'development-only-test-database-password'

$composeArguments = @(
    'compose',
    '-f', $composeFile,
    '--profile', 'go-test',
    '--profile', $Profile
)
$runnerEnvironment = @(
    '-e', 'APP_ENV=test',
    '-e', 'ALLOW_TEST_DATABASE_RESET=1',
    '-e', "DB_DRIVER=$driver",
    '-e', "DB_FLAVOR=$flavor",
    '-e', "DB_HOST=$hostName",
    '-e', "DB_PORT=$port",
    '-e', "DB_NAME=$databaseName",
    '-e', "DB_USER=$user",
    '-e', "DB_PASSWORD=$password",
    '-e', 'DB_TLS_MODE=disable',
    '-e', 'DB_TIME_ZONE=UTC',
    '-e', 'DB_MAX_OPEN_CONNECTIONS=10',
    '-e', 'DB_MAX_IDLE_CONNECTIONS=5',
    '-e', 'DB_CONNECTION_MAX_LIFETIME=5m'
)

$lockPath = Join-Path $repositoryRoot ".go-api-starter-test-db-$Profile.lock"
$lockHandle = New-TestRunLock -LockPath $lockPath -ProfileName $Profile
$startedByScript = $false
$resultCode = 0

try {
    $runningServices = & $dockerExecutable @($composeArguments + @(
        'ps', '--status', 'running', '--services'
    ))
    if ($LASTEXITCODE -ne 0) {
        throw 'Unable to inspect disposable database service state.'
    }
    $wasRunning = $runningServices -contains $service
    if (-not $wasRunning) {
        $startedByScript = $true
    }
    & $dockerExecutable @($composeArguments + @(
        'up', '-d', '--wait', $service
    ))
    if ($LASTEXITCODE -ne 0) {
        throw "Unable to start or await disposable $Profile database."
    }

    & $dockerExecutable @(
        $composeArguments +
        @('run', '--rm', '-T') +
        $runnerEnvironment +
        @(
            'go-test',
            'run',
            './tests/testsupport/cmd/database-guard'
        )
    )
    $resultCode = $LASTEXITCODE
    if ($resultCode -eq 0) {
        & $dockerExecutable @(
            $composeArguments +
            @('run', '--rm', '-T') +
            $runnerEnvironment +
            @('go-test') +
            $testArguments
        )
        $resultCode = $LASTEXITCODE
    }
} finally {
    try {
        if ($startedByScript) {
            & $dockerExecutable @($composeArguments + @('stop', $service))
            $cleanupCode = $LASTEXITCODE
            if ($resultCode -eq 0 -and $cleanupCode -ne 0) {
                $resultCode = $cleanupCode
            }
        }
    } finally {
        Remove-TestRunLock -LockHandle $lockHandle -LockPath $lockPath
    }
}

exit $resultCode
