package ip

type IPProtocol int

const (
	IPv4 IPProtocol = iota
	IPv6
)

func (p IPProtocol) String() string {
	switch p {
	case IPv4:
		return "IPv4"
	case IPv6:
		return "IPv6"
	default:
		return "Unknown"
	}
}
