package ntp

import (
	"regexp"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

var (
	offsetRegex    = regexp.MustCompile(`offset=(-?\d*\.\d*),`)
	clockRegex     = regexp.MustCompile(`clock=.*,\s+(\w+)\s+(\d+)\s+.*\s+(\d+:\d+:\d+)\..*`)
	badServerRegex = regexp.MustCompile(`(timed out|Connection refused)`)
)

type Info struct {
	Offset    string `json:"offset,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Service interface {
	GetInfo() (ntpInfo Info)
}

type concreteService struct {
	cmdRunner boshsys.CmdRunner
}

func NewConcreteService(cmdRunner boshsys.CmdRunner) Service {
	return concreteService{
		cmdRunner: cmdRunner,
	}
}

func (oc concreteService) GetInfo() Info {
	stdout, _, _, err := oc.cmdRunner.RunCommand("sh", "-c", "ntpq -c 'readvar 0 clock,offset'")
	if err != nil {
		return Info{Message: "can not query time by ntpq"}
	}

	if badServerRegex.MatchString(stdout) {
		return Info{Message: "ntp service is not available"}
	}

	offsetMatches := offsetRegex.FindAllStringSubmatch(stdout, -1)
	clockMatches := clockRegex.FindAllStringSubmatch(stdout, -1)

	info := Info{}

	if len(offsetMatches) > 0 && len(offsetMatches[0]) == 2 {
		info.Offset = offsetMatches[0][1]
	}

	if len(clockMatches) > 0 && len(clockMatches[0]) == 4 {
		info.Timestamp = clockMatches[0][2] + " " + clockMatches[0][1] + " " + clockMatches[0][3]
		return info
	}

	return Info{Message: "error querying time by ntpq"}
}
