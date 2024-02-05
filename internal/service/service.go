package service

import (
	"github.com/notnull-co/pesca/internal/domain"
	"github.com/notnull-co/pesca/internal/integration/kubernetes"
	"github.com/notnull-co/pesca/internal/repository"
)

type svc struct {
	repo repository.Repository
	k8s  kubernetes.Kubernetes
}

type Service interface {
	UpdateDeployments() error
}

func New() Service {
	svc := &svc{
		repo: repository.New(),
		k8s:  kubernetes.New(),
	}

	go svc.UpdateDeployments()

	return svc
}

func (r *svc) StartRollback(isca domain.Isca, image domain.ImageRevision) error {
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

				_, err = r.repo.CreateIsca(isca)

				if err != nil {
					panic(err)
				}
			}
		}

	}
}
