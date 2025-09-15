package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type GhcrClient struct {
	Client *http.Client
	Token  string
}

func NewGhcrClient(token string) *GhcrClient {
	return &GhcrClient{
		Client: &http.Client{},
		Token:  token,
	}
}

// GetTags retrieves all image tags for a given image.
func (c *GhcrClient) GetTags(image string) ([]string, error) {
	url := fmt.Sprintf("https://ghcr.io/v2/%s/tags/list", image)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
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
