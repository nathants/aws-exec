#!/bin/bash
set -eou pipefail

source env.sh

# auto reload frontend
bash bin/dev_frontend.sh 2>&1 | sed 's/^/shadow-cljs: /' &
pid=$!
trap "kill $pid &>/dev/null || true" EXIT

# auto reload backend
find -type f | grep -F -v -e .shadow-cljs -e backups -e node_modules | entr -r bash bin/quick.sh 2>&1 | sed 's/^/libaws: /'
