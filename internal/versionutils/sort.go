package versionutils

import (
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
)

func SortAndFilterTags(tags []string, n int) []string {
	// Filter and parsing tags
	var versions []*semver.Version
	re := regexp.MustCompile(`^(\d+\.\d+\.\d+.*)`)

	for _, tag := range tags {
		// Используем более гибкую логику для обработки тегов типа "1.29.1-alpine"
		if re.MatchString(tag) {
			v, err := semver.NewVersion(strings.Split(tag, "-")[0])
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	// Sort in descend mode
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].GreaterThan(versions[j])
	})

	// Return N tags
	if n > len(versions) {
		n = len(versions)
	}

	latestTags := make([]string, n)
	for i := 0; i < n; i++ {
		latestTags[i] = versions[i].Original()
	}

	return latestTags
}
