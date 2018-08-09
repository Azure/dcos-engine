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
    $BootstrapURL
)

filter Timestamp {"[$(Get-Date -Format o)] $_"}

function Write-Log($message)
{
    $msg = $message | Timestamp
    Write-Output $msg
}

function CreateDcosConfig($fileName)
{
    $config = "BOOTSTRAP_WIN_CONFIG"

    Set-Content -Path $fileName -Value $config
}

function CreateIpDetect($fileName)
{
    $content = '$headers = @{"Metadata" = "true"}
    $r = Invoke-WebRequest -headers $headers "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-04-02&format=text" -UseBasicParsing
    $r.Content'

    Set-Content -Path $fileName -Value $content
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
    Write-Log "Setting up Windows bootstrap node. BootstrapURL:$BootstrapURL"

    # Resize C: partition to 60 GB
    $newSize = 64420315136
    $systempart = (Get-Partition | where { $_.driveletter -eq "C" })
    $systempart | Resize-Partition -size $newSize

    InstallOpehSSH

    New-item -itemtype directory -erroraction silentlycontinue c:\temp
    cd c:\temp
    New-item -itemtype directory -erroraction silentlycontinue c:\temp\genconf

    CreateDcosConfig "c:\temp\genconf\config.yaml"

    CreateIpDetect "c:\temp\genconf\ip-detect.ps1"

    & curl.exe --keepalive-time 2 -fLsS --retry 20 -Y 100000 -y 60 -o c:\temp\dcos_generate_config.windows.tar.xz $BootstrapURL
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download $BootstrapURL"
    }

    & tar -xvf .\dcos_generate_config.windows.tar.xz
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to untar dcos_generate_config.windows.tar.xz"
    }

    & .\dcos_generate_config.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to run dcos_generate_config.ps1"
    }

    # Run docker container with nginx
    New-item -itemtype directory -erroraction silentlycontinue c:\docker
    cd c:\docker

    & curl.exe --keepalive-time 2 -fLsS --retry 20 -Y 100000 -y 60 -o c:\docker\dockerfile https://dcos-mirror.azureedge.net/winbootstrap/dockerfile
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download dockerfile"
    }

    & curl.exe --keepalive-time 2 -fLsS --retry 20 -Y 100000 -y 60 -o c:\docker\nginx.conf https://dcos-mirror.azureedge.net/winbootstrap/nginx.conf
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download nginx.conf"
    }

    # only create customnat if it does not exist
    $a = docker network ls | select-string -pattern "customnat"
    if ($a.count -eq 0)
    {
        & docker.exe network create --driver="nat" --opt "com.docker.network.windowsshim.disable_gatewaydns=true" "customnat"
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to create customnat docker network"
        }
    }

    & docker.exe build --network customnat -t nginx:1803 c:\docker
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to build docker image"
    }

    & docker.exe run --rm -d --network customnat -p 8086:80 -v C:/temp/genconf/serve/:c:/nginx/html:ro nginx:1803
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to run docker image"
    }
} catch {
    Write-Log "Failed to provision Windows bootstrap node: $_"
    exit 1
}

Write-Log "Successfully provisioned Windows bootstrap node"
exit 0
