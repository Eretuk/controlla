package registry

import (
	"encoding/json"
	"fmt"
	"io"
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
	authReq, _ := http.NewRequest(http.MethodGet, authURL, nil)
	authReq.SetBasicAuth(c.User, c.Pass)

	authResp, err := c.Client.Do(authReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(authResp.Body)
		return nil, fmt.Errorf("auth token request failed with status %d: %s", authResp.StatusCode, string(body))
	}

	var authData map[string]string
	if err := json.NewDecoder(authResp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}
	token := authData["token"]

	// Now, use the token to get the tags from the registry API.
	tagsURL := fmt.Sprintf("https://registry.hub.docker.com/v2/%s/tags/?page_size=100", image)
	tagsReq, _ := http.NewRequest(http.MethodGet, tagsURL, nil)
	tagsReq.Header.Set("Authorization", "Bearer "+token)

	tagsResp, err := c.Client.Do(tagsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer tagsResp.Body.Close()

	if tagsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tagsResp.Body)
		return nil, fmt.Errorf("tags request failed with status %d: %s", tagsResp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.NewDecoder(tagsResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse tags response: %w", err)
	}

	var tagNames []string
	for _, t := range result.Results {
		tagNames = append(tagNames, t.Name)
	}

	return tagNames, nil
}
