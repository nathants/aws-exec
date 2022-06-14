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
# with current spot pricing for c5.large in us-west-2, if you run this instance 8 hours a day 20 days a month, it will cost ~$5/month.
#
# this instance will self destruct after 1 hour without activity, via `--seconds-timeout 3600`.
#

# usage:
#
#   bash bin/relay.sh
#

source env.sh

watchdir1=aws-exec
remote_cmd="bash -c 'cd $watchdir1 && ZIP_COMPRESSION=0 bash bin/quick.sh'"
name=relay
ssh_opts="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

## spinup the instance if it doesn't already exist
if ! libaws ec2-ls -s running $name &>/dev/null; then
    libaws ec2-new \
           --ami alpine \
           --type ${relay_type:-c5.large} \
           --key $name \
           --sg $name \
           --vpc $name \
           --profile $name \
           --gigs 32 \
           --spot capacityOptimized \
           --seconds-timeout 3600 \
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

cd ..

watch_files=$(
    find $watchdir1 -type f | grep -e '\.go$' -e '\.mod$' -e '\.sum$' -e '\.yml$' -e '\.yaml$' -e '\.sh$'
)

watch_directories=$(
    echo "$watch_files" | sed -r 's:/[^/]+*$::' | sort -u | grep '/'
)

# when a file is added/removed, the outer loop starts over. when a file is changed, the inner loop handles it.
while true; do (

    # rsync files, this is only slow the first time
    libaws ec2-rsync $(cd $watchdir1 && pwd)/ :$watchdir1/ $name

    (
        # watch these files and directories
        echo "$watch_files"
        echo "$watch_directories"

    ) | (

        ## when a file changes, send its name and base64 content over ssh on a single line
        EV_TRACE=y entr -d -r echo 2>&1 | while read line; do
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

) || continue; done
