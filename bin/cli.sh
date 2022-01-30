#!/bin/bash
source env.sh
go build cmd/cli.go
./cli "$@"
