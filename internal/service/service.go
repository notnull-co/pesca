package service

import (
	"strings"
	"time"

	"github.com/notnull-co/pesca/internal/domain"
	"github.com/notnull-co/pesca/internal/integration/kubernetes"
	"github.com/notnull-co/pesca/internal/integration/registry"
	"github.com/notnull-co/pesca/internal/repository"
	"github.com/rs/zerolog/log"
)

type svc struct {
	repo repository.Repository
	k8s  kubernetes.Kubernetes
	reg  registry.RegistryClient
}

type Service interface {
	UpdateDeployments() error
	StartRollback(isca domain.Isca, image domain.ImageRevision) error
}

func New() Service {
	svc := &svc{
		repo: repository.New(),
		k8s:  kubernetes.New(),
		reg:  registry.NewRegistry(),
	}

	go svc.UpdateDeployments()
	go svc.UpdateImagesThroughPollingStrategy()

	return svc
}

var (
	// TODO: fix this names
	registryMap = map[string]string{
		"docker.io": "https://index.docker.io",
		"quay.io":   "https://quay.io",
		"gcr.io":    "https://gcr.io",
		"amazonaws": "https://amazonaws",
		"azure":     "https://azure",
	}
)

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

				// TODO: read this from a config file
				isca.PullingStrategy = domain.LatestByDateStrategy

				registry, repository := extractRegistryAndRepository(createdDeployment.Image)

				isca.Registry.RegistryURL = registry
				isca.Registry.Repository = repository

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

func (r *svc) startPolling() (chan *domain.NewImage, error) {
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

			latestImage, err := r.reg.PollingImage(isca.Registry.RegistryURL, isca.Registry.Repository, isca.PullingStrategy)
			if err != nil {
				return err
			}

			imageRevision, err := r.repo.GetImageRevisionByIscaId(isca.Id)
			if err != nil {
				return err
			}

			if imageRevision != nil && imageRevision.Version == latestImage.Digest {
				return nil
			}

			newImage, err := r.repo.CreateImageRevision(domain.ImageRevision{
				IscaId:                  isca.Id,
				PreviousImageRevisionId: imageRevision.Id,
				Version:                 latestImage.Digest,
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

func (r *svc) UpdateImagesThroughPollingStrategy() {
	channel, err := r.startPolling()
	if err != nil {
		log.Error().Err(err).Msg("polling failed")
	}

	for {
		imageToUpdate := <-channel

		err := r.k8s.UpdateImage(imageToUpdate.Isca, imageToUpdate.ImageRevision)
		if err != nil {
			log.Error().Err(err).Msg("an error occurred when trying to updated the image")
		}
	}
}

func extractRegistryAndRepository(image string) (string, string) {
	parts := strings.Split(image, "/")

	tagIndex := strings.LastIndex(parts[len(parts)-1], ":")
	if tagIndex != -1 {
		parts[len(parts)-1] = parts[len(parts)-1][:tagIndex]
	}

	if len(parts) < 2 {
		return registryMap["docker.io"], parts[0]
	}

	if len(parts) < 3 {
		return registryMap["docker.io"], strings.Join(parts[:2], "/")
	}

	registry := parts[0]
	if _, ok := registryMap[registry]; ok && len(parts) > 2 {
		return registry, strings.Join(parts[1:3], "/")
	}

	return "", ""
}
