package openiscsi

type OpenIscsi interface {
	Setup(iqn, username, password string) (err error)
	Start() (err error)
	Stop() (err error)
	Restart() (err error)
	Discovery(ipAddress string) (err error)
	Login() (err error)
	Logout() (err error)
}
