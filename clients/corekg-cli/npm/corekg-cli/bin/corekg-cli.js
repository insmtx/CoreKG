#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const { existsSync } = require("node:fs");
const path = require("node:path");

const platformNames = {
  darwin: "darwin",
  linux: "linux",
  win32: "win32",
};
const architectureNames = {
  arm64: "arm64",
  x64: "x64",
};
const platform = platformNames[process.platform];
const architecture = architectureNames[process.arch];

if (!platform || !architecture || (platform === "win32" && architecture !== "x64")) {
  console.error(`corekg-cli does not provide a binary for ${process.platform}/${process.arch}`);
  process.exit(1);
}

const binaryName = platform === "win32" ? "corekg-cli.exe" : "corekg-cli";
const binaryPath = path.join(__dirname, `${platform}-${architecture}`, binaryName);

if (!existsSync(binaryPath)) {
  console.error(`@insmtx/corekg-cli does not contain a binary for ${process.platform}/${process.arch}`);
  console.error("Reinstall @insmtx/corekg-cli and check the published package contents.");
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
