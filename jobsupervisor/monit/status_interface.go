package monit

type Status interface {
	GetIncarnation() (int, error)
	ServicesInGroup(name string) (services []Service)
}

type Service struct {
	Name      string
	Monitored bool
	Status    string
}
