package domain

import "time"

const (
	ImageStatusUnknown  ImageStatus = 0
	ImageStatusPending  ImageStatus = 1
	ImageStatusStable   ImageStatus = 2
	ImageStatusError    ImageStatus = 3
	ImageStatusRollback ImageStatus = 4
	ImageStatusOutdated ImageStatus = 5

	RollbackStrategyUnknown    RollbackStrategy = 0
	RollbackStrategyChangeback RollbackStrategy = 1
	RollbackStrategyRecreate   RollbackStrategy = 2
	RollbackStrategyRestart    RollbackStrategy = 3

	UnknownStrategy       PullingStrategy = 0
	LexicographicStrategy PullingStrategy = 1
	LatestByDateStrategy  PullingStrategy = 2
)

type (
	PullingStrategy  int
	ImageStatus      int
	RollbackStrategy int
)

type ImageRevision struct {
	Id                      int
	IscaId                  int
	PreviousImageRevisionId int
	Version                 string
	Status                  ImageStatus
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

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
	Id              int
	AnzolId         int
	Registry        Registry
	Deployment      Deployment
	Rollback        Rollback
	PullingStrategy PullingStrategy
}

type Image struct {
	Tag       string
	CreatedAt time.Time
	Digest    string
}

type NewImage struct {
	Isca
	ImageRevision
}
