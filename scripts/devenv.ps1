$pwd = (Get-Location).Path

docker build --pull -t dcos-engine .
docker run --security-opt seccomp:unconfined -it `
	-v ${pwd}:/gopath/src/github.com/Azure/dcos-engine `
	-w /gopath/src/github.com/Azure/dcos-engine `
		dcos-engine /bin/bash

