#!/bin/bash

out=$(mktemp)
trap "rm $out &>/dev/null || true" EXIT

bash -c "cd frontend && npx shadow-cljs compile app > $out 2>/dev/null" &
pid=$!
trap "kill -9 $pid &>/dev/null || true" EXIT

set -eou pipefail

which staticcheck >/dev/null   || (cd ~ && go install honnef.co/go/tools/cmd/staticcheck@latest)
which golint      >/dev/null   || (cd ~ && go install golang.org/x/lint/golint@latest)
which ineffassign >/dev/null   || (cd ~ && go install github.com/gordonklaus/ineffassign@latest)
which errcheck    >/dev/null   || (cd ~ && go install github.com/kisielk/errcheck@latest)
which bodyclose   >/dev/null   || (cd ~ && go install github.com/timakin/bodyclose@latest)
which nargs       >/dev/null   || (cd ~ && go install github.com/alexkohler/nargs/cmd/nargs@latest)
which go-hasdefault >/dev/null || (cd ~ && go install github.com/nathants/go-hasdefault@latest)

echo go-hasdefault
go-hasdefault $(find -type f -name "*.go") || true

echo go fmt
go fmt ./... >/dev/null

echo nargs
nargs ./...

echo bodyclose
go vet -vettool=$(which bodyclose) ./...

echo go lint
golint ./... | grep -v -e unexported -e "should be" || true

echo static check
staticcheck ./...

echo ineffassign
ineffassign ./...

echo errcheck
errcheck ./...

echo go vet
go vet ./...

echo go build
go build -o /dev/null backend/backend.go
go build -o /dev/null main.go

wait $pid
echo shadow-cljs compile
if cat $out | grep -i -e warning -e error &>/dev/null; then
    cat $out
    exit 1
fi
