#!/bin/bash
set -eou pipefail

source env.sh

cd frontend
npm ci
npx shadow-cljs watch app
