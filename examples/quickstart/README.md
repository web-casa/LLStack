# LLStack Quickstart Examples

## Command Discovery

```bash
llstack --help
llstack install --help
llstack site:create --help
```

## Release Smoke

```bash
make build
make smoke
```

## Package And Install

```bash
make package
make verify-release
bash scripts/install.sh --from dist/packages/0.1.0-dev/llstack-0.1.0-dev-linux-amd64.tar.gz
```

## Install From Remote URL

```bash
bash scripts/install.sh --from https://example.invalid/llstack-0.1.0-dev-linux-amd64.tar.gz
```

## Install From Release Index

```bash
bash scripts/install-release.sh --index https://example.invalid/index.json --platform linux-amd64
```

## Install From Signed Release Index

```bash
bash scripts/install-release.sh \
  --index https://example.invalid/index.json \
  --platform linux-amd64 \
  --pubkey /path/to/release-public.pem \
  --require-signature
```

## Dry-Run Install Plan

```bash
llstack install \
  --backend apache \
  --php_version 8.3 \
  --db mariadb \
  --with_memcached \
  --site example.com \
  --dry-run \
  --json
```

## Config-Driven Install Plan

```bash
llstack install --config examples/install/basic.yaml --json
```

## Legacy Flat Install Profile

```bash
llstack install --config examples/install/legacy-flat.yaml --json
```

## Dry-Run Site Create

```bash
llstack site:create example.com \
  --backend apache \
  --profile generic \
  --non-interactive \
  --dry-run \
  --json
```
