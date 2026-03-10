package radarr

import "time"

// Language represents a Radarr language value.
type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MediaCover represents a poster or fanart image.
type MediaCover struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

// Rating holds votes and score from a single ratings source.
type Rating struct {
	Votes int     `json:"votes"`
	Value float64 `json:"value"`
	Type  string  `json:"type,omitempty"`
}

// Ratings aggregates scores from multiple sources.
type Ratings struct {
	IMDB           *Rating `json:"imdb,omitempty"`
	TMDB           *Rating `json:"tmdb,omitempty"`
	Metacritic     *Rating `json:"metacritic,omitempty"`
	RottenTomatoes *Rating `json:"rottenTomatoes,omitempty"`
}

// MovieCollection is the collection a movie belongs to.
type MovieCollection struct {
	Title  string `json:"title"`
	TmdbID int    `json:"tmdbId"`
}

// AddMovieOptions controls behaviour when a movie is first added.
type AddMovieOptions struct {
	SearchForMovie bool   `json:"searchForMovie"`
	Monitor        string `json:"monitor"`
}

// Movie is the full Radarr movie resource.
type Movie struct {
	ID                  int              `json:"id,omitempty"`
	Title               string           `json:"title"`
	OriginalTitle       string           `json:"originalTitle,omitempty"`
	OriginalLanguage    *Language        `json:"originalLanguage,omitempty"`
	SortTitle           string           `json:"sortTitle,omitempty"`
	SizeOnDisk          int64            `json:"sizeOnDisk,omitempty"`
	Status              string           `json:"status,omitempty"`
	Overview            string           `json:"overview,omitempty"`
	InCinemas           *time.Time       `json:"inCinemas,omitempty"`
	DigitalRelease      *time.Time       `json:"digitalRelease,omitempty"`
	PhysicalRelease     *time.Time       `json:"physicalRelease,omitempty"`
	Images              []MediaCover     `json:"images,omitempty"`
	Year                int              `json:"year,omitempty"`
	Studio              string           `json:"studio,omitempty"`
	Path                string           `json:"path,omitempty"`
	QualityProfileID    int              `json:"qualityProfileId"`
	HasFile             bool             `json:"hasFile,omitempty"`
	Monitored           bool             `json:"monitored"`
	MinimumAvailability string           `json:"minimumAvailability"`
	IsAvailable         bool             `json:"isAvailable,omitempty"`
	Runtime             int              `json:"runtime,omitempty"`
	ImdbID              string           `json:"imdbId,omitempty"`
	TmdbID              int              `json:"tmdbId"`
	TitleSlug           string           `json:"titleSlug,omitempty"`
	RootFolderPath      string           `json:"rootFolderPath,omitempty"`
	Certification       string           `json:"certification,omitempty"`
	Genres              []string         `json:"genres,omitempty"`
	Tags                []int            `json:"tags,omitempty"`
	Added               *time.Time       `json:"added,omitempty"`
	AddOptions          *AddMovieOptions `json:"addOptions,omitempty"`
	Ratings             *Ratings         `json:"ratings,omitempty"`
	Collection          *MovieCollection `json:"collection,omitempty"`
	Popularity          float64          `json:"popularity,omitempty"`
}

// MovieSummary is a condensed representation used for list responses.
type MovieSummary struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Year             int      `json:"year"`
	Status           string   `json:"status,omitempty"`
	Overview         string   `json:"overview,omitempty"`
	ImdbID           string   `json:"imdbId,omitempty"`
	TmdbID           int      `json:"tmdbId"`
	HasFile          bool     `json:"hasFile"`
	Monitored        bool     `json:"monitored"`
	QualityProfileID int      `json:"qualityProfileId"`
	Genres           []string `json:"genres,omitempty"`
	Runtime          int      `json:"runtime,omitempty"`
	Certification    string   `json:"certification,omitempty"`
}

// QualityItem is a single quality definition within a profile.
type QualityItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// QualityProfileItem represents one entry (or group) in a quality profile.
type QualityProfileItem struct {
	Quality *QualityItem         `json:"quality,omitempty"`
	Items   []QualityProfileItem `json:"items,omitempty"`
	Allowed bool                 `json:"allowed"`
	Name    string               `json:"name,omitempty"`
	ID      int                  `json:"id,omitempty"`
}

// QualityProfile is a named Radarr quality profile.
type QualityProfile struct {
	ID             int                  `json:"id"`
	Name           string               `json:"name"`
	UpgradeAllowed bool                 `json:"upgradeAllowed"`
	Cutoff         int                  `json:"cutoff"`
	Items          []QualityProfileItem `json:"items"`
}
