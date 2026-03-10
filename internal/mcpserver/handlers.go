package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/drewmcminn/mcp-radarr/internal/config"
	"github.com/drewmcminn/mcp-radarr/internal/radarr"
	"github.com/mark3labs/mcp-go/mcp"
)

type handlers struct {
	client *radarr.Client
	config *config.Config
}

// lookupMovie searches TMDB via Radarr for movies matching a title (and optional year).
func (h *handlers) lookupMovie(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, ok := req.Params.Arguments["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	term := title
	var year int
	if y, ok := req.Params.Arguments["year"].(float64); ok && y > 0 {
		year = int(y)
	} else if y, ok := req.Params.Arguments["year"].(int); ok && y > 0 {
		year = y
	} else if y, ok := req.Params.Arguments["year"].(int64); ok && y > 0 {
		year = int(y)
	}

	if year > 0 {
		term = fmt.Sprintf("%s %d", title, year)
	}

	movies, err := h.client.LookupMovies(ctx, term)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("lookup failed: %v", err)), nil
	}
	if len(movies) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No movies found matching %q", title)), nil
	}

	type lookupResult struct {
		InLibrary bool   `json:"in_library"`
		ID        int    `json:"id,omitempty"`
		Title     string `json:"title"`
		Year      int    `json:"year"`
		Status    string `json:"status,omitempty"`
		Overview  string `json:"overview,omitempty"`
		ImdbID    string `json:"imdbId,omitempty"`
		TmdbID    int    `json:"tmdbId"`
		HasFile   bool   `json:"has_file,omitempty"`
		Monitored bool   `json:"monitored,omitempty"`
	}

	results := make([]lookupResult, 0, len(movies))
	for _, m := range movies {
		results = append(results, lookupResult{
			InLibrary: m.ID > 0,
			ID:        m.ID,
			Title:     m.Title,
			Year:      m.Year,
			Status:    m.Status,
			Overview:  m.Overview,
			ImdbID:    m.ImdbID,
			TmdbID:    m.TmdbID,
			HasFile:   m.HasFile,
			Monitored: m.Monitored,
		})
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize results"), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// getMovies returns library movies, optionally filtered by download status, monitored, or year.
func (h *handlers) getMovies(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	movies, err := h.client.GetMovies(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get movies: %v", err)), nil
	}

	summaries := make([]radarr.MovieSummary, 0, len(movies))
	for _, m := range movies {
		if downloaded, ok := req.Params.Arguments["downloaded"].(bool); ok && m.HasFile != downloaded {
			continue
		}
		if monitored, ok := req.Params.Arguments["monitored"].(bool); ok && m.Monitored != monitored {
			continue
		}
		if y, ok := req.Params.Arguments["year"].(float64); ok && y > 0 && m.Year != int(y) {
			continue
		}
		summaries = append(summaries, radarr.MovieSummary{
			ID:               m.ID,
			Title:            m.Title,
			Year:             m.Year,
			Status:           m.Status,
			Overview:         m.Overview,
			ImdbID:           m.ImdbID,
			TmdbID:           m.TmdbID,
			HasFile:          m.HasFile,
			Monitored:        m.Monitored,
			QualityProfileID: m.QualityProfileID,
			Genres:           m.Genres,
			Runtime:          m.Runtime,
			Certification:    m.Certification,
		})
	}

	if len(summaries) == 0 {
		return mcp.NewToolResultText("No movies found matching the specified filters"), nil
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize results"), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// addMovie looks up a movie, resolves the default quality profile in parallel, then adds it.
// Returns a disambiguation list when multiple TMDB results share the same title.
func (h *handlers) addMovie(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, ok := req.Params.Arguments["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	// Helper to extract int from map[string]any
	getInt := func(m map[string]any, key string) int {
		val, ok := m[key]
		if !ok {
			return 0
		}
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		default:
			return 0
		}
	}

	year := getInt(req.Params.Arguments, "year")
	tmdbID := getInt(req.Params.Arguments, "tmdb_id")

	searchForMovie := true
	if s, ok := req.Params.Arguments["search_for_movie"].(bool); ok {
		searchForMovie = s
	}

	// Fan out: lookup + quality profile resolution run concurrently.
	type lookupResult struct {
		movies []radarr.Movie
		err    error
	}
	type profileResult struct {
		profile *radarr.QualityProfile
		err     error
	}

	lookupCh := make(chan lookupResult, 1)
	profileCh := make(chan profileResult, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// If tmdbID is provided, try direct lookup first for efficiency.
		if tmdbID > 0 {
			movie, err := h.client.LookupMovieByTmdbID(ctx, tmdbID)
			if err == nil {
				lookupCh <- lookupResult{[]radarr.Movie{*movie}, nil}
				return
			}
		}
		term := title
		if year > 0 {
			term = fmt.Sprintf("%s %d", title, year)
		}
		movies, err := h.client.LookupMovies(ctx, term)
		lookupCh <- lookupResult{movies, err}
	}()

	go func() {
		defer wg.Done()
		profile, err := h.client.GetQualityProfileByName(ctx, h.config.DefaultQualityProfile)
		profileCh <- profileResult{profile, err}
	}()

	wg.Wait()
	close(lookupCh)
	close(profileCh)

	lr := <-lookupCh
	pr := <-profileCh

	if lr.err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("movie lookup failed: %v", lr.err)), nil
	}
	if pr.err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("quality profile %q not found: %v", h.config.DefaultQualityProfile, pr.err)), nil
	}

	movies := lr.movies
	if len(movies) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No movies found matching %q", title)), nil
	}

	// Narrow by tmdb_id when provided.
	if tmdbID > 0 {
		var filtered []radarr.Movie
		for _, m := range movies {
			if m.TmdbID == tmdbID {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) > 0 {
			movies = filtered
		}
	}

	// Narrow by year when provided.
	if year > 0 {
		var filtered []radarr.Movie
		for _, m := range movies {
			if m.Year == year {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) > 0 {
			movies = filtered
		}
	}

	// Multiple distinct titles — ask the caller to disambiguate.
	if len(movies) > 1 {
		type candidate struct {
			Title    string `json:"title"`
			Year     int    `json:"year"`
			TmdbID   int    `json:"tmdbId"`
			ImdbID   string `json:"imdbId,omitempty"`
			Overview string `json:"overview,omitempty"`
			Status   string `json:"status,omitempty"`
		}
		list := make([]candidate, 0, len(movies))
		for _, m := range movies {
			list = append(list, candidate{
				Title:    m.Title,
				Year:     m.Year,
				TmdbID:   m.TmdbID,
				ImdbID:   m.ImdbID,
				Overview: m.Overview,
				Status:   m.Status,
			})
		}
		data, _ := json.MarshalIndent(map[string]any{
			"message":    "Multiple movies found. Provide the year or tmdbId to narrow the result.",
			"candidates": list,
		}, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}

	movie := movies[0]

	// Already in the library (ID > 0 means Radarr cross-referenced it).
	if movie.ID > 0 {
		data, _ := json.MarshalIndent(map[string]any{
			"status":  "exists",
			"message": "Movie is already in your Radarr library",
			"movie":   map[string]any{"id": movie.ID, "title": movie.Title, "year": movie.Year},
		}, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}

	newMovie := h.prepareMovieToAdd(&movie, pr.profile.ID, searchForMovie)
	added, err := h.client.AddMovie(ctx, newMovie)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add movie: %v", err)), nil
	}

	data, err := json.MarshalIndent(map[string]any{
		"status":  "added",
		"message": "Movie successfully added to Radarr",
		"movie": map[string]any{
			"id":    added.ID,
			"title": added.Title,
			"year":  added.Year,
		},
	}, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize result"), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// bulkAddMovies allows adding/ensuring multiple movies in one turn.
func (h *handlers) bulkAddMovies(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var moviesArg []any
	arg, exists := req.Params.Arguments["movies"]
	if !exists {
		return mcp.NewToolResultError("movies list is required"), nil
	}

	// Handle case where movies might be passed as a single object or a JSON string.
	switch v := arg.(type) {
	case []any:
		moviesArg = v
	case map[string]any:
		moviesArg = []any{v}
	case string:
		if err := json.Unmarshal([]byte(v), &moviesArg); err != nil {
			// If not an array, try unmarshaling as a single object.
			var single map[string]any
			if err := json.Unmarshal([]byte(v), &single); err == nil {
				moviesArg = []any{single}
			} else {
				return mcp.NewToolResultError("invalid movies format: must be an array of objects or a single object"), nil
			}
		}
	default:
		return mcp.NewToolResultError("movies must be an array or an object"), nil
	}

	if len(moviesArg) == 0 {
		return mcp.NewToolResultError("movies list cannot be empty"), nil
	}

	searchForMovie := true
	if s, ok := req.Params.Arguments["search_for_movie"].(bool); ok {
		searchForMovie = s
	}

	profile, err := h.client.GetQualityProfileByName(ctx, h.config.DefaultQualityProfile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("quality profile %q not found: %v", h.config.DefaultQualityProfile, err)), nil
	}

	type movieInput struct {
		Title  string `json:"title"`
		Year   int    `json:"year,omitempty"`
		TmdbID int    `json:"tmdb_id,omitempty"`
	}

	type movieResult struct {
		Input   movieInput `json:"input"`
		Status  string     `json:"status"`
		Message string     `json:"message"`
		Movie   any        `json:"movie,omitempty"`
		Error   string     `json:"error,omitempty"`
	}

	results := make([]movieResult, 0, len(moviesArg))

	// Helper to extract int from map[string]any
	getInt := func(m map[string]any, key string) int {
		val, ok := m[key]
		if !ok {
			return 0
		}
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		case string:
			var n int
			fmt.Sscanf(v, "%d", &n)
			return n
		default:
			return 0
		}
	}

	for _, arg := range moviesArg {
		mMap, ok := arg.(map[string]any)
		if !ok {
			results = append(results, movieResult{Status: "error", Message: "Invalid input format for movie", Error: "input must be an object"})
			continue
		}

		title, _ := mMap["title"].(string)
		title = strings.TrimSpace(title)
		year := getInt(mMap, "year")
		tmdbID := getInt(mMap, "tmdb_id")

		input := movieInput{Title: title, Year: year, TmdbID: tmdbID}

		if title == "" && tmdbID == 0 {
			results = append(results, movieResult{Input: input, Status: "error", Message: "Title or tmdb_id is required"})
			continue
		}

		// Perform lookup for this movie.
		var movies []radarr.Movie
		var lookupErr error
		if input.TmdbID > 0 {
			m, err := h.client.LookupMovieByTmdbID(ctx, input.TmdbID)
			if err == nil {
				movies = []radarr.Movie{*m}
			} else {
				lookupErr = err
			}
		}

		if len(movies) == 0 {
			term := input.Title
			if term == "" {
				msg := "No movie found and no title provided for fallback lookup"
				if lookupErr != nil {
					msg = fmt.Sprintf("TMDB lookup failed: %v", lookupErr)
				}
				results = append(results, movieResult{Input: input, Status: "error", Message: msg})
				continue
			}
			if input.Year > 0 {
				term = fmt.Sprintf("%s %d", input.Title, input.Year)
			}
			found, err := h.client.LookupMovies(ctx, term)
			if err != nil {
				results = append(results, movieResult{Input: input, Status: "error", Message: "Lookup failed", Error: err.Error()})
				continue
			}
			movies = found
		}

		if len(movies) == 0 {
			results = append(results, movieResult{Input: input, Status: "not_found", Message: "No movies found matching title"})
			continue
		}

		// Disambiguate if needed.
		if len(movies) > 1 {
			// Try to narrow by TMDB ID or year if provided.
			var filtered []radarr.Movie
			for _, m := range movies {
				if (input.TmdbID > 0 && m.TmdbID == input.TmdbID) || (input.Year > 0 && m.Year == input.Year) {
					filtered = append(filtered, m)
				}
			}
			if len(filtered) == 1 {
				movies = filtered
			} else {
				results = append(results, movieResult{Input: input, Status: "ambiguous", Message: "Multiple candidates found. Use year or tmdb_id to disambiguate."})
				continue
			}
		}

		movie := movies[0]

		if movie.ID > 0 {
			results = append(results, movieResult{
				Input:   input,
				Status:  "exists",
				Message: "Already in library",
				Movie:   map[string]any{"id": movie.ID, "title": movie.Title, "year": movie.Year},
			})
			continue
		}

		newMovie := h.prepareMovieToAdd(&movie, profile.ID, searchForMovie)
		added, err := h.client.AddMovie(ctx, newMovie)
		if err != nil {
			results = append(results, movieResult{Input: input, Status: "error", Message: "Failed to add", Error: err.Error()})
			continue
		}

		results = append(results, movieResult{
			Input:   input,
			Status:  "added",
			Message: "Successfully added",
			Movie:   map[string]any{"id": added.ID, "title": added.Title, "year": added.Year},
		})
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

// prepareMovieToAdd builds the Radarr model for addition.
func (h *handlers) prepareMovieToAdd(movie *radarr.Movie, profileID int, searchForMovie bool) *radarr.Movie {
	return &radarr.Movie{
		Title:               movie.Title,
		TmdbID:              movie.TmdbID,
		TitleSlug:           movie.TitleSlug,
		Year:                movie.Year,
		QualityProfileID:    profileID,
		RootFolderPath:      h.config.RootFolderPath,
		Monitored:           true,
		MinimumAvailability: "released",
		AddOptions: &radarr.AddMovieOptions{
			SearchForMovie: searchForMovie,
			Monitor:        "movieOnly",
		},
		Images:  movie.Images,
		Ratings: movie.Ratings,
		Genres:  movie.Genres,
	}
}

// updateMovie patches monitored status and/or quality profile on an existing movie.
func (h *handlers) updateMovie(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Helper to extract int from map[string]any
	getInt := func(m map[string]any, key string) (int, bool) {
		val, ok := m[key]
		if !ok {
			return 0, false
		}
		switch v := val.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case int64:
			return int(v), true
		default:
			return 0, false
		}
	}

	id, ok := getInt(req.Params.Arguments, "id")
	if !ok {
		return mcp.NewToolResultError("id is required"), nil
	}

	movie, err := h.client.GetMovie(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get movie %d: %v", id, err)), nil
	}

	if monitored, ok := req.Params.Arguments["monitored"].(bool); ok {
		movie.Monitored = monitored
	}
	if qpID, ok := getInt(req.Params.Arguments, "quality_profile_id"); ok {
		movie.QualityProfileID = qpID
	}

	updated, err := h.client.UpdateMovie(ctx, movie)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update movie: %v", err)), nil
	}

	data, _ := json.MarshalIndent(map[string]any{
		"message": "Movie updated successfully",
		"movie": map[string]any{
			"id":               updated.ID,
			"title":            updated.Title,
			"monitored":        updated.Monitored,
			"qualityProfileId": updated.QualityProfileID,
		},
	}, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

// deleteMovie removes a movie from the Radarr library.
func (h *handlers) deleteMovie(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Helper to extract int from map[string]any
	getInt := func(m map[string]any, key string) (int, bool) {
		val, ok := m[key]
		if !ok {
			return 0, false
		}
		switch v := val.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case int64:
			return int(v), true
		default:
			return 0, false
		}
	}

	id, ok := getInt(req.Params.Arguments, "id")
	if !ok {
		return mcp.NewToolResultError("id is required"), nil
	}

	deleteFiles := false
	if df, ok := req.Params.Arguments["delete_files"].(bool); ok {
		deleteFiles = df
	}

	if err := h.client.DeleteMovie(ctx, id, deleteFiles); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to delete movie %d: %v", id, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"Movie %d removed from Radarr (deleteFiles=%t)", id, deleteFiles,
	)), nil
}
