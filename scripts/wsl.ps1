param(
  [Parameter(Position = 0)]
  [ValidateSet("bootstrap", "build", "check", "spec-init", "spec-check", "dev", "clean")]
  [string]$Task = "dev"
)

$ErrorActionPreference = "Stop"
$repoPath = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$wslPath = (& wsl.exe -d Ubuntu-24.04 -e wslpath -a $repoPath).Trim()

if (-not $wslPath) {
  throw "WSL上のプロジェクトパスを取得できませんでした。"
}

& wsl.exe -d Ubuntu-24.04 -e bash --noprofile --norc -c "cd -- '$wslPath' && exec make $Task"
exit $LASTEXITCODE
