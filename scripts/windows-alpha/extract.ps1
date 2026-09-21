param([Parameter(Mandatory=$true)][string]$Zip, [Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference = 'Stop'
if (Test-Path -LiteralPath $Destination) { throw 'Extraction destination must be new' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$archive = [System.IO.Compression.ZipFile]::OpenRead($Zip)
try {
    foreach ($entry in $archive.Entries) {
        if ($entry.FullName -match '(^/|\\|(^|/)\.\.(/|$)|:)' ) { throw 'Unsafe ZIP entry' }
    }
    $entries = @($archive.Entries | ForEach-Object FullName)
} finally { $archive.Dispose() }
[System.IO.Compression.ZipFile]::ExtractToDirectory($Zip, $Destination)
$entries | ConvertTo-Json
