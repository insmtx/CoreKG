#!/usr/bin/env bash
set -euo pipefail
out="${1:-docs/architecture/phase0/route-inventory.csv}"
mkdir -p "$(dirname "$out")"
printf 'file,line,method,action\n' > "$out"
grep -rnE '\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("[^"]+"' apps \
  --include='*.go' --exclude='*_docs.go' --exclude-dir=workflow --exclude-dir=testdata \
  | grep -v '/internal/docs/' \
  | sed -E 's#^([^:]+):([0-9]+):.*\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("([^"]+)".*#\1,\2,\3,\4#' \
  | sort -t, -k4,4 >> "$out"
