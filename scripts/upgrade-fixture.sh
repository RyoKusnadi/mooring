#!/usr/bin/env bash
# Builds an upgrade fixture: the database of an older release, as a text dump.
# Usage: scripts/upgrade-fixture.sh <name> <commit>
#   name    folder under cmd/mooring/testdata/upgrade/, for example schema-6
#   commit  the commit whose internal/store/migrations make up that release
# The folder must hold seed.sql (made-up rows). Needs the sqlite3 program.
# Writes mooring.sql, token and SOURCE next to seed.sql. Review the diff before committing.
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: $0 <name> <commit>" >&2
	exit 2
fi
cd "$(dirname "$0")/.."
name="$1"
commit="$(git rev-parse --verify "$2^{commit}")"
dir="cmd/mooring/testdata/upgrade/$name"
[ -f "$dir/seed.sql" ] || { echo "missing $dir/seed.sql" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
db="$tmp/mooring.db"
sqlite3 "$db" "CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)"
count=0
for path in $(git ls-tree --name-only "$commit" internal/store/migrations/ | sort); do
	version=$((10#$(basename "$path" | cut -d_ -f1)))
	git show "$commit:$path" | sqlite3 "$db"
	sqlite3 "$db" "INSERT INTO schema_migrations (version, applied_at) VALUES ($version, 1700000000)"
	count=$((count + 1))
done
sqlite3 "$db" <"$dir/seed.sql"
sqlite3 "$db" .dump >"$dir/mooring.sql"
echo "fixture-token-for-tests" >"$dir/token"
cat >"$dir/SOURCE" <<EOT
commit: $commit
migrations applied: $count
made by: scripts/upgrade-fixture.sh $name $commit
rows: seed.sql (made-up; nothing personal). {{ROOT}} in mooring.sql is replaced by the test's data folder.
token: a fixed string, not a secret.
EOT
echo "wrote $dir ($count migrations)"
