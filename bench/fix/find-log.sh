#!/bin/sh
# Canned fix for the stand-in agent: write the answer.
cd "$1" || exit 1
echo fixture/internal/ledger > ANSWER.txt
