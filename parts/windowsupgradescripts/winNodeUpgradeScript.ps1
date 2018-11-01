filter Timestamp {"[$(Get-Date -Format o)] $_"}

function Write-Log {
    Param(
        [string]$Message
    )
    $msg = $Message | Timestamp
    Write-Output $msg
}


function Start-ExecuteWithRetry {
    Param(
        [Parameter(Mandatory=$true)]
        [ScriptBlock]$ScriptBlock,
        [int]$MaxRetryCount=10,
        [int]$RetryInterval=3,
        [string]$RetryMessage,
        [array]$ArgumentList=@()
    )
    $currentErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $retryCount = 0
    while ($true) {
        Write-Log "Start-ExecuteWithRetry attempt $retryCount"
        try {
            $res = Invoke-Command -ScriptBlock $ScriptBlock `
                                  -ArgumentList $ArgumentList
            $ErrorActionPreference = $currentErrorActionPreference
            Write-Log "Start-ExecuteWithRetry terminated"
            return $res
        } catch [System.Exception] {
            $retryCount++
            if ($retryCount -gt $MaxRetryCount) {
                $ErrorActionPreference = $currentErrorActionPreference
                Write-Log "Start-ExecuteWithRetry exception thrown"
                throw
            } else {
                if($RetryMessage) {
                    Write-Log "Start-ExecuteWithRetry RetryMessage: $RetryMessage"
                } elseif($_) {
                    Write-Log "Start-ExecuteWithRetry Retry: $_.ToString()"
                }
                Start-Sleep $RetryInterval
            }
        }
    }
}


function Start-FileDownload {
    Param(
        [Parameter(Mandatory=$true)]
        [string]$URL,
        [Parameter(Mandatory=$true)]
        [string]$Destination,
        [Parameter(Mandatory=$false)]
        [int]$RetryCount=10
    )
    $params = @('-fLsS', '-o', "`"${Destination}`"", "`"${URL}`"")
    Start-ExecuteWithRetry -ScriptBlock {
        $p = Start-Process -FilePath 'curl.exe' -NoNewWindow -ArgumentList $params -Wait -PassThru
        if($p.ExitCode -ne 0) {
            Throw "Fail to download $URL"
        }
    } -MaxRetryCount $RetryCount -RetryInterval 3 -RetryMessage "Failed to download ${URL}. Retrying"
}


try {
    $upgradeScriptURL = "WIN_UPGRADE_SCRIPT_URL"
    $upgradeDir = Join-Path $env:SystemDrive "AzureData\upgrade\NEW_VERSION"
    $log = Join-Path $env:SystemDrive "AzureData\upgrade_NEW_VERSION.log"

    Start-Transcript -Path $log -append
    Write-Log "Starting node upgrade to DCOS NEW_VERSION"
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $upgradeDir
    New-Item -ItemType Directory -Force -Path $upgradeDir
    Push-Location $upgradeDir

    Start-FileDownload -URL $upgradeScriptURL -Destination "dcos_node_upgrade.ps1"

    .\dcos_node_upgrade.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to run dcos_node_upgrade.ps1"
    }
}catch {
    Write-Log "Failed to upgrade Windows agent node: $_"
    Stop-Transcript
    exit 1
}
Write-Log "Successfully upgraded Windows agent node"
Stop-Transcript
exit 0
