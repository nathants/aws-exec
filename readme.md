# AWS-Exec

## Why

So you can just ship services on AWS.

## What

A project scaffold for a backend service on AWS with an [infrastructure set](https://github.com/nathants/aws-exec/blob/master/infra.yaml) ready-to-deploy with [libaws](https://github.com/nathants/libaws).

The project scaffold makes it easy to:

- Authenticate callers.

- Implement fast synchronous APIs that return all results immediately.

- Implement slow asynchronous APIs with streaming logs, exit code, and 15 minutes max duration.

- Use the [web](#web-demo) interface, even from a [phone](#mobile-demo).

- Use the [cli](#cli-demo) interface, executing locally or on Lambda.

- Use the [api](#api-demo) interface, calling efficiently from other backend services.

## How

Synchronous APIs are normal HTTP on Lambda.

Asynchronous APIs are a HTTP POST that triggers an async Lambda which invokes a command via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go) or [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go) and stores the results in S3.

  - Each invocation creates 3 objects in S3:
    - Log: all stdout and stderr, updated in its entirety every second.
    - Exit: the exit code of the command, written once.
    - Size: the size in bytes of the log after the final update, written once, written last.

  - Objects are stored in either:
    - AWS-exec private S3.
    - Presigned S3 put URLs provided by the caller.

  - To follow invocation status, the caller:
    - Polls the log object with increasing range-start.
    - Stops when the size object exists and range-start equals size.
    - Returns the exit object.

There are three ways to invoke an asynchronous API:
- [API](#api-demo) invoke via [rpc](https://github.com/nathants/aws-exec/tree/master/cmd/rpc/rpc.go), this is faster.
- [CLI](#cli-demo) invoke via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.
- [Web](#web-demo) invoke via [subprocess](https://github.com/nathants/aws-exec/tree/master/cmd/exec/exec.go), this is slower.

## Add a New Synchronous Functionality

Add to [api/](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L353).

Duplicate the [httpExecGet](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L140) or [httpExecPost](https://github.com/nathants/aws-exec/tree/master/backend/backend.go#L224) handler and modify it to introduce new functionality.

## Add a New Asynchronous Functionality

Add to [cmd/](https://github.com/nathants/aws-exec/tree/master/cmd).

Duplicate the [listdir](https://github.com/nathants/aws-exec/tree/master/cmd/listdir/listdir.go) command and modify it to introduce new functionality.

## Web Demo

![](https://github.com/nathants/aws-exec/raw/master/gif/web.gif)

## CLI Demo

![](https://github.com/nathants/aws-exec/raw/master/gif/cli.gif)

## API Demo

![](https://github.com/nathants/aws-exec/raw/master/gif/api.gif)

## Mobile Demo

![](https://github.com/nathants/aws-exec/raw/master/gif/mobile.gif)

## Dependencies

Use the included [Dockerfile](./Dockerfile) or install the following dependencies:
- npm
- JDK
- go
- bash
- [entr](https://formulae.brew.sh/formula/entr)
- [libaws](https://github.com/nathants/libaws)

## AWS Prerequisites

- AWS [route53](https://console.aws.amazon.com/route53/v2/hostedzones) has the domain or its parent from env.sh

- AWS [acm](https://us-west-2.console.aws.amazon.com/acm/home) has a wildcard cert for the domain or its parent from env.sh

## Usage

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

## Usage with Docker

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

## Create Auth

```bash
bash bin/cli.sh env.sh auth-new test-user
```

## Install and Use CLI

```bash
go install github.com/nathants/aws-exec@latest
export PATH=$PATH:$(go env GOPATH)/bin

export AUTH=$AUTH
export PROJECT_DOMAIN=$DOMAIN
aws-exec exec -- whoami
```

## Install and Use API

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
	val, err := json.Marshal(map[string]any{
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
