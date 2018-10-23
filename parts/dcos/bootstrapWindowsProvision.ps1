<#
    .SYNOPSIS
        Provisions VM as a Windows bootstrap node.

    .DESCRIPTION
        Provisions VM as a Windows bootstrap node. This script is invoked
        by the Azure Windows VMs user data script.
#>

[CmdletBinding(DefaultParameterSetName="Standard")]
param(
    [string]
    [ValidateNotNullOrEmpty()]
    $BootstrapURL,
    [Int64]
    $SystemPartitionSize
)

$ErrorActionPreference = "Stop"

$global:AzureDataDir = Join-Path $env:SystemDrive "AzureData"
$global:BootstrapDir = Join-Path $env:SystemDrive "temp"
$global:BootstrapDockerDir = Join-Path $env:SystemDrive "docker"
$global:BootstrapDockerStartScript = @"
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Starting bootstrap node Nginx Docker container")
docker.exe run --rm -d --network customnat -p 8086:80 -v $(${global:BootstrapDir} -replace '\\', '/')/genconf/serve/:c:/nginx/html:ro nginx:1803
if(`$LASTEXITCODE -ne 0) {
    Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Failed to run bootstrap node Nginx Docker image")
    Stop-Transcript
    exit 1
}
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Successfully started bootstrap node Nginx Docker container")
Stop-Transcript
exit 0
"@
$global:IpDetectScript = @'
$headers = @{"Metadata" = "true"}
$r = Invoke-WebRequest -headers $headers "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-04-02&format=text" -UseBasicParsing
$r.Content
'@


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

function Add-ToSystemPath {
    Param(
        [Parameter(Mandatory=$true)]
        [string[]]$Path
    )
    $systemPath = [System.Environment]::GetEnvironmentVariable('Path', 'Machine').Split(';')
    $currentPath = $env:PATH.Split(';')
    foreach($p in $Path) {
        if($p -notin $systemPath) {
            $systemPath += $p
        }
        if($p -notin $currentPath) {
            $currentPath += $p
        }
    }
    $env:PATH = $currentPath -join ';'
    setx.exe /M PATH ($systemPath -join ';')
    if($LASTEXITCODE) {
        Throw "Failed to set the new system path"
    }
}

function Install-7Zip {
    Write-Log "Installing 7-Zip"
    $installerPath = Join-Path $global:AzureDataDir "7z1801-x64.msi"
    Start-FileDownload -URL "https://dcos-mirror.azureedge.net/winbootstrap/7z1801-x64.msi" -Destination $installerPath
    $parameters = @{
        'FilePath' = 'msiexec.exe'
        'ArgumentList' = @("/i", $installerPath, "/qn")
        'Wait' = $true
        'PassThru' = $true
    }
    $p = Start-Process @parameters
    if($p.ExitCode -ne 0) {
        Throw "Failed to install $installerPath"
    }
    $7ZipDir = Join-Path $env:ProgramFiles "7-Zip"
    Add-ToSystemPath $7ZipDir
    Remove-Item $installerPath -ErrorAction SilentlyContinue
}

function Install-OpenSSH {
    Write-Log "Installing OpenSSH"
    $sshdService = Get-Service -Name "sshd" -ErrorAction SilentlyContinue
    if(!$sshdService) {
        $list = (Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*')
        Add-WindowsCapability -Online -Name $list.Name
        Install-PackageProvider -Name "NuGet" -Force
        Install-Module "OpenSSHUtils" -Confirm:$false -Force
    }
    Start-Service "sshd"

    Write-Log "Creating authorized key"
    # Create authorized_keys
    $publicKeysFile = Join-Path $global:AzureDataDir "authorized_keys"
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

function Set-SystemPartitionSize {
    $systemPartition = Get-Partition | Where-Object { $_.DriveLetter -eq $env:SystemDrive[0] }
    if(!$SystemPartitionSize) {
        # Use maximum size if the parameter $SystemPartitionSize is not specified
        $systemDisk = Get-Disk -Number $systemPartition.DiskNumber
        $supportedSize = Get-PartitionSupportedSize -PartitionNumber $systemPartition.PartitionNumber `
                                                    -DiskNumber $systemDisk.DiskNumber
        $SystemPartitionSize = $supportedSize.SizeMax
    }
    if($systemPartition.Size -eq $SystemPartitionSize) {
        Write-Log "The system partition has already the requested size of $($SystemPartitionSize / 1GB) GB"
        return
    }
    $systemPartition | Resize-Partition -Size $SystemPartitionSize
}

function New-BootstrapNodeFiles {
    New-Item -ItemType "Directory" -Force -Path "${global:BootstrapDir}\genconf"
    Set-Content -Path "${global:BootstrapDir}\genconf\config.yaml" -Value "BOOTSTRAP_WIN_CONFIG"
    Set-Content -Path "${global:BootstrapDir}\genconf\ip-detect.ps1" -Value $global:IpDetectScript
    Start-FileDownload -URL $BootstrapURL -Destination "${global:BootstrapDir}\dcos_generate_config.windows.tar.xz"
    Push-Location $global:BootstrapDir
    cmd.exe /C "7z.exe e .\dcos_generate_config.windows.tar.xz -so | 7z.exe x -si -ttar"
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to untar dcos_generate_config.windows.tar.xz"
    }
    & .\dcos_generate_config.ps1
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to run dcos_generate_config.ps1"
    }
    Pop-Location
}

function New-BootstrapNodeDockerContainer {
    New-Item -ItemType "Directory" -Force -Path $global:BootstrapDockerDir
    Start-FileDownload -URL "https://dcos-mirror.azureedge.net/winbootstrap/dockerfile" -Destination "${global:BootstrapDockerDir}\dockerfile"
    Start-FileDownload -URL "https://dcos-mirror.azureedge.net/winbootstrap/nginx.conf" -Destination "${global:BootstrapDockerDir}\nginx.conf"
    $networkName = "customnat"
    $network = $(docker.exe network ls --quiet --filter name=$networkName)
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to list Docker networks"
    }
    if(!$network) {
        docker.exe network create --driver="nat" --opt "com.docker.network.windowsshim.disable_gatewaydns=true" $networkName
        if ($LASTEXITCODE -ne 0) {
            Throw "Failed to create $networkName Docker network"
        }
    }
    Push-Location $global:BootstrapDockerDir
    docker.exe build --network $networkName -t nginx:1803 $global:BootstrapDockerDir
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to build bootstrap node Docker image"
    }
    $script = Join-Path $global:BootstrapDockerDir "StartDocker.ps1"
    $log = Join-Path $global:BootstrapDockerDir "StartDocker.log"
    Add-Content -Path $script -Value "Start-Transcript -Path $log -Append"
    Add-Content -Path $script -Value $global:BootstrapDockerStartScript
    schtasks.exe /CREATE /F /SC ONSTART /RU SYSTEM /RL HIGHEST /TN "Docker start" /TR "powershell.exe -ExecutionPolicy Bypass -File $script"
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to add scheduled task to start bootstrap node Docker container"
    }
    powershell.exe -ExecutionPolicy Bypass -File $script
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to start bootstrap node Docker container"
    }
    Pop-Location
}


try {
    Write-Log "Setting up Windows bootstrap node. BootstrapURL:$BootstrapURL"
    Set-SystemPartitionSize
    Install-OpenSSH
    Install-7Zip
    New-BootstrapNodeFiles
    New-BootstrapNodeDockerContainer
} catch {
    Write-Output $_.ScriptStackTrace
    Write-Log "Failed to provision Windows bootstrap node: $_"
    exit 1
}
Write-Log "Successfully provisioned Windows bootstrap node"
exit 0
