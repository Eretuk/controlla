package versionutils

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
)

var (
	dateTagRegexp = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	bigNumRegexp  = regexp.MustCompile(`^\d{7,}`)
)

// Deduplicate, sort and filter tags.
// It removes duplicates, ignores tags with specified keywords, filters out weird tags (dates, big numbers),
// sorts them in descending order and returns the top N tags.
// It preserves the original tag format (with 'v' prefix if present).
func SortAndFilterTags(tags []string, n int, ignoreKeywords []string) []string {
	uniqueVersions := make(map[string]*semver.Version)
	tagMap := make(map[string]string) // semver.String() -> original tag

	for _, tag := range tags {
		if IsIgnoredTag(tag, ignoreKeywords) {
			continue
		}
		if isWeirdTag(tag) {
			continue
		}
		cleanTag := strings.TrimPrefix(tag, "v")
		v, err := semver.NewVersion(cleanTag)
		if err == nil {
			uniqueVersions[v.String()] = v
			tagMap[v.String()] = tag // сохраняем оригинальный тег!
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
	// Filter out date-like tags (e.g., 2023.10.05) and big number tags (e.g., 9799770991.0.0)
	// as they are unlikely to be valid SemVer tags.
	if dateTagRegexp.MatchString(tag) {
		return true
	}
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
