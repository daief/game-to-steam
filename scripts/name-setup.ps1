$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$config = Get-Content -LiteralPath (Join-Path $projectRoot 'mygo.json') -Raw | ConvertFrom-Json
$output = Join-Path $projectRoot "$($config.out)/windows-amd64"
$source = Join-Path $output "$($config.name) Setup $($config.version) amd64.exe"
$destination = Join-Path $output "game-to-steam-setup-$($config.version)-amd64.exe"
if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    throw "MyGo setup not found: $source"
}
Move-Item -LiteralPath $source -Destination $destination -Force
Write-Output $destination
