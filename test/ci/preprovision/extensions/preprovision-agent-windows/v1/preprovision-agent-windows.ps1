$ErrorActionPreference = "Stop"

$MESOS_CREDENTIALS_DIR = Join-Path $env:SystemDrive "AzureData\mesos"


filter Timestamp { "[$(Get-Date -Format o)] $_" }

function Write-Log {
    Param(
        [string]$Message
    )
    $msg = $message | Timestamp
    Write-Output $msg
}

function Write-MesosSecretFiles {
    # Write the credential files
    # NOTE: These are only some dumb secrets used for testing. DO NOT use in production!
    if(Test-Path $MESOS_CREDENTIALS_DIR) {
        Remove-Item -Recurse -Force $MESOS_CREDENTIALS_DIR
    }
    New-Item -ItemType "Directory" -Path $MESOS_CREDENTIALS_DIR -Force
    $utf8NoBOM = New-Object System.Text.UTF8Encoding $false
    $credentials = @{
        "principal" = "mycred1"
        "secret" = "mysecret1"
    }
    $json = ConvertTo-Json -InputObject $credentials -Compress
    [System.IO.File]::WriteAllLines("$MESOS_CREDENTIALS_DIR\credential.json", $json, $utf8NoBOM)
    $httpCredentials = @{
        "credentials" = @(
            @{
                "principal" = "mycred2"
                "secret" = "mysecret2"
            }
        )
    }
    $json = ConvertTo-Json -InputObject $httpCredentials -Compress
    [System.IO.File]::WriteAllLines("$MESOS_CREDENTIALS_DIR\http_credential.json", $json, $utf8NoBOM)
    # Create the Mesos service environment file with authentication enabled
    $serviceEnv = @(
        "`$env:MESOS_AUTHENTICATE_HTTP_READONLY='true'",
        "`$env:MESOS_AUTHENTICATE_HTTP_READWRITE='true'",
        "`$env:MESOS_HTTP_CREDENTIALS=`"$MESOS_CREDENTIALS_DIR\http_credential.json`"",
        "`$env:MESOS_CREDENTIAL=`"$MESOS_CREDENTIALS_DIR\credential.json`""
    )
    Set-Content -Path "$MESOS_CREDENTIALS_DIR\auth-env.ps1" -Value $serviceEnv -Encoding "default"
}

try {
    Write-MesosSecretFiles
    Write-Output "Successfully executed the preprovision-agent-windows.ps1 script"
} catch {
    Write-Log "The pre-provision setup for the DC/OS Windows node failed"
    Write-Log "preprovision-agent-windows-setup.ps1 exception: $_.ToString()"
    Write-Log $_.ScriptStackTrace
    exit 1
}
