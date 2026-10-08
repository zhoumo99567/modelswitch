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
# Use the same semantic version validation as the publishing workflow.
& node (Join-Path $PSScriptRoot "release-manifest.mjs") prepare $Version
if ($LASTEXITCODE -ne 0) { throw "Invalid release version or tag" }

$ldflags = "-X main.AppVersion=$Version"
if ($env:MODELSWITCHER_BUILD_UPDATE_URL) {
    $updateUrl = $env:MODELSWITCHER_BUILD_UPDATE_URL.Trim()
    if ($updateUrl -notmatch '^https://[^\s\x27]+$') { throw "Build update URL must be HTTPS without spaces or quotes" }
    $ldflags += " -X 'main.defaultUpdateManifestURL=$updateUrl'"
}
$outputDir = Join-Path $Root "release\$Version"
$windowsName = "ModelSwitcher-$Version-windows-amd64.exe"
$windowsPath = Join-Path $outputDir $windowsName
$manifestPath = Join-Path $outputDir "latest.json"

New-Item -ItemType Directory -Force $outputDir | Out-Null
Write-Host "Building Windows $Version..."
# Keep the ignored portable profiles.json beside the app across local builds.
& wails build -platform windows/amd64 -ldflags $ldflags
if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
$builtVersion = (& (Join-Path $Root "build\bin\model-switcher.exe") --version | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $builtVersion -ne $Version) { throw "Built application version does not match VERSION: $builtVersion" }
Copy-Item -Force (Join-Path $Root "build\bin\model-switcher.exe") $windowsPath

$baseUrl = if ($env:MODELSWITCHER_UPDATE_BASE_URL) { $env:MODELSWITCHER_UPDATE_BASE_URL.TrimEnd('/') } else { "" }
$macZip = Join-Path $outputDir "ModelSwitcher-$Version-macos-universal.app.zip"
if ($Publish -and -not $baseUrl) { throw "Set MODELSWITCHER_UPDATE_BASE_URL before S3 publishing" }
$previousDownloadBase = $env:MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL
try {
    if (-not $previousDownloadBase -and $baseUrl) { $env:MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL = "$baseUrl/$Version" }
    & node (Join-Path $PSScriptRoot "release-manifest.mjs") manifest $Version $outputDir
    if ($LASTEXITCODE -ne 0) { throw "Generating release manifest failed" }
} finally {
    $env:MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL = $previousDownloadBase
}
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
    & aws s3 cp (Join-Path $outputDir "SHA256SUMS.txt") "$s3Uri/$Version/SHA256SUMS.txt" --only-show-errors
    if ($LASTEXITCODE -ne 0) { throw "Uploading SHA256SUMS.txt failed" }
    Write-Host "Published version $Version to $s3Uri"
} else {
    Write-Host "Build only. Add -Publish after setting MODELSWITCHER_S3_URI and MODELSWITCHER_UPDATE_BASE_URL."
}
