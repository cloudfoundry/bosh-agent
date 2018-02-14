// +build !windows

package net

import "errors"

type unixInterfaceManager struct{}

func NewInterfaceManager() InterfaceManager {
	return unixInterfaceManager{}
}

func (unixInterfaceManager) GetInterfaces() ([]Interface, error) {
	return nil, errors.New("Not implemented on unix")
}
