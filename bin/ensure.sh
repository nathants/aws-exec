#!/bin/bash
set -eou pipefail

source ${1:-env.sh}

# inline js and add csp to the markup from frontend/public/index.html
inline() {
    path=$1
    js=$(cat $path)
    hash=$(echo -n "$js" | openssl sha256 -binary | openssl base64)
    cat <<EOF
<!DOCTYPE html>
<html>
    <head>
      <meta charset="utf-8">
      <meta http-equiv="Content-Security-Policy" content="script-src 'sha256-${hash}'">
      <link rel="icon" href="/favicon.png">
      <link rel="apple-touch-icon" href="/favicon.png">
      <meta name='viewport' content='width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no'/>
    </head>
    <body>
        <div id="app"></div>
        <script type="text/javascript" charset="utf-8">${js}</script>
    </body>
</html>
EOF
}

if [ -z "${NOJS:-}" ] || [ ! -f frontend/public/index.html.gz ]; then
    (
        cd frontend
        npm ci
        rm -rf public/js/
        npx shadow-cljs release app
    ) 2>&1 | sed 's/^/shadow-cljs: /'
    inline frontend/public/js/main.js | gzip --best > frontend/public/index.html.gz
    rm -rf frontend/public/js/*
fi

libaws infra-ensure infra.yaml 2>&1 | sed 's/^/libaws: /'
