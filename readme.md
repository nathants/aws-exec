# aws-exec

## why

sometimes adhoc shell execution is exactly what's needed, like for continuous integration.

## what

adhoc shell execution in lambda with streaming logs, exitcode, and 15 minutes max duration.

## how

a http post to apigateway triggers an async lambda which runs a shell command and stores the result in s3.

each invocation creates 3 objects in s3:
- log: all stdout and stderr of the command, updated in its entirety every 3 seconds.
- exit: the exit code of the command, written once.
- size: the size in bytes of the log after the final update, written once, written last.

the caller:
- polls the log object with increasing range-start.
- stops when the size object exists and range-start equals size.
- returns the exit object.

the caller can either:
- let aws-exec manage the objects in its own s3 bucket.
- provide 3 presigned s3 urls for aws-exec to push to.

there are two ways to use it:
- cli
- web

the provided [infrastructure set](https://github.com/nathants/aws-exec/blob/master/infra.yaml) is ready-to-deploy with [libaws](https://github.com/nathants/libaws).

## web demo

![](https://github.com/nathants/aws-exec/raw/master/gif/web.gif)

## cli demo

![](https://github.com/nathants/aws-exec/raw/master/gif/cli.gif)

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
