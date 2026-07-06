// В файле internal/skiplist/skiplist.go

package skiplist

import (
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// structure to represent a skipped update
type SkippedUpdate struct {
	Image  string `yaml:"image"`
	NewTag string `yaml:"new_tag"`
}

// fileData structure to hold both updates and ignored images
type fileData struct {
	Updates       []SkippedUpdate `yaml:"updates"`
	IgnoredImages []string        `yaml:"ignored_images"`
}

// SkipList for managing skipped updates and ignored images
type SkipList struct {
	sync.Mutex
	Path string
	data *fileData
}

// Creates a new SkipList instance
func NewSkipList(path string) *SkipList {
	return &SkipList{
		Path: path,
		data: &fileData{},
	}
}

// Load lists from the YAML file.
func (sl *SkipList) Load() error {
	sl.Lock()
	defer sl.Unlock()

	data, err := os.ReadFile(sl.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file, nothing to load
		}
		return fmt.Errorf("failed to read skip list file: %w", err)
	}

	return yaml.Unmarshal(data, sl.data)
}

// Add adds a new skipped update to the list and saves it to the file.
func (sl *SkipList) Add(image, newTag string) error {
	sl.Lock()
	defer sl.Unlock()

	// Check if the update already exists
	for _, update := range sl.data.Updates {
		if update.Image == image && update.NewTag == newTag {
			return nil
		}
	}

	sl.data.Updates = append(sl.data.Updates, SkippedUpdate{
		Image:  image,
		NewTag: newTag,
	})

	data, err := yaml.Marshal(sl.data)
	if err != nil {
		return fmt.Errorf("failed to marshal skip list: %w", err)
	}

	return os.WriteFile(sl.Path, data, 0644)
}

// Checks if a specific image and tag combination is in the skip list.
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

// Check is if an image is in the ignored images list.
func (sl *SkipList) IsImageIgnored(image string) bool {
	sl.Lock()
	defer sl.Unlock()

	for _, img := range sl.data.IgnoredImages {
		if img == image {
			return true
		}
	}
	return false
}

// AddIgnoredImage adds an image to the ignored images list and saves it to the file.
func (sl *SkipList) AddIgnoredImage(image string) error {
	sl.Lock()
	defer sl.Unlock()

	for _, img := range sl.data.IgnoredImages {
		if img == image {
			return nil // already in the list
		}
	}
	sl.data.IgnoredImages = append(sl.data.IgnoredImages, image)
	logrus.Infof("Image %s added to ignored list (skipped.yaml)", image)

	data, err := yaml.Marshal(sl.data)
	if err != nil {
		return fmt.Errorf("failed to marshal skip list: %w", err)
	}

	return os.WriteFile(sl.Path, data, 0644)
}
