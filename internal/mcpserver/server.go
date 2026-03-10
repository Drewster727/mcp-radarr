package mcpserver

import (
	"github.com/drewmcminn/mcp-radarr/internal/config"
	"github.com/drewmcminn/mcp-radarr/internal/radarr"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server is the MCP server wired to a Radarr client.
type Server struct {
	mcp *server.MCPServer
}

// New builds a Server, registers all tools, and applies the feature flag for mutations.
func New(cfg *config.Config, client *radarr.Client) *Server {
	mcp := server.NewMCPServer("Radarr MCP Server", "1.0.0")

	h := &handlers{client: client, config: cfg}

	// ── Read-only tools (always enabled) ──────────────────────────────────────

	mcp.AddTool(
		newTool("lookup_movie",
			"Check whether a specific movie is in your Radarr library, or search TMDB for movie metadata. "+
				"ALWAYS use this tool first when the user asks if a movie exists or wants details about a specific title. "+
				"A non-zero 'id' in the result means the movie is already in your Radarr library. "+
				"Only falls back to TMDB results (id=0) when the movie is not yet in the library.",
			withString("title", "Movie title to search for", true),
			withNumber("year", "Optional release year to narrow results", false),
		),
		h.lookupMovie,
	)

	mcp.AddTool(
		newTool("get_movies",
			"Return all movies in the Radarr library, optionally filtered by download status, monitored state, or release year. "+
				"WARNING: this returns the full library and can be very large. "+
				"Use 'lookup_movie' instead when checking if a specific title is in the library.",
			withBool("downloaded", "Filter by whether the movie file has been downloaded", false),
			withBool("monitored", "Filter by monitored status", false),
			withNumber("year", "Filter by release year", false),
		),
		h.getMovies,
	)

	mcp.AddTool(
		newTool("add_movie",
			"Add a movie to the Radarr library using the server's default quality profile. "+
				"When multiple TMDB results share the same title, a candidate list is returned — "+
				"use 'tmdb_id' or 'year' to disambiguate.",
			withString("title", "Movie title to add", true),
			withNumber("year", "Optional release year to disambiguate titles", false),
			withNumber("tmdb_id", "TMDB ID to precisely identify the movie when disambiguation is needed", false),
			withBool("search_for_movie", "Trigger an immediate release search after adding (default: true)", false),
		),
		h.addMovie,
	)

	// ── Mutation tools (opt-in via RADARR_ALLOW_MUTATIONS) ────────────────────

	if cfg.AllowMutations {
		mcp.AddTool(
			newTool("update_movie",
				"Update an existing movie's monitored status or quality profile.",
				withNumber("id", "Radarr movie ID", true),
				withBool("monitored", "Set monitored state", false),
				withNumber("quality_profile_id", "Quality profile ID to assign", false),
			),
			h.updateMovie,
		)

		mcp.AddTool(
			newTool("delete_movie",
				"Remove a movie from the Radarr library.",
				withNumber("id", "Radarr movie ID", true),
				withBool("delete_files", "Also delete movie files from disk (default: false)", false),
			),
			h.deleteMovie,
		)
	}

	return &Server{mcp: mcp}
}

// ServeStdio runs the MCP server over stdin/stdout (standard Claude Desktop transport).
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcp)
}

// ── Tool builder helpers ──────────────────────────────────────────────────────

type toolOption = mcp.ToolOption

func newTool(name, description string, opts ...toolOption) mcp.Tool {
	return mcp.NewTool(name, append([]toolOption{mcp.WithDescription(description)}, opts...)...)
}

func withString(name, description string, required bool) toolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithString(name, opts...)
}

func withNumber(name, description string, required bool) toolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithNumber(name, opts...)
}

func withBool(name, description string, required bool) toolOption {
	opts := []mcp.PropertyOption{mcp.Description(description)}
	if required {
		opts = append(opts, mcp.Required())
	}
	return mcp.WithBoolean(name, opts...)
}
