[CmdletBinding()]
param([string]$Worker)
$ErrorActionPreference = 'Stop'
$sourceRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$localRoot = Join-Path $env:LOCALAPPDATA 'rsWGCNA-build/windows'
$runRoot = Join-Path $localRoot ('project-tests/' + [guid]::NewGuid().ToString('N'))
$go = Join-Path $env:LOCALAPPDATA 'Programs/Go/bin/go.exe'
$gofmt = Join-Path $env:LOCALAPPDATA 'Programs/Go/bin/gofmt.exe'
if (-not (Test-Path -LiteralPath $go)) { $go = (Get-Command go -ErrorAction Stop).Source }
if (-not (Test-Path -LiteralPath $gofmt)) { $gofmt = (Get-Command gofmt -ErrorAction Stop).Source }
$moduleCache = & $go env GOMODCACHE
if ($LASTEXITCODE -ne 0) { throw 'Cannot locate machine-local Go module cache.' }
$moduleCache = [IO.Path]::GetFullPath($moduleCache)
$syncRoot = Split-Path $sourceRoot -Parent
if ($moduleCache.StartsWith($syncRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Go module cache must be outside synchronized source.'
}
New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
# Copy only tracked/unignored source files, never shared dependencies or binaries.
$files = & git -C $sourceRoot -c core.quotepath=false ls-files --cached --others --exclude-standard -- backend
if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate backend source.' }
foreach ($relative in $files) {
    $source = Join-Path $sourceRoot $relative
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { continue }
    $target = Join-Path $runRoot $relative
    New-Item -ItemType Directory -Path (Split-Path $target -Parent) -Force | Out-Null
    Copy-Item -LiteralPath $source -Destination $target
}
$saved = @{}
$names = @('GOCACHE','GOMODCACHE','GOTMPDIR','WGCNA_PROJECT_TEST_WORKER')
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
try {
    $env:GOCACHE = Join-Path $localRoot 'go-cache'
    $env:GOMODCACHE = $moduleCache
    $env:GOTMPDIR = Join-Path $localRoot 'go-tmp'
    New-Item -ItemType Directory -Path $env:GOTMPDIR -Force | Out-Null
    $env:WGCNA_PROJECT_TEST_WORKER = $Worker
    Push-Location (Join-Path $runRoot 'backend')
    try {
        & $go test -count=1 -v ./internal/project
        if ($LASTEXITCODE -ne 0) { throw 'Project tests failed.' }
        & $go test -count=1 -v -run '^TestProjectHTTP' ./internal/api
        if ($LASTEXITCODE -ne 0) { throw 'Project HTTP tests failed.' }
    } finally { Pop-Location }
    Write-Host "Project test snapshot: $runRoot"
} finally {
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name,$saved[$name],'Process') }
}
