#!/bin/sh
. "$(dirname "$0")/lib.sh"
old_name_gone
new_name_used
go build ./... || fail "go build fails"
go vet ./... >/dev/null 2>&1 || fail "go vet fails"
echo "pass"
