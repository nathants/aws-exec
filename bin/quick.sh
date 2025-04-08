#!/bin/bash
set -eou pipefail

source ${1:-env.sh}

echo rebuild ${PROJECT_NAME}

mkdir -p frontend/public/
touch frontend/public/index.html.gz
touch frontend/public/favicon.png

time libaws infra-ensure infra.yaml --quick ${PROJECT_NAME}
