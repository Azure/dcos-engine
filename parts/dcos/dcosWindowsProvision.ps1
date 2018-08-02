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
}

try {
    Write-Log "Setting up Windows Agent node. BootstrapIP:$BootstrapIP"
    Write-Log "Admin user is $adminUser"
    Write-Log "User Domain is $env:computername"

    Write-Log "Run preprovision extension (if present)"

    PREPROVISION_EXTENSION

    InstallOpehSSH

    # First up, download the runasxbox util
    curl.exe -fLsS -o c:\AzureData\runasxbox.exe https://dcosdevstorage.blob.core.windows.net/tmp/RunAsXbox.exe
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download windows runasxbox.exe"
    }

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
    get-wmiobject -class Win32_UserAccount

    # Add all the known dcos users (2do)

    $password = "ADMIN_PASSWORD"

  #  $adminUser = "dcos-service"   # Overwriting the arg
    & net user $adminUser $password /add /yes
    & net localgroup administrators $adminUser /add
    c:\AzureData\setcreds.ps1 -User $adminUser -Password $password -Domain $env:computername

    $dcosInstallUrl = "http://${BootstrapIP}:8086/dcos_install.ps1"
    & curl.exe $dcosInstallUrl -o $global:BootstrapInstallDir\dcos_install.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download $dcosInstallUrl"
    }

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
    exit 1
}

Write-Log "Successfully provisioned Windows agent node"
exit 0
