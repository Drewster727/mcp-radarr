package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drewmcminn/mcp-radarr/internal/config"
	"github.com/drewmcminn/mcp-radarr/internal/radarr"
	"github.com/mark3labs/mcp-go/mcp"
)

// newHandlers wires test handlers to a mock Radarr server.
func newHandlers(srv *httptest.Server, allowMutations bool) *handlers {
	return &handlers{
		client: radarr.NewClient(srv.URL, "test-key"),
		config: &config.Config{
			DefaultQualityProfile: "Any",
			RootFolderPath:        "/movies",
			AllowMutations:        allowMutations,
		},
	}
}

// callArgs is a shorthand for building tool request arguments.
func callArgs(kv ...any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = make(map[string]any)
	for i := 0; i+1 < len(kv); i += 2 {
		req.Params.Arguments[kv[i].(string)] = kv[i+1]
	}
	return req
}

func isError(r *mcp.CallToolResult) bool {
	return r.IsError
}

func resultText(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// ── lookup_movie ─────────────────────────────────────────────────────────────

func TestLookupMovie_Found(t *testing.T) {
	movies := []radarr.Movie{{Title: "Inception", Year: 2010, TmdbID: 27205}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(movies)
	}))
	defer srv.Close()

	result, err := newHandlers(srv, false).lookupMovie(context.Background(), callArgs("title", "Inception"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Fatalf("expected success, got error: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "Inception") {
		t.Error("expected 'Inception' in result text")
	}
}

func TestLookupMovie_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]radarr.Movie{})
	}))
	defer srv.Close()

	result, err := newHandlers(srv, false).lookupMovie(context.Background(), callArgs("title", "NoSuchMovie9999"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Error("expected a text response for not-found, not an error result")
	}
	if !strings.Contains(resultText(result), "No movies found") {
		t.Errorf("unexpected text: %s", resultText(result))
	}
}

func TestLookupMovie_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]radarr.Movie{})
	}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).lookupMovie(context.Background(), callArgs())
	if !isError(result) {
		t.Error("expected error when title is missing")
	}
}

// ── get_movies ────────────────────────────────────────────────────────────────

func TestGetMovies_NoFilter(t *testing.T) {
	movies := []radarr.Movie{
		{ID: 1, Title: "The Matrix", Year: 1999, HasFile: true, Monitored: true},
		{ID: 2, Title: "Inception", Year: 2010, HasFile: false, Monitored: true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(movies)
	}))
	defer srv.Close()

	result, err := newHandlers(srv, false).getMovies(context.Background(), callArgs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Fatalf("expected success: %s", resultText(result))
	}

	var summaries []radarr.MovieSummary
	if err := json.Unmarshal([]byte(resultText(result)), &summaries); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("expected 2 movies, got %d", len(summaries))
	}
}

func TestGetMovies_FilterDownloaded(t *testing.T) {
	movies := []radarr.Movie{
		{ID: 1, Title: "The Matrix", HasFile: true},
		{ID: 2, Title: "Inception", HasFile: false},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(movies)
	}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).getMovies(context.Background(), callArgs("downloaded", true))
	var summaries []radarr.MovieSummary
	json.Unmarshal([]byte(resultText(result)), &summaries)
	if len(summaries) != 1 || summaries[0].Title != "The Matrix" {
		t.Errorf("expected only The Matrix (downloaded), got %+v", summaries)
	}
}

func TestGetMovies_FilterYear(t *testing.T) {
	movies := []radarr.Movie{
		{ID: 1, Title: "The Matrix", Year: 1999},
		{ID: 2, Title: "Dune", Year: 2021},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(movies)
	}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).getMovies(context.Background(), callArgs("year", float64(2021)))
	var summaries []radarr.MovieSummary
	json.Unmarshal([]byte(resultText(result)), &summaries)
	if len(summaries) != 1 || summaries[0].Title != "Dune" {
		t.Errorf("expected only Dune (2021), got %+v", summaries)
	}
}

// ── add_movie ─────────────────────────────────────────────────────────────────

func TestAddMovie_Success(t *testing.T) {
	profiles := []radarr.QualityProfile{{ID: 1, Name: "Any"}}
	lookup := []radarr.Movie{{Title: "Dune", Year: 2021, TmdbID: 438631, TitleSlug: "dune-2021"}}
	added := radarr.Movie{ID: 42, Title: "Dune", Year: 2021, TmdbID: 438631, QualityProfileID: 1, Path: "/movies/Dune (2021)"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/qualityprofile":
			json.NewEncoder(w).Encode(profiles)
		case r.URL.Path == "/api/v3/movie/lookup":
			json.NewEncoder(w).Encode(lookup)
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(added)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result, err := newHandlers(srv, false).addMovie(context.Background(), callArgs("title", "Dune", "year", float64(2021)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Fatalf("expected success: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "successfully added") {
		t.Errorf("unexpected response: %s", resultText(result))
	}
}

func TestAddMovie_AlreadyInLibrary(t *testing.T) {
	profiles := []radarr.QualityProfile{{ID: 1, Name: "Any"}}
	// ID > 0 signals the movie is already tracked by Radarr.
	existing := []radarr.Movie{{ID: 10, Title: "The Matrix", Year: 1999, TmdbID: 603}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/qualityprofile" {
			json.NewEncoder(w).Encode(profiles)
		} else {
			json.NewEncoder(w).Encode(existing)
		}
	}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).addMovie(context.Background(), callArgs("title", "The Matrix"))
	if isError(result) {
		t.Fatalf("expected text response: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "already in your Radarr library") {
		t.Errorf("unexpected response: %s", resultText(result))
	}
}

func TestAddMovie_MultipleResults(t *testing.T) {
	profiles := []radarr.QualityProfile{{ID: 1, Name: "Any"}}
	// Two movies with the same title but different years.
	candidates := []radarr.Movie{
		{Title: "Dune", Year: 1984, TmdbID: 841},
		{Title: "Dune", Year: 2021, TmdbID: 438631},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/qualityprofile" {
			json.NewEncoder(w).Encode(profiles)
		} else {
			json.NewEncoder(w).Encode(candidates)
		}
	}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).addMovie(context.Background(), callArgs("title", "Dune"))
	if isError(result) {
		t.Fatalf("expected text response: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "Multiple movies found") {
		t.Errorf("expected disambiguation message: %s", resultText(result))
	}
}

func TestAddMovie_MissingTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	result, _ := newHandlers(srv, false).addMovie(context.Background(), callArgs())
	if !isError(result) {
		t.Error("expected error when title is missing")
	}
}

// ── update_movie ──────────────────────────────────────────────────────────────

func TestUpdateMovie(t *testing.T) {
	movie := radarr.Movie{ID: 3, Title: "Interstellar", Monitored: true, QualityProfileID: 1}
	updated := radarr.Movie{ID: 3, Title: "Interstellar", Monitored: false, QualityProfileID: 2}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(movie)
		} else {
			json.NewEncoder(w).Encode(updated)
		}
	}))
	defer srv.Close()

	result, err := newHandlers(srv, true).updateMovie(context.Background(),
		callArgs("id", float64(3), "monitored", false, "quality_profile_id", float64(2)),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Fatalf("expected success: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "updated successfully") {
		t.Errorf("unexpected response: %s", resultText(result))
	}
}

// ── delete_movie ──────────────────────────────────────────────────────────────

func TestDeleteMovie(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			called = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	result, err := newHandlers(srv, true).deleteMovie(context.Background(), callArgs("id", float64(5)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isError(result) {
		t.Fatalf("expected success: %s", resultText(result))
	}
	if !called {
		t.Error("DELETE request was not issued")
	}
}

func TestDeleteMovie_MissingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	result, _ := newHandlers(srv, true).deleteMovie(context.Background(), callArgs())
	if !isError(result) {
		t.Error("expected error when id is missing")
	}
}
