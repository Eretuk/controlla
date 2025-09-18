package versionutils

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
)

// SortAndFilterTags удаляет дубликаты, фильтрует теги по SemVer,
// сортирует их и возвращает N последних тегов.
func SortAndFilterTags(tags []string, n int) []string {
	// 1. Используем map для хранения уникальных тегов
	uniqueVersions := make(map[string]*semver.Version)
	for _, tag := range tags {
		// Убираем префикс "v" и другие возможные
		cleanTag := strings.TrimPrefix(tag, "v")

		// Пытаемся распарсить как SemVer.
		// Это автоматически отфильтрует все некорректные теги.
		v, err := semver.NewVersion(cleanTag)
		if err == nil {
			uniqueVersions[v.String()] = v
		}
	}

	// 2. Преобразуем map в срез для сортировки
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

	// 5. Преобразуем обратно в строки
	var result []string
	for _, v := range semverTags {
		result = append(result, v.String())
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
	lowerTag := strings.ToLower(tag)

	// Игнорируем теги с архитектурой, статусом и ОС
	ignoredKeywords := []string{
		"alpha", "beta", "rc", "test", "testing", "preview", "next", "latest",
		"amd64", "arm64", "linux", "windows", "i386", "armhf", "aarch64", "ppc64le",
		"bookworm", "trixie", "ubuntu", "buster", "bullseye", "debug",
	}
	for _, keyword := range ignoredKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}

	// Дополнительная проверка на нестандартные теги
	parts := strings.Split(tag, ".")
	if len(parts) > 3 {
		// Игнорируем теги с более чем 3 точками (например, 3.5.5.0, RELEASE.2025...)
		return true
	}

	// Игнорируем теги, которые начинаются с очень больших чисел (как в Grafana)
	firstPart, err := strconv.Atoi(parts[0])
	if err == nil && firstPart > 100000000 {
		return true
	}

	return false
}
