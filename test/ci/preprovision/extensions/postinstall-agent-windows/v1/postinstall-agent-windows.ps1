$ErrorActionPreference = "Stop"

$DCOS_DIR = Join-Path $env:SystemDrive "opt\mesosphere"
$ETC_DIR = Join-Path $env:SystemDrive "etc"


#
# Enable Docker debug logging and capture stdout and stderr to a file.
# We're using the updated service wrapper for this.
#
$serviceName = "Docker"
$dockerHome = Join-Path $env:ProgramFiles "Docker"
$wrapperUrl = "http://dcos-win.westus2.cloudapp.azure.com/downloads/service-wrapper.exe"
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

#
# Disable dcos-metrics agent
#
Stop-Service -Force "dcos-metrics-agent.service"
sc.exe delete "dcos-metrics-agent.service"
if($LASTEXITCODE) {
    Throw "Failed to delete dcos-metrics-agent.service"
}
Remove-Item -Force "$ETC_DIR\systemd\active\dcos-metrics-agent.service"
Remove-Item -Force "$ETC_DIR\systemd\active\dcos.target.wants\dcos-metrics-agent.service"
Remove-Item -Force "$ETC_DIR\systemd\system\dcos-metrics-agent.service"
Remove-Item -Force "$ETC_DIR\systemd\system\dcos.target.wants\dcos-metrics-agent.service"

#
# Remove dcos-metrics from the list of monitored services for dcos-diagnostics
#
$serviceListFile = Join-Path $DCOS_DIR "bin\servicelist.txt"
$newContent = Get-Content $serviceListFile | Where-Object { $_ -notmatch 'dcos-metrics-agent.service' }
Set-Content -Path $serviceListFile -Value $newContent -Encoding ascii
Restart-Service -Force "dcos-diagnostics.service"
