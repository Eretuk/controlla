package checker

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ComposeFile struct {
	Services map[string]struct {
		Image string `yaml:"image"`
	} `yaml:"services"`
}

// FindImagesFromCompose reads a docker-compose.yml file and returns a list of images.
func FindImagesFromCompose(filePath string) ([]string, error) {
	// Read the file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var composeFile ComposeFile
	// Unmarshal the YAML content into our struct
	err = yaml.Unmarshal(data, &composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", filePath, err)
	}

	var images []string
	// Iterate through the services and extract the image names
	for _, service := range composeFile.Services {
		if service.Image != "" {
			images = append(images, service.Image)
		}
	}

	return images, nil
}

// FindRelevantFiles recursively searches a directory for specific file types.
// It returns a slice of absolute paths to the files found.
func FindRelevantFiles(searchDir string) ([]string, error) {
	var filePaths []string

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// This handles errors during traversal, such as permissions issues.
			return fmt.Errorf("error accessing path %q: %v", path, err)
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check for specific file names or patterns.
		// You can easily add more conditions here.
		fileName := d.Name()
		if fileName == "docker-compose.yml" ||
			fileName == "docker-compose.yaml" ||
			fileName == "Dockerfile" {
			filePaths = append(filePaths, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return filePaths, nil
}
