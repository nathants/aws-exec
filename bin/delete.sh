#!/bin/bash
set -eou pipefail
source env.sh
libaws lambda-rm --everything backend/*.go 2>&1 | sed 's/^/libaws: /'
