package checker

import (
	"strings"
)

type ParsedImage struct {
	Full      string
	Registry  string
	Namespace string
	Name      string
	Tag       string
}

func ParseImageName(image string) ParsedImage {
	out := ParsedImage{Full: image}

	// 1. tag (после : )
	tagSplit := strings.Split(image, ":")
	repo := tagSplit[0]
	if len(tagSplit) > 1 {
		out.Tag = tagSplit[1]
	} else {
		out.Tag = "latest"
	}

	// 2. разбить на части
	parts := strings.Split(repo, "/")

	switch len(parts) {
	case 1:
		out.Registry = "docker.io"
		out.Namespace = "library"
		out.Name = parts[0]
	case 2:
		out.Registry = "docker.io"
		out.Namespace = parts[0]
		out.Name = parts[1]
	default:
		out.Registry = parts[0]
		out.Namespace = parts[1]
		out.Name = strings.Join(parts[2:], "/")
	}

	return out
}
