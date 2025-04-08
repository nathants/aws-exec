#!/bin/bash
set -eou pipefail

source ${1:-env.sh}

cd frontend
npm ci
npx shadow-cljs watch app
