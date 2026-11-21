package mcp

import "regexp"

// What a registry entry may put into a command line. The registry is text from
// strangers: only names and versions that match these patterns are used, and
// only inside the fixed templates in kinds.go.
var (
	npmName    = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9][a-z0-9._~-]*$`)
	pypiName   = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	ociName    = regexp.MustCompile(`^([a-z0-9][a-z0-9.-]*(:[0-9]{1,5})?/)?[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$`)
	ociTag     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
	pkgVersion = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_!-]{0,63}$`)
	envName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	headerName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)
)

func validPackage(kind, name string) bool {
	if len(name) == 0 || len(name) > 214 {
		return false
	}
	switch kind {
	case "npm":
		return npmName.MatchString(name)
	case "pypi":
		return pypiName.MatchString(name)
	}
	return false
}
