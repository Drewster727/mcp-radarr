package radarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer creates a mock Radarr server that serves a fixed JSON response
// for any request matching the given path.
func newTestServer(t *testing.T, path, method string, status int, payload any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path != "" && r.URL.Path != path {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, path)
		}
		if method != "" && r.Method != method {
			t.Errorf("unexpected method: got %s, want %s", r.Method, method)
		}
		if r.Header.Get("X-Api-Key") == "" {
			t.Error("X-Api-Key header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Errorf("encode response: %v", err)
			}
		}
	}))
}

func TestGetMovies(t *testing.T) {
	want := []Movie{
		{ID: 1, Title: "The Matrix", Year: 1999, TmdbID: 603},
		{ID: 2, Title: "Interstellar", Year: 2014, TmdbID: 157336},
	}
	srv := newTestServer(t, "/api/v3/movie", http.MethodGet, http.StatusOK, want)
	defer srv.Close()

	got, err := NewClient(srv.URL, "key").GetMovies(context.Background())
	if err != nil {
		t.Fatalf("GetMovies() error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d movies, want %d", len(got), len(want))
	}
	if got[0].Title != want[0].Title {
		t.Errorf("title mismatch: got %s, want %s", got[0].Title, want[0].Title)
	}
}

func TestGetMovie(t *testing.T) {
	want := Movie{ID: 7, Title: "Dune", Year: 2021, TmdbID: 438631}
	srv := newTestServer(t, "/api/v3/movie/7", http.MethodGet, http.StatusOK, want)
	defer srv.Close()

	got, err := NewClient(srv.URL, "key").GetMovie(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetMovie() error = %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLookupMovies(t *testing.T) {
	want := []Movie{{Title: "Inception", Year: 2010, TmdbID: 27205}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie/lookup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("term") == "" {
			t.Error("missing term query parameter")
		}
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "key").LookupMovies(context.Background(), "Inception")
	if err != nil {
		t.Fatalf("LookupMovies() error = %v", err)
	}
	if len(got) != 1 || got[0].TmdbID != 27205 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestAddMovie(t *testing.T) {
	var received Movie
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		received.ID = 42
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(received)
	}))
	defer srv.Close()

	input := &Movie{Title: "Dune", TmdbID: 438631, Year: 2021, QualityProfileID: 1, RootFolderPath: "/movies", Monitored: true}
	got, err := NewClient(srv.URL, "key").AddMovie(context.Background(), input)
	if err != nil {
		t.Fatalf("AddMovie() error = %v", err)
	}
	if got.ID != 42 {
		t.Errorf("expected ID 42, got %d", got.ID)
	}
	if received.TmdbID != 438631 {
		t.Errorf("expected TmdbID 438631 in request, got %d", received.TmdbID)
	}
}

func TestUpdateMovie(t *testing.T) {
	updated := Movie{ID: 3, Title: "The Matrix", Monitored: false, QualityProfileID: 2}
	srv := newTestServer(t, "/api/v3/movie/3", http.MethodPut, http.StatusAccepted, updated)
	defer srv.Close()

	got, err := NewClient(srv.URL, "key").UpdateMovie(context.Background(), &updated)
	if err != nil {
		t.Fatalf("UpdateMovie() error = %v", err)
	}
	if got.Monitored != false || got.QualityProfileID != 2 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestDeleteMovie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie/5" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "key").DeleteMovie(context.Background(), 5, false); err != nil {
		t.Fatalf("DeleteMovie() error = %v", err)
	}
}

func TestGetQualityProfiles(t *testing.T) {
	want := []QualityProfile{
		{ID: 1, Name: "Any"},
		{ID: 2, Name: "HD-1080p"},
	}
	srv := newTestServer(t, "/api/v3/qualityprofile", http.MethodGet, http.StatusOK, want)
	defer srv.Close()

	got, err := NewClient(srv.URL, "key").GetQualityProfiles(context.Background())
	if err != nil {
		t.Fatalf("GetQualityProfiles() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(got))
	}
}

func TestGetQualityProfileByName(t *testing.T) {
	profiles := []QualityProfile{
		{ID: 1, Name: "Any"},
		{ID: 2, Name: "HD-1080p"},
	}
	srv := newTestServer(t, "/api/v3/qualityprofile", http.MethodGet, http.StatusOK, profiles)
	defer srv.Close()

	client := NewClient(srv.URL, "key")

	got, err := client.GetQualityProfileByName(context.Background(), "hd-1080p")
	if err != nil {
		t.Fatalf("GetQualityProfileByName() error = %v", err)
	}
	if got.ID != 2 {
		t.Errorf("expected ID 2, got %d", got.ID)
	}

	_, err = client.GetQualityProfileByName(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "bad-key").GetMovies(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}
