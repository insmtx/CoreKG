# @insmtx/corekg-cli

The npm distribution of the CoreKG command-line client. One package contains
the supported macOS, Linux, and Windows binaries and exposes `corekg-cli`.

```sh
npm install --global @insmtx/corekg-cli
corekg-cli version
```

Build and inspect the package from the repository root with:

```sh
VERSION=0.1.0 make corekg-cli-npm-preflight
cd clients/corekg-cli/npm/corekg-cli
npm pack --dry-run
```

Publish the public scoped package with:

```sh
npm publish --access public
```

The platform binaries are generated build artifacts and are ignored by Git.
