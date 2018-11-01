<#
    .SYNOPSIS
        Provisions VM as a Windows agent node.

    .DESCRIPTION
        Provisions VM as a Windows agent node. This script is invoked
        by the Azure Windows VMs user data script.
#>

[CmdletBinding(DefaultParameterSetName="Standard")]
Param(
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

$ErrorActionPreference = "Stop"


$global:BootstrapInstallDir = Join-Path $env:SystemDrive "AzureData"


filter Timestamp { "[$(Get-Date -Format o)] $_" }

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

function Update-Docker {
    #
    # Stop Docker service, disable Docker Host Networking Service
    #
    Write-Log "Stopping Docker"
    Stop-Service "Docker"

    Write-Log "Disabling Docker Host Networking Service"
    Get-HNSNetwork | Remove-HNSNetwork
    $dockerData = Join-Path $env:ProgramData "Docker"
    Set-Content -Path "$dockerData\config\daemon.json" -Value '{ "bridge" : "none" }' -Encoding Ascii
    # Upgrade and start Docker
    if ("WINDOWS_DOCKER_VERSION" -ne "current") {
        Write-Log "Updating Docker to WINDOWS_DOCKER_VERSION"
        Install-Module -Name DockerMsftProvider -Repository PSGallery -Force
        Install-Package -Name docker -ProviderName DockerMsftProvider -Force -RequiredVersion WINDOWS_DOCKER_VERSION
    }
    $dockerd = Join-Path $env:ProgramFiles "Docker\dockerd.exe"
    $dockerVersion = & $dockerd --version # This returns a string of the form: "Docker version 18.03.1-ee-3, build b9a5c95"
    Write-Log "Docker string version returned by the CLI: '$dockerVersion'"
    if($LASTEXITCODE) {
        Throw "Failed to get the Docker server version"
    }
    $version = $dockerVersion.Split()[2].Trim(',') # Parse the string get the version
    switch ($version.Substring(0,5)) {
        "17.06" {
            Write-Log "Docker 17.06 found, setting DOCKER_API_VERSION to 1.30"
            $apiVersion = "1.30"
        }
        "18.03" {
            Write-Log "Docker 18.03 found, setting DOCKER_API_VERSION to 1.37"
            $apiVersion = "1.37"
        }
        default {
            Write-Log "Docker version $version found, clearing DOCKER_API_VERSION system environment variable"
            $apiVersion = $null
        }
    }
    [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', $apiVersion, [System.EnvironmentVariableTarget]::Machine)
    $env:DOCKER_API_VERSION = $apiVersion
    Write-Log "Restarting Docker"
    Restart-Service Docker
}

function Install-OpenSSH {
    Write-Log "Installing OpenSSH"
    $sshdService = Get-Service -Name "sshd" -ErrorAction SilentlyContinue
    if(!$sshdService) {
        $fullList = Start-ExecuteWithRetry -ScriptBlock { Get-WindowsCapability -Online } -RetryMessage "Failed to get full list of features"
        $list = ($fullList | Where-Object Name -like 'OpenSSH.Server*')
        Add-WindowsCapability -Online -Name $list.Name
        Install-PackageProvider -Name "NuGet" -Force
        Install-Module "OpenSSHUtils" -Confirm:$false -Force
    }
    Start-Service "sshd"

    Write-Log "Creating authorized key"
    # Create authorized_keys
    $publicKeysFile = Join-Path $global:BootstrapInstallDir "authorized_keys"
    Set-Content -Path $publicKeysFile -Value "SSH_PUB_KEY" -Encoding Ascii
    # Point sshd daemon to this file
    $sshdConfigFile = Join-Path $env:ProgramData "ssh\sshd_config"
    $newSshdConfig = (Get-Content $sshdConfigFile) -replace "AuthorizedKeysFile(\s+).*$", "AuthorizedKeysFile $publicKeysFile"
    Set-Content -Path $sshdConfigFile -Value $newSshdConfig -Encoding ascii
    # Update the acl to this file, restricting access to it
    $acl = Get-Acl -Path $publicKeysFile
    $acl.SetAccessRuleProtection($True, $True)
    $acl | Set-Acl -Path $publicKeysFile
    $acl = Get-Acl -Path $publicKeysFile
    $rules = $acl.Access
    $usersToRemove = @("Everyone","BUILTIN\Users","NT AUTHORITY\Authenticated Users")
    foreach ($u in $usersToRemove) {
        $targetrule = $rules | where IdentityReference -eq $u
        if ($targetrule) {
            $acl.RemoveAccessRule($targetrule)
        }
    }
    $acl | Set-Acl -Path $publicKeysFile
    Restart-Service "sshd"
    Set-Service -Name "sshd" -StartupType Automatic
}

function Get-AgentPrivateIP {
    $primaryIfIndex = (Get-NetRoute -DestinationPrefix "0.0.0.0/0").ifIndex
    return (Get-NetIPAddress -AddressFamily IPv4 -ifIndex $primaryIfIndex).IPAddress
}

function Set-MesosCustomAttributes {
    if (!$customAttrs) {
        # Mesos custom attributes were not set
        return
    }
    Write-Log "Setting custom attributes to: $customAttrs"
    $dcosLibDir = Join-Path $env:SystemDrive "var\lib\dcos"
    New-Item -ItemType "Directory" -Path $dcosLibDir -Force
    $envContent = @(
        "`$env:MESOS_ATTRIBUTES=`"$customAttrs`"",
        "`$env:MESOS_IP=`"$(Get-AgentPrivateIP)`""
    )
    Set-Content -Path "${dcosLibDir}\mesos-slave-common.ps1" -Value $envContent -Encoding Ascii
}

function Confirm-DCOSServices {
    $role = "ROLENAME" -replace '_','-'
    $dcosServices = [ordered]@{}
    $dcosServices.Add("dcos.target", "Stopped")
    $dcosServices.Add("dcos-adminrouter-agent.service", "Running")
    $dcosServices.Add("dcos-diagnostics.service", "Running")
    $dcosServices.Add("dcos-mesos-$role.service", "Running")
    $dcosServices.Add("dcos-net.service", "Running")
    $dcosServices.Add("dcos-net-watchdog.service", "Running")
    $dcosServices.Add("dcos-telegraf.service", "Running")

    $timeout = New-TimeSpan -Minutes 20
    $sw = [diagnostics.stopwatch]::StartNew()
    while($sw.elapsed -lt $timeout) {
        $cnt = 0
        foreach($serviceName in $dcosServices.keys) {
            $svc = Get-Service $serviceName -ErrorAction SilentlyContinue
            if(!$svc) {
                Write-Log "Service $serviceName is not listed. Waiting ..."
                break
            }
            $desiredStatus = $dcosServices.$serviceName
            $actualStatus = $svc.Status
            if ($actualStatus -ne $desiredStatus) {
                Write-Log "Service $serviceName is $actualStatus (expected $desiredStatus). Waiting ..."
                break
            }
            Write-Log "Service $serviceName is $actualStatus (as expected)"
            $cnt++
        }
        if ($cnt -eq $dcosServices.Count) {
            Write-Log "All services have the expected status"
            return
        }
        Start-Sleep -Seconds 15
    }
    Throw "Not all required DCOS services are available or have the expected status"
}

function Start-ExtensionScript {
    Param(
        [Parameter(Mandatory=$false)]
        [string]$URL,
        [Parameter(Mandatory=$false)]
        [string]$Directory,
        [Parameter(Mandatory=$false)]
        [string]$Path
    )
    if(!$URL) {
        return
    }
    if(!$Directory -or !$Path) {
        Throw "The parameters -Directory and -Path must be specified to run the extension script from $URL"
    }
    New-Item -ItemType "Directory" -Force -Path $Directory
    Start-FileDownload -URL $URL -Destination $Path
    powershell.exe -File $Path
    if($LASTEXITCODE) {
        Throw "Failed to run the PowerShell extension script from $URL"
    }
}

function Start-DCOSSetup {
    Start-FileDownload -URL "http://${BootstrapIP}:8086/dcos_install.ps1" -Destination "${global:BootstrapInstallDir}\dcos_install.ps1"
    & "${global:BootstrapInstallDir}\dcos_install.ps1" ROLENAME
    if($LASTEXITCODE -ne 0) {
        throw "Failed run DC/OS install script"
    }
}

try {
    Write-Log "Setting up Windows Agent node"
    Write-Log "BootstrapIP:$BootstrapIP"
    Write-Log "Admin user: $adminUser"
    Write-Log "Custom node attributes: $customAttrs"
    Write-Log "User Domain: $env:computername"

    Write-Log "Run pre-provision extension (if present)"
    Start-ExtensionScript PREPROVISION_EXTENSION_URL PREPROVISION_EXTENSION_DIR PREPROVISION_EXTENSION_PATH

    Update-Docker
    Install-OpenSSH
    Write-Log "Set Custom node attributes (if present)"
    Set-MesosCustomAttributes
    Start-DCOSSetup
    Confirm-DCOSServices

    Write-Log "Run post-provision extension (if present)"
    Start-ExtensionScript POSTPROVISION_EXTENSION_URL POSTPROVISION_EXTENSION_DIR POSTPROVISION_EXTENSION_PATH
} catch {
    Write-Output $_.ScriptStackTrace
    Write-Log "Failed to provision Windows agent node: $_"
    exit 1
}
Write-Log "Successfully provisioned Windows agent node"
exit 0
