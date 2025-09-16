package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ImageInfo struct {
	Path    string
	Name    string
	Host    string
	Service string
}

func FindImagesInCompose(root string) ([]ImageInfo, error) {
	var images []ImageInfo

	pattern := filepath.Join(root, "*", "*", "docker-compose.yml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob files: %w", err)
	}

	for _, path := range matches {
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 4 {
			continue
		}
		host := parts[len(parts)-3]
		service := parts[len(parts)-2]

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}

		var parsed map[string]interface{}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse YAML %s: %w", path, err)
		}

		services, ok := parsed["services"].(map[string]interface{})
		if !ok {
			continue
		}

		for _, svc := range services {
			svcMap, ok := svc.(map[string]interface{})
			if !ok {
				continue
			}
			if img, ok := svcMap["image"].(string); ok {
				images = append(images, ImageInfo{
					Path:    path,
					Name:    img,
					Host:    host,
					Service: service,
				})
			}
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
