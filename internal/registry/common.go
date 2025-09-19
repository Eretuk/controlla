package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type CommonRegistryClient struct {
	Client *http.Client
	Host   string
}

// NewCommonRegistryClient creates a new CommonRegistryClient for the specified registry host.
func NewCommonRegistryClient(host string) *CommonRegistryClient {
	return &CommonRegistryClient{
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		Host: host,
	}
}

// GetTags retrieves a list of tags for a given image from the registry.
// It first tries the standard Docker Registry API, and if that fails,
// it falls back to a Gitea-like Web API.
func (c *CommonRegistryClient) GetTags(image string) ([]string, error) {
	tags, err := c.getTagsFromDockerAPI(image)
	if err != nil {
		logrus.Warnf("Standard Docker API failed for %s (error: %v). Trying Web API fallback...", c.Host, err)

		// Try the Web API fallback
		fallbackTags, fallbackErr := c.getTagsFromWebAPI(image)
		if fallbackErr != nil {
			logrus.Errorf("Web API fallback also failed for %s (error: %v).", c.Host, fallbackErr)
			// Возвращаем оригинальную, более информативную ошибку от Docker API.
			return nil, err
		}

		logrus.Infof("Successfully fetched tags for %s using Web API fallback.", c.Host)
		return fallbackTags, nil
	}
	return tags, nil
}

// getTagsFromWebAPI fetches tags using a Gitea-like Web API endpoint.
func (c *CommonRegistryClient) getTagsFromWebAPI(image string) ([]string, error) {
	url := fmt.Sprintf("https://%s/%s/tags/list", c.Host, image)
	req, err := http.NewRequest("GET", url, nil)
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
		return nil, fmt.Errorf("web api request for %s failed with status %d: %s", c.Host, resp.StatusCode, string(body))
	}

	// Gitea uses "results", some others might use "tags". Let's try to be flexible.
	var result struct {
		Tags    []string `json:"tags"`
		Results []string `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) > 0 {
		return result.Results, nil
	}
	return result.Tags, nil
}

// getTagsFromDockerAPI fetches tags using the standard Docker Registry v2 API.
func (c *CommonRegistryClient) getTagsFromDockerAPI(image string) ([]string, error) {
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
		return nil, fmt.Errorf("registry %s request failed with status %d: %s", c.Host, resp.StatusCode, string(body))
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
