#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/../../.." && pwd)
package_dir="$repo_dir/clients/corekg-cli/npm/corekg-cli"
go_bin=${GO:-go}
version=${CLI_VERSION:-dev}
git_commit=${CLI_GIT_COMMIT:-unknown}
built_at=${CLI_BUILD_AT:-unknown}
ld_flags="-X github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo.Version=$version -X github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo.GitCommit=$git_commit -X github.com/insmtx/corekg/clients/corekg-cli/internal/buildinfo.BuiltAt=$built_at"

sync_package_version() {
	PACKAGE_ROOT="$package_dir" PACKAGE_VERSION="$version" node <<'NODE'
const fs = require("node:fs");
const path = require("node:path");

const root = process.env.PACKAGE_ROOT;
const version = process.env.PACKAGE_VERSION;
const filename = path.join(root, "package.json");
const packageJSON = JSON.parse(fs.readFileSync(filename, "utf8"));
packageJSON.version = version;
fs.writeFileSync(filename, `${JSON.stringify(packageJSON, null, 2)}\n`, { mode: 0o644 });
NODE
}

build_binary() {
	platform=$1
	architecture=$2
	target_name=$3
	binary_name=corekg-cli
	if [ "$platform" = "win32" ]; then
		goos=windows
		binary_name=corekg-cli.exe
	else
		goos=$platform
	fi

	output_dir="$package_dir/bin/$target_name"
	mkdir -p "$output_dir"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$architecture" "$go_bin" build -mod=vendor -trimpath -ldflags="$ld_flags" -o "$output_dir/$binary_name" "$repo_dir/clients/corekg-cli"
	if [ "$goos" != "windows" ]; then
		chmod 0755 "$output_dir/$binary_name"
	fi
}

sync_package_version
build_binary darwin arm64 darwin-arm64
build_binary darwin amd64 darwin-x64
build_binary linux arm64 linux-arm64
build_binary linux amd64 linux-x64
build_binary win32 amd64 win32-x64

printf '%s\n' "Built @insmtx/corekg-cli platform binaries under $package_dir/bin"
