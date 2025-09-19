package checker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
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

// FindRelevantFiles looking for relevant files in the repository or specified watchFolder.
func FindRelevantFiles(repoPath, watchFolder string) ([]string, error) {
	var relevantFiles []string
	searchPath := repoPath

	// Если указана watchFolder и она не пуста, добавляем ее к пути поиска
	if watchFolder != "" {
		searchPath = filepath.Join(repoPath, watchFolder)
		logrus.Debugf("Looking for relevant files in folder: %s", searchPath)
	} else {
		logrus.Debugf("Looking for relevant files in repository: %s", searchPath)
	}

	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logrus.Errorf("Error with path %s: %v", path, err)
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), "docker-compose.yaml") || strings.HasSuffix(d.Name(), "docker-compose.yml")) {
			relevantFiles = append(relevantFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error in search in directory: %w", err)
	}

	return relevantFiles, nil
}

// Serarch and Parse relevant files and extract service and image information.
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
			continue // Ignore files that cannot be parsed
		}

		for serviceName, service := range composeData.Services {
			if service.Image == "" {
				continue // Skip services without an image
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

// ParseDockerImage decomposes a Docker image string into its components:
// registry, image name, tag, and digest.
// It handles various formats of Docker image strings.
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
