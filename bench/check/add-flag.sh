#!/bin/sh
. "$(dirname "$0")/lib.sh"
out="${TMPDIR:-/tmp}/tally-$$.out"
go run ./cmd/tally -limit 3 < testdata/in.txt > "$out" 2>/dev/null || { rm -f "$out"; fail "tally -limit 3 fails"; }
diff -q "$out" testdata/want.txt >/dev/null || { rm -f "$out"; fail "output differs from testdata/want.txt"; }
rm -f "$out"
echo "pass"
