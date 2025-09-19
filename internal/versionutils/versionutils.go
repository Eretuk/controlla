package versionutils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
)

// CheckForUpdate compares the current tag with the latest tag from the registry.
// It returns the latest available tag and a boolean indicating if an update is available.
func CheckForUpdate(currentTag string, latestTags []string) (string, bool) {
	if len(latestTags) == 0 {
		return "", false
	}

	latest := latestTags[0]

	// Если текущий тэг совпадает с последним, обновления нет
	if latest == currentTag {
		return latest, false
	}

	// Парсим текущий тэг
	currentVersion, err := semver.NewVersion(strings.TrimPrefix(currentTag, "v"))
	if err != nil {
		logrus.Errorf("Could not parse current tag '%s' as SemVer. Skipping comparison.", currentTag)
		return latest, false
	}

	// Парсим последний тэг
	latestVersion, err := semver.NewVersion(strings.TrimPrefix(latest, "v"))
	if err != nil {
		logrus.Errorf("Could not parse latest tag '%s' as SemVer. Skipping comparison.", latest)
		return latest, false
	}

	// Сравниваем версии
	isUpdateAvailable := latestVersion.GreaterThan(currentVersion)

	return latest, isUpdateAvailable
}

// GetLatestTags returns the N latest SemVer tags from a list of tags.
// It now supports tags with 'v' prefix and ignores pre-releases unless no stable found.
func GetLatestTags(tags []string, count int) []string {
	var versions []*semver.Version
	tagMap := make(map[string]string) // semver.String() -> original tag

	for _, tag := range tags {
		// Remove possible 'v' prefix for semver parsing
		cleanTag := strings.TrimPrefix(tag, "v")
		v, err := semver.NewVersion(cleanTag)
		if err == nil {
			versions = append(versions, v)
			tagMap[v.String()] = tag
		}
	}

	if len(versions) == 0 {
		return nil
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))

	var latest []string
	for _, v := range versions {
		latest = append(latest, tagMap[v.String()])
		if len(latest) >= count {
			break
		}
	}
	return latest
}

func NormalizeTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// TryNormalizeTagForSemVer attempts to normalize a non-standard tag into a SemVer-compatible format.
// For example, "1.2.3.4" might become "1.2.3-rev.4".
// This is a heuristic approach for specific non-SemVer patterns.
func TryNormalizeTagForSemVer(tag string) string {
	// If it already parses as SemVer, no need to normalize
	if _, err := semver.NewVersion(strings.TrimPrefix(tag, "v")); err == nil {
		return tag
	}

	// Pattern: X.Y.Z.A (e.g., 1.2.3.4)
	parts := strings.Split(tag, ".")
	if len(parts) == 4 {
		// Try to form X.Y.Z-rev.A
		if _, err := strconv.Atoi(parts[3]); err == nil { // Check if the last part is a number
			normalized := fmt.Sprintf("%s.%s.%s-rev.%s", parts[0], parts[1], parts[2], parts[3])
			if _, err := semver.NewVersion(normalized); err == nil {
				logrus.Debugf("Normalized tag '%s' to '%s' for SemVer parsing.", tag, normalized)
				return normalized
			}
		}
	}

	// Add other normalization patterns here if needed

	return tag // Return original tag if no normalization helps
}
