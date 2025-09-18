package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GhcrClient struct {
	Client *http.Client
}

func NewGhcrClient() *GhcrClient {
	return &GhcrClient{
		Client: &http.Client{},
	}
}

// GetTags retrieves all image tags for a given image, handling pagination.
func (c *GhcrClient) GetTags(image string) ([]string, error) {
	// Request an anonymous token
	authURL := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s:pull", image)
	req, err := http.NewRequest(http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get GHCR token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GHCR token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}
	token := authData.Token

	// Step 2: Use the token to get the tags list with pagination
	var allTags []string
	url := fmt.Sprintf("https://ghcr.io/v2/%s/tags/list?n=200", image)
	for {
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create tags request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get GHCR tags: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GHCR request failed with status code: %d", resp.StatusCode)
		}

		var result map[string]json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode GHCR response: %w", err)
		}
		resp.Body.Close()

		var tags []string
		if err := json.Unmarshal(result["tags"], &tags); err != nil {
			return nil, fmt.Errorf("failed to parse tags from response: %w", err)
		}
		allTags = append(allTags, tags...)

		// Check for pagination
		link := resp.Header.Get("Link")
		if link == "" || !containsRelNext(link) {
			break
		}
		nextURL := parseNextURL(link)
		if nextURL == "" {
			break
		}
		// Ensure the nextURL is absolute
		if strings.HasPrefix(nextURL, "/") {
			nextURL = "https://ghcr.io" + nextURL
		}
		url = nextURL
	}
	return allTags, nil
}

// containsRelNext checks if the Link header contains rel="next"
func containsRelNext(link string) bool {
	return strings.Contains(link, `rel="next"`)
}

// parseNextURL extracts the next page URL from the Link header
func parseNextURL(link string) string {
	// Example: <https://ghcr.io/v2/immich-app/immich-server/tags/list?next_page_token=...&n=200>; rel="next"
	parts := strings.Split(link, ";")
	if len(parts) < 2 {
		return ""
	}
	urlPart := strings.TrimSpace(parts[0])
	urlPart = strings.Trim(urlPart, "<>")
	return urlPart
}

func (c *GhcrClient) GetDigest(image, tag string) (string, error) {
	url := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", image, tag)
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
