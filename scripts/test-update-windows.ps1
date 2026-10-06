# Requires PowerShell 7, a Windows desktop and two built application files. Tests the real
# updater helper, including a visible restart and preservation of portable data.
param(
    [Parameter(Mandatory = $true)][string]$SourceApp,
    [Parameter(Mandatory = $true)][string]$TargetApp
)

$ErrorActionPreference = 'Stop'
$taskSource = (Resolve-Path -LiteralPath $SourceApp).Path
$taskArtifactSource = (Resolve-Path -LiteralPath $TargetApp).Path
$taskID = [guid]::NewGuid().ToString('N')
$taskRoot = Join-Path (Split-Path -Parent $PSScriptRoot) ".codex\validation\update-restart-$taskID"
$taskTemp = Join-Path ([System.IO.Path]::GetTempPath()) "model-switcher-restart-test-$taskID"
$taskTarget = Join-Path $taskRoot 'ModelSwitcher.exe'
$taskHelper = Join-Path $taskTemp "model-switcher-updater-$taskID.exe"
$taskArtifact = Join-Path $taskTemp 'update.exe'
$taskProcesses = @()
New-Item -ItemType Directory -Path $taskRoot, $taskTemp -Force | Out-Null
foreach ($taskDirectory in @('appdata', 'localappdata', 'home', 'codex', 'pi')) {
    New-Item -ItemType Directory -Path (Join-Path $taskRoot $taskDirectory) -Force | Out-Null
}
$taskUTF8 = [System.Text.UTF8Encoding]::new($false)
$taskProfiles = Join-Path $taskRoot 'profiles.json'
[System.IO.File]::WriteAllText($taskProfiles, '{"profiles":[],"target":"chatgpt"}', $taskUTF8)
$taskProfilesHash = (Get-FileHash -LiteralPath $taskProfiles -Algorithm SHA256).Hash
Copy-Item -LiteralPath $taskSource -Destination $taskTarget
Copy-Item -LiteralPath $taskSource -Destination $taskHelper
Copy-Item -LiteralPath $taskArtifactSource -Destination $taskArtifact
$taskExpectedHash = (Get-FileHash -LiteralPath $taskArtifactSource -Algorithm SHA256).Hash
$taskStart = [System.Diagnostics.ProcessStartInfo]::new()
$taskStart.FileName = $taskHelper
$taskStart.ArgumentList.Add('--update-helper')
$taskStart.ArgumentList.Add($taskTarget)
$taskStart.ArgumentList.Add($taskArtifact)
$taskStart.WorkingDirectory = $taskRoot
$taskStart.UseShellExecute = $false
$taskStart.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
$taskStart.RedirectStandardError = $true
foreach ($taskVariable in @{'APPDATA'='appdata';'LOCALAPPDATA'='localappdata';'USERPROFILE'='home';'HOME'='home';'CODEX_HOME'='codex';'PI_CODING_AGENT_DIR'='pi'}.GetEnumerator()) {
    $taskStart.Environment[$taskVariable.Key] = Join-Path $taskRoot $taskVariable.Value
}
$taskStart.Environment['TEMP'] = $taskTemp
$taskStart.Environment['TMP'] = $taskTemp
$null = $taskStart.Environment.Remove('MODELSWITCHER_UPDATE_URL')
try {
    $taskHelperProcess = [System.Diagnostics.Process]::Start($taskStart)
    if (-not $taskHelperProcess.WaitForExit(15000)) { throw 'Updater helper timed out' }
    if ($taskHelperProcess.ExitCode -ne 0) { throw $taskHelperProcess.StandardError.ReadToEnd() }
    for ($taskAttempt = 0; $taskAttempt -lt 100; $taskAttempt++) {
        $taskProcesses = @(Get-Process -Name ModelSwitcher -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $taskTarget })
        if ($taskProcesses.Count -eq 1 -and $taskProcesses[0].MainWindowHandle -ne 0) { break }
        Start-Sleep -Milliseconds 100
    }
    if ($taskProcesses.Count -ne 1 -or $taskProcesses[0].MainWindowHandle -eq 0) {
        throw 'Updated app did not display its main window'
    }
    if ((Get-FileHash -LiteralPath $taskTarget -Algorithm SHA256).Hash -ne $taskExpectedHash) { throw 'Updated executable hash mismatch' }
    if ((Get-FileHash -LiteralPath $taskProfiles -Algorithm SHA256).Hash -ne $taskProfilesHash) { throw 'Portable profiles changed' }
    for ($taskAttempt = 0; $taskAttempt -lt 50 -and (Test-Path -LiteralPath $taskHelper); $taskAttempt++) { Start-Sleep -Milliseconds 100 }
    if ((Test-Path -LiteralPath $taskArtifact) -or (Test-Path -LiteralPath $taskHelper)) { throw 'Temporary updater files were not cleaned up' }
    [ordered]@{
        result = 'passed'; source = $taskSource; artifact = $taskArtifactSource
        restartedPID = $taskProcesses[0].Id; mainWindowTitle = $taskProcesses[0].MainWindowTitle
        version = (& $taskTarget --version | Out-String).Trim()
        profilesPreserved = $true; temporaryFilesCleaned = $true
        artifactVolume = [System.IO.Path]::GetPathRoot($taskArtifact)
        applicationVolume = [System.IO.Path]::GetPathRoot($taskTarget)
        directory = $taskRoot
    } | ConvertTo-Json | Tee-Object -FilePath (Join-Path $taskRoot 'result.json')
} finally {
    if ($taskHelperProcess -and -not $taskHelperProcess.HasExited) {
        $taskCurrentHelper = Get-Process -Id $taskHelperProcess.Id -ErrorAction SilentlyContinue
        if ($taskCurrentHelper -and $taskCurrentHelper.Path -eq $taskHelper) { Stop-Process -Id $taskCurrentHelper.Id }
    }
    foreach ($taskProcess in $taskProcesses) {
        $taskCurrent = Get-Process -Id $taskProcess.Id -ErrorAction SilentlyContinue
        if ($taskCurrent -and $taskCurrent.Path -eq $taskTarget) { Stop-Process -Id $taskCurrent.Id }
    }
}
