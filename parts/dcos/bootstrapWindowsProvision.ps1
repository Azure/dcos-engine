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

function CreateDockerStart($fileName, $log)
{
    $content = "Start-Transcript -path $log -append"
    Set-Content -Path $fileName -Value $content
    $content = '
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Starting docker container")
& docker.exe run --rm -d --network customnat -p 8086:80 -v C:/temp/genconf/serve/:c:/nginx/html:ro nginx:1803
if ($LASTEXITCODE -ne 0) {
    Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Failed to run docker image")
    Stop-Transcript
    Exit 1
}
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Successfully started docker container")
Stop-Transcript
Exit 0
'
    Add-Content -Path $fileName -Value $content

    & schtasks.exe /CREATE /F /SC ONSTART /RU SYSTEM /RL HIGHEST /TN "Docker start" /TR "powershell.exe -ExecutionPolicy Bypass -File $filename"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to add scheduled task $fileName"
    }
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

function Install7zip
{
    # install 7zip in order to unpack the bootstrap node
    Write-Log "Installing 7zip"
    New-Item -itemtype directory -erroraction silentlycontinue "C:\AzureData\7z"

    RetryCurl "https://dcos-mirror.azureedge.net/winbootstrap/7z1801-x64.msi" "C:\AzureData\7z\7z1801-x64.msi"

    & cmd.exe /c start /wait msiexec /i C:\AzureData\7z\7z1801-x64.msi INSTALLDIR="C:\AzureData\7z" /qn
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to install 7zip"
    }
    Remove-Item C:\AzureData\7z\7z1801-x64.msi
}

function InstallOpenSSH()
{
    Write-Log "Installing OpenSSH"
    try {
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
    catch {
       Write-Log "OpenSSH install failed: $_"
    }
}

try {
    Write-Log "Setting up Windows bootstrap node. BootstrapURL:$BootstrapURL"

    # Resize C: partition to 60 GB
    $newSize = 64420315136
    $systempart = (Get-Partition | where { $_.driveletter -eq "C" })
    $systempart | Resize-Partition -size $newSize

    InstallOpenSSH

    Install7zip

    New-Item -itemtype directory -erroraction silentlycontinue c:\temp
    cd c:\temp
    New-Item -itemtype directory -erroraction silentlycontinue c:\temp\genconf

    CreateDcosConfig "c:\temp\genconf\config.yaml"

    CreateIpDetect "c:\temp\genconf\ip-detect.ps1"

    RetryCurl $BootstrapURL "c:\temp\dcos_generate_config.windows.tar.xz"

    & cmd /c "c:\AzureData\7z\7z.exe e .\dcos_generate_config.windows.tar.xz -so | c:\AzureData\7z\7z.exe x -si -ttar"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to untar dcos_generate_config.windows.tar.xz"
    }

    & .\dcos_generate_config.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to run dcos_generate_config.ps1"
    }

    # Run docker container with nginx
    New-Item -itemtype directory -erroraction silentlycontinue c:\docker
    cd c:\docker
    RetryCurl "https://dcos-mirror.azureedge.net/winbootstrap/dockerfile" "c:\docker\dockerfile"
    RetryCurl "https://dcos-mirror.azureedge.net/winbootstrap/nginx.conf" "c:\docker\nginx.conf"

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

    CreateDockerStart "c:\docker\StartDocker.ps1" "c:\docker\StartDocker.log"
} catch {
    Write-Log "Failed to provision Windows bootstrap node: $_"
    exit 1
}

Write-Log "Successfully provisioned Windows bootstrap node"
exit 0
