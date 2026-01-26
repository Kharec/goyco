package version

import (
	"regexp"
	"testing"
)

func TestVersionSemver(t *testing.T) {
	semverRegex := regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	if !semverRegex.MatchString(GetVersion()) {
		t.Errorf("Version %q does not follow semantic versioning format (MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD])", GetVersion())
	}
}

func TestVersionSemverFlexible(t *testing.T) {
	flexibleSemverRegex := regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	if !flexibleSemverRegex.MatchString(GetVersion()) {
		t.Errorf("Version %q does not follow semantic versioning format (MAJOR.MINOR[.PATCH][-PRERELEASE][+BUILD])", GetVersion())
	}
}

func TestVersionNotEmpty(t *testing.T) {
	if GetVersion() == "" {
		t.Error("Version should not be empty")
	}
}

func TestVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`\d+\.\d+`).MatchString(GetVersion()) {
		t.Errorf("Version %q should contain at least MAJOR.MINOR format", GetVersion())
	}
}

func TestVersionStartsWithNumber(t *testing.T) {
	if !regexp.MustCompile(`^\d+`).MatchString(GetVersion()) {
		t.Errorf("Version %q should start with a number", GetVersion())
	}
}

func TestVersionNoLeadingZeros(t *testing.T) {
	parts := regexp.MustCompile(`^(\d+)\.(\d+)`).FindStringSubmatch(GetVersion())
	if len(parts) >= 3 {
		major := parts[1]
		minor := parts[2]

		if len(major) > 1 && major[0] == '0' {
			t.Errorf("Major version %q should not have leading zeros", major)
		}

		if len(minor) > 1 && minor[0] == '0' {
			t.Errorf("Minor version %q should not have leading zeros", minor)
		}
	}
}

func TestVersionSemverExamples(t *testing.T) {
	testCases := []struct {
		version string
		valid   bool
		desc    string
	}{
		{"1.0.0", true, "Standard semver"},
		{"1.0", false, "Missing patch version"},
		{"1.0.0-alpha", true, "With prerelease"},
		{"1.0.0-alpha.1", true, "With prerelease and number"},
		{"1.0.0+20130313144700", true, "With build metadata"},
		{"1.0.0-alpha+20130313144700", true, "With prerelease and build metadata"},
		{"0.1.0", true, "Zero major version"},
		{"01.0.0", false, "Leading zero in major"},
		{"1.01.0", false, "Leading zero in minor"},
		{"1.0.01", false, "Leading zero in patch"},
		{"1.0.0-", false, "Invalid prerelease"},
		{"1.0.0+", false, "Invalid build metadata"},
		{"v1.0.0", false, "Version prefix not allowed"},
		{"", false, "Empty version"},
	}

	semverRegex := regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := semverRegex.MatchString(tc.version)
			if result != tc.valid {
				t.Errorf("Version %q: expected valid=%v, got valid=%v", tc.version, tc.valid, result)
			}
		})
	}
}
