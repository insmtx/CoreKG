#!/usr/bin/env bash
set -euo pipefail
# 固定 locale：grep/sort 的字节序与运行环境 locale 无关，输出可复现。
# 注意：仓库已提交的 route-inventory.csv 是 en_US.UTF-8 排序产物，与本脚本（C locale）
# 行集相同但行序不同；重新生成基线 CSV 时必须用本脚本一次性重写并整体 review。
export LC_ALL=C
out="${1:-docs/architecture/phase0/route-inventory.csv}"
mkdir -p "$(dirname "$out")"
printf 'file,line,method,action\n' > "$out"
grep -rnE '\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("[^"]+"' apps \
  --include='*.go' --exclude='*_docs.go' --exclude-dir=workflow --exclude-dir=testdata \
  | grep -v '/internal/docs/' \
  | sed -E 's#^([^:]+):([0-9]+):.*\.(P|G|Any|PRequireLogin|PRequireEmployee|GRequireLogin|HandleDoc)\("([^"]+)".*#\1,\2,\3,\4#' \
  | sort -t, -k4,4 >> "$out"
