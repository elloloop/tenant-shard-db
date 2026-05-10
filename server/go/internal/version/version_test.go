package version

import "testing"

func TestVersionDefaultsToDev(t *testing.T) {
	// The package-level default must be "dev"; release builds override via
	// -ldflags. If a Go test run sees a non-"dev" value, the test binary
	// itself was linked with -X, which would be a misconfiguration.
	if Version != "dev" {
		t.Fatalf("Version default = %q, want %q", Version, "dev")
	}
}

func TestStringReturnsVersion(t *testing.T) {
	prev := Version
	t.Cleanup(func() { Version = prev })

	cases := []string{"dev", "v1.2.3", "v0.0.0-dev+local", ""}
	for _, want := range cases {
		Version = want
		if got := String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}
