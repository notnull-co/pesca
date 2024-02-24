package service

import (
	"context"
	"time"

	"github.com/notnull-co/pesca/internal/domain"
	"github.com/notnull-co/pesca/internal/integration/kubernetes"
	"github.com/notnull-co/pesca/internal/integration/registry"
	"github.com/notnull-co/pesca/internal/repository"
)

type svc struct {
	repo repository.Repository
	k8s  kubernetes.Kubernetes
	reg  registry.RegistryClient
}

type Service interface {
	UpdateDeployments() error
	StartRollback(isca domain.Isca, image domain.ImageRevision) error
	StartPolling(ctx context.Context) (chan *domain.NewImage, error)
}

func New() Service {
	svc := &svc{
		repo: repository.New(),
		k8s:  kubernetes.New(),
		reg:  registry.NewRegistry(),
	}

	go svc.UpdateDeployments()

	return svc
}

func (r *svc) StartRollback(isca domain.Isca, revision domain.ImageRevision) error {
	oldRevision, err := r.repo.GetImageRevisionById(revision.PreviousImageRevisionId)
	if err != nil {
		return err
	}
	if oldRevision == nil {
		return domain.NoBackwardsRevision
	}
	err = r.k8s.UpdateImage(isca, *oldRevision)
	if err != nil {
		return err
	}

	return nil
}

func (r *svc) UpdateDeployments() error {
	createdCh := make(chan map[domain.Deployment]string)
	deletedCh := make(chan map[domain.Deployment]string)
	updatedCh := make(chan *kubernetes.DeploymentUpdate)

	filters := map[string]string{
		"pescar": "true",
	}

	stop, err := r.k8s.WatchDeployments(filters, updatedCh, createdCh, deletedCh)

	if err != nil {
		return err
	}

	defer func() {
		stop <- struct{}{}
	}()

	for {
		select {
		case deployments := <-updatedCh:
			for updatedDeployment := range deployments.New {
				isca, err := r.repo.GetIsca(updatedDeployment.Namespace, updatedDeployment.Name, updatedDeployment.ContainerName)

				if err != nil {
					panic(err)
				}

				isca.Deployment = updatedDeployment

				_, err = r.repo.UpdateIsca(*isca)

				if err != nil {
					panic(err)
				}
			}

		case deployments := <-deletedCh:
			for deletedDeployment := range deployments {
				isca, err := r.repo.GetIsca(deletedDeployment.Namespace, deletedDeployment.Name, deletedDeployment.ContainerName)

				if err != nil {
					panic(err)
				}

				isca.Deployment.Active = false

				_, err = r.repo.UpdateIsca(*isca)

				if err != nil {
					panic(err)
				}
			}

		case deployments := <-createdCh:
			for createdDeployment := range deployments {
				existingIsca, err := r.repo.GetIsca(createdDeployment.Namespace, createdDeployment.Name, createdDeployment.ContainerName)

				if err != nil {
					panic(err)
				}

				if existingIsca != nil && existingIsca.Id > 0 {
					continue
				}

				var isca domain.Isca

				isca.Deployment = createdDeployment

				// TODO: Fix this. Add Anzol repository methods
				isca.AnzolId = 1

				isca.Rollback.Timeout = time.Hour * 1
				_, err = r.repo.CreateIsca(isca)

				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func (r *svc) StartPolling(ctx context.Context) (chan *domain.NewImage, error) {
	iscas, err := r.repo.GetIscas()
	if err != nil {
		return nil, err
	}

	revisionsToUpdate := make(chan *domain.NewImage)
	for _, isca := range iscas {
		if !isca.Deployment.Active {
			continue
		}

		go func(isca domain.Isca) error {
			lastUpdatedImage, err := r.reg.PollingImage(isca.Registry.Url, isca.Deployment.Repository, isca.PullingStrategy)
			if err != nil {
				return err
			}

			imageRevision, err := r.repo.GetImageRevisionByIscaId(isca.Id)
			if err != nil {
				return err
			}

			if imageRevision == nil {
				imageRevision = &domain.ImageRevision{}
			}

			if imageRevision.Version == lastUpdatedImage.Digest {
				return nil
			}

			newImage, err := r.repo.CreateImageRevision(domain.ImageRevision{
				IscaId:                  isca.Id,
				PreviousImageRevisionId: imageRevision.Id,
				Version:                 lastUpdatedImage.Digest,
				CreatedAt:               time.Now(),
				Status:                  domain.ImageStatusPending,
			})
			if err != nil {
				return err
			}

			revisionsToUpdate <- &domain.NewImage{
				Isca:          isca,
				ImageRevision: *newImage,
			}

			return nil
		}(*isca)
	}

	return revisionsToUpdate, nil
}
