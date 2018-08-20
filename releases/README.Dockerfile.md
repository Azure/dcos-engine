# Build Docker image

**Bash**
```bash
$ VERSION=0.2.0
$ docker build --no-cache --build-arg BUILD_DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"` --build-arg DCOSENGINE_VERSION="$VERSION" -t microsoft/dcos-engine:$VERSION --file ./Dockerfile.linux .
```
**PowerShell**
```powershell
PS> $VERSION="0.2.0"
PS> docker build --no-cache --build-arg BUILD_DATE=$(Get-Date((Get-Date).ToUniversalTime()) -UFormat "%Y-%m-%dT%H:%M:%SZ") --build-arg DCOSENGINE_VERSION="$VERSION" -t microsoft/dcos-engine:$VERSION --file .\Dockerfile.linux .
```

# Inspect Docker image metadata

**Bash**
```bash
$ docker image inspect microsoft/dcos-engine:0.1.0 --format "{{json .Config.Labels}}" | jq
{
  "maintainer": "Microsoft",
  "org.label-schema.build-date": "2017-10-25T04:35:06Z",
  "org.label-schema.description": "DC/OS Engine (dcos-engine) generates ARM (Azure Resource Manager) templates for Docker enabled clusters on Microsoft Azure with DC/OS orchestrator.",
  "org.label-schema.docker.cmd": "docker run -v ${PWD}:/dcos-engine/workspace -it --rm microsoft/dcos-engine:0.1.0",
  "org.label-schema.license": "MIT",
  "org.label-schema.name": "DC/OS Engine (dcos-engine)",
  "org.label-schema.schema-version": "1.0",
  "org.label-schema.url": "https://github.com/Azure/dcos-engine",
  "org.label-schema.usage": "https://github.com/Azure/dcos-engine/blob/master/docs/dcos-engine.md",
  "org.label-schema.vcs-url": "https://github.com/Azure/dcos-engine.git",
  "org.label-schema.vendor": "Microsoft",
  "org.label-schema.version": "0.1.0"
}
```

**PowerShell**
```powershell
PS> docker image inspect microsoft/dcos-engine:0.2.0 --format "{{json .Config.Labels}}" | ConvertFrom-Json | ConvertTo-Json
{
    "maintainer":  "Microsoft",
    "org.label-schema.build-date":  "2017-10-25T04:35:06Z",
    "org.label-schema.description":  "DC/OS Engine (dcos-engine) generates ARM (Azure Resource Manager) templates for Docker enabled clusters on Microsoft Azure with DC/OS orchestrator.",
    "org.label-schema.docker.cmd":  "docker run -v ${PWD}:/dcos-engine/workspace -it --rm microsoft/dcos-engine:0.1.0",
    "org.label-schema.license":  "MIT",
    "org.label-schema.name":  "DC/OS Engine (dcos-engine)",
    "org.label-schema.schema-version":  "1.0",
    "org.label-schema.url":  "https://github.com/Azure/dcos-engine",
    "org.label-schema.usage":  "https://github.com/Azure/dcos-engine/blob/master/docs/dcos-engine.md",
    "org.label-schema.vcs-url":  "https://github.com/Azure/dcos-engine.git",
    "org.label-schema.vendor":  "Microsoft",
    "org.label-schema.version":  "0.1.0"
}
```

# Run Docker image

```
$ docker run -v ${PWD}:/dcos-engine/workspace -it --rm microsoft/dcos-engine:0.2.0
```
