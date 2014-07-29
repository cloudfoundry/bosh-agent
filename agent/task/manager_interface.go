package task

import (
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type Info struct {
	TaskID  string
	Method  string
	Payload []byte
}

type ManagerProvider interface {
	NewManager(boshsys.FileSystem, string) Manager
}

type Manager interface {
	GetInfos() ([]Info, error)
	AddInfo(taskInfo Info) error
	RemoveInfo(taskID string) error
}
