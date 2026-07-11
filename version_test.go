package utils_test

import (
	"regexp"
	"testing"

	utils "github.com/danielPoloWork/egl-utils-go"
)

// TestVersionIsSemVer pins Version to strict X.Y.Z form so the consistency
// lint's version-lockstep gate always has a parseable source of truth. Being
// an external test package, it also compiles the module's public import path,
// mechanically verifying the consumer-facing import promised by ADR-0003.
func TestVersionIsSemVer(t *testing.T) {
	t.Parallel()

	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !semver.MatchString(utils.Version) {
		t.Fatalf("Version %q is not strict X.Y.Z SemVer", utils.Version)
	}
}
