# aws-rce

## why

sometimes remote code execution is exactly what's needed, like for continuous integration.

## what

adhoc execution in lambda with streaming logs, exitcode, and 15 minutes max duration.

## how

a http post to apigateway triggers an async lambda which runs a shell command, streaming logs and then exit code back to the caller.

there are two ways to use it:

- cli

- web

## web demo

![](https://github.com/nathants/aws-rce/raw/master/gif/web.gif)

## cli demo

![](https://github.com/nathants/aws-rce/raw/master/gif/cli.gif)

## deploy

```bash
go install github.com/nathants/libaws@latest
export PATH=$PATH:$(go env GOPATH)/bin

cp env.sh.template env.sh # update values
bash bin/check.sh         # lint
bash bin/ensure.sh        # ensure aws infra and deploy prod release
bash bin/dev.sh           # rapidly iterate by updating lambda zip
bash bin/cli.sh -h        # interact with the service via the cli
```

## deploy with docker

```bash
cp env.sh.template env.sh # update values
docker build -t aws-rce:latest .
docker run -it --rm \
    -v $(pwd):/code \
    -e AWS_DEFAULT_REGION \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    aws-rce:latest \
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
go install github.com/nathants/aws-rce@latest
export PATH=$PATH:$(go env GOPATH)/bin

export AUTH=$AUTH
export PROJECT_DOMAIN=$DOMAIN
aws-rce exec -- whoami
```
