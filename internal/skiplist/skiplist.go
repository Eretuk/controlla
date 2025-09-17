// В файле internal/skiplist/skiplist.go

package skiplist

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// SkippedUpdate представляет запись о пропущенном обновлении.
type SkippedUpdate struct {
	Image  string `yaml:"image"`
	NewTag string `yaml:"new_tag"`
}

// fileData представляет структуру, которая будет храниться в файле.
type fileData struct {
	Updates []SkippedUpdate `yaml:"updates"`
}

// SkipList управляет списком пропущенных обновлений.
type SkipList struct {
	sync.Mutex
	Path string
	data *fileData
}

// NewSkipList создает новый экземпляр SkipList.
func NewSkipList(path string) *SkipList {
	return &SkipList{
		Path: path,
		data: &fileData{},
	}
}

// Load загружает список пропущенных обновлений из файла.
func (sl *SkipList) Load() error {
	sl.Lock()
	defer sl.Unlock()

	data, err := os.ReadFile(sl.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Файла нет, это не ошибка
		}
		return fmt.Errorf("failed to read skip list file: %w", err)
	}

	// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Загружаем данные в единую структуру
	return yaml.Unmarshal(data, sl.data)
}

// Add добавляет обновление в список и сохраняет его в файл.
func (sl *SkipList) Add(image, newTag string) error {
	sl.Lock()
	defer sl.Unlock()

	// Проверяем, не было ли уже добавлено это обновление
	for _, update := range sl.data.Updates {
		if update.Image == image && update.NewTag == newTag {
			return nil
		}
	}

	sl.data.Updates = append(sl.data.Updates, SkippedUpdate{
		Image:  image,
		NewTag: newTag,
	})

	// 🚨 ИСХОДНОЕ ИСПРАВЛЕНИЕ: Сериализуем и записываем единую структуру
	data, err := yaml.Marshal(sl.data)
	if err != nil {
		return fmt.Errorf("failed to marshal skip list: %w", err)
	}

	return os.WriteFile(sl.Path, data, 0644)
}

// IsSkipped проверяет, находится ли обновление в списке пропущенных.
func (sl *SkipList) IsSkipped(image, newTag string) bool {
	sl.Lock()
	defer sl.Unlock()

	for _, update := range sl.data.Updates {
		if update.Image == image && update.NewTag == newTag {
			return true
		}
	}
	return false
}
