package applyspec

import (
	models "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
)

type ApplySpec interface {
	Jobs() []models.Job
	Packages() []models.Package
	MaxLogFileSize() string
}
