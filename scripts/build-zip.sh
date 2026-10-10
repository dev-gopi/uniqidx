#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
rm -f uniqidx-v1.zip
zip -r uniqidx-v1.zip . -x '.git/*' 'uniqidx-v1.zip' '*__pycache__*'
