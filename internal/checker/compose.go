package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ImageInfo struct {
	Path    string // полный путь до docker-compose.yml
	Name    string // образ целиком
	Host    string // имя хоста (напр. "auth-stack")
	Service string // имя сервиса (напр. "authelia")
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
			continue // должно быть как минимум repos/myrepo/services/host/service/docker-compose.yml
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
