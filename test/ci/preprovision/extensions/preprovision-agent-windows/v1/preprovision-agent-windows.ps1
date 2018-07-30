$ErrorActionPreference = "Stop"

$MESOS_ETC_SERVICE_DIR = Join-Path $env:SystemDrive "DCOS-etc\mesos\service"
$MESOS_SERVICE_NAME = "dcos-mesos-slave"


filter Timestamp { "[$(Get-Date -Format o)] $_" }

function Write-Log {
    Param(
        [string]$Message
    )
    $msg = $message | Timestamp
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

function Start-FileDownloadWithCurl {
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

function Start-CIAgentSetup {
    # Pre-pull the IIS image
    Start-ExecuteWithRetry -ScriptBlock { docker.exe pull "microsoft/iis:windowsservercore-1803" } `
                           -MaxRetryCount 30 -RetryInterval 3 -RetryMessage "Failed to pull IIS image. Retrying"
    # Enable Docker debug logging and capture stdout and stderr to a file.
    # We're using the updated service wrapper for this.
    $serviceName = "Docker"
    $dockerHome = Join-Path $env:ProgramFiles "Docker"
    $wrapperUrl = "http://dcos-win.westus.cloudapp.azure.com/downloads/service-wrapper.exe"
    Stop-Service $serviceName
    sc.exe delete $serviceName
    if($LASTEXITCODE) {
        Throw "Failed to delete service: $serviceName"
    }
    Start-FileDownloadWithCurl -URL $wrapperUrl -Destination "${dockerHome}\service-wrapper.exe" -RetryCount 30
    $binPath = ("`"${dockerHome}\service-wrapper.exe`" " +
                "--service-name `"$serviceName`" " +
                "--exec-start-pre `"powershell.exe if(Test-Path '${env:ProgramData}\docker\docker.pid') { Remove-Item -Force '${env:ProgramData}\docker\docker.pid' }`" " +
                "--log-file `"$dockerHome\dockerd.log`" " +
                "`"$dockerHome\dockerd.exe`" -D")
    New-Service -Name $serviceName -StartupType "Automatic" -Confirm:$false `
                -DisplayName "Docker Windows Agent" -BinaryPathName $binPath
    sc.exe failure $serviceName reset=5 actions=restart/1000
    if($LASTEXITCODE) {
        Throw "Failed to set $serviceName service recovery options"
    }
    sc.exe failureflag $serviceName 1
    if($LASTEXITCODE) {
        Throw "Failed to set $serviceName service recovery options"
    }
    Start-Service $serviceName
}

function Write-MesosSecretFiles {
    # Write the credential files
    # NOTE: These are only some dumb secrets used for testing. DO NOT use in production!
    if(Test-Path $MESOS_ETC_SERVICE_DIR) {
        Remove-Item -Recurse -Force $MESOS_ETC_SERVICE_DIR
    }
    New-Item -ItemType "Directory" -Path $MESOS_ETC_SERVICE_DIR -Force
    $utf8NoBOM = New-Object System.Text.UTF8Encoding $false
    $credentials = @{
        "principal" = "mycred1"
        "secret" = "mysecret1"
    }
    $json = ConvertTo-Json -InputObject $credentials -Compress
    [System.IO.File]::WriteAllLines("$MESOS_ETC_SERVICE_DIR\credential.json", $json, $utf8NoBOM)
    $httpCredentials = @{
        "credentials" = @(
            @{
                "principal" = "mycred2"
                "secret" = "mysecret2"
            }
        )
    }
    $json = ConvertTo-Json -InputObject $httpCredentials -Compress
    [System.IO.File]::WriteAllLines("$MESOS_ETC_SERVICE_DIR\http_credential.json", $json, $utf8NoBOM)
    # Create the Mesos service environment file with authentication enabled
    $serviceEnv = @(
        "MESOS_AUTHENTICATE_HTTP_READONLY=true",
        "MESOS_AUTHENTICATE_HTTP_READWRITE=true",
        "MESOS_HTTP_CREDENTIALS=$MESOS_ETC_SERVICE_DIR\http_credential.json",
        "MESOS_CREDENTIAL=$MESOS_ETC_SERVICE_DIR\credential.json"
    )
    Set-Content -Path "${MESOS_ETC_SERVICE_DIR}\environment-file" -Value $serviceEnv -Encoding utf8
}

try {
    Start-CIAgentSetup
    Write-MesosSecretFiles
    Write-Output "Successfully executed the preprovision-agent-windows.ps1 script"
} catch {
    Write-Log "The pre-provision setup for the DC/OS Windows node failed"
    Write-Log "preprovision-agent-windows-setup.ps1 exception: $_.ToString()"
    Write-Log $_.ScriptStackTrace
    Throw $_
}
