# Microsoft DC/OS Engine - Large Clusters

## Overview

DCOS-Engine enables you to create customized Docker enabled cluster on Microsoft Azure with 1200 nodes.

The examples show you how to configure up to 12 agent pools with 100 nodes each:

1. **dcos.json** - deploying and using [DC/OS](../../docs/dcos.md)
2. **dcos-vmas.json** - this provides an example using availability sets instead of the default virtual machine scale sets.  You will want to use availability sets if you want to dynamically attach/detach disks.
