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
	GetIsca(namespace, deploymentName, containerName string) (*domain.Isca, error)
	GetIscaById(id int) (*domain.Isca, error)
	UpdateIsca(isca domain.Isca) (*domain.Isca, error)
	DisableIscaById(id int) (*domain.Isca, error)
	DisableIsca(isca domain.Isca) (*domain.Isca, error)
	CreateIsca(isca domain.Isca) (*domain.Isca, error)
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

func (r *repository) CreateIsca(isca domain.Isca) (*domain.Isca, error) {
	result, err := r.db.Exec("INSERT INTO Isca(AnzolId, DeploymentActive, DeploymentNamespace, DeploymentName, DeploymentContainerName) VALUES (?, ?, ?, ?, ?)", isca.AnzolId, isca.Deployment.Active, isca.Deployment.Namespace, isca.Deployment.Name, isca.Deployment.ContainerName)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()

	if err != nil {
		return nil, err
	}

	return r.GetIscaById(int(id))
}

func (r *repository) UpdateIsca(isca domain.Isca) (*domain.Isca, error) {
	_, err := r.db.Exec("UPDATE Isca SET DeploymentActive = ?, DeploymentNamespace = ?, DeploymentName = ?, DeploymentContainerName = ? WHERE Id = ?", isca.Deployment.Active, isca.Deployment.Namespace, isca.Deployment.Name, isca.Deployment.ContainerName, isca.Id)
	if err != nil {
		return nil, err
	}

	return r.GetIscaById(isca.Id)
}

func (r *repository) DisableIscaById(id int) (*domain.Isca, error) {
	isca, err := r.GetIscaById(id)

	if err != nil {
		return nil, err
	}

	return r.DisableIsca(*isca)
}

func (r *repository) DisableIsca(isca domain.Isca) (*domain.Isca, error) {
	isca.Deployment.Active = false
	return r.UpdateIsca(isca)
}

func (r *repository) GetIsca(namespace, deploymentName, containerName string) (*domain.Isca, error) {
	return r.getIsca("WHERE I.DeploymentNamespace = ? AND I.DeploymentName = ? AND I.DeploymentContainerName = ?", namespace, deploymentName, containerName)
}

func (r *repository) GetIscaById(id int) (*domain.Isca, error) {
	return r.getIsca("WHERE I.Id = ?", id)
}

func (r *repository) getIsca(where string, args ...any) (*domain.Isca, error) {

	// TODO: change this left join to inner join to ensure that the anzol exists
	rows, err := r.db.Query(`
	SELECT 
		I.Id,
		I.AnzolId,
		I.DeploymentActive,
		I.DeploymentNamespace,
		I.DeploymentName, 
		I.DeploymentContainerName,
		A.RollbackTimeout,
		A.RollbackStrategy,
		A.RollbackEnabled
	FROM Isca I
	LEFT JOIN Anzol A ON A.Id = I.AnzolId
	`+where, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var isca domain.Isca
		var rollbackTimeout, rollbackStrategy *int
		var rollbackEnabled *bool

		err = rows.Scan(&isca.Id,
			&isca.AnzolId,
			&isca.Deployment.Active,
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

	return nil, nil
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
