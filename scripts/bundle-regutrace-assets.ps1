[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Output,
    [Parameter(Mandatory = $true)][string]$PackSource,
    [string[]]$PackNames = @(
        'plant-jaspar-2024-rbh',
        'nematode-beta-2026-11-15-r2',
        'drosophila-beta-2026-11-15',
        'yeast-2026-11-15'
    ),
    [Parameter(Mandatory = $true)][string]$ToolSource
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Hash([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Copy-Required([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "required asset missing: $Source"
    }
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

$Output = [IO.Path]::GetFullPath($Output)
$PackSource = [IO.Path]::GetFullPath($PackSource)
$ToolSource = [IO.Path]::GetFullPath($ToolSource)
if (-not (Test-Path -LiteralPath $Output -PathType Container)) {
    throw "output directory does not exist: $Output"
}

$packsOut = Join-Path $Output 'packs'
$toolsOut = Join-Path $Output 'tools'
New-Item -ItemType Directory -Path $packsOut -Force | Out-Null
New-Item -ItemType Directory -Path $toolsOut -Force | Out-Null

$packReadme = @('# Bundled ReguTrace reference packs', '')
$packScopes = @()
foreach ($name in $PackNames) {
    $source = Join-Path $PackSource $name
    $manifestPath = Join-Path $source 'manifest.json'
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw "reference pack missing manifest: $manifestPath"
    }
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($manifest.pack_version)) {
        throw "reference pack has empty pack_version: $manifestPath"
    }
    $destination = Join-Path $packsOut $name
    Copy-Item -LiteralPath $source -Destination $destination -Recurse -Force
    $scope = ''
    if ($manifest.PSObject.Properties.Name -contains 'motif_scope') { $scope = [string]$manifest.motif_scope }
    $species = ''
    if ($manifest.PSObject.Properties.Name -contains 'species') { $species = [string]$manifest.species }
    $packReadme += "- ``$name`` — $species. $scope"
    $packScopes += [ordered]@{
        key = $name
        version = $manifest.pack_version
        species = $species
        motif_scope = $scope
        manifest_sha256 = (Hash $manifestPath)
    }
}
$packReadme += ''
$packReadme += 'These packs are software-side reference data. ReguTrace verifies each manifest and file SHA-256 at pipeline start.'
[IO.File]::WriteAllText((Join-Path $packsOut 'README.md'), ($packReadme -join "`n") + "`n", [Text.UTF8Encoding]::new($false))

$hmmSource = Join-Path $ToolSource 'hmmer-win'
$blastSource = Join-Path $ToolSource 'blast-win'
$hmmExe = Join-Path $toolsOut 'hmmsearch.exe'
$blastExe = Join-Path $toolsOut 'blastp.exe'
Copy-Required (Join-Path $hmmSource 'hmmsearch.exe') $hmmExe
Copy-Required (Join-Path $hmmSource 'cygwin1.dll') (Join-Path $toolsOut 'cygwin1.dll')
Copy-Required (Join-Path $blastSource 'blastp.exe') $blastExe
Copy-Required (Join-Path $blastSource 'nghttp2.dll') (Join-Path $toolsOut 'nghttp2.dll')
Copy-Required (Join-Path $hmmSource 'LICENSE') (Join-Path $toolsOut 'HMMER-LICENSE.txt')
Copy-Required (Join-Path $blastSource 'LICENSE') (Join-Path $toolsOut 'BLAST-LICENSE.txt')
Copy-Required (Join-Path $hmmSource 'CYGWIN-LICENSE.txt') (Join-Path $toolsOut 'CYGWIN-LICENSE.txt')
Copy-Required (Join-Path $blastSource 'NGHTTP2-LICENSE.txt') (Join-Path $toolsOut 'NGHTTP2-LICENSE.txt')
$sourcesOut = Join-Path $toolsOut 'sources'
New-Item -ItemType Directory -Path $sourcesOut -Force | Out-Null
Copy-Required (Join-Path $hmmSource 'cygwin-3.3.3-source.tar.xz') (Join-Path $sourcesOut 'cygwin-3.3.3-source.tar.xz')

$hmmText = [string]::Join("`n", (& $hmmExe -h 2>&1))
$blastText = [string]::Join("`n", (& $blastExe -version 2>&1))
$hmmVersion = [string][regex]::Match($hmmText, 'HMMER\s+([0-9][0-9.]*)').Groups[1].Value
$blastVersion = [string][regex]::Match($blastText, 'blastp:\s*([0-9][^\s]*)').Groups[1].Value
if ([string]::IsNullOrWhiteSpace($hmmVersion)) { throw 'could not read hmmsearch version' }
if ([string]::IsNullOrWhiteSpace($blastVersion)) { throw 'could not read blastp version' }

$toolManifest = [ordered]@{
    schema = 1
    tools = [ordered]@{
        hmmsearch = [ordered]@{
            version = $hmmVersion
            file = 'hmmsearch.exe'
            sha256 = (Hash $hmmExe)
            license = 'HMMER BSD-3-Clause'
        }
        blastp = [ordered]@{
            version = $blastVersion
            file = 'blastp.exe'
            sha256 = (Hash $blastExe)
            license = 'NCBI BLAST+ public domain'
        }
    }
    files = [ordered]@{
        'cygwin1.dll' = (Hash (Join-Path $toolsOut 'cygwin1.dll'))
        'nghttp2.dll' = (Hash (Join-Path $toolsOut 'nghttp2.dll'))
        'CYGWIN-LICENSE.txt' = (Hash (Join-Path $toolsOut 'CYGWIN-LICENSE.txt'))
        'NGHTTP2-LICENSE.txt' = (Hash (Join-Path $toolsOut 'NGHTTP2-LICENSE.txt'))
        'sources/cygwin-3.3.3-source.tar.xz' = (Hash (Join-Path $sourcesOut 'cygwin-3.3.3-source.tar.xz'))
    }
    packs = $packScopes
}
$toolJson = $toolManifest | ConvertTo-Json -Depth 8
[IO.File]::WriteAllText((Join-Path $toolsOut 'manifest.json'), $toolJson + "`n", [Text.UTF8Encoding]::new($false))

$notices = @(
    '# Third-party notices',
    '',
    'rsWGCNA bundles HMMER, NCBI BLAST+ and ReguTrace reference packs so that',
    'reference-pack mode runs without a separate tool installation or environment variables.',
    '',
    '## HMMER',
    '',
    ('Bundled executable: hmmsearch ' + $hmmVersion + ' (hmmsearch.exe + cygwin1.dll)'),
    'License: BSD 3-Clause. Full text in tools/HMMER-LICENSE.txt.',
    '',
    '## NCBI BLAST+',
    '',
    ('Bundled executable: blastp ' + $blastVersion + ' (blastp.exe + nghttp2.dll)'),
    'License: public domain. Full text in tools/BLAST-LICENSE.txt.',
    '',
    '## Cygwin runtime (required by the HMMER Windows build)',
    '',
    'Bundled DLL: cygwin1.dll 3.3.3.',
    'Cygwin library source is distributed under LGPLv3+; some Cygwin utilities',
    'are GPLv3+. The complete corresponding source is shipped at',
    'tools/sources/cygwin-3.3.3-source.tar.xz (newlib-cygwin tag',
    'cygwin-3_3_3-release). Full license texts are in tools/CYGWIN-LICENSE.txt.',
    '',
    '## nghttp2 (required by the BLAST+ Windows build)',
    '',
    'Bundled DLL: nghttp2.dll 1.40.0. License: MIT.',
    'Full text in tools/NGHTTP2-LICENSE.txt.',
    '',
    '## Reference packs',
    ''
)
foreach ($pack in $packScopes) {
    $notices += "- $($pack.key): $($pack.species). $($pack.motif_scope)"
}
$notices += @(
    '',
    'JASPAR matrices are distributed under CC BY 4.0',
    '(https://creativecommons.org/licenses/by/4.0/); cite JASPAR 2024 and the',
    'original matrix sources. Family rules and reference maps are derived from',
    'AnimalTFDB (http://bioinfo.life.hust.edu.cn/AnimalTFDB/) and PlantTFDB',
    '(http://planttfdb.gao-lab.org/); cite those resources. Pack manifests record',
    'the pack version and SHA-256 checksums; ReguTrace verifies them before scanning.',
    ''
)
[IO.File]::WriteAllText((Join-Path $Output 'THIRD_PARTY_NOTICES.md'), ($notices -join "`n") + "`n", [Text.UTF8Encoding]::new($false))

Write-Host ("Bundled {0} reference pack(s), hmmsearch {1}, blastp {2} into {3}" -f $PackNames.Count, $hmmVersion, $blastVersion, $Output)
