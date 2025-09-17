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

// SkipList управляет списком пропущенных обновлений.
type SkipList struct {
	sync.Mutex
	Path    string
	Updates []SkippedUpdate `yaml:"updates"`
}

// NewSkipList создает новый экземпляр SkipList.
func NewSkipList(path string) *SkipList {
	return &SkipList{
		Path: path,
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

	return yaml.Unmarshal(data, &sl.Updates)
}

// Add добавляет обновление в список и сохраняет его в файл.
func (sl *SkipList) Add(image, newTag string) error {
	sl.Lock()
	defer sl.Unlock()

	// Проверяем, не было ли уже добавлено это обновление
	for _, update := range sl.Updates {
		if update.Image == image && update.NewTag == newTag {
			return nil
		}
	}

	sl.Updates = append(sl.Updates, SkippedUpdate{
		Image:  image,
		NewTag: newTag,
	})

	data, err := yaml.Marshal(&sl)
	if err != nil {
		return fmt.Errorf("failed to marshal skip list: %w", err)
	}

	return os.WriteFile(sl.Path, data, 0644)
}

// IsSkipped проверяет, есть ли обновление в списке пропущенных.
func (sl *SkipList) IsSkipped(image, newTag string) bool {
	sl.Lock()
	defer sl.Unlock()

	for _, update := range sl.Updates {
		if update.Image == image && update.NewTag == newTag {
			return true
		}
	}
	return false
}
