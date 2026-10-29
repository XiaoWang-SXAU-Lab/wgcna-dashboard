[CmdletBinding()]
param(
    [string]$WorkerSource,
    [string]$BuildRoot = (Join-Path $env:LOCALAPPDATA 'rsWGCNA-build/windows'),
    [string]$GccBin = (Join-Path $env:LOCALAPPDATA 'w64devkit/w64devkit/bin'),
    [switch]$Launch
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:OS -ne 'Windows_NT') { throw 'Use build-macos.sh on macOS.' }
$dashboard = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $WorkerSource) { $WorkerSource = Join-Path $dashboard '../rs_WGCNA' }
$worker = (Resolve-Path $WorkerSource).Path
$BuildRoot = [IO.Path]::GetFullPath($BuildRoot)
$syncRoot = Split-Path $dashboard -Parent
if ($BuildRoot.Equals($syncRoot, [StringComparison]::OrdinalIgnoreCase) -or
    $BuildRoot.StartsWith($syncRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'BuildRoot must be outside the synchronized source directory.'
}

function Run([string]$Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Command failed ($LASTEXITCODE)." }
}

# A fresh source snapshot avoids stale source files. Caches stay machine-local.
# git lists files, but copies the working files so uncommitted edits are included.
function Copy-Source([string]$Repository, [string]$Destination, [string[]]$Paths) {
    $files = & git -C $Repository -c core.quotepath=false ls-files --cached --others --exclude-standard -- @Paths
    if ($LASTEXITCODE -ne 0) { throw "Cannot list source files in $Repository" }
    foreach ($relative in $files) {
        if ([IO.Path]::IsPathRooted($relative) -or $relative.Split('/') -contains '..') {
            throw "Unexpected source path: $relative"
        }
        $source = Join-Path $Repository $relative
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { continue }
        $target = Join-Path $Destination $relative
        New-Item -ItemType Directory -Path (Split-Path $target -Parent) -Force | Out-Null
        Copy-Item -LiteralPath $source -Destination $target -Force
    }
}

$saved = @{}
$variables = @('PATH', 'CGO_ENABLED', 'GOOS', 'GOARCH', 'CC', 'CXX', 'RUSTFLAGS', 'CARGO_ENCODED_RUSTFLAGS', 'CARGO_TARGET_DIR', 'CI')
foreach ($name in $variables) { $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
$run = Join-Path $BuildRoot ('runs/' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N').Substring(0, 6))
try {
    if (-not (Test-Path -LiteralPath (Join-Path $GccBin 'gcc.exe'))) { throw "gcc.exe missing in $GccBin" }
    $env:PATH = "$GccBin;$env:LOCALAPPDATA\Programs\Go\bin;$env:PATH"
    $env:CC = Join-Path $GccBin 'gcc.exe'
    $env:CXX = Join-Path $GccBin 'g++.exe'
    $env:CGO_ENABLED = '1'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    # Apply to host build scripts as well: do not pass --target here.
    $env:RUSTFLAGS = '-C link-self-contained=yes'
    Remove-Item Env:CARGO_ENCODED_RUSTFLAGS -ErrorAction SilentlyContinue
    $env:CARGO_TARGET_DIR = Join-Path $BuildRoot 'cargo-target'
    $env:CI = 'true'
    New-Item -ItemType Directory -Path $run -Force | Out-Null
    Copy-Source $dashboard (Join-Path $run 'dashboard') -Paths @('frontend', 'backend')
    Copy-Source $worker (Join-Path $run 'worker') -Paths @('Cargo.toml', 'Cargo.lock', 'src', 'build.rs')

    Push-Location (Join-Path $run 'worker')
    try { Run cargo -Arguments @('+stable-x86_64-pc-windows-gnu', 'build', '--locked', '--release', '--bin', 'rs-wgcna-worker') }
    finally { Pop-Location }
    $workerExe = Join-Path $env:CARGO_TARGET_DIR 'release/rs-wgcna-worker.exe'

    Push-Location (Join-Path $run 'dashboard/frontend')
    try {
        Run pnpm -Arguments @('--config.manage-package-manager-versions=false', 'install', '--frozen-lockfile')
        Run node -Arguments @('node_modules/vue-tsc/bin/vue-tsc.js', '-b')
        Run node -Arguments @('node_modules/vite/bin/vite.js', 'build')
    } finally { Pop-Location }

    $backend = Join-Path $run 'dashboard/backend'
    Copy-Item -LiteralPath $workerExe -Destination (Join-Path $backend 'internal/worker/worker-bin') -Force
    Copy-Item -LiteralPath (Join-Path $run 'dashboard/frontend/dist') -Destination (Join-Path $backend 'internal/web/dist') -Recurse -Force
    $output = Join-Path $run 'app'
    New-Item -ItemType Directory -Path $output -Force | Out-Null
    $exe = Join-Path $output 'rsWGCNA.exe'
    Push-Location $backend
    try { Run go -Arguments @('build', '-tags', 'release', '-ldflags', '-H=windowsgui', '-o', $exe, './cmd/server') }
    finally { Pop-Location }
    Copy-Item -LiteralPath $workerExe -Destination (Join-Path $output 'rs-wgcna-worker.exe') -Force
    [ordered]@{
        platform = 'windows-x64'; built_at = (Get-Date -Format o)
        dashboard_commit = (& git -C $dashboard rev-parse HEAD)
        worker_commit = (& git -C $worker rev-parse HEAD)
        source = 'working files, including uncommitted edits'
        exe_sha256 = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
        worker_sha256 = (Get-FileHash -LiteralPath $workerExe -Algorithm SHA256).Hash
    } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $output 'build-info.json') -Encoding utf8
    Set-Content -LiteralPath (Join-Path $BuildRoot 'latest-app.txt') -Value $output -Encoding utf8
    Write-Host "Windows build: $exe"
    if ($Launch) {
        # Use the embedded, freshly built Windows worker, never a stale override.
        $oldWorker = $env:WGCNA_WORKER
        try {
            Remove-Item Env:WGCNA_WORKER -ErrorAction SilentlyContinue
            Start-Process -FilePath $exe -WorkingDirectory $output | Out-Null
        } finally { $env:WGCNA_WORKER = $oldWorker }
    }
} finally {
    foreach ($name in $variables) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}

