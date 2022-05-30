#!/bin/bash
set -eou pipefail

source env.sh

mkdir -p frontend/public/
touch frontend/public/index.html.gz
touch frontend/public/favicon.png

libaws infra-ensure infra.yaml --preview 2>&1 | sed 's/^/libaws: /'
