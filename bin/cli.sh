#!/bin/bash
source env.sh

hash=$(find -name "*.go" -o -name "go.*" | sort | xargs cat | sha256sum | awk '{print $1}')
path=/tmp/aws-exec.$hash

if ! [ -f $path ]; then
    go build -o $path .
fi

$path "$@"
