package packages

import (
	boshbc "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
)

type ApplierProvider interface {
	Root() Applier
	JobSpecific(jobName string) Applier
	RootBundleCollection() boshbc.BundleCollection
}
