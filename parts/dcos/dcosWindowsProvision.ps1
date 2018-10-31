<#
    .SYNOPSIS
        Provisions VM as a Windows bootstrap node.

    .DESCRIPTION
        Provisions VM as a Windows bootstrap node. This script is invoked
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
$global:SetCredsScript = @'
[CmdletBinding(DefaultParameterSetName="Standard")]
Param(
    [ValidateNotNullOrEmpty()]
    [string]$user,
    [ValidateNotNullOrEmpty()]
    [string]$password,
    [ValidateNotNullOrEmpty()]
    [string]$domain
)
Install-Module CredentialManager -Force
New-StoredCredential -Target dcos/app -Username "$domain\$user" -Password $password -Type GENERIC -Persist LocalMachine
'@


filter Timestamp { "[$(Get-Date -Format o)] $_" }

function Write-Log {
    Param(
        [string]$Message
    )
    $msg = $Message | Timestamp
    Write-Output $msg
}

function Get-AccountObjectByName {
    <#
    .SYNOPSIS
    Returns a CimInstance or a ManagementObject containing the Win32_Account representation of the requested username.
    .PARAMETER Username
    User name to lookup.
    #>
    [CmdletBinding()]
    Param(
        [parameter(Mandatory=$true)]
        [string]$Username
    )
    PROCESS {
        $u = Get-CimInstance -Class "Win32_Account" -Filter ("Name='{0}'" -f $Username)
        if (!$u) {
            Throw [System.Management.Automation.ItemNotFoundException] "User not found: $Username"
        }
        return $u
    }
}

function Get-GroupObjectBySID {
    <#
    .SYNOPSIS
    This will return a win32_group object. If running on a system with powershell >= 4, this will be a CimInstance.
    Systems running powershell <= 3 will return a ManagementObject.
    .PARAMETER SID
    The SID of the user we want to find
    .PARAMETER Exact
    This is $true by default. If set to $false, the query will use the 'LIKE' operator instead of '='.
    .NOTES
    If $Exact is $false, multiple win32_account objects may be returned.
    #>
    [CmdletBinding()]
    Param(
        [Parameter(Mandatory=$true)]
        [string]$SID,
        [Parameter(Mandatory=$false)]
        [switch]$Exact=$true
    )
    PROCESS {
        $modifier = " LIKE "
        if ($Exact){
            $modifier = "="
        }
        $query = ("SID{0}'{1}'" -f @($modifier, $SID))
        $s = Get-CimInstance -Class Win32_Group -Filter $query
        if(!$s){
            Throw "SID not found: $SID"
        }
        return $s
    }
}

function Get-GroupObjectByName {
    <#
    .SYNOPSIS
    Returns a CimInstance or a ManagementObject containing the Win32_Group representation of the requested group name.
    .PARAMETER GroupName
    Group name to lookup.
    #>
    [CmdletBinding()]
    Param(
        [parameter(Mandatory=$true)]
        [string]$GroupName
    )
    PROCESS {
        $g = Get-CimInstance -Class "Win32_Group" -Filter ("Name='{0}'" -f $GroupName)
        if (!$g) {
            Throw "Group not found: $GroupName"
        }
        return $g
    }
}

function Get-GroupNameFromSID {
    <#
    .SYNOPSIS
    This function exists for compatibility. Please use Get-GroupObjectBySID.
    .PARAMETER SID
    The SID of the group we want to find
    .PARAMETER Exact
    This is $true by default. If set to $false, the query will use the 'LIKE' operator instead of '='.
    .NOTES
    If $Exact is $false, multiple win32_group objects may be returned.
    #>
    [CmdletBinding()]
    Param(
        [Parameter(Mandatory=$true)]
        [string]$SID,
        [Parameter(Mandatory=$false)]
        [switch]$Exact=$true
    )
    PROCESS {
        return (Get-GroupObjectBySID -SID $SID -Exact:$Exact).Name
    }
}

function Add-WindowsUser {
    <#
    .SYNOPSIS
    Creates a new local Windows account.
    .PARAMETER Username
    The user name of the new user
    .PARAMETER Password
    The password the user will authenticate with
    .PARAMETER Fullname
    The user full name. Applies only when user doesn't exist already.
    .PARAMETER Description
    The description for the user. Applies only when user doesn't exist already.
    .NOTES
    This commandlet creates a local user that never expires, and which is not required to reset the password on first logon.
    #>
    [CmdletBinding()]
    Param(
        [parameter(Mandatory=$true)]
        [string]$Username,
        [parameter(Mandatory=$true)]
        [string]$Password,
        [parameter(Mandatory=$false)]
        [string]$Fullname,
        [parameter(Mandatory=$false)]
        [String]$Description
    )
    PROCESS {
        try {
            $exists = Get-AccountObjectByName $Username
        } catch [System.Management.Automation.ItemNotFoundException] {
            $exists = $false
        }
        if($exists) {
            Write-Output "Username $Username already exists"
            return
        }
        $params = @("user", $Username)
        $params += @($Password, "/add", "/expires:never", "/active:yes", "/Y")
        if($Fullname) {
            $params += "/fullname:{0}" -f @($Fullname)
        }
        if($Description) {
            $params += "/comment:{0}" -f @($Description)
        }
        $p = Start-Process -FilePath "net.exe" -ArgumentList $params -NoNewWindow -Wait -PassThru
        if($p.ExitCode -ne 0) {
            Throw "Failed to set the Windows user $Username"
        }
    }
}

function Confirm-IsMemberOfGroup {
    [CmdletBinding()]
    Param(
        [Parameter(Mandatory=$true)]
        [string]$GroupSID,
        [Parameter(Mandatory=$true)]
        [string]$Username
    )
    PROCESS {
        $name = Get-GroupNameFromSID -SID $GroupSID
        return Get-LocalUserGroupMembership -Group $name -Username $Username
    }
}

function Get-LocalUserGroupMembership {
    [CmdletBinding()]
    Param(
        [Parameter(Mandatory=$true)]
        [string]$Group,
        [Parameter(Mandatory=$true)]
        [string]$Username
    )
    PROCESS {
        $ret = net.exe localgroup $Group
        if($LASTEXITCODE) {
            Throw "Failed to run: net.exe localgroup $Group"
        }
        $members =  $ret | where {$_ -AND $_ -notmatch "command completed successfully"} | select -skip 4
        foreach ($i in $members){
            if ($Username -eq $i){
                return $true
            }
        }
        return $false
    }
}

function Add-UserToLocalGroup {
    <#
    .SYNOPSIS
    Add a user to a localgroup
    .PARAMETER Username
    The username to add
    .PARAMETER GroupSID
    The SID of the group to add the user to
    .PARAMETER GroupName
    The name of the group to add the user to
    .NOTES
    GroupSID and GroupName are mutually exclusive
    #>
    [CmdletBinding()]
    Param(
        [Parameter(Mandatory=$true)]
        [string]$Username,
        [Parameter(Mandatory=$false)]
        [string]$GroupSID,
        [Parameter(Mandatory=$false)]
        [string]$GroupName
    )
    PROCESS {
        if(!$GroupSID -and !$GroupName) {
            Throw "Neither GroupSID, nor GroupName have been specified"
        }
        if($GroupName -and $GroupSID){
            Throw "The -GroupName and -GroupSID options are mutually exclusive"
        }
        if($GroupSID){
            $GroupName = Get-GroupNameFromSID $GroupSID
        }
        if($GroupName) {
            $GroupSID = (Get-GroupObjectByName $GroupName).SID
        }
        $isInGroup = Confirm-IsMemberOfGroup -User $Username -Group $GroupSID
        if($isInGroup){
            return
        }
        $params = @("net.exe", "localgroup", $GroupName, $Username, "/add")
        $p = Start-Process -FilePath "net.exe" -ArgumentList $params -NoNewWindow -Wait -PassThru
        if($p.ExitCode -ne 0) {
            Throw "Failed to add the Windows user $Username to the group $GroupName"
        }
    }
}

function New-LocalAdmin {
    <#
    .SYNOPSIS
    Create a local user account and add it to the local Administrators group. This works with internationalized versions of Windows as well.
    .PARAMETER Username
    The user name of the new user
    .PARAMETER Password
    The password the user will authenticate with
    .NOTES
    This commandlet creates an administrator user that never expires, and which is not required to reset the password on first logon.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true)]
        [Alias("LocalAdminUsername")]
        [string]$Username,
        [Parameter(Mandatory=$true)]
        [Alias("LocalAdminPassword")]
        [string]$Password
    )
    PROCESS {
        Add-WindowsUser $Username $Password | Out-Null
        $administratorsGroupSID = "S-1-5-32-544"
        Add-UserToLocalGroup -Username $Username -GroupSID $administratorsGroupSID
    }
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
        Install-Module DockerMsftProvider -Force
        Install-Package -Name docker -ProviderName DockerMsftProvider -Force -RequiredVersion WINDOWS_DOCKER_VERSION
    }
    Write-Log "Starting Docker"
    Start-Service Docker
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
    $password = "ADMIN_PASSWORD"
    New-LocalAdmin -Username $adminUser -Password $password
    $setCredsScriptFile = Join-Path $global:BootstrapInstallDir "setcreds.ps1"
    Set-Content -Path $setCredsScriptFile -Value $global:SetCredsScript -Encoding ascii
    & "$setCredsScriptFile" -User $adminUser -Password $password -Domain $env:computername
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to execute: $setCredsScript"
    }

    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\$adminUser", "Machine")
    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Machine")
    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "$env:computername\$adminUser", "Process")
    [Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", $password, "Process")

    $runasxboxFile = Join-Path $global:BootstrapInstallDir "runasxbox.exe"
    Start-FileDownload -URL "https://dcos-mirror.azureedge.net/winbootstrap/RunAsXbox.exe" -Destination $runasxboxFile
    Start-FileDownload -URL "http://${BootstrapIP}:8086/dcos_install.ps1" -Destination "${global:BootstrapInstallDir}\dcos_install.ps1"

    $cmd = "powershell -command ${global:BootstrapInstallDir}\dcos_install.ps1 ROLENAME"
    $runasargs = "/fix /type:4 /user:$env:computername\$adminUser /password:$password /command:'$cmd'"
    Invoke-Expression -Command "$runasxboxFile $runasargs"
    if ($LASTEXITCODE -ne 0) {
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
