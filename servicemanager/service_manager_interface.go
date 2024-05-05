package servicemanager

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . ServiceManager
type ServiceManager interface {
	Kill(serviceName string) error
	Setup(serviceName string) error
	Start(serviceName string) error
	Stop(serviceName string) error
}
