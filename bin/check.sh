#!/bin/bash
set -eou pipefail

which staticcheck >/dev/null || (cd ~ && go get -u github.com/dominikh/go-tools/cmd/staticcheck)
which golint      >/dev/null || (cd ~ && go get -u golang.org/x/lint/golint)
which ineffassign >/dev/null || (cd ~ && go get -u github.com/gordonklaus/ineffassign)
which errcheck    >/dev/null || (cd ~ && go get -u github.com/kisielk/errcheck)
which bodyclose   >/dev/null || (cd ~ && go get -u github.com/timakin/bodyclose)
which nargs       >/dev/null || (cd ~ && go get -u github.com/alexkohler/nargs/cmd/nargs)
which go-hasdefault >/dev/null || (cd ~ && go get -u github.com/nathants/go-hasdefault)

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
ineffassign ./*

echo errcheck
errcheck ./...

echo go vet
go vet ./...
