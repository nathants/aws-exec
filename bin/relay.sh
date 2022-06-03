#!/bin/bash
set -eou pipefail
#
# relay builds on ec2.
#
# if you have bad upload bandwidth, you can speed up `infra-ensure --quick` by doing it from ec2!
#
# this spins up an ec2 instance, watches local files, and when they change updates them on ec2 and runs a command.
#
# this means your local internet only needs to upload source code changes, not lambda zips.
#
# usage:
#
#   bash bin/relay.sh "bash -c 'cd aws-rce && ZIP_COMPRESSION=0 bash bin/quick.sh'"
#

. env.sh

remote_cmd=$1
name=relay
ssh_opts="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

## spinup the instance if it doesn't already exist
if ! libaws ec2-ls -s running $name &>/dev/null; then
    libaws ec2-new \
           --ami alpine-3.16.0 \
           --key $name \
           --sg $name \
           --vpc $name \
           --profile $name \
           --gigs 32 \
           --type c5.large \
           --spot capacityOptimized \
           $name
    libaws ec2-wait-ssh $name
fi

libaws ec2-ssh $name -c '
    sudo apk update
    sudo apk upgrade -a
    sudo apk add \
        bash \
        coreutils \
        curl \
        docker \
        docker-compose \
        git \
        glances \
        go \
        grep \
        htop \
        libuser \
        musl-dev \
        ncurses-terminfo \
        procps \
        rsync \
        sed \
        vim \
        wget \
        zip
    if ! which libaws &>/dev/null; then
        go install github.com/nathants/libaws@latest
        sudo mv -fv $(go env GOPATH)/bin/libaws /usr/local/bin
        sudo sed -i s:/bin/sh:/bin/bash: /etc/passwd
    fi
'

## copy all source to to relay, after this we only copy what changes
export RSYNC_OPTIONS="--exclude .shadow-cljs --exclude node_modules --exclude .backups --exclude *.~undo-tree~ --exclude .clj-kondo --exclude frontend"
libaws ec2-rsync $(pwd)/ :aws-rce/ $name

cd ..

(
    ## watch these files for changes
    find aws-rce -type f | grep -e '\.go$' -e '\.mod$' -e '\.sum$' -e '\.yaml$' -e '\.sh$' | grep -v '/frontend/'

) | (

    ## when a file changes, send its name and base64 content over ssh on a single line
    EV_TRACE=y entr -r echo 2>&1 | while read line; do
        file=$(echo $line | awk '{print $NF}')
        if [ -f "$file" ]; then
            echo $file $(cat $file | base64 -w0)
        fi
    done

) | (

    ## update local files and run remote_cmd
    ssh $ssh_opts $(libaws ec2-ssh-user $name)@$(libaws ec2-ip $name) "
        export AWS_DEFAULT_REGION=$(libaws aws-region)
        while read line; do
            read -r file content <<< \"\$line\"
            echo \$file
            echo \$content | base64 -d > \$file
            date +%s | sudo tee /etc/timeout.start.seconds >/dev/null || true # reset start time for: libaws ec2-new --seconds-timeout
            echo $remote_cmd
            $remote_cmd
        done
    "
)
