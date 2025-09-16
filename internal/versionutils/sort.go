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
		// More flexible logic for type "1.29.1-alpine"
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

// RemoveDuplicatesAndSort removes duplicate tags and sorts them semantically.
func RemoveDuplicatesAndSort(tags []string) []string {
	// Используем map для хранения уникальных тегов
	uniqueMap := make(map[string]struct{})
	for _, tag := range tags {
		uniqueMap[tag] = struct{}{}
	}

	// Создаем новый срез из ключей map
	uniqueTags := make([]string, 0, len(uniqueMap))
	for tag := range uniqueMap {
		uniqueTags = append(uniqueTags, tag)
	}

	// Сортируем теги семантически
	sort.Slice(uniqueTags, func(i, j int) bool {
		v1, err1 := semver.NewVersion(uniqueTags[i])
		v2, err2 := semver.NewVersion(uniqueTags[j])

		// Если одна из версий не является семантической, используем строковое сравнение
		if err1 != nil || err2 != nil {
			return uniqueTags[i] > uniqueTags[j]
		}

		// Сортируем в порядке убывания (descending)
		return v1.GreaterThan(v2)
	})

	return uniqueTags
}

// GetLatestTags takes a slice of raw tags, removes duplicates, sorts them semantically,
// and returns a specified number of the latest tags.
func GetLatestTags(tags []string, count int) []string {
	// 1. Фильтруем теги, чтобы убрать лишние
	filteredTags := []string{}
	for _, tag := range tags {
		if !isIgnoredTag(tag) {
			filteredTags = append(filteredTags, tag)
		}
	}

	// 2. Удаляем дубликаты и парсим как SemVer
	uniqueVersions := make(map[string]*semver.Version)
	for _, tag := range filteredTags {
		// Убираем префикс 'v' для корректного парсинга
		cleanTag := strings.TrimPrefix(tag, "v")

		v, err := semver.NewVersion(cleanTag)
		if err == nil {
			uniqueVersions[v.String()] = v
		} else {
			// Опционально, можно логировать теги, которые не удалось распарсить
			// log.Printf("Skipping non-semver tag: %s", tag)
		}
	}

	// ... (Остальная часть функции остается такой же) ...
	// ... (3. Создаем срез из уникальных версий) ...
	// ... (4. Сортируем версии в порядке убывания) ...
	// ... (5. Фильтруем теги до нужного количества) ...
	// ... (6. Преобразуем обратно в строки) ...

	// 2. Создаем срез из уникальных версий
	uniqueSemverTags := make([]*semver.Version, 0, len(uniqueVersions))
	for _, v := range uniqueVersions {
		uniqueSemverTags = append(uniqueSemverTags, v)
	}

	// 3. Сортируем версии в порядке убывания (от новейших к старейшим)
	sort.Sort(sort.Reverse(semver.Collection(uniqueSemverTags)))

	// 4. Фильтруем теги до нужного количества
	if len(uniqueSemverTags) > count {
		uniqueSemverTags = uniqueSemverTags[:count]
	}

	// 5. Преобразуем обратно в строки
	var result []string
	for _, v := range uniqueSemverTags {
		result = append(result, v.String())
	}

	return result
}

// isIgnoredTag checks if a tag contains keywords that should be ignored.
func isIgnoredTag(tag string) bool {
	// Список игнорируемых слов и фраз
	ignoredKeywords := []string{"alpha", "beta", "rc", "test", "testing", "preview", "amd64", "arm64", "linux", "windows", "debug"}

	lowerTag := strings.ToLower(tag)
	for _, keyword := range ignoredKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}
	return false
}
