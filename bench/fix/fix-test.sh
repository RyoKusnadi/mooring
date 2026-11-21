#!/bin/sh
# Canned fix for the stand-in agent: trim the name.
cd "$1" || exit 1
sed -i.bak 's/Name: fields\[0\]/Name: strings.TrimSpace(fields[0])/' internal/parse/parse.go
find . -name '*.bak' -delete
