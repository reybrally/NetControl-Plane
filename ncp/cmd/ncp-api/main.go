package main

import (
	"context"
	"log"
	"time"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/wiring"
)

func main() {
	cfg := config.Load()
	if cfg.DBDSN == "" {
		log.Fatal("NCP_DB_DSN is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	auth := wiring.AuthConfig(cfg)
	c, err := wiring.Build(ctx, cfg, auth)
	if err != nil {
		log.Fatal(err)
	}
	defer c.DB.Close()

	srv, err := wiring.BuildHTTPRouter(c)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("api listening on %s", cfg.HTTPAddr)
	log.Fatal(srv.Start(cfg.HTTPAddr))
}
