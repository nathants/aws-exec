#!/bin/bash
set -eou pipefail

source env.sh

if [ -z "${NOJS:-}" ]; then
    (
        cd frontend
        npm ci
        rm -rf public/js/
        npx shadow-cljs release app
    ) 2>&1 | sed 's/^/shadow-cljs: /'

    # inline js into html then gzip
    temp=$(mktemp)
    hash=$(cat frontend/public/js/main.js | openssl sha256 -binary | openssl base64)
    cat frontend/public/index.html | grep "text/javascript" -B100 | grep -v "text/javascript" | sed "s:HASH:$hash:" >> $temp
    echo -n '<script type="text/javascript" charset="utf-8">' >> $temp
    cat frontend/public/js/main.js >> $temp
    echo '</script>' >> $temp
    cat frontend/public/index.html | grep "text/javascript" -A100 | grep -v "text/javascript" >> $temp
    cat $temp | gzip --best > frontend/public/index.html.gz
    rm $temp frontend/public/js/*
fi

libaws infra-ensure infra.yaml 2>&1 | sed 's/^/libaws: /'
