package repository

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"sync"

	"github.com/notnull-co/pesca/internal/config"
	"github.com/notnull-co/pesca/internal/domain"
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
	GetIsca(id int) (*domain.Isca, error)
}

func New() Repository {
	once.Do(func() {
		conf := config.Get()
		if err := ensureParentPathExists(conf.Database.Path); err != nil {
			log.Fatal().Err(err).Msg("database dir creation failed")
		}
		db, err := sql.Open("sqlite3", conf.Database.Path)
		if err != nil {
			log.Fatal().Err(err).Msg("data layer initialization failed")
		}
		instance = repository{
			db: db,
		}

		if err := instance.applySchema(conf.Database.Schema); err != nil {
			log.Fatal().Err(err).Msg("failed to create database schema")
		}
	})
	return &instance
}

func (r *repository) GetIsca(id int) (*domain.Isca, error) {
	rows, err := r.db.Query(`
	SELECT 
		I.Id, 
		I.DeploymentNamespace,
		I.DeploymentName, 
		I.DeploymentContainerName,
		COALESCE(I.RollbackTimeout, A.RollbackTimeout) AS RollbackTimeout,
		COALESCE(I.RollbackStrategy, A.RollbackStrategy) AS RollbackStrategy,
		COALESCE(I.RollbackEnabled, A.RollbackEnabled) AS RollbackEnabled
	FROM Isca I
	INNER JOIN Anzol A ON A.Id = I.AnzolId
	WHERE I.Id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rows.Next()

	var isca domain.Isca
	var rollbackTimeout, rollbackStrategy *int
	var rollbackEnabled *bool

	err = rows.Scan(&isca.Id,
		&isca.Deployment.Namespace,
		&isca.Deployment.Name,
		&isca.Deployment.ContainerName,
		&rollbackTimeout,
		&rollbackStrategy,
		&rollbackEnabled,
	)
	if err != nil {
		return nil, err
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}

	mapToDomain(&isca, rollbackTimeout, rollbackStrategy, rollbackEnabled)

	return &isca, nil
}

func mapToDomain(isca *domain.Isca, rollbackTimeout, rollbackStrategy *int, rollbackEnabled *bool) {
	if rollbackEnabled != nil && *rollbackEnabled {
		isca.Rollback = domain.Rollback{
			Enabled: true,
		}

		if rollbackTimeout != nil {
			isca.Rollback.Timeout = time.Duration(*rollbackTimeout)
		}

		if rollbackStrategy != nil {
			isca.Rollback.Strategy = domain.RollbackStrategy(*rollbackStrategy)
		}
	}
}

func (r *repository) applySchema(schemaFile string) error {
	schema, err := os.ReadFile(schemaFile)

	if err != nil {
		return err
	}

	_, err = r.db.Exec(string(schema))
	if err != nil {
		return err
	}
	return nil
}

func ensureParentPathExists(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, os.ModePerm)
}
