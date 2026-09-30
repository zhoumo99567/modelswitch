param(
    [string]$Version = "",
    [switch]$Publish
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -Raw (Join-Path $Root "VERSION")).Trim()
}
if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
    throw "Version must use semantic versioning, for example 0.2.0"
}

$ldflags = "-X main.AppVersion=$Version"
$outputDir = Join-Path $Root "release\$Version"
$windowsName = "ModelSwitcher-$Version-windows-amd64.exe"
$windowsPath = Join-Path $outputDir $windowsName
$manifestPath = Join-Path $outputDir "latest.json"

New-Item -ItemType Directory -Force $outputDir | Out-Null
Write-Host "Building Windows $Version..."
& wails build -clean -platform windows/amd64 -ldflags $ldflags
if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
Copy-Item -Force (Join-Path $Root "build\bin\model-switcher.exe") $windowsPath

function Get-Sha256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
}

$baseUrl = if ($env:MODELSWITCHER_UPDATE_BASE_URL) { $env:MODELSWITCHER_UPDATE_BASE_URL.TrimEnd('/') } else { "" }
$windowsUrl = if ($baseUrl) { "$baseUrl/$Version/$windowsName" } else { "" }
$manifest = [ordered]@{
    version = $Version
    windows = [ordered]@{
        url = $windowsUrl
        sha256 = Get-Sha256 $windowsPath
    }
    macos = [ordered]@{
        url = ""
        sha256 = ""
    }
}

$macZip = Join-Path $outputDir "ModelSwitcher-$Version-macos-universal.app.zip"
if (Test-Path $macZip) {
    $manifest.macos.url = if ($baseUrl) { "$baseUrl/$Version/$(Split-Path $macZip -Leaf)" } else { "" }
    $manifest.macos.sha256 = Get-Sha256 $macZip
}
$json = $manifest | ConvertTo-Json -Depth 5
$utf8 = New-Object System.Text.UTF8Encoding -ArgumentList $false
[System.IO.File]::WriteAllText($manifestPath, $json, $utf8)
Write-Host "Created $manifestPath"

if ($Publish) {
    $s3Uri = if ($env:MODELSWITCHER_S3_URI) { $env:MODELSWITCHER_S3_URI.TrimEnd('/') } else { "" }
    if (-not $s3Uri) { throw "Set MODELSWITCHER_S3_URI, for example s3://bucket/model-switcher" }
    if (-not (Get-Command aws -ErrorAction SilentlyContinue)) { throw "AWS CLI is required for -Publish" }
    & aws s3 cp $windowsPath "$s3Uri/$Version/$windowsName" --only-show-errors
    if ($LASTEXITCODE -ne 0) { throw "Uploading Windows artifact failed" }
    if (Test-Path $macZip) {
        & aws s3 cp $macZip "$s3Uri/$Version/$(Split-Path $macZip -Leaf)" --only-show-errors
        if ($LASTEXITCODE -ne 0) { throw "Uploading macOS artifact failed" }
    }
    & aws s3 cp $manifestPath "$s3Uri/latest.json" --only-show-errors --content-type application/json
    if ($LASTEXITCODE -ne 0) { throw "Uploading latest.json failed" }
    Write-Host "Published version $Version to $s3Uri"
} else {
    Write-Host "Build only. Add -Publish after setting MODELSWITCHER_S3_URI and MODELSWITCHER_UPDATE_BASE_URL."
}
