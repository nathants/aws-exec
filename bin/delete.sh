#!/bin/bash
set -eou pipefail
source ${1:-env.sh}
libaws infra-rm infra.yaml 2>&1 | sed 's/^/libaws: /'
