package registry

import (
	"controlla/internal/checker"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DockerTagsResponse represents the JSON response from the Docker Hub tags endpoint.
type DockerTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// DockerTag represents a single tag object in the JSON response.
type DockerTag struct {
	Name string `json:"name"`
}

// DockerAuthResponse represents the JSON response from the Docker Hub auth endpoint.
type DockerAuthResponse struct {
	Token string `json:"token"`
}
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

func (c *DockerHubClient) GetTags(image string) ([]string, error) {
	// Get token from registry with username and password
	authURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", image)

	req, err := http.NewRequest(http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}

	if c.User != "" {
		req.SetBasicAuth(c.User, c.Pass)
	}

	authResp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker Hub token: %w", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Docker Hub token with status: %d", authResp.StatusCode)
	}

	var authData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}
	token := authData.Token

	// Getting acess to registry
	tagsURL := fmt.Sprintf("https://registry-1.docker.io/v2/%s/tags/list", image)

	req, err = http.NewRequest(http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tags request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker Hub tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tags request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tagsData DockerTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsData); err != nil {
		return nil, fmt.Errorf("failed to decode tags response: %w", err)
	}

	return tagsData.Tags, nil
}

// GetFullImageName создает полное имя образа из структуры DockerImageInfo.
// Эта функция обрабатывает имена образов, как docker.io/library/ubuntu.
func GetFullImageName(info *checker.DockerImageInfo) string {
	// Специальный случай для официальных образов Docker Hub
	if info.Registry == "docker.io" && strings.HasPrefix(info.Image, "library/") {
		return strings.TrimPrefix(info.Image, "library/")
	}
	// Для остальных образов
	return fmt.Sprintf("%s/%s", info.Registry, info.Image)
}
