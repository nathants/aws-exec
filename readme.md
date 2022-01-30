# aws-rce

## why

sometimes remote code execution is exactly what's needed, like for continuous integration.

## what

adhoc execution in lambda with streaming logs, exitcode, and 15 minutes max duration.

## install

```bash
go install github.com/nathants/cli-aws@latest
cp env.sh.template env.sh # update values
bash bin/check.sh         # lint
bash bin/deploy.sh        # ensure aws infra and deploy prod release
bash bin/dev.sh           # rapidly iterate by updating lambda zip
bash bin/cli.sh -h        # interact with the service via the cli
```

## examples

- whoami
  ```
  >> bash bin/cli.sh exec whoami
  sbx_user1051
  ```

- download and run a binary
  ```
  >> bash bin/cli.sh exec -- bash -c 'cd /tmp && curl -L https://github.com/peak/s5cmd/releases/download/v1.4.0/s5cmd_1.4.0_Linux-64bit.tar.gz | tar xfz - && ./s5cmd -h | head -n5'
  exec/exec.go:125: waiting 9f78c457-7fd1-4459-baec-e3510a9dea29 2022-01-31 15:17:49.045568486 -1000 HST m=+0.730464864
  exec/exec.go:125: waiting 9f78c457-7fd1-4459-baec-e3510a9dea29 2022-01-31 15:17:50.202140345 -1000 HST m=+1.887036728
  exec/exec.go:125: waiting 9f78c457-7fd1-4459-baec-e3510a9dea29 2022-01-31 15:17:51.352839698 -1000 HST m=+3.037736085
    % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                   Dload  Upload   Total   Spent    Left  Speed
  100   667  100   667    0     0   3206      0 --:--:-- --:--:-- --:--:--  3923
  100 3856k  100 3856k    0     0  1650k      0  0:00:02  0:00:02 --:--:-- 1883k
  NAME:
     s5cmd - Blazing fast S3 and local filesystem execution tool

  USAGE:
     s5cmd [global options] command [command options] [arguments...]
  ```


- checkip from multiple simulataneous exeuctions
  ```
  >> for i in {1..5}; do bash bin/cli.sh exec -- curl -s checkip.amazonaws.com & done
  52.34.141.29
  54.202.6.176
  34.221.149.14
  34.219.161.143
  34.219.244.139
  ```
