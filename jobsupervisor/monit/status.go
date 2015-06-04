package monit

import (
	"encoding/xml"
	"strconv"
)

type status struct {
	XMLName     xml.Name `xml:"monit"`
	ID          string   `xml:"id,attr"`
	Incarnation string   `xml:"incarnation,attr"`
	Version     string   `xml:"version,attr"`

	Services      servicesTag
	ServiceGroups serviceGroupsTag
}

type servicesTag struct {
	XMLName  xml.Name     `xml:"services"`
	Services []serviceTag `xml:"service"`
}

type serviceTag struct {
	XMLName       xml.Name `xml:"service"`
	Name          string   `xml:"name,attr"`
	Monitor       int      `xml:"monitor"`
	Pending       int      `xml:"pendingaction"`
	Status        int      `xml:"status"`
	StatusMessage string   `xml:"status_message"`
}

type serviceGroupsTag struct {
	XMLName       xml.Name          `xml:"servicegroups"`
	ServiceGroups []serviceGroupTag `xml:"servicegroup"`
}

type serviceGroupTag struct {
	XMLName xml.Name `xml:"servicegroup"`
	Name    string   `xml:"name,attr"`

	Services []string `xml:"service"`
}

func (s serviceTag) StatusString() string {
	switch {
	case s.Monitor == 0:
		return StatusUnknown
	case s.Monitor == 2:
		return StatusStarting
	case s.Status == 0:
		return StatusRunning
	default:
		return StatusFailing
	}
}

func (t serviceGroupsTag) Get(name string) (group serviceGroupTag, found bool) {
	for _, g := range t.ServiceGroups {
		if g.Name == name {
			group = g
			found = true
			return
		}
	}
	return
}

func (t serviceGroupTag) Contains(name string) bool {
	for _, serviceName := range t.Services {
		if serviceName == name {
			return true
		}
	}
	return false
}

func (status status) ServicesInGroup(name string) (services []Service) {
	services = []Service{}

	serviceGroupTag, found := status.ServiceGroups.Get(name)
	if !found {
		return
	}

	for _, serviceTag := range status.Services.Services {
		if serviceGroupTag.Contains(serviceTag.Name) {
			service := Service{
				Name:      serviceTag.Name,
				Monitored: serviceTag.Monitor > 0,
				Pending:   serviceTag.Pending > 0,
				Status:    serviceTag.StatusString(),
				Errored:   serviceTag.Status > 0 && serviceTag.StatusMessage != "", // review this
			}

			services = append(services, service)
		}
	}

	return
}

func (status status) GetIncarnation() (int, error) {
	return strconv.Atoi(status.Incarnation)
}
