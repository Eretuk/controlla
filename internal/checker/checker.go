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

type ComposeFile struct {
	Services map[string]struct {
		Image string `yaml:"image"`
	} `yaml:"services"`
}

type ParsedImage struct {
	Registry string
	Image    string
	Tag      string
	Digest   string
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

// ParseDockerImage takes a full image string and breaks it into its components.
func ParseDockerImage(imageString string) ParsedImage {
	parsed := ParsedImage{}

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
		// A tag must not contain a slash unless it's a registry port.
		// For example, ghcr.io:5000/image:tag
		if strings.Contains(partsWithTag[len(partsWithTag)-1], "/") {
			// This is likely a registry with a port, not a tag.
			// Leave the tag as empty and handle the registry/image separation later.
		} else {
			parsed.Image = strings.Join(partsWithTag[:len(partsWithTag)-1], ":")
			parsed.Tag = partsWithTag[len(partsWithTag)-1]
		}
	} else {
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
