package versionutils

import (
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestTryNormalizeTagForSemVer(t *testing.T) {
	cases := []struct {
		input    string
		wantParseable bool
	}{
		{"1.19.2", true},
		{"v1.19.2", true},
		{"ee-1.19.2", true},
		{"lts-3.0.1", true},
		{"v-2.0.0", true},
		{"1.2.3.4", true},
		{"latest", false},
		// "26.03.20250318" is technically valid SemVer but should be filtered
		// out by isWeirdTag (bigNumRegexp + year check), so we don't test it here.
	}

	for _, tc := range cases {
		norm := TryNormalizeTagForSemVer(tc.input)
		_, err := semver.NewVersion(strings.TrimPrefix(norm, "v"))
		got := err == nil
		if got != tc.wantParseable {
			t.Errorf("TryNormalizeTagForSemVer(%q) -> %q, parseable=%v, want %v",
				tc.input, norm, got, tc.wantParseable)
		}
	}
}

func TestSortAndFilterTagsWithPrefix(t *testing.T) {
	tags := []string{"ee-1.19.1", "ee-1.19.2", "ee-1.19.3"}
	result := SortAndFilterTags(tags, 3, []string{})
	if len(result) == 0 {
		t.Fatal("expected at least one result, got none")
	}
	if result[0] != "ee-1.19.3" {
		t.Errorf("expected first result to be ee-1.19.3, got %s", result[0])
	}
}

func TestSortAndFilterTagsCollision(t *testing.T) {
	// When both "ee-1.19.2" and "1.19.2" are present, "1.19.2" (exact SemVer)
	// should win over the normalised "ee-1.19.2".
	tags := []string{"ee-1.19.2", "1.19.2", "ee-1.19.3"}
	result := SortAndFilterTags(tags, 3, []string{})
	for _, tag := range result {
		if tag == "ee-1.19.2" {
			t.Errorf("expected ee-1.19.2 to be replaced by 1.19.2, got results: %v", result)
		}
	}
}

func TestSortAndFilterTagsDateTagsFiltered(t *testing.T) {
	// Date-like tags (2023.10.05), build-timestamp tags (26.03.20250318),
	// and date-dash tags (2026-05-26-debian-rootless) must never appear in output.
	tags := []string{
		"2023.10.05",
		"26.03.20250318",
		"2026-05-26-debian-rootless",
		"2026-05-26-debian",
		"2026-05-26",
		"1.19.2",
		"1.19.1",
	}
	result := SortAndFilterTags(tags, 5, []string{})
	badTags := []string{"2023.10.05", "26.03.20250318", "2026-05-26-debian-rootless", "2026-05-26-debian", "2026-05-26"}
	for _, bad := range badTags {
		for _, got := range result {
			if got == bad {
				t.Errorf("tag %q should have been filtered out, got: %v", bad, result)
			}
		}
	}
}
