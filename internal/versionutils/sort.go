package versionutils

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	// Exact date tag with dots: 2023.10.05
	dateTagRegexp = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	// Date tag with dashes (with optional suffix): 2026-05-26, 2026-05-26-debian-rootless
	dateDashTagRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	// Tag that starts with a very large number (7+ digits), e.g. build timestamps.
	bigNumRegexp = regexp.MustCompile(`^\d{7,}`)
	// Four-part numeric tag: X.Y.Z.A — explicitly allowed (normalised to X.Y.Z-rev.A).
	fourPartTagRegex = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	// A segment containing 7+ consecutive digits — catches build timestamps
	// embedded in any part of the tag, e.g. "26.03.20250318" or "1.0.20231005".
	embeddedTimestampRegexp = regexp.MustCompile(`\b\d{7,}\b`)
)

// SortAndFilterTags deduplicates, sorts and filters tags.
// It removes duplicates, ignores tags with specified keywords, filters out
// weird tags (dates, big numbers), sorts them in descending SemVer order and
// returns the top N tags.
//
// Collision handling: when two tags normalise to the same SemVer string
// (e.g. "ee-1.19.2" and "1.19.2" both → "1.19.2"), the exact-match tag is
// preferred over the normalised one so that the version being tracked stays
// consistent.
func SortAndFilterTags(tags []string, n int, ignoreKeywords []string) []string {
	uniqueVersions := make(map[string]*semver.Version)
	tagMap := make(map[string]string) // semver key → original tag

	for _, tag := range tags {
		if IsIgnoredTag(tag, ignoreKeywords) {
			continue
		}
		if isWeirdTag(tag) {
			continue
		}

		normalizedTag := TryNormalizeTagForSemVer(tag)
		cleanTag := strings.TrimPrefix(normalizedTag, "v")
		v, err := semver.NewVersion(cleanTag)
		if err != nil {
			continue
		}

		key := v.String()
		if existing, seen := tagMap[key]; seen {
			// Prefer the tag that doesn't need normalisation (i.e. is already
			// a valid SemVer or just has a "v" prefix).
			existingNorm := TryNormalizeTagForSemVer(existing)
			tagNorm := TryNormalizeTagForSemVer(tag)
			if existingNorm != existing && tagNorm == tag {
				// Current tag is "cleaner" — replace.
				tagMap[key] = tag
			}
			// Otherwise keep what we already have.
		} else {
			uniqueVersions[key] = v
			tagMap[key] = tag
		}
	}

	var semverTags []*semver.Version
	for _, v := range uniqueVersions {
		semverTags = append(semverTags, v)
	}

	// Sort in descending order
	sort.Sort(sort.Reverse(semver.Collection(semverTags)))

	// Filter to top N
	if len(semverTags) > n {
		semverTags = semverTags[:n]
	}

	var result []string
	for _, v := range semverTags {
		// retrieve original tag, if that was with 'v' prefix, keep it
		result = append(result, tagMap[v.String()])
	}

	return result
}

// RemoveDuplicatesAndSort removes duplicate tags and sorts them semantically.
func RemoveDuplicatesAndSort(tags []string) []string {
	// Use a map to track unique tags
	uniqueMap := make(map[string]struct{})
	for _, tag := range tags {
		uniqueMap[tag] = struct{}{}
	}

	// Create a slice from the map keys
	uniqueTags := make([]string, 0, len(uniqueMap))
	for tag := range uniqueMap {
		uniqueTags = append(uniqueTags, tag)
	}

	// Sort the unique tags using semver
	sort.Slice(uniqueTags, func(i, j int) bool {
		v1, err1 := semver.NewVersion(uniqueTags[i])
		v2, err2 := semver.NewVersion(uniqueTags[j])

		// If either tag is not a valid semver, fall back to string comparison
		if err1 != nil || err2 != nil {
			return uniqueTags[i] > uniqueTags[j]
		}

		// Sort in descending order
		return v1.GreaterThan(v2)
	})

	return uniqueTags
}

// Check if a tag should be ignored based on keywords.
func IsIgnoredTag(tag string, ignoreKeywords []string) bool {
	lowerTag := strings.ToLower(tag)
	for _, keyword := range ignoreKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}
	return false
}

func isWeirdTag(tag string) bool {
	// Explicitly allow X.Y.Z.A tags — they are normalised to X.Y.Z-rev.A by
	// TryNormalizeTagForSemVer and must not be discarded here.
	if fourPartTagRegex.MatchString(tag) {
		return false
	}

	// Filter out pure date tags like 2023.10.05.
	if dateTagRegexp.MatchString(tag) {
		return true
	}

	// Filter out date-prefixed tags like 2026-05-26, 2026-05-26-debian-rootless.
	if dateDashTagRegexp.MatchString(tag) {
		return true
	}

	// Filter out tags containing a build timestamp anywhere in them,
	// e.g. "26.03.20250318", "1.0.20231005000000". A 7+ digit numeric segment
	// is a strong signal for a CI timestamp rather than a real version number.
	if embeddedTimestampRegexp.MatchString(tag) {
		return true
	}

	// Filter out tags that start with a very large number (7+ digits leading)
	// and whose first segment exceeds the plausible year range.
	if bigNumRegexp.MatchString(tag) {
		parts := strings.Split(tag, ".")
		if len(parts) > 0 {
			if n, err := strconv.Atoi(parts[0]); err == nil && n > 2100 {
				return true
			}
		}
	}

	return false
}
