package openiscsi

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . OpenIscsi

type OpenIscsi interface {
	Setup(iqn, username, password string) (err error)
	Start() (err error)
	Stop() (err error)
	Restart() (err error)
	Discovery(ipAddress string) (err error)
	IsLoggedin() (bool, error)
	Login() (err error)
	Logout() (err error)
}
