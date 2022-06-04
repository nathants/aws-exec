#!/bin/bash
set -eou pipefail

source env.sh

# auto reload frontend
bash bin/dev_frontend.sh 2>&1 | sed 's/^/shadow-cljs: /' &
pid=$!
trap "kill $pid &>/dev/null || true" EXIT

# auto reload backend
find -name "*.go" -o -name "go.*" | entr -r bash bin/quick.sh 2>&1 | sed 's/^/libaws: /'
