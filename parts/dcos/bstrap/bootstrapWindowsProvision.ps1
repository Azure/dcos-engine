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
    $BootstrapURL,
    [string]
    [ValidateNotNullOrEmpty()]
    $BootstrapIP
)

filter Timestamp {"[$(Get-Date -Format o)] $_"}

function Write-Log($message)
{
    $msg = $message | Timestamp
    Write-Output $msg
}

function CreateDcosConfig($fileName)
{
    $config = "bootstrap_url: http://${BootstrapIP}:8086
cluster_name: azure-dcos
exhibitor_storage_backend: static
master_discovery: static
oauth_enabled: BOOTSTRAP_OAUTH_ENABLED
ip_detect_public_filename: genconf/ip-detect.ps1
master_list:
MASTER_IP_LIST
resolvers:
- 168.63.129.16
- 8.8.4.4
- 8.8.8.8
"

    Set-Content -Path $fileName -Value $config
}

function CreateIpDetect($fileName)
{
    $content = '$headers = @{"Metadata" = "true"}
    $r = Invoke-WebRequest -headers $headers "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-04-02&format=text" -UseBasicParsing
    $r.Content'

    Set-Content -Path $fileName -Value $content
}

try {
    Write-Log "Setting up Windows bootstrap node. BootstrapURL:$BootstrapURL BootstrapIP:$BootstrapIP"

    New-item -itemtype directory -erroraction silentlycontinue c:\temp
    cd c:\temp
    New-item -itemtype directory -erroraction silentlycontinue c:\temp\genconf

    CreateDcosConfig "c:\temp\genconf\config.yaml"

    CreateIpDetect "c:\temp\genconf\ip-detect.ps1"

    & curl.exe --keepalive-time 2 -fLsSv --retry 20 -Y 100000 -y 60 -o c:\temp\dcos_generate_config.windows.tar.xz $BootstrapURL
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download $BootstrapURL"
    }

    & tar -xvf .\dcos_generate_config.windows.tar.xz
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to untar dcos_generate_config.windows.tar.xz"
    }

    & .\install_bootstrap_windows.ps1
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to run install_bootstrap_windows.ps1"
    }

    # Run docker container with nginx
    New-item -itemtype directory -erroraction silentlycontinue c:\docker
    cd c:\docker

    & curl.exe --keepalive-time 2 -fLsSv --retry 20 -Y 100000 -y 60 -o c:\docker\dockerfile https://dcos-mirror.azureedge.net/winbootstrap/dockerfile
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to download dockerfile"
    }

    & curl.exe --keepalive-time 2 -fLsSv --retry 20 -Y 100000 -y 60 -o c:\docker\nginx.conf https://dcos-mirror.azureedge.net/winbootstrap/nginx.conf
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
