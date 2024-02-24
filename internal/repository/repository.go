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
	GetIscas() ([]*domain.Isca, error)
	GetIsca(namespace, deploymentName, containerName string) (*domain.Isca, error)
	GetIscaById(id int) (*domain.Isca, error)
	UpdateIsca(isca domain.Isca) (*domain.Isca, error)
	DisableIscaById(id int) (*domain.Isca, error)
	DisableIsca(isca domain.Isca) (*domain.Isca, error)
	CreateIsca(isca domain.Isca) (*domain.Isca, error)
	GetImageRevisionById(id int) (*domain.ImageRevision, error)
	GetImageRevisionByIscaId(iscaId int) (*domain.ImageRevision, error)
	CreateImageRevision(imageRevision domain.ImageRevision) (*domain.ImageRevision, error)
	UpdateStatusImageRevision(imageRevision domain.ImageRevision) (*domain.ImageRevision, error)
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
	result, err := r.db.Exec("INSERT INTO Isca (AnzolId, RegistryUrl, DeploymentName, DeploymentActive, DeploymentNamespace, DeploymentContainerName, DeploymentRepository, PullingStrategy) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", isca.AnzolId, isca.Registry.Url, isca.Deployment.Name, isca.Deployment.Active, isca.Deployment.Namespace, isca.Deployment.ContainerName, isca.Deployment.Repository, isca.PullingStrategy)
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

func (r *repository) CreateImageRevision(imageRevision domain.ImageRevision) (*domain.ImageRevision, error) {
	result, err := r.db.Exec("INSERT INTO ImageRevision(IscaId, PreviousImageRevisionId, Version, Status, CreatedAt, UpdatedAt) VALUES (?, ?, ?, ?, ?, ?)", imageRevision.IscaId, imageRevision.PreviousImageRevisionId, imageRevision.Version, imageRevision.Status, imageRevision.CreatedAt, imageRevision.UpdatedAt)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()

	if err != nil {
		return nil, err
	}

	return r.GetImageRevisionById(int(id))
}

func (r *repository) UpdateStatusImageRevision(imageRevision domain.ImageRevision) (*domain.ImageRevision, error) {
	_, err := r.db.Exec("UPDATE ImageRevision SET Status = ?, UpdatedAt = ? WHERE Id = ?", imageRevision.Status, imageRevision.UpdatedAt, imageRevision.Id)
	if err != nil {
		return nil, err
	}

	return r.GetImageRevisionById(imageRevision.Id)
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

func (r *repository) GetIscas() ([]*domain.Isca, error) {
	return r.getIscas("")
}

func (r *repository) getIsca(where string, args ...any) (*domain.Isca, error) {
	iscas, err := r.getIscas(where, args...)

	if len(iscas) > 0 {
		return iscas[0], err
	}

	return nil, err
}

func (r *repository) getIscas(where string, args ...any) ([]*domain.Isca, error) {

	// TODO: change this left join to inner join to ensure that the Anzol exists
	rows, err := r.db.Query(`
	SELECT 
		I.Id,
		I.AnzolId,
		I.RegistryUrl,
		I.DeploymentName, 
		I.DeploymentActive,
		I.DeploymentNamespace,
		I.DeploymentContainerName,
		I.DeploymentRepository,
		A.RollbackTimeout,
		A.RollbackStrategy,
		A.RollbackEnabled,
		I.PullingStrategy
	FROM Isca I
	LEFT JOIN Anzol A ON A.Id = I.AnzolId
	`+where, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var iscas []*domain.Isca

	for rows.Next() {
		var isca domain.Isca
		var rollbackTimeout, rollbackStrategy *int
		var rollbackEnabled *bool

		err = rows.Scan(&isca.Id,
			&isca.AnzolId,
			&isca.Registry.Url,
			&isca.Deployment.Name,
			&isca.Deployment.Active,
			&isca.Deployment.Namespace,
			&isca.Deployment.ContainerName,
			&isca.Deployment.Repository,
			&rollbackTimeout,
			&rollbackStrategy,
			&rollbackEnabled,
			&isca.PullingStrategy,
		)
		if err != nil {
			return nil, err
		}
		err = rows.Err()
		if err != nil {
			return nil, err
		}

		mapIscaToDomain(&isca, rollbackTimeout, rollbackStrategy, rollbackEnabled)

		iscas = append(iscas, &isca)
	}

	return iscas, nil
}

func (r *repository) GetImageRevisionById(id int) (*domain.ImageRevision, error) {
	return r.getImageRevision("WHERE I.Id = ?", id)
}

func (r *repository) GetImageRevisionByIscaId(iscaId int) (*domain.ImageRevision, error) {
	return r.getImageRevision("WHERE I.IscaId = ?", iscaId)
}

func (r *repository) getImageRevision(where string, args ...any) (*domain.ImageRevision, error) {
	iscas, err := r.getImageRevisions(where, args...)

	if len(iscas) > 0 {
		return iscas[0], err
	}

	return nil, err
}

func (r *repository) getImageRevisions(where string, args ...any) ([]*domain.ImageRevision, error) {

	rows, err := r.db.Query(`
	SELECT
    	I.Id,
    	I.IscaId,
    	I.PreviousImageRevisionId,
    	I.Version,
    	I.Status,
    	I.CreatedAt,
    	I.UpdatedAt
	FROM ImageRevision I
	`+where+`
	ORDER BY I.CreatedAt DESC`, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*domain.ImageRevision

	for rows.Next() {
		var revision domain.ImageRevision

		err = rows.Scan(&revision.Id,
			&revision.IscaId,
			&revision.PreviousImageRevisionId,
			&revision.Version,
			&revision.Status,
			&revision.CreatedAt,
			&revision.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		err = rows.Err()
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, &revision)
	}

	return revisions, nil
}

func mapIscaToDomain(isca *domain.Isca, rollbackTimeout, rollbackStrategy *int, rollbackEnabled *bool) {
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
