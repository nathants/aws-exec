#!/bin/bash
set -eou pipefail

source ${1:-env.sh}

seen=$(mktemp)
trap "rm -f $seen || true" EXIT

while true; do

    # list all logs younger than n minutes
    libaws s3-ls -s $PROJECT_BUCKET/logs/$(date --utc --date="${start:-1 minute ago}" +%s) | awk '{print $4}' | while read log; do

        # if we haven't already seen it
        if ! grep $log $seen &>/dev/null; then

            # echo new source
            echo logs: s3://$PROJECT_BUCKET/$log 1>&2


            # print it
            if [ -n "${serial:-}" ]; then
                libaws s3-get s3://$PROJECT_BUCKET/$log
            else

                # check max concurrency
                while true; do
                    if [ $(ps -ef | grep "libaws s3-get s3://$PROJECT_BUCKET" | wc -l) -lt 64 ]; then
                        break
                    fi
                    sleep .1
                done
                libaws s3-get s3://$PROJECT_BUCKET/$log &
            fi

            # mark it as seen, and prune old seen data
            updated_seen=$(mktemp)
            awk "\$1 > $(date --utc --date="5 minutes ago" +%s) {print}" $seen > $updated_seen
            echo $(date --utc +%s) $log >> $updated_seen
            mv -f $updated_seen $seen

        fi

    done

    # if start specified exit immediately
    if [ -n "${start:-}" ]; then
        break
    fi

    sleep 1
done
