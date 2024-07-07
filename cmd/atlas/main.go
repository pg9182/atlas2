package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/r2northstar/atlas/v2/db/pdatadb"
	"github.com/r2northstar/atlas/v2/db/sessiondb"
	"github.com/r2northstar/atlas/v2/pkg/atlas"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if err := os.Mkdir("data", 0777); err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}

	var cfg atlas.Config

	if db, err := pdatadb.Open("./data/pdata.db"); err != nil {
		panic(fmt.Errorf("pdatadb: open: %w", err))
	} else {
		if cur, to, err := db.Version(); err != nil {
			panic(fmt.Errorf("pdatadb: migrate: %w", err))
		} else if cur > to {
			panic(fmt.Errorf("pdatadb: migrate: database version %d is too new", cur))
		} else if cur != to {
			if err := db.MigrateUp(context.Background(), to); err != nil {
				panic(fmt.Errorf("pdatadb: migrate: migrate (%d to %d): %w", cur, to, err))
			}
		}
		cfg.PdataStorage = db
	}

	if db, err := sessiondb.Open("./data/session.db"); err != nil {
		panic(fmt.Errorf("sessiondb: open: %w", err))
	} else {
		if cur, to, err := db.Version(); err != nil {
			panic(fmt.Errorf("sessiondb: migrate: %w", err))
		} else if cur > to {
			panic(fmt.Errorf("sessiondb: migrate: database version %d is too new", cur))
		} else if cur != to {
			if err := db.MigrateUp(context.Background(), to); err != nil {
				panic(fmt.Errorf("sessiondb: migrate: migrate (%d to %d): %w", cur, to, err))
			}
		}
		cfg.SessionStorage = db
	}

	h, err := atlas.New(cfg)
	if err != nil {
		panic(err)
	}

	panic(http.ListenAndServe(":8080", h))
}
