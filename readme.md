# aws-exec

## why

building services on lambda should be easy and fun.

## what

a project scaffold for aws providing asynchronous execution on lambda with streaming logs, exitcode, and 15 minutes max duration.

the [web](#web-demo) interface is easy to use, even from a [phone](#mobile-demo).

the [cli](#cli-demo) interface is easy to use locally, and can execute locally or on lambda.

the [api](#api-demo) interface is easy to use efficiently from other services.

## how

a http post to apigateway triggers an async lambda which invokes a command via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go) or [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go) and stores the result in s3.

each invocation creates 3 objects in s3:
- log: all stdout and stderr, updated in its entirety every second.
- exit: the exit code of the command, written once.
- size: the size in bytes of the log after the final update, written once, written last.

objects are stored in either:
- aws-exec private s3.
- presigned s3 put urls provided by the caller.

to follow invocation status, the caller:
- polls the log object with increasing range-start.
- stops when the size object exists and range-start equals size.
- returns the exit object.

there are three ways to invoke a command:
- [api](#api-demo) invoke a command via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go), this is faster.
- [cli](#cli-demo) invoke a command via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.
- [web](#web-demo) invoke a command via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.

the provided [infrastructure set](https://github.com/nathants/aws-exec/blob/master/infra.yaml) is ready-to-deploy with [libaws](https://github.com/nathants/libaws).

## tradeoffs

asynchronous invocation means that there is a low, but minimum, execution time.

to add a synchronous command that is fast and returns all results immediately, add to [api/](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L353) instead of [cmd/](https://github.com/nathants/aws-exec/tree/master/cmd).

## usage

define new commands in [cmd/](https://github.com/nathants/aws-exec/tree/master/cmd) to add functionality to your service. these commands can be exposed via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go) and/or [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go).

```bash
>> tree cmd/

cmd/
├── auth
│   ├── ls.go
│   ├── new.go
│   └── rm.go
├── exec
│   └── exec.go
├── listdir
│   └── listdir.go
└── rpc
    └── rpc.go
```

duplicate the [listdir](https://github.com/nathants/aws-exec/tree/master/cmd/listdir/listdir.go) command and modify it to introduce new functionality.

[listdir](https://github.com/nathants/aws-exec/tree/master/cmd/listdir/listdir.go) provided as a simple example command that is exposed as both [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go) and [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go).

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

## deploy

```bash
go install github.com/nathants/libaws@latest
export PATH=$PATH:$(go env GOPATH)/bin

cp env.sh.template env.sh # update values
bash bin/check.sh         # lint
bash bin/preview.sh       # preview changes to aws infra
bash bin/ensure.sh        # ensure aws infra
bash bin/dev.sh           # iterate on backend and frontend
bash bin/logs.sh          # tail the logs
bash bin/delete.sh        # delete aws infra
bash bin/cli.sh -h        # interact with the service via the cli
```

## deploy with docker

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
bash bin/cli.sh auth-new test-user
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
