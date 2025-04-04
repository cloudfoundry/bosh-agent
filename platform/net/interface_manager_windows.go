package net

import (
	"bytes"
	"errors"
	goNet "net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetAdaptersInfo = syscall.MustLoadDLL("iphlpapi.dll").MustFindProc("GetAdaptersInfo") //nolint:gochecknoglobals
)

type windowsInterfaceManager struct{}

func NewInterfaceManager() InterfaceManager {
	return windowsInterfaceManager{}
}

func (windowsInterfaceManager) GetInterfaces() ([]Interface, error) {
	ifs, err := goNet.Interfaces()
	if err != nil {
		return nil, err
	}

	interfaces := make([]Interface, 0, len(ifs))
	for _, fs := range ifs {
		gateway, err := getGateway(fs.Index)
		if err != nil {
			return nil, err
		}
		netInterface := Interface{
			Name:    fs.Name,
			Gateway: gateway,
		}
		interfaces = append(interfaces, netInterface)
	}

	return interfaces, nil
}

func toString(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n == -1 {
		n = len(b)
	}
	return string(b[:n])
}

func getGateway(index int) (string, error) {
	first, err := getAdaptersInfo()
	if err != nil {
		return "", err
	}
	for info := first; info != nil; info = info.Next {
		if int(info.Index) == index {
			addr := toString(info.GatewayList.IpAddress.String[0:])
			return addr, nil
		}
	}

	return "", errors.New("interface not found")
}

func getAdaptersInfo() (*windows.IpAdapterInfo, error) {
	for n := 4096; n < 65536; n *= 2 {
		bufLen := uint32(n)
		buf := make([]byte, n)
		r0, _, _ := syscall.SyscallN(procGetAdaptersInfo.Addr(), //nolint:errcheck
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufLen)),
		)
		switch syscall.Errno(r0) { //nolint:exhaustive
		case 0:
			return (*windows.IpAdapterInfo)(unsafe.Pointer(&buf[0])), nil
		case windows.ERROR_BUFFER_OVERFLOW:
			// continue
		default:
			return nil, syscall.Errno(r0)
		}
	}
	return nil, errors.New("insufficient allocation")
}
