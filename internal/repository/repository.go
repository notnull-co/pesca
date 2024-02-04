package repository

import (
	"database/sql"

	"sync"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"
)

var (
	instance repository
	once     sync.Once
)

type repository struct {
	db *sql.DB
}

type Repository interface {
}

func New() Repository {
	once.Do(func() {
		db, err := sql.Open("sqlite3", "./foo.db")
		if err != nil {
			log.Fatal().Err(err).Msg("data layer initialization failed")
		}
		instance = repository{
			db: db,
		}
	})
	return &instance
}
