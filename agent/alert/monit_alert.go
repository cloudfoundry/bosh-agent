package alert

// JobFailureAlert carries information about a job process failure,
// regardless of whether it originated from monit, systemd, or another supervisor.
type JobFailureAlert struct {
	ID          string
	Service     string
	Event       string
	Action      string
	Date        string // RFC1123Z formatted date string
	Description string
}
