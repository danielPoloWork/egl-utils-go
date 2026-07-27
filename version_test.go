package utils_test

import (
	"regexp"
	"testing"

	utils "github.com/danielPoloWork/egl-utils-go/v2"
)

// TestVersionIsSemVer pins Version to strict X.Y.Z form so the consistency
// lint's version-lockstep gate always has a parseable source of truth. Being
// an external test package, it also compiles the module's public import path,
// mechanically verifying the consumer-facing import — the `/v2` major suffix
// included, which is the part a mechanical check earns its keep on: a module
// path that forgets its major suffix still builds locally and breaks only for
// consumers.
func TestVersionIsSemVer(t *testing.T) {
	t.Parallel()

	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !semver.MatchString(utils.Version) {
		t.Fatalf("Version %q is not strict X.Y.Z SemVer", utils.Version)
	}
}
