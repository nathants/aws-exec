#!/bin/bash
source ${1:-env.sh}

hash=$(find -name "*.go" -o -name "go.*" | sort | xargs cat | sha256sum | awk '{print $1}')
path=/tmp/aws-exec-cli
hash_path=${path}.hash

if [ ! -f $path ] || [ "$(cat $hash_path 2>/dev/null)" != "$hash" ]; then
    echo rebuild $path $hash 1>&2
    go build -o $path .
    echo -n "$hash" > $hash_path
fi

shift

$path "$@"
