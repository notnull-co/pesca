package domain

import "time"

const (
	ImageStatusUnknown  ImageStatus = 0
	ImageStatusPending  ImageStatus = 1
	ImageStatusStable   ImageStatus = 2
	ImageStatusError    ImageStatus = 3
	ImageStatusRollback ImageStatus = 3
)

type ImageStatus int

type ImageRevision struct {
	Id                      int
	IscaId                  int
	PreviousImageRevisionId int
	Version                 string
	Status                  ImageStatus
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

const (
	RollbackStrategyUnknown    RollbackStrategy = 0
	RollbackStrategyChangeback RollbackStrategy = 1
)

type RollbackStrategy int

type Rollback struct {
	Timeout  time.Duration
	Strategy RollbackStrategy
	Enabled  bool
}

type Deployment struct {
	Name          string
	Active        bool
	Namespace     string
	ContainerName string
}

type Anzol struct {
	Id int
}

type Isca struct {
	Id         int
	AnzolId    int
	Deployment Deployment
	Rollback   Rollback
}

type DeploymentImage struct {
	Deployment
	Image string
}
