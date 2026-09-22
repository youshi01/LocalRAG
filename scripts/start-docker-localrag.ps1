[CmdletBinding()]
param(
    [switch]$Up,
    [string]$ComposeFile = 'docker-compose.dev.yml',
    [int]$ReadyTimeoutSeconds = 300
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$dockerDesktop = 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
$dockerRoot = Join-Path $env:LOCALAPPDATA 'Docker'
$runDirectory = Join-Path $dockerRoot 'run'
$secretsDirectory = Join-Path $env:LOCALAPPDATA 'docker-secrets-engine'
$settingsPath = Join-Path $env:APPDATA 'Docker\settings-store.json'

function Stop-DockerDesktopSafely {
    $running = Get-Process 'Docker Desktop', 'com.docker.backend' -ErrorAction SilentlyContinue
    if (-not $running) {
        return
    }

    try {
        & docker desktop stop 2>$null | Out-Null
    } catch {
        # A crashed backend may not answer the Desktop CLI. Process cleanup below
        # is limited to Docker Desktop processes and does not touch Docker data.
    }

    for ($i = 0; $i -lt 12; $i++) {
        if (-not (Get-Process 'Docker Desktop', 'com.docker.backend' -ErrorAction SilentlyContinue)) {
            return
        }
        Start-Sleep -Seconds 2
    }

    Get-Process 'Docker Desktop', 'com.docker.backend' -ErrorAction SilentlyContinue |
        Stop-Process -Force -ErrorAction SilentlyContinue
}

function Is-ExistingDirectory([string]$path) {
    return Test-Path -LiteralPath $path -PathType Container
}

function Is-ExistingPath([string]$path) {
    return Test-Path -LiteralPath $path
}

function Isolate-RuntimeDirectory([string]$path, [string]$label) {
    if (-not (Is-ExistingPath $path)) {
        New-Item -ItemType Directory -Path $path -Force | Out-Null
        return
    }

    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $stalePath = "$path.stale-$label-$stamp"
    if (Is-ExistingPath $stalePath) {
        throw "stale runtime target already exists: $stalePath"
    }

    # Rename only the runtime directory. Images, volumes, WSL disks and project
    # data are outside this path and are never removed by this script.
    Rename-Item -LiteralPath $path -NewName (Split-Path -Leaf $stalePath)
    New-Item -ItemType Directory -Path $path -Force | Out-Null
    Write-Host "Isolated stale Docker runtime: $path"
}

function Disable-AutoUpdateAndModelRunner {
    if (-not (Test-Path -LiteralPath $settingsPath)) {
        return
    }

    $settings = Get-Content -LiteralPath $settingsPath -Raw | ConvertFrom-Json
    $settings.AutoDownloadUpdates = $false
    $settings.EnableDockerAI = $false
    $settings.UseDockerAI = $false
    $json = $settings | ConvertTo-Json -Depth 10
    [IO.File]::WriteAllText($settingsPath, $json, [Text.UTF8Encoding]::new($false))
}

function Wait-DockerLinuxDaemon([int]$timeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($timeoutSeconds)
    do {
        $job = Start-Job -ScriptBlock {
            & docker info --format 'OSType={{.OSType}} Server={{.ServerVersion}}' 2>$null
        }
        try {
            if (Wait-Job $job -Timeout 5) {
                $output = Receive-Job $job -ErrorAction SilentlyContinue
                if ($output -match 'OSType=linux') {
                    return $output
                }
            }
        } finally {
            Stop-Job $job -ErrorAction SilentlyContinue
            Remove-Job $job -Force -ErrorAction SilentlyContinue
        }
        Start-Sleep -Seconds 5
    } while ((Get-Date) -lt $deadline)

    throw "Docker Linux daemon did not become ready within $timeoutSeconds seconds"
}

if (-not (Test-Path -LiteralPath $dockerDesktop)) {
    throw "Docker Desktop executable not found: $dockerDesktop"
}

Stop-DockerDesktopSafely
& wsl.exe --shutdown 2>$null | Out-Null
Disable-AutoUpdateAndModelRunner
Isolate-RuntimeDirectory $runDirectory 'run'
Isolate-RuntimeDirectory $secretsDirectory 'secrets'

Start-Process -FilePath $dockerDesktop -WindowStyle Hidden | Out-Null
$daemonInfo = Wait-DockerLinuxDaemon $ReadyTimeoutSeconds
Write-Host "Docker ready: $daemonInfo"

# Apply the official setting when the Desktop backend is available. The JSON
# update above also makes startup independent of this command's availability.
& docker desktop disable model-runner 2>$null | Out-Null

if ($Up) {
    $composePath = Join-Path $repoRoot $ComposeFile
    if (-not (Test-Path -LiteralPath $composePath)) {
        throw "Compose file not found: $composePath"
    }

    Push-Location $repoRoot
    try {
        & docker compose -f $ComposeFile up -d --no-build --pull never
        if ($LASTEXITCODE -ne 0) {
            throw "Docker Compose failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

Write-Host 'Docker startup completed without pulling or building images.'
