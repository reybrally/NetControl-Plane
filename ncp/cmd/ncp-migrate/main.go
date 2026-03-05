package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
)

func main() {
	cfg := config.Load()
	if cfg.DBDSN == "" {
		log.Fatal("NCP_DB_DSN is required")
	}

	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	args := os.Args
	cmd := "up"
	if len(args) >= 2 {
		cmd = args[1]
	}

	dir := "migrations"
	switch cmd {
	case "up":
		if err := goose.Up(db, dir); err != nil {
			log.Fatalf("goose up: %v", err)
		}
		log.Println("migrations applied")
	case "status":
		if err := goose.Status(db, dir); err != nil {
			log.Fatalf("goose status: %v", err)
		}
	default:
		log.Fatalf("unknown command %q. use: up|status", cmd)
	}
}
