param(
    [Parameter(Mandatory)][string]$Directory,
    [string]$Windres = 'windres.exe'
)

$ErrorActionPreference = 'Stop'
$setup = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '../packaging/setup.iss')
if ($setup -notmatch '(?m)^AppVersion=([0-9]+\.[0-9]+\.[0-9]+)\r?$') {
    throw 'Expected a three-part numeric AppVersion in setup.iss.'
}
$version = $Matches[1]
$numericVersion = $version.Replace('.', ',') + ',0'
$descriptions = [ordered]@{
    'rsWGCNA.exe' = 'rsWGCNA desktop application'
    'rs-wgcna-worker.exe' = 'rsWGCNA analysis worker'
    'regutrace.exe' = 'ReguTrace command-line application'
}

if (-not ('RsWgcnaVersionResource' -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;

public static class RsWgcnaVersionResource
{
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern IntPtr BeginUpdateResource(string path, bool deleteExisting);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool UpdateResource(IntPtr handle, IntPtr type, IntPtr name,
        ushort language, byte[] data, uint size);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool EndUpdateResource(IntPtr handle, bool discard);

    public static void Write(string path, string resourceFile)
    {
        byte[] file = File.ReadAllBytes(resourceFile);
        // windres emits a 32-byte null header followed by the VERSIONINFO record.
        if (file.Length < 64 || BitConverter.ToUInt32(file, 0) != 0 ||
            BitConverter.ToUInt32(file, 4) != 32 ||
            BitConverter.ToUInt32(file, 36) != 32 ||
            BitConverter.ToUInt32(file, 40) != 0x0010ffff ||
            BitConverter.ToUInt32(file, 44) != 0x0001ffff ||
            BitConverter.ToUInt16(file, 54) != 0x0409)
            throw new InvalidDataException("Unexpected VERSIONINFO resource header.");
        uint size = BitConverter.ToUInt32(file, 32);
        if (size == 0 || size > file.Length - 64)
            throw new InvalidDataException("Unexpected VERSIONINFO resource length.");
        byte[] data = new byte[size];
        Buffer.BlockCopy(file, 64, data, 0, data.Length);
        IntPtr handle = BeginUpdateResource(path, false);
        if (handle == IntPtr.Zero)
            throw new Win32Exception();
        if (!UpdateResource(handle, (IntPtr)16, (IntPtr)1, 0x0409, data, size))
        {
            int error = Marshal.GetLastWin32Error();
            EndUpdateResource(handle, true);
            throw new Win32Exception(error);
        }
        if (!EndUpdateResource(handle, false))
            throw new Win32Exception();
    }
}
"@
}

$work = Join-Path ([IO.Path]::GetTempPath()) ('rswgcna-version-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $work | Out-Null
try {
    $source = Join-Path $work 'version.rc'
    $resource = Join-Path $work 'version.res'
    foreach ($name in $descriptions.Keys) {
        $path = (Get-Item -LiteralPath (Join-Path $Directory $name)).FullName
        if ((Get-AuthenticodeSignature -LiteralPath $path).Status -ne 'NotSigned') {
            throw "Expected unsigned application executable: $path"
        }
        @"
LANGUAGE 9, 1
1 VERSIONINFO
FILEVERSION $numericVersion
PRODUCTVERSION $numericVersion
FILEOS 0x40004
FILETYPE 1
BEGIN
    BLOCK "StringFileInfo"
    BEGIN
        BLOCK "040904B0"
        BEGIN
            VALUE "CompanyName", "XiaoWang-SXAU-Lab"
            VALUE "FileDescription", "$($descriptions[$name])"
            VALUE "FileVersion", "$version"
            VALUE "OriginalFilename", "$name"
            VALUE "ProductName", "rsWGCNA"
            VALUE "ProductVersion", "$version"
        END
    END
    BLOCK "VarFileInfo"
    BEGIN
        VALUE "Translation", 0x0409, 1200
    END
END
"@ | Set-Content -LiteralPath $source -Encoding ascii
        & $Windres -O res -i $source -o $resource
        if ($LASTEXITCODE -ne 0) { throw "windres failed for $name" }
        [RsWgcnaVersionResource]::Write($path, $resource)
        $info = [Diagnostics.FileVersionInfo]::GetVersionInfo($path)
        if ($info.ProductName -cne 'rsWGCNA' -or $info.ProductVersion -cne $version -or
            $info.FileVersion -cne $version -or $info.OriginalFilename -cne $name) {
            throw "Unexpected version resource in $name"
        }
        Write-Host "$name product version $version"
    }
} finally {
    foreach ($file in @('version.rc', 'version.res')) {
        Remove-Item -LiteralPath (Join-Path $work $file) -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $work
}
