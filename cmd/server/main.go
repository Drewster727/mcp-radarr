package main

import (
	"log"
	"os"

	"github.com/drewmcminn/mcp-radarr/internal/config"
	"github.com/drewmcminn/mcp-radarr/internal/mcpserver"
	"github.com/drewmcminn/mcp-radarr/internal/radarr"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("configuration error: %v", err)
		os.Exit(1)
	}

	log.Printf("starting Radarr MCP Server — quality profile: %q, mutations: %v",
		cfg.DefaultQualityProfile, cfg.AllowMutations)

	client := radarr.NewClient(cfg.RadarrURL, cfg.APIKey)
	srv := mcpserver.New(cfg, client)

	if err := srv.ServeStdio(); err != nil {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
