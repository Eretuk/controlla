package versionutils

import (
	"sort"
	"strings"

	"github.com/Masterminds/semver"
)

// SortAndFilterTags удаляет дубликаты, фильтрует теги по SemVer,
// сортирует их и возвращает N последних тегов.
func SortAndFilterTags(tags []string, n int, ignoreKeywords []string) []string {
	uniqueVersions := make(map[string]*semver.Version)
	tagMap := make(map[string]string) // semver.String() -> original tag

	for _, tag := range tags {
		if IsIgnoredTag(tag, ignoreKeywords) {
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

	// 3. Сортируем версии в порядке убывания (от новейших к старейшим)
	sort.Sort(sort.Reverse(semver.Collection(semverTags)))

	// 4. Фильтруем теги до нужного количества
	if len(semverTags) > n {
		semverTags = semverTags[:n]
	}

	var result []string
	for _, v := range semverTags {
		// возвращаем оригинальный тег (с v, если был)
		result = append(result, tagMap[v.String()])
	}

	return result
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

// IsIgnoredTag проверяет, содержит ли тег игнорируемые ключевые слова.
func IsIgnoredTag(tag string, ignoreKeywords []string) bool {
	lowerTag := strings.ToLower(tag)
	for _, keyword := range ignoreKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}
	return false
}
