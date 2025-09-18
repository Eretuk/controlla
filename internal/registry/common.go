package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type CommonRegistryClient struct {
	Client *http.Client
	Host   string
}

// NewCommonRegistryClient creates a new CommonRegistryClient for the specified registry host.
func NewCommonRegistryClient(host string) *CommonRegistryClient {
	return &CommonRegistryClient{
		Client: &http.Client{},
		Host:   host,
	}
}

// GetTags fetches the list of tags for a given image from the registry.
func (c *CommonRegistryClient) GetTags(image string) ([]string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", c.Host, image)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry %s request failed: %s", c.Host, string(body))
	}
	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Tags, nil
}

// GetDigest fetches the digest for a specific image:tag from the registry.
func (c *CommonRegistryClient) GetDigest(image, tag string) (string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", c.Host, image, tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("digest request failed: %s", string(body))
	}
	return resp.Header.Get("Docker-Content-Digest"), nil
}
