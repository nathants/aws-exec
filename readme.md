# aws-exec

## why

building services on lambda should be easy and fun.

## what

a project scaffold for a backend service on aws with an [infrastructure set](https://github.com/nathants/aws-exec/blob/master/infra.yaml) ready-to-deploy with [libaws](https://github.com/nathants/libaws).

the project scaffold makes it easy to:

- authenticate callers.

- implement fast synchronous apis that return all results immediately.

- implement slow asynchronous apis with streaming logs, exit code, and 15 minutes max duration.

- use the [web](#web-demo) admin interface, even from a [phone](#mobile-demo).

- use the [cli](#cli-demo) admin interface, executing locally or on lambda.

- use the [api](#api-demo) interface, calling efficiently from other backend services.

## how

synchronous apis are normal http on lambda.

asynchronous apis are a http post that triggers an async lambda which invokes a command via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go) or [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go) and stores the results in s3.

  - each invocation creates 3 objects in s3:
    - log: all stdout and stderr, updated in its entirety every second.
    - exit: the exit code of the command, written once.
    - size: the size in bytes of the log after the final update, written once, written last.

  - objects are stored in either:
    - aws-exec private s3.
    - presigned s3 put urls provided by the caller.

  - to follow invocation status, the caller:
    - polls the log object with increasing range-start.
    - stops when the size object exists and range-start equals size.
    - returns the exit object.

there are three ways to invoke an asynchronous api:
- [api](#api-demo) invoke via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go), this is faster.
- [cli](#cli-demo) invoke via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.
- [web](#web-demo) invoke via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.

## add a new synchronous functionality

add to [api/](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L353).

duplicate the [httpExecGet](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L140) or [httpExecPost](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L224) handler and modify it to introduce new functionality.

## add a new asynchronous functionality

add to [cmd/](https://github.com/nathants/aws-exec/tree/master/cmd).

duplicate the [listdir](https://github.com/nathants/aws-exec/tree/master/cmd/listdir/listdir.go) command and modify it to introduce new functionality.

## web demo

![](https://github.com/nathants/aws-exec/raw/master/gif/web.gif)

## cli demo

![](https://github.com/nathants/aws-exec/raw/master/gif/cli.gif)

## api demo

![](https://github.com/nathants/aws-exec/raw/master/gif/api.gif)

## mobile demo

![](https://github.com/nathants/aws-exec/raw/master/gif/mobile.gif)

## dependencies

use the included [Dockerfile](./Dockerfile) or install the following dependencies:
- npm
- jdk
- go
- bash
- [entr](https://formulae.brew.sh/formula/entr)
- [libaws](https://github.com/nathants/libaws)

## aws prerequisites

- aws [route53](https://console.aws.amazon.com/route53/v2/hostedzones) has the domain or its parent from env.sh

- aws [acm](https://us-west-2.console.aws.amazon.com/acm/home) has a wildcard cert for the domain or its parent from env.sh

## usage

```bash
go install github.com/nathants/libaws@latest
export PATH=$PATH:$(go env GOPATH)/bin

cp env.sh.template env.sh # update values
bash bin/check.sh env.sh         # lint
bash bin/preview.sh env.sh       # preview changes to aws infra
bash bin/ensure.sh env.sh        # ensure aws infra
bash bin/dev.sh env.sh           # iterate on backend and frontend
bash bin/logs.sh env.sh          # tail the logs
bash bin/delete.sh env.sh        # delete aws infra
bash bin/cli.sh env.sh -h        # interact with the service via the cli
```

## usage with docker

```bash
cp env.sh.template env.sh # update values
docker build -t aws-exec:latest .
docker run -it --rm \
    -v $(pwd):/code \
    -e AWS_DEFAULT_REGION \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    aws-exec:latest \
    bash -c '
        cd /code
        bash bin/ensure.sh
    '
```

## create auth

```bash
bash bin/cli.sh env.sh auth-new test-user
```

## install and use cli

```bash
go install github.com/nathants/aws-exec@latest
export PATH=$PATH:$(go env GOPATH)/bin

export AUTH=$AUTH
export PROJECT_DOMAIN=$DOMAIN
aws-exec exec -- whoami
```

## install and use api

```bash
go get github.com/nathants/aws-exec@latest
```

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	awsexec "github.com/nathants/aws-exec/exec"
)

func main() {
	val, err := json.Marshal(map[string]interface{}{
		"path": ".",
	})
	if err != nil {
	    panic(err)
	}
	exitCode, err := awsexec.Exec(context.Background(), &awsexec.Args{
		Url:     "https://%s" + os.Getenv("PROJECT_DOMAIN"),
		Auth:    os.Getenv("AUTH"),
		RpcName: "listdir",
		RpcArgs: string(val),
		LogDataCallback: func(logs string) {
			fmt.Print(logs)
		},
	})
	if err != nil {
		panic(err)
	}
	os.Exit(exitCode)
}
```
