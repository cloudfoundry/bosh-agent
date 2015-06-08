package agentclient

import (
	bias "github.com/cloudfoundry/bosh-agent/deployment/applyspec"
	"github.com/cloudfoundry/bosh-agent/settings"
)

type AgentClient interface {
	Ping() (string, error)
	Stop() error
	Apply(bias.ApplySpec) error
	Start() error
	GetState() (AgentState, error)
	MountDisk(string) error
	UnmountDisk(string) error
	ListDisk() ([]string, error)
	MigrateDisk() error
	CompilePackage(packageSource BlobRef, compiledPackageDependencies []BlobRef) (compiledPackageRef BlobRef, err error)
	UpdateSettings(settings settings.Settings) (string, error)
}

type AgentState struct {
	JobState string
}

type BlobRef struct {
	Name        string
	Version     string
	BlobstoreID string
	SHA1        string
}
