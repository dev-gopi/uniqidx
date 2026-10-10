#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
rm -f uniqidx-v1.zip
zip -r uniqidx-v1.zip \
  README.md SPEC.md PLAN.md SECURITY.md DATABASE.md KEY_MANAGEMENT.md \
  deep-research-report-final.md go.mod \
  *.go cmd lease keyring scripts \
  -x 'uniqidx-v1.zip' '*__pycache__*'
