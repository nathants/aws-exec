#!/bin/bash
set -eou pipefail
source env.sh
cli-aws lambda-rm --everything backend/*.go 2>&1 | sed 's/^/cli-aws: /'
