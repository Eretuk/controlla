package checker

import "strings"

type ParsedImage struct {
	Registry string
	Image    string
	Tag      string
	Digest   string
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
