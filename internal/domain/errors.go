package domain

import "errors"

var (
	NoBackwardsRevision = errors.New("no backwards revision found")
)
