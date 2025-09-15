package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type DockerHubClient struct {
	Client *http.Client
	User   string
	Pass   string
}

func NewDockerHubClient(user, pass string) *DockerHubClient {
	return &DockerHubClient{
		Client: &http.Client{},
		User:   user,
		Pass:   pass,
	}
}

// GetTags retrieves all image tags for a given image.
func (c *DockerHubClient) GetTags(image string) ([]string, error) {
	// 1. Get auth token
	authURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.com&scope=repository:%s:pull", image)
	req, _ := http.NewRequest(http.MethodGet, authURL, nil)
	req.SetBasicAuth(c.User, c.Pass)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker hub auth failed with status: %d", resp.StatusCode)
	}

	var authData map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}
	token := authData["token"]

	// 2. Use the token to get the tags
	tagsURL := fmt.Sprintf("https://registry.hub.docker.com/v2/%s/tags/?page_size=100", image)
	req, _ = http.NewRequest(http.MethodGet, tagsURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode tags response: %w", err)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(result["results"], &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags from response: %w", err)
	}

	var tagNames []string
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}

	return tagNames, nil
}
