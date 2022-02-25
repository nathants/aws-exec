#!/bin/bash
source env.sh

hash=$(find -name *.go | sort | xargs cat | sha256sum | awk '{print $1}')
path=/tmp/cli.$hash

if ! [ -f $path ]; then
    go build -o $path cmd/cli.go
fi

$path "$@"
