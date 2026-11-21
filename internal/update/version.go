package update

import (
	"strconv"
	"strings"
)

// compareVersions returns -1, 0 or 1 for a<b, a==b, a>b and ok=false when
// either operand is not a dotted numeric version. A trailing suffix
// (1.2.3-rc1) is ignored, matching how the CI tags releases.
func compareVersions(a, b string) (int, bool) {
	pa, okA := parseVersionParts(a)
	pb, okB := parseVersionParts(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

func parseVersionParts(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return out, false
	}
	// Drop any prerelease/build suffix after the numeric core.
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	fields := strings.Split(v, ".")
	if len(fields) > 3 {
		return out, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
