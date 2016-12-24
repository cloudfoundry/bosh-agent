package syslog

type Logger interface {
	Debug(msg string) error
	Err(msg string) error
}
