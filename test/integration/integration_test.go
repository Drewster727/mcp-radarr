//go:build integration

// Package integration provides end-to-end tests against a mock Radarr server.
// Run with: go test -tags=integration ./test/integration/...
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drewmcminn/mcp-radarr/internal/config"
	"github.com/drewmcminn/mcp-radarr/internal/mcpserver"
	"github.com/drewmcminn/mcp-radarr/internal/radarr"
)

// mockRadarr holds the state of a lightweight in-memory Radarr instance.
type mockRadarr struct {
	movies   []radarr.Movie
	profiles []radarr.QualityProfile
	nextID   int
}

func newMockRadarr() *mockRadarr {
	return &mockRadarr{
		profiles: []radarr.QualityProfile{
			{ID: 1, Name: "Any"},
			{ID: 2, Name: "HD-1080p"},
		},
		movies: []radarr.Movie{
			{ID: 1, Title: "The Matrix", Year: 1999, TmdbID: 603, HasFile: true, Monitored: true, QualityProfileID: 1},
		},
		nextID: 2,
	}
}

func (m *mockRadarr) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(m.profiles)
	})

	mux.HandleFunc("/api/v3/movie/lookup", func(w http.ResponseWriter, r *http.Request) {
		term := strings.ToLower(r.URL.Query().Get("term"))
		var results []radarr.Movie
		// Return a library hit if the term matches; otherwise synthesise a TMDB result.
		for _, mv := range m.movies {
			if strings.Contains(strings.ToLower(mv.Title), term) {
				results = append(results, mv)
			}
		}
		if len(results) == 0 {
			// Fake a TMDB-only result (no library ID).
			results = []radarr.Movie{
				{Title: "Dune", Year: 2021, TmdbID: 438631, TitleSlug: "dune-2021"},
			}
		}
		json.NewEncoder(w).Encode(results)
	})

	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(m.movies)
		case http.MethodPost:
			var mv radarr.Movie
			json.NewDecoder(r.Body).Decode(&mv)
			mv.ID = m.nextID
			m.nextID++
			m.movies = append(m.movies, mv)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(mv)
		}
	})

	mux.HandleFunc("/api/v3/movie/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			var mv radarr.Movie
			json.NewDecoder(r.Body).Decode(&mv)
			json.NewEncoder(w).Encode(mv)
		case http.MethodGet:
			if len(m.movies) > 0 {
				json.NewEncoder(w).Encode(m.movies[0])
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})

	return mux
}

// ── helpers ───────────────────────────────────────────────────────────────────

func setup(t *testing.T, allowMutations bool) (*mcpserver.Server, *httptest.Server) {
	t.Helper()
	mock := newMockRadarr()
	srv := httptest.NewServer(mock.handler())
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		RadarrURL:             srv.URL,
		APIKey:                "test-key",
		DefaultQualityProfile: "Any",
		RootFolderPath:        "/movies",
		AllowMutations:        allowMutations,
	}
	client := radarr.NewClient(cfg.RadarrURL, cfg.APIKey)
	return mcpserver.New(cfg, client), srv
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestIntegration_LookupExistingMovie(t *testing.T) {
	_, apiSrv := setup(t, false)
	client := radarr.NewClient(apiSrv.URL, "test-key")

	movies, err := client.LookupMovies(context.Background(), "The Matrix")
	if err != nil {
		t.Fatalf("LookupMovies error: %v", err)
	}
	if len(movies) == 0 {
		t.Fatal("expected at least one result")
	}
	if movies[0].Title != "The Matrix" {
		t.Errorf("unexpected title: %s", movies[0].Title)
	}
}

func TestIntegration_GetMovies(t *testing.T) {
	_, apiSrv := setup(t, false)
	client := radarr.NewClient(apiSrv.URL, "test-key")

	movies, err := client.GetMovies(context.Background())
	if err != nil {
		t.Fatalf("GetMovies error: %v", err)
	}
	if len(movies) == 0 {
		t.Fatal("expected movies in library")
	}
}

func TestIntegration_AddMovie(t *testing.T) {
	_, apiSrv := setup(t, false)
	client := radarr.NewClient(apiSrv.URL, "test-key")

	// Lookup returns a TMDB-only (no library ID) result for "Dune".
	lookup, err := client.LookupMovies(context.Background(), "Dune")
	if err != nil || len(lookup) == 0 {
		t.Fatalf("lookup failed: %v (results: %d)", err, len(lookup))
	}

	profile, err := client.GetQualityProfileByName(context.Background(), "Any")
	if err != nil {
		t.Fatalf("profile lookup failed: %v", err)
	}

	movie := &radarr.Movie{
		Title:               lookup[0].Title,
		TmdbID:              lookup[0].TmdbID,
		TitleSlug:           lookup[0].TitleSlug,
		Year:                lookup[0].Year,
		QualityProfileID:    profile.ID,
		RootFolderPath:      "/movies",
		Monitored:           true,
		MinimumAvailability: "released",
		AddOptions:          &radarr.AddMovieOptions{SearchForMovie: true, Monitor: "movieOnly"},
	}

	added, err := client.AddMovie(context.Background(), movie)
	if err != nil {
		t.Fatalf("AddMovie error: %v", err)
	}
	if added.ID == 0 {
		t.Error("expected a non-zero ID for the added movie")
	}
	if added.Title != "Dune" {
		t.Errorf("unexpected title: %s", added.Title)
	}

	// Verify the movie now appears in the library.
	all, _ := client.GetMovies(context.Background())
	found := false
	for _, m := range all {
		if m.TmdbID == 438631 {
			found = true
			break
		}
	}
	if !found {
		t.Error("newly added movie not found in library")
	}
}

func TestIntegration_MutationsDisabled_ServerRegistration(t *testing.T) {
	mcpSrv, _ := setup(t, false)
	// Just verify the server builds without panic when mutations are off.
	if mcpSrv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestIntegration_MutationsEnabled_ServerRegistration(t *testing.T) {
	mcpSrv, _ := setup(t, true)
	if mcpSrv == nil {
		t.Fatal("expected non-nil server")
	}
}
