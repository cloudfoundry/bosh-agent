package net

type Interface struct {
	Name    string
	Gateway string
}

type InterfaceManager interface {
	GetInterfaces() ([]Interface, error)
}
