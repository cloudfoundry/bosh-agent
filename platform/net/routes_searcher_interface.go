package net

type Route struct {
	Destination   string
	Gateway       string
	Netmask       string
	InterfaceName string
}

type RoutesSearcher interface {
	SearchRoutes() ([]Route, error)
}

const DefaultAddress = `0.0.0.0`

func (r Route) IsDefault() bool {
	return r.Destination == DefaultAddress
}
