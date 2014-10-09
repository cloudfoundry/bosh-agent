package alert

import (
	"fmt"
	"regexp"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsyslog "github.com/cloudfoundry/bosh-agent/syslog"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
	boshuuid "github.com/cloudfoundry/bosh-agent/uuid"
)

var syslogMessagePatterns = map[string]string{
	"disconnected by user":                  "SSH Logout",
	"Accepted publickey for":                "SSH Login",
	"Accepted password for":                 "SSH Login",
	"Failed password for":                   "SSH Access Denied",
	"Connection closed by .* \\[preauth\\]": "SSH Access Denied",
}

type sshAdapter struct {
	message         boshsyslog.Msg
	settingsService boshsettings.Service
	uuidGenerator   boshuuid.Generator
	timeService     boshtime.Service
	logger          boshlog.Logger
}

func NewSSHAdapter(
	message boshsyslog.Msg,
	settingsService boshsettings.Service,
	uuidGenerator boshuuid.Generator,
	timeService boshtime.Service,
	logger boshlog.Logger,
) Adapter {
	return &sshAdapter{
		message:         message,
		settingsService: settingsService,
		uuidGenerator:   uuidGenerator,
		timeService:     timeService,
		logger:          logger,
	}
}

func (m *sshAdapter) IsIgnorable() bool {
	_, found := m.title()
	return !found
}

func (m *sshAdapter) Alert() (Alert, error) {
	title, found := m.title()
	if !found {
		return Alert{}, nil
	}

	uuid, err := m.uuidGenerator.Generate()
	if err != nil {
		return Alert{}, bosherr.WrapError(err, "Generating uuid")
	}

	return Alert{
		ID:        uuid,
		Severity:  SeverityWarning,
		Title:     title,
		Summary:   m.message.Content,
		CreatedAt: m.timeService.Now().Unix(),
	}, nil
}

func (m *sshAdapter) title() (title string, found bool) {
	for pattern, title := range syslogMessagePatterns {
		matched, err := regexp.MatchString(pattern, m.message.Content)
		if err != nil {
			// Pattern failed to compile...
			fmt.Errorf("Failed matching syslog message: %s", err.Error())
		}
		if matched {
			return title, true
		}
	}
	return "", false
}
