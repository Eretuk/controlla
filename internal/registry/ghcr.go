package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	ghcrMaxRetries    = 3
	ghcrRetryBaseWait = 10 * time.Second
)

type GhcrClient struct {
	Client      *http.Client
	GithubToken string // optional; if set, used for authenticated token requests
}

func NewGhcrClient(githubToken string) *GhcrClient {
	return &GhcrClient{
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		GithubToken: githubToken,
	}
}

// doWithRetry executes an HTTP request and retries up to ghcrMaxRetries times
// on 429 Too Many Requests, honouring the Retry-After header when present.
func (c *GhcrClient) doWithRetry(req *http.Request) (*http.Response, error) {
	// We need to be able to replay the request on retry. Since these are all
	// GET requests with no body, we only need to clone the headers/URL.
	url := req.URL.String()
	method := req.Method
	headerCopy := req.Header.Clone()

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= ghcrMaxRetries; attempt++ {
		if attempt > 0 {
			// Rebuild the request for subsequent attempts so it is fresh.
			req, err = http.NewRequest(method, url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to rebuild request: %w", err)
			}
			req.Header = headerCopy.Clone()
		}

		resp, err = c.Client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// 429 — parse Retry-After and wait before the next attempt.
		resp.Body.Close()
		if attempt == ghcrMaxRetries {
			break
		}

		wait := retryAfterDuration(resp.Header.Get("Retry-After"), ghcrRetryBaseWait, attempt)
		time.Sleep(wait)
	}

	return nil, fmt.Errorf("GHCR request failed with status code: %d (rate limited after %d retries)", http.StatusTooManyRequests, ghcrMaxRetries)
}

// retryAfterDuration parses a Retry-After header value (seconds or HTTP-date)
// and returns the sleep duration. Falls back to exponential backoff when the
// header is absent or unparseable.
func retryAfterDuration(header string, base time.Duration, attempt int) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(header); err == nil {
			return time.Duration(secs) * time.Second
		}
		if t, err := http.ParseTime(header); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
		}
	}
	// Exponential backoff: base * 2^attempt
	return base * (1 << uint(attempt))
}

// GetTags retrieves image tags for a given image, handling pagination.
// maxPages limits the number of pages fetched; 0 means unlimited.
// For images with thousands of tags (e.g. immich-server) set this to a small
// value (e.g. 5) to avoid burning through registry rate-limit quota.
func (c *GhcrClient) GetTags(image string, maxPages int) ([]string, error) {
	token, err := c.getToken(image)
	if err != nil {
		return nil, err
	}

	var allTags []string
	url := fmt.Sprintf("https://ghcr.io/v2/%s/tags/list?n=200", image)
	page := 0
	for {
		page++
		if maxPages > 0 && page > maxPages {
			logrus.Debugf("GHCR: reached max_pages limit (%d) for %s, stopping pagination.", maxPages, image)
			break
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create tags request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.doWithRetry(req)
		if err != nil {
			return nil, err
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

		link := resp.Header.Get("Link")
		if link == "" || !containsRelNext(link) {
			break
		}
		nextURL := parseNextURL(link)
		if nextURL == "" {
			break
		}
		if strings.HasPrefix(nextURL, "/") {
			nextURL = "https://ghcr.io" + nextURL
		}
		url = nextURL
	}
	return allTags, nil
}

// getToken requests an anonymous (or authenticated) pull token for image.
func (c *GhcrClient) getToken(image string) (string, error) {
	authURL := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s:pull", image)
	req, err := http.NewRequest(http.MethodGet, authURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	// Using a GitHub personal access token raises the rate limit and allows
	// access to packages visible to that account.
	// GHCR expects Basic auth with the literal username "token" and the PAT as password.
	if c.GithubToken != "" {
		req.SetBasicAuth("token", c.GithubToken)
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("failed to get GHCR token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GHCR token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authData); err != nil {
		return "", fmt.Errorf("failed to decode auth response: %w", err)
	}
	return authData.Token, nil
}

func (c *GhcrClient) GetDigest(image, tag string) (string, error) {
	// Need a fresh token scoped to this image for the manifest request.
	token, err := c.getToken(image)
	if err != nil {
		// Fall back to unauthenticated if token fetch fails.
		token = ""
	}

	url := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", image, tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.doWithRetry(req)
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

// containsRelNext checks if the Link header contains rel="next"
func containsRelNext(link string) bool {
	return strings.Contains(link, `rel="next"`)
}

// parseNextURL extracts the next page URL from the Link header.
// Example: <https://ghcr.io/v2/immich-app/immich-server/tags/list?next_page_token=...&n=200>; rel="next"
func parseNextURL(link string) string {
	parts := strings.Split(link, ";")
	if len(parts) < 2 {
		return ""
	}
	urlPart := strings.TrimSpace(parts[0])
	urlPart = strings.Trim(urlPart, "<>")
	return urlPart
}
