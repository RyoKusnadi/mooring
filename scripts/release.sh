#!/usr/bin/env bash
# Builds the release tarballs. The web app must already be built (make release does that).
# Usage: VERSION=v0.13.0 scripts/release.sh   (make release sets VERSION)
# Output: bin/release/<version>/mooring-<version>-<os>-<arch>.tar.gz and SHA256SUMS.
set -euo pipefail

cd "$(dirname "$0")/.."
version="${VERSION:-$(git describe --tags --always --dirty)}"
case "$version" in
*-dirty)
	if [ "${ALLOW_DIRTY:-}" != "1" ]; then
		echo "release: version $version has uncommitted changes. Commit them, or run with ALLOW_DIRTY=1." >&2
		exit 1
	fi
	;;
esac
case "$version" in
*[!A-Za-z0-9._-]* | "")
	echo "release: version \"$version\" must use letters, digits, dot, dash and underscore only." >&2
	exit 1
	;;
esac
if [ ! -f THIRD_PARTY_NOTICES.txt ]; then
	echo "release: THIRD_PARTY_NOTICES.txt is missing. Run node scripts/notices.mjs." >&2
	exit 1
fi
if [ ! -f internal/webui/dist/index.html ]; then
	echo "release: the web app is not built (internal/webui/dist/index.html is missing). Run make release." >&2
	exit 1
fi

out="bin/release/$version"
rm -rf "$out"
mkdir -p "$out"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do
	goos="${target%/*}"
	goarch="${target#*/}"
	name="mooring-$version-$goos-$goarch"
	echo "release: building $name"
	mkdir -p "$stage/$name"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
		-ldflags "-s -w -X main.version=$version" -o "$stage/$name/mooring" ./cmd/mooring
	cp LICENSE THIRD_PARTY_NOTICES.txt "$stage/$name/"
	tar -C "$stage" -czf "$out/$name.tar.gz" "$name"
done

(
	cd "$out"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum ./*.tar.gz | sed 's|  \./|  |' >SHA256SUMS
	else
		shasum -a 256 ./*.tar.gz | sed 's|  \./|  |' >SHA256SUMS
	fi
)
echo "release: wrote $out"
ls -l "$out"
