#!/bin/bash
set -eou pipefail

source ${1:-env.sh}

cd frontend
pnpm install --frozen-lockfile
pnpm exec shadow-cljs watch app
