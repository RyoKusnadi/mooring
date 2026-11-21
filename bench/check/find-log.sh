#!/bin/sh
. "$(dirname "$0")/lib.sh"
[ -f ANSWER.txt ] || fail "ANSWER.txt is missing"
[ "$(tr -d ' \t\r\n' < ANSWER.txt)" = "fixture/internal/ledger" ] || fail "ANSWER.txt names the wrong package"
echo "pass"
