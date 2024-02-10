package domain

import "time"

const (
	ImageStatusUnknown  ImageStatus = 0
	ImageStatusPending  ImageStatus = 1
	ImageStatusStable   ImageStatus = 2
	ImageStatusError    ImageStatus = 3
	ImageStatusRollback ImageStatus = 4
	ImageStatusOutdated ImageStatus = 5
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
	RollbackStrategyRecreate   RollbackStrategy = 2
	RollbackStrategyRestart    RollbackStrategy = 3
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
	Repository    string
}

type Anzol struct {
	Id       int
	Registry Registry
}

type Registry struct {
	Url string
	// TODO: Add credentials HERE
}

type Isca struct {
	Id         int
	AnzolId    int
	Registry   Registry
	Deployment Deployment
	Rollback   Rollback
}
