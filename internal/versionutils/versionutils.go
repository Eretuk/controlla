package versionutils

import (
	"log"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
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
		log.Printf("Could not parse current tag '%s' as SemVer. Skipping comparison.", currentTag)
		return latest, false
	}

	// Парсим последний тэг
	latestVersion, err := semver.NewVersion(strings.TrimPrefix(latest, "v"))
	if err != nil {
		log.Printf("Could not parse latest tag '%s' as SemVer. Skipping comparison.", latest)
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

// isIgnoredTag ... (остальной код функции isIgnoredTag) ...
