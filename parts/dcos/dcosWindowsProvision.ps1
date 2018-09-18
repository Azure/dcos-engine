<#
    .SYNOPSIS
        Provisions VM as a Windows bootstrap node.

    .DESCRIPTION
        Provisions VM as a Windows bootstrap node.

     Invoke by:

#>

[CmdletBinding(DefaultParameterSetName="Standard")]
param(
    [string]
    [ValidateNotNullOrEmpty()]
    $BootstrapIP,

    [string]
    [ValidateNotNullOrEmpty()]
    $adminUser,

    [string]
    [AllowEmptyString()]
    $customAttrs
)

$global:BootstrapInstallDir = "C:\AzureData"

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

function UpdateDocker()
{
    # Stop Docker service, disable Docker Host Networking Service
    Write-Log "Stopping Docker"
    Stop-Service Docker

    Write-Log "Disabling Docker Host Networking Service"
    Get-HNSNetwork | Remove-HNSNetwork
    $dockerData = Join-Path $env:ProgramData "Docker"
    Set-Content -Path "$dockerData\config\daemon.json" -Value '{ "bridge" : "none" }' -Encoding Ascii

    # Upgrade and restart Docker
    if ("WINDOWS_DOCKER_VERSION" -ne "current") {
        Write-Log "Updating Docker to WINDOWS_DOCKER_VERSION"
        Install-Module DockerMsftProvider -Force
        Install-Package -Name docker -ProviderName DockerMsftProvider -Force -RequiredVersion WINDOWS_DOCKER_VERSION
    }
    Write-Log "Starting Docker"
    Start-Service Docker
}

function InstallOpenSSH()
{
    Write-Log "Installing OpenSSH"
    $rslt = ( get-service | where { $_.name -like "sshd" } )
    if ( $rslt.count -eq 0) {
        $list = (Get-WindowsCapability -Online | ? Name -like 'OpenSSH.Server*')
        Add-WindowsCapability -Online -Name $list.Name
        Install-Module -Force OpenSSHUtils
    }
    Start-Service sshd

    Write-Log "Creating authorized key"
    $path = "C:\AzureData\authorized_keys"
    Set-Content -Path $path -Value "SSH_PUB_KEY" -Encoding Ascii

    (Get-Content C:\ProgramData\ssh\sshd_config) -replace "AuthorizedKeysFile(\s+).ssh/authorized_keys", "AuthorizedKeysFile $path" | Set-Content C:\ProgramData\ssh\sshd_config
    $acl = Get-Acl -Path $path
    $acl.SetAccessRuleProtection($True, $True)
    $acl | Set-Acl -Path $path

    $acl = Get-Acl -Path $path
    $rules = $acl.Access
    $usersToRemove = @("Everyone","BUILTIN\Users","NT AUTHORITY\Authenticated Users")
    foreach ($u in $usersToRemove) {
        $targetrule = $rules | where IdentityReference -eq $u
        if ($targetrule) {
            $acl.RemoveAccessRule($targetrule)
        }
    }
    $acl | Set-Acl -Path $path

    Restart-Service sshd

    $sshStartCmd = "C:\AzureData\OpenSSHStart.ps1"
    Set-Content -Path $sshStartCmd -Value "Start-Service sshd"

    & schtasks.exe /CREATE /F /SC ONSTART /RU SYSTEM /RL HIGHEST /TN "SSH start" /TR "powershell.exe -ExecutionPolicy Bypass -File $sshStartCmd"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to add scheduled task $sshStartCmd"
    }
}

function SetCustomAttributes()
{
    if ($customAttrs -ne "") {
        Write-Log "Setting custom attributes to $customAttrs"
        New-Item -ItemType "Directory" -Path "C:\var\lib\dcos" -Force
        $val = "`$env:MESOS_ATTRIBUTES=`"$customAttrs`""
        Set-Content -Path "C:\var\lib\dcos\mesos-slave-common.ps1" -Value $val -Encoding Ascii
    }
}

function ConfirmServices {
    $role = "ROLENAME" -replace '_','-'
    $dcosServices = [ordered]@{}
    $dcosServices.Add("dcos.target", "Stopped")
    $dcosServices.Add("dcos-adminrouter-agent.service", "Running")
    $dcosServices.Add("dcos-diagnostics.service", "Running")
    $dcosServices.Add("dcos-mesos-$role.service", "Running")
    $dcosServices.Add("dcos-metrics-agent.service", "Running")
    $dcosServices.Add("dcos-net.service", "Running")
    $dcosServices.Add("dcos-net-watchdog.service", "Running")

    $timeout = New-TimeSpan -Minutes 20
    $sw = [diagnostics.stopwatch]::StartNew()
    while ($sw.elapsed -lt $timeout) {
        $cnt = 0
        foreach($serviceName in $dcosServices.keys) {
            if (Get-Service $serviceName -ErrorAction SilentlyContinue) {
                $desiredStatus = $dcosServices.$serviceName
                $actualStatus = (Get-Service $serviceName).Status
                if ($actualStatus -eq $desiredStatus) {
                    Write-Log "Service $serviceName is $actualStatus (as expected)"
                    $cnt++
                } else {
                    Write-Log "Service $serviceName is $actualStatus (expected $desiredStatus). Waiting ..."
                    break
                }
            } else {
                Write-Log "Service $serviceName is not listed. Waiting ..."
                break
            }
        }
        if ($cnt -eq $dcosServices.Count) {
            Write-Log "All services have the expected status"
            return
        }
        Start-Sleep -Seconds 15
    }
    Throw "Not all required DCOS services are available or have the expected status"
}

try {
    Write-Log "Setting up Windows Agent node. BootstrapIP:$BootstrapIP"
    Write-Log "Admin user is $adminUser"
    Write-Log "Custom node attributes are $customAttrs"
    Write-Log "User Domain is $env:computername"

    Write-Log "Run preprovision extension (if present)"

    PREPROVISION_EXTENSION

    UpdateDocker

    InstallOpenSSH

    # First up, download the runasxbox util
    RetryCurl "https://dcos-mirror.azureedge.net/winbootstrap/RunAsXbox.exe" "c:\AzureData\runasxbox.exe"

    # Create the setcreds script
    $setcred_content = @'
     # usage: setcreds.ps1 -adminUser domain\user -password password
    [CmdletBinding(DefaultParameterSetName="Standard")]
       param(
            [string]
            [ValidateNotNullOrEmpty()]
            $user,
            [string]
            [ValidateNotNullOrEmpty()]
            $password,
            [string]
            [ValidateNotNullOrEmpty()]
            $domain
       )

    Install-Module CredentialManager -force

    & net user $user $password /add /yes
    & net localgroup administrators $user /add
    # & cmdkey /generic:dcos/app /user:$domain\$user /pass:$password

    New-StoredCredential -Target dcos/app -Username "$domain\$user" -Password $password -Type GENERIC -Persist LocalMachine

'@
    $setcred_content | out-file -encoding ascii c:\AzureData\setcreds.ps1
    # prime the credential cache
    #Set-PSDebug -trace 1
    #get-wmiobject -class Win32_UserAccount

    # Add all the known dcos users (2do)

    $password = "ADMIN_PASSWORD"

    & net user $adminUser $password /add /yes
    & net localgroup administrators $adminUser /add
    c:\AzureData\setcreds.ps1 -User $adminUser -Password $password -Domain $env:computername

    # Set Custom node attributes (if present)
    SetCustomAttributes

    $dcosInstallUrl = "http://${BootstrapIP}:8086/dcos_install.ps1"
    RetryCurl $dcosInstallUrl "$global:BootstrapInstallDir\dcos_install.ps1"

    $cmd = @'
powershell -command c:\AzureData\dcos_install.ps1 ROLENAME
'@

    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\$adminUser", "Machine")
    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Machine")

    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\$adminUser", "Process")
    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Process")

    $runasargs = "/fix /type:4 /user:$env:computername\$adminUser /password:$password /command:'$cmd'"
    Invoke-Expression -command ("c:\AzureData\runasxbox.exe "+$runasargs)

    if ($LASTEXITCODE -ne 0) {
        throw "Failed run DC/OS install script"
    }

    # Confirm DCOS Services
    ConfirmServices

    POSTPROVISION_EXTENSION

} catch {
    Write-Log "Failed to provision Windows agent node: $_"
    #Set-PSDebug -Off
    exit 1
}

#Set-PSDebug -Off
Write-Log "Successfully provisioned Windows agent node"
exit 0
