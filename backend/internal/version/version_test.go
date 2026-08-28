package version

import (
	"regexp"
	"testing"
)

func TestVersionIsReleaseLike(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}

	if !regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`).MatchString(Version) {
		t.Fatalf("Version %q is not release-like", Version)
	}
}
