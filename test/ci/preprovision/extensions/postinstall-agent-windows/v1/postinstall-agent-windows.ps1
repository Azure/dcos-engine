$ErrorActionPreference = "Stop"

$DCOS_DIR = Join-Path $env:SystemDrive "opt\mesosphere"
$ETC_DIR = Join-Path $env:SystemDrive "etc"


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
