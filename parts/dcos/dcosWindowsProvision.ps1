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
    $adminUser
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

function InstallOpehSSH()
{
    Write-Log "Installing OpehSSH"
    $list = (Get-WindowsCapability -Online | ? Name -like 'OpenSSH.Server*')
    Add-WindowsCapability -Online -Name $list.Name
    Install-Module -Force OpenSSHUtils
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

try {
    Write-Log "Setting up Windows Agent node. BootstrapIP:$BootstrapIP"
    Write-Log "Admin user is $adminUser"
    Write-Log "User Domain is $env:computername"

    Write-Log "Run preprovision extension (if present)"

    PREPROVISION_EXTENSION

    InstallOpehSSH

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
    Set-PSDebug -trace 1
    get-wmiobject -class Win32_UserAccount

    # Add all the known dcos users (2do)

    $password = "ADMIN_PASSWORD"

  #  $adminUser = "dcos-service"   # Overwriting the arg
    & net user $adminUser $password /add /yes
    & net localgroup administrators $adminUser /add
    c:\AzureData\setcreds.ps1 -User $adminUser -Password $password -Domain $env:computername

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
} catch {
    Write-Log "Failed to provision Windows agent node: $_"
    Set-PSDebug -Off
    exit 1
}

Set-PSDebug -Off
Write-Log "Successfully provisioned Windows agent node"
exit 0
