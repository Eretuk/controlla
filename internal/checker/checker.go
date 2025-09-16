package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DockerImageInfo содержит информацию об образе из Docker Registry.
type DockerImageInfo struct {
	Registry string
	Image    string
	Tag      string
	Digest   string
}

// ServiceInfo содержит всю информацию о сервисе и его образе.
type ServiceInfo struct {
	FilePath    string
	ServiceName string
	ImageInfo   DockerImageInfo
}

// FindRelevantFiles рекурсивно ищет файлы docker-compose.yml и docker-compose.yaml в директории.
func FindRelevantFiles(searchDir string) ([]string, error) {
	var filePaths []string

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %q: %v", path, err)
		}

		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		if fileName == "docker-compose.yml" || fileName == "docker-compose.yaml" {
			filePaths = append(filePaths, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return filePaths, nil
}

// ParseRelevantFiles считывает и парсит указанные файлы, возвращая список ServiceInfo.
func ParseRelevantFiles(filePaths []string) ([]ServiceInfo, error) {
	var services []ServiceInfo

	for _, filePath := range filePaths {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		var composeData struct {
			Services map[string]struct {
				Image string `yaml:"image"`
			} `yaml:"services"`
		}

		if err := yaml.Unmarshal(data, &composeData); err != nil {
			continue // Игнорируем файлы с ошибками YAML
		}

		for serviceName, service := range composeData.Services {
			if service.Image == "" {
				continue // Пропускаем сервисы без указанного образа
			}

			parsedImage := ParseDockerImage(service.Image)

			serviceInfo := ServiceInfo{
				FilePath:    filePath,
				ServiceName: serviceName,
				ImageInfo:   parsedImage,
			}
			services = append(services, serviceInfo)
		}
	}

	return services, nil
}

// ParseDockerImage разбивает строку образа на компоненты.
func ParseDockerImage(imageString string) DockerImageInfo {
	parsed := DockerImageInfo{}

	// Step 1: Separate the digest and the image/tag part.
	partsWithDigest := strings.Split(imageString, "@sha256:")
	if len(partsWithDigest) > 1 {
		parsed.Image = partsWithDigest[0]
		parsed.Digest = partsWithDigest[1]
	} else {
		parsed.Image = imageString
	}

	// Step 2: Separate the tag and the image part.
	partsWithTag := strings.Split(parsed.Image, ":")
	if len(partsWithTag) > 1 {
		if !strings.Contains(partsWithTag[len(partsWithTag)-1], "/") {
			parsed.Image = strings.Join(partsWithTag[:len(partsWithTag)-1], ":")
			parsed.Tag = partsWithTag[len(partsWithTag)-1]
		}
	}
	if parsed.Tag == "" {
		parsed.Tag = "latest"
	}

	// Step 3: Separate the registry from the image name.
	if strings.Contains(parsed.Image, "/") {
		split := strings.SplitN(parsed.Image, "/", 2)
		parsed.Registry = split[0]
		parsed.Image = split[1]
	} else {
		parsed.Registry = "docker.io"
		parsed.Image = "library/" + parsed.Image
	}

	return parsed
}
