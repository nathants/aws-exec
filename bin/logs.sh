#!/bin/bash
set -eou pipefail

source env.sh

seen=$(mktemp)
trap "rm -f $seen || true" EXIT

while true; do

    # list all logs younger than 1 minute
    cli-aws s3-ls -r $PROJECT_BUCKET/logs/$(date --utc --date="1 minute ago" +%s) | awk '{print $4}' | while read log; do

        # if we haven't already printed the log
        if ! grep $log $seen &>/dev/null; then

            # echo new source
            echo logs: s3://$PROJECT_BUCKET/$log

            # check max concurrency
            while true; do
                if [ $(ps -ef | grep "cli-aws s3-get s3://$PROJECT_BUCKET" | wc -l) -lt 8 ]; then
                    break
                fi
                sleep .1
            done

            # print it, excluding blank lines
            cli-aws s3-get s3://$PROJECT_BUCKET/$log &

            # mark it as seen, and prune old seen data
            updated_seen=$(mktemp)
            awk "\$1 > $(date --utc --date="5 minutes ago" +%s) {print}" $seen > $updated_seen
            echo $(date --utc +%s) $log >> $updated_seen
            mv -f $updated_seen $seen

        fi
    done

    sleep 1
done
