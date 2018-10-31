$ErrorActionPreference = "Stop"

$global:DockerDir      = Join-Path $env:SystemDrive "docker"
$global:upgradeDir     = Join-Path $env:SystemDrive "AzureData\upgrade\NEW_VERSION"
$global:genconfDir     = Join-Path $global:upgradeDir "genconf"
$global:upgradeUrlPath = Join-Path $global:upgradeDir "upgrade_url"
$global:volume         = "$(${global:genconfDir} -replace '\\', '/')/serve/:c:/nginx/html:ro"
$global:networkName    = "customnat"

$global:DockerStartScript = @"
Start-Transcript -path "${global:DockerDir}\StartDocker.log" -append
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Starting docker container")
& docker.exe run --rm -d --network $global:networkName -p 8086:80 -v $global:volume nginx:1803
if ($LASTEXITCODE -ne 0) {
    Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Failed to run docker image")
    Stop-Transcript
    Exit 1
}
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Successfully started docker container")
Stop-Transcript
Exit 0
"@


filter Timestamp {"[$(Get-Date -Format o)] $_"}

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


function New-NginxDockerContainer {
    # Stop docker container
    $process = docker ps -q
    if ($LASTEXITCODE -ne 0) {
            throw "Failed to run docker ps command"
    }
    if ($process) {
            Write-Log "Stopping nginx service $process"
            & docker.exe kill $process
            if ($LASTEXITCODE -ne 0) {
              throw "Failed to run docker kill $process"
            }
    }
    Write-Log "Starting nginx service"
 
    # Run docker container with nginx
 
    $network = $(docker.exe network ls --quiet --filter name=$global:networkName)
    if($LASTEXITCODE -ne 0) {
        Throw "Failed to list Docker networks"
    }
    # only create customnat if it does not exist
    if(!$network) {
        docker.exe network create --driver="nat" --opt "com.docker.network.windowsshim.disable_gatewaydns=true" $global:networkName
        if ($LASTEXITCODE -ne 0) {
             Throw "Failed to create $global:networkName Docker network"
        }
    }
 
    Push-Location $global:DockerDir
    & docker.exe build --network $global:networkName -t nginx:1803 $global:DockerDir
    if ($LASTEXITCODE -ne 0) {
            throw "Failed to build docker image"
    }
    & docker.exe run --rm -d --network $global:networkName -p 8086:80 -v $global:volume nginx:1803
    if ($LASTEXITCODE -ne 0) {
            throw "Failed to run docker image"
    }

    $script = Join-Path $global:DockerDir "StartDocker.ps1"
    Set-Content -Path $script -Value $global:DockerStartScript
}


function New-ConfigFiles {
    Write-Log "Starting upgrade configuration"
    $BootstrapURL = "WIN_BOOTSTRAP_URL"
    $logPath = Join-Path $global:upgradeDir "dcos_generate_config.log"

    Write-Log "Setting up Windows bootstrap node for upgrade"
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $global:upgradeDir
    New-Item -ItemType Directory -Force -Path $global:genconfDir
    $path = Join-Path $global:genconfDir "config.yaml"
    Copy-Item "C:\AzureData\config-win.NEW_VERSION.yaml" $path
    Copy-Item "c:\temp\genconf\ip-detect.ps1" $global:genconfDir
    Push-Location $global:upgradeDir

    $path = Join-Path $global:upgradeDir "dcos_generate_config.windows.tar.xz"
    Start-FileDownload -URL $BootstrapURL -Destination $path

    $7z_exe = Join-Path $env:ProgramFiles "7-Zip\7z.exe"
    $7z_cmd = "`"$7z_exe`" e .\dcos_generate_config.windows.tar.xz -so | `"$7z_exe`" x -si -ttar"
    cmd.exe /c "$7z_cmd"
    if ($LASTEXITCODE -ne 0) {
            throw "Failed to untar dcos_generate_config.windows.tar.xz"
    }
 
    & .\dcos_generate_config.ps1 --generate-node-upgrade-script CURR_VERSION > $logPath
    if ($LASTEXITCODE -ne 0) {
           throw "Failed to run dcos_generate_config.ps1"
    }
 
    # Fetch upgrade script URL
    $match = Select-String -Path $logPath -Pattern "Node upgrade script URL:" -CaseSensitive
    if (-Not $match) {
            throw "Missing Node upgrade script URL in $logPath"
    }
    $upgradeUrl = ($match.Line -replace 'Node upgrade script URL:','').Trim()
    if (-Not $upgradeUrl) {
             throw "Bad Node upgrade script URL in $logPath"
    }

    Set-Content -Path $global:upgradeUrlPath -Value $upgradeUrl -Encoding Ascii
    $upgradeUrl = Get-Content -Path $global:upgradeUrlPath -Encoding Ascii
    if (-Not $upgradeUrl) {
            Remove-Item $global:upgradeUrlPath -Force
            throw "Failed to set up bootstrap node. Please try again"
    } else {
            # keep Write-Output - used in parsing
            Write-Output "Setting up bootstrap node completed. Node upgrade script URL $upgradeUrl"
    }
}

try {
    New-ConfigFiles
    New-NginxDockerContainer
    Write-Log "Setting up bootstrap node completed"
    exit 0
} catch {
    Write-Log "Failed to upgrade Windows bootstrap node: $_"
    exit 1
}
