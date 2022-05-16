#!/bin/bash
set -eou pipefail
source env.sh
libaws infra-rm infra.yaml 2>&1 | sed 's/^/libaws: /'
