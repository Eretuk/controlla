package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GhcrClient struct {
	Client *http.Client
}

func NewGhcrClient() *GhcrClient {
	return &GhcrClient{
		Client: &http.Client{},
	}
}

// GetTags retrieves all image tags for a given image.
func (c *GhcrClient) GetTags(image string) ([]string, error) {
	// Step 1: Request an anonymous token
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

	// Step 2: Use the token to get the tags list
	url := fmt.Sprintf("https://ghcr.io/v2/%s/tags/list", image)
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tags request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get GHCR tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GHCR request failed with status code: %d", resp.StatusCode)
	}

	var result map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode GHCR response: %w", err)
	}

	var tags []string
	if err := json.Unmarshal(result["tags"], &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags from response: %w", err)
	}

	return tags, nil
}
