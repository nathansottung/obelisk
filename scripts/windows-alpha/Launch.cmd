@echo off
rem Obelisk developer-alpha launcher. Runs launcher.mjs from this folder with an
rem existing Node.js found on PATH, passing every argument unchanged, and returns
rem its exit code. No download, no settings change and no PowerShell.
setlocal DisableDelayedExpansion
set "OBELISK_NODE="
for %%I in (node.exe) do set "OBELISK_NODE=%%~$PATH:I"
if not defined OBELISK_NODE goto :missing
"%OBELISK_NODE%" "%~dp0launcher.mjs" %*
exit /b %ERRORLEVEL%
:missing
echo Existing Node.js 24 x64 is required. No runtime will be downloaded. Ask the tester coordinator to resolve this prerequisite. 1>&2
exit /b 1
