filter Timestamp {"[$(Get-Date -Format o)] $_"}

function Write-Log($message)
{
    $msg = $message | Timestamp
    Write-Output $msg
}

function RetryCurl($url, $path)
{
    for($i = 1; $i -le 10; $i++) {
        try {
            & curl.exe --keepalive-time 2 -fLsS --retry 20 -o $path $url
            if ($LASTEXITCODE -eq 0) {
                Write-Log "Downloaded $url in $i attempts"
                return
            }
        } catch {
        }
        Sleep(2)
    }
    throw "Failed to download $url"
}

$upgradeScriptURL = "WIN_UPGRADE_SCRIPT_URL"
$upgradeDir = "C:\AzureData\upgrade\NEW_VERSION"
$log = "C:\AzureData\upgrade_NEW_VERSION.log"
$adminUser = "ADMIN_USER"
$password = "ADMIN_PASSWORD"
try {
        Start-Transcript -Path $log -append
        Write-Log "Starting node upgrade to DCOS NEW_VERSION"
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $upgradeDir
        New-Item -ItemType Directory -Force -Path $upgradeDir
        cd $upgradeDir

        [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\\$adminUser", "Machine")
        [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Machine")

        [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\\$adminUser", "Process")
        [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Process")

        RetryCurl $upgradeScriptURL "dcos_node_upgrade.ps1"

        .\dcos_node_upgrade.ps1
}catch {
        Write-Log "Failed to upgrade Windows agent node: $_"
        Stop-Transcript
    exit 1
}
Write-Log "Successfully upgraded Windows agent node"
Stop-Transcript
exit 0
