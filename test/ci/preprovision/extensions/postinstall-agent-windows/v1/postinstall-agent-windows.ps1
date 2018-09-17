$ErrorActionPreference = "Stop"

filter Timestamp { "[$(Get-Date -Format o)] $_" }

function Write-Log {
    Param(
        [string]$Message
    )
    $msg = $message | Timestamp
    Write-Output $msg
}

& C:\opt\mesosphere\bin\systemctl.exe stop dcos-metrics-agent.service
if($LASTEXITCODE) {
    Throw "Failed to stop dcos-metrics-agent.service"
}

& C:\opt\mesosphere\bin\systemctl.exe disable dcos-metrics-agent.service
if($LASTEXITCODE) {
    Throw "Failed to disable dcos-metrics-agent.service"
}

& C:\opt\mesosphere\bin\systemctl.exe stop dcos-diagnostics.service
if($LASTEXITCODE) {
    Throw "Failed to stop dcos-diagnostics.service"
}

$ServiceList = "C:\opt\mesosphere\bin\servicelist.txt"
$Content = Get-Content $ServiceList | Where-Object {$_ -notmatch 'dcos-metrics-agent.service'}
Set-Content -Path $ServiceList -Value $Content

& C:\opt\mesosphere\bin\systemctl.exe start dcos-diagnostics.service
if($LASTEXITCODE) {
    Throw "Failed to start dcos-diagnostics.service"
}
