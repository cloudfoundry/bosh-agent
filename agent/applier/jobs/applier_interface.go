package jobs

import (
	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Applier

type Applier interface {
	Prepare(job models.Job) error
	Apply(job models.Job) error
	Configure(job models.Job, jobIndex int) error
	KeepOnly(jobs []models.Job) error
	DeleteSourceBlobs(jobs []models.Job) error
}
