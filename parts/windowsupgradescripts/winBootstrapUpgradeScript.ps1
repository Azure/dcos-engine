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

function CreateDockerStart($fileName, $log, $volume)
{
    $content = "Start-Transcript -path $log -append"
    Set-Content -Path $fileName -Value $content
        $content = 'Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Starting docker container")'
        Add-Content -Path $fileName -Value $content
        $content = "& docker.exe run --rm -d --network customnat -p 8086:80 -v $volume nginx:1803"
        Add-Content -Path $fileName -Value $content
        $content = '
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
}

try {
        Write-Log "Starting upgrade configuration"
        $BootstrapURL = "WIN_BOOTSTRAP_URL"
        $upgradeDir = "C:\AzureData\upgrade\NEW_VERSION"
        $genconfDir = Join-Path $upgradeDir "genconf"
        $logPath = Join-Path $upgradeDir "dcos_generate_config.log"
        $upgradeUrlPath = Join-Path $upgradeDir "upgrade_url"

        Write-Log "Setting up Windows bootstrap node for upgrade"
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $upgradeDir
        New-Item -ItemType Directory -Force -Path $genconfDir
        $path = Join-Path $genconfDir "config.yaml"
        cp "C:\AzureData\config-win.NEW_VERSION.yaml" $path
        cp "c:\temp\genconf\ip-detect.ps1" $genconfDir
        cd $upgradeDir

        $path = Join-Path $upgradeDir "dcos_generate_config.windows.tar.xz"
        RetryCurl $BootstrapURL $path

        cmd.exe /c '"C:\Program Files\7-Zip\7z.exe" e .\dcos_generate_config.windows.tar.xz -so | "C:\Program Files\7-Zip\7z.exe" x -si -ttar'
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
        $url = ($match.Line -replace 'Node upgrade script URL:','').Trim()
        if (-Not $url) {
                throw "Bad Node upgrade script URL in $logPath"
        }

        # Stop docker container
        $process = docker ps -q
        if ($process) {
                Write-Log "Stopping nginx service $process"
                & docker.exe kill $process
        }
        Write-Log "Starting nginx service"

        # Run docker container with nginx
        cd c:\docker

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

        $volume = ($genconfDir+"/serve/:c:/nginx/html:ro")
        & docker.exe run --rm -d --network customnat -p 8086:80 -v $volume nginx:1803
        if ($LASTEXITCODE -ne 0) {
                throw "Failed to run docker image"
        }

        CreateDockerStart "c:\docker\StartDocker.ps1" "c:\docker\StartDocker.log" $volume

        Set-Content -Path $upgradeUrlPath -Value $url -Encoding Ascii

        $url = Get-Content -Path $upgradeUrlPath -Encoding Ascii
        if (-Not $url) {
                Remove-Item $upgradeUrlPath -Force
                throw "Failed to set up bootstrap node. Please try again"
        } else {
                # keep Write-Output - used in parsing
                Write-Output "Setting up bootstrap node completed. Node upgrade script URL $url"
        }
} catch {
    Write-Log "Failed to upgrade Windows bootstrap node: $_"
    exit 1
}
Write-Log "Setting up bootstrap node completed"
exit 0
