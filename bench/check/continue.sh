#!/bin/sh
. "$(dirname "$0")/lib.sh"
old_name_gone
new_name_used
go build ./... || fail "go build fails"
tests_pass
echo "pass"
