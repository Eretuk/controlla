package versionutils

import (
	"log"
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

// GetLatestTags ... (остальной код функции GetLatestTags) ...

// isIgnoredTag ... (остальной код функции isIgnoredTag) ...
