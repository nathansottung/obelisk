param([Parameter(ValueFromRemainingArguments=$true)][string[]]$ActionArgs)
$ErrorActionPreference = 'Stop'
$node = Get-Command node.exe -CommandType Application -ErrorAction SilentlyContinue
if (-not $node) {
    Write-Error 'Existing Node.js 24 x64 is required. No runtime will be downloaded. Ask the tester coordinator to resolve this prerequisite.'
    exit 1
}
& $node.Source (Join-Path $PSScriptRoot 'launcher.mjs') @ActionArgs
exit $LASTEXITCODE
