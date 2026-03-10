package radarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout      = 30 * time.Second
	maxIdleConns        = 100
	maxConnsPerHost     = 10
	idleConnTimeout     = 90 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
)

// APIError wraps a non-2xx response from the Radarr API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("radarr API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// Client is a thread-safe Radarr API client backed by a pooled HTTP transport.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Client with connection pooling configured.
func NewClient(baseURL, apiKey string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxConnsPerHost:     maxConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		ForceAttemptHTTP2:   true,
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
		},
	}
}

// newRequest builds an authenticated request to the Radarr v3 API.
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v3"+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// do executes the request, checks the status, and optionally decodes the JSON body.
func (c *Client) do(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// GetMovies returns every movie in the Radarr library.
func (c *Client) GetMovies(ctx context.Context) ([]Movie, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/movie", nil)
	if err != nil {
		return nil, err
	}
	var movies []Movie
	return movies, c.do(req, &movies)
}

// GetMovie returns a single movie by its Radarr ID.
func (c *Client) GetMovie(ctx context.Context, id int) (*Movie, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/movie/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var movie Movie
	return &movie, c.do(req, &movie)
}

// LookupMovies searches TMDB (via Radarr) for movies matching the given term.
// Results include an `id` > 0 when the movie is already in the local library.
func (c *Client) LookupMovies(ctx context.Context, term string) ([]Movie, error) {
	path := "/movie/lookup?term=" + url.QueryEscape(term)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var movies []Movie
	return movies, c.do(req, &movies)
}

// AddMovie adds a movie to the Radarr library and returns the created resource.
func (c *Client) AddMovie(ctx context.Context, movie *Movie) (*Movie, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/movie", movie)
	if err != nil {
		return nil, err
	}
	var created Movie
	return &created, c.do(req, &created)
}

// UpdateMovie updates an existing movie and returns the updated resource.
func (c *Client) UpdateMovie(ctx context.Context, movie *Movie) (*Movie, error) {
	req, err := c.newRequest(ctx, http.MethodPut, fmt.Sprintf("/movie/%d", movie.ID), movie)
	if err != nil {
		return nil, err
	}
	var updated Movie
	return &updated, c.do(req, &updated)
}

// DeleteMovie removes a movie from Radarr. Set deleteFiles to also remove files from disk.
func (c *Client) DeleteMovie(ctx context.Context, id int, deleteFiles bool) error {
	path := fmt.Sprintf("/movie/%d?deleteFiles=%t", id, deleteFiles)
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

// GetQualityProfiles returns all quality profiles configured in Radarr.
func (c *Client) GetQualityProfiles(ctx context.Context) ([]QualityProfile, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/qualityprofile", nil)
	if err != nil {
		return nil, err
	}
	var profiles []QualityProfile
	return profiles, c.do(req, &profiles)
}

// GetQualityProfileByName looks up a quality profile by case-insensitive name.
func (c *Client) GetQualityProfileByName(ctx context.Context, name string) (*QualityProfile, error) {
	profiles, err := c.GetQualityProfiles(ctx)
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for i := range profiles {
		if strings.ToLower(profiles[i].Name) == lower {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("quality profile %q not found", name)
}
