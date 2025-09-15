package registry

import (
	"encoding/base64"
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
	// Добавь другие поля, если они нужны, например:
	// FullSize int       `json:"full_size"`
	// LastUpdated time.Time `json:"last_updated"`
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
	isPublic := c.User == "" || strings.HasPrefix(image, "library/")
	// Шаг 1: Получаем токен с использованием ЛОГИНА И ТОКЕНА
	authURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", image)

	req, err := http.NewRequest(http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}

	if !isPublic {
		// Если образ приватный, используем Basic Auth с логином и токеном.
		req.SetBasicAuth(c.User, c.Pass)
	}
	// Вместо Basic Auth, используем заголовок Authorization, как в большинстве API
	req.Header.Set("Authorization", "Basic "+basicAuth(c.User, c.Pass))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authData DockerAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authData); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	token := authData.Token
	fmt.Println("Successfully got Docker Hub token. Token:", token[:10]+"...")

	// Шаг 2: Используем полученный токен для доступа к реестру
	tagsURL := fmt.Sprintf("https://registry-1.docker.io/v2/%s/tags/list", image)
	req, err = http.NewRequest(http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tags request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("Response:", resp)
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

// basicAuth returns the base64-encoded string for a username and password
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
