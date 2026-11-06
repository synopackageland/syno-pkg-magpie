#!/usr/bin/env bash
# Native payload hook: compile the merged upstream tree and this fork's token
# adapter into each declared architecture's daemon payload.
#
# The hook runs from synology/ with SPL_UPSTREAM_SOURCE (the merged fork root),
# SPL_UPSTREAM_TAG and SPL_UPSTREAM_COMMIT set by upgrade.sh. It never edits the
# upstream tree: every byte it produces lands below synology/daemon/<arch>.
set -euo pipefail

packaging=$(cd -- "$(dirname -- "$0")" && pwd)
source_root=${SPL_UPSTREAM_SOURCE:-$(cd -- "$packaging/.." && pwd)}
tag=${SPL_UPSTREAM_TAG:-dev}
version=${tag#v}

command -v go >/dev/null 2>&1 || {
  echo 'Go is not on PATH: install Go 1.26.10 or later to build the payload.' >&2
  exit 2
}

# The upstream tree builds for the NAS, never for this machine, and needs no C
# toolchain: magpie's Dockerfile builds the same way (-tags nogui, CGO off).
export CGO_ENABLED=0
export GOOS=linux
export GOFLAGS=-mod=readonly
export GOTOOLCHAIN=local

for pair in x86_64:amd64 aarch64:arm64; do
  arch=${pair%%:*}
  goarch=${pair##*:}
  out="$packaging/daemon/$arch"
  rm -rf "$out"
  mkdir -p "$out"
  echo "building magpie $version for linux/$goarch" >&2
  (
    cd "$source_root"
    GOARCH=$goarch go build -tags nogui -trimpath \
      -ldflags "-s -w -X main.version=$version" \
      -o "$out/magpie" .
  )
  echo "building the panel token adapter for linux/$goarch" >&2
  (
    cd "$packaging/adapter"
    GOARCH=$goarch go build -trimpath -ldflags "-s -w" \
      -o "$out/webtoken" ./cmd/webtoken
  )
  echo "building the launcher for linux/$goarch" >&2
  (
    cd "$packaging/adapter"
    GOARCH=$goarch go build -trimpath -ldflags "-s -w" \
      -o "$out/magpie-launch" ./cmd/magpie-launch
  )
done

# The payload is checked by name and mode in the SPK; keep it to the files the
# manifest names, and never stage a directory link.
if find "$packaging/daemon" -type l -print -quit | grep -q .; then
  echo 'daemon payload must not contain symbolic links' >&2
  exit 1
fi
chmod 0755 "$packaging"/daemon/*/magpie "$packaging"/daemon/*/webtoken "$packaging"/daemon/*/magpie-launch
echo "payload staged in $packaging/daemon" >&2
