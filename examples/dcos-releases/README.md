# Microsoft DC/OS Engine - DC/OS Versions

## Overview

This section provides example templates enable creation of Docker enabled cluster with older version of the DC/OS orchestrator.

Here are the release channels dcos-engine is able to deploy:

1. **dcos1.11.json** - deploying latest supported version off DC/OS 1.11 release by specifying `"orchestratorRelease": "1.11"`.
2. **dcos1.11.4.json** - deploying DC/OS 1.11.4 by specifying `"orchestratorVersion": "1.11.4"`.

To get the complete list of supported versions, run
```shell
$ dcos-engine orchestrator
```

Deploying and using [DC/OS](../../docs/dcos.md)
