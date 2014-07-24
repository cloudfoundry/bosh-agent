package packageapplier

import (
	models "github.com/cloudfoundry/bosh-agent/agent/applier/models"
)

type PackageApplier interface {
	Prepare(pkg models.Package) error
	Apply(pkg models.Package) error
	KeepOnly(pkgs []models.Package) error
}
