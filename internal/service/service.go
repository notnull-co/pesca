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
}

func New() Service {
	return &svc{
		repo: repository.New(),
		k8s:  kubernetes.New(),
	}
}

func (r *svc) StartRollback(isca domain.Isca, image domain.ImageRevision) error {
	return nil
}
