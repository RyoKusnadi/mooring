#!/bin/sh
# Canned fix for the stand-in agent: rename ParseRow everywhere.
cd "$1" || exit 1
grep -rl ParseRow --include='*.go' . | while read -r f; do sed -i.bak 's/ParseRow/ParseRecord/g' "$f"; done
find . -name '*.bak' -delete
