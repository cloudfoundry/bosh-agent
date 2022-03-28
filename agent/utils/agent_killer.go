package utils

import (
	"os"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Killer

type Killer interface {
	KillAgent()
}

type AgentKiller struct {
}

func NewAgentKiller() AgentKiller {
	return AgentKiller{}
}

func (a AgentKiller) KillAgent() {
	// As actions are run before the task has been returned over the message bus,
	// this gives a bit of time for the director to learn about the task
	time.Sleep(1 * time.Second)
	os.Exit(0)
}
