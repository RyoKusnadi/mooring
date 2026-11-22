# Shared by the check scripts. Each script takes the task folder and exits 0 for a pass.
set -u
dir=${1:?usage: check/<task>.sh <folder>}
cd "$dir" || exit 2

fail() { echo "FAIL: $*"; exit 1; }

# tests_untouched fails when a _test.go file differs from the first commit.
tests_untouched() {
	base=$(git rev-list --max-parents=0 HEAD | tail -1)
	git diff --quiet "$base" -- '*_test.go' || fail "a test file was changed"
	[ -z "$(git ls-files --others --exclude-standard -- '*_test.go')" ] || fail "a test file was added"
}

tests_pass() { go test -count=1 ./... >/dev/null 2>&1 || fail "go test ./... fails"; }

old_name_gone() { ! grep -rq ParseRow --include='*.go' . || fail "ParseRow is still used"; }

new_name_used() { grep -rq ParseRecord --include='*.go' . || fail "ParseRecord is not defined"; }
