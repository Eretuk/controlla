package checker

import (
	"controlla/internal/config"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

func GetImageDigest(image ParsedImage, cfg *config.Config) (string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/%s/manifests/%s",
		image.Registry, image.Namespace, image.Name, image.Tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	if strings.Contains(image.Registry, "ghcr.io") && cfg.Registry.GHCR.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Registry.GHCR.Token)
	}
	if strings.Contains(image.Registry, "docker.io") && cfg.Registry.DockerHub.Username != "" {
		req.SetBasicAuth(cfg.Registry.DockerHub.Username, cfg.Registry.DockerHub.Password)
	}
	if strings.Contains(image.Registry, "public.ecr.aws") {
		logrus.Warnf("Skipping ECR: not yet supported")
		return "", nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no Docker-Content-Digest header")
	}

	return digest, nil
}
