package platform

import (
	"crypto/rand"
	"encoding/ascii85"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	userenv           = windows.MustLoadDLL("userenv.dll")
	procCreateProfile = userenv.MustFindProc("CreateProfile")
	procDeleteProfile = userenv.MustFindProc("DeleteProfileW")
)

func createProfile(sid, username string) (string, error) {
	const S_OK = 0x00000000
	psid, err := syscall.UTF16PtrFromString(sid)
	if err != nil {
		return "", err
	}
	pusername, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return "", err
	}
	var pathbuf [260]uint16
	r1, _, e1 := syscall.Syscall6(procCreateProfile.Addr(), 4,
		uintptr(unsafe.Pointer(psid)),        // _In_  LPCWSTR pszUserSid
		uintptr(unsafe.Pointer(pusername)),   // _In_  LPCWSTR pszUserName
		uintptr(unsafe.Pointer(&pathbuf[0])), // _Out_ LPWSTR  pszProfilePath
		uintptr(len(pathbuf)),                // _In_  DWORD   cchProfilePath
		0, // unused
		0, // unused
	)
	if r1 != S_OK {
		if e1 == 0 {
			return "", os.NewSyscallError("CreateProfile", syscall.EINVAL)
		}
		return "", os.NewSyscallError("CreateProfile", e1)
	}
	profilePath := syscall.UTF16ToString(pathbuf[0:])
	return profilePath, nil
}

func deleteProfile(sid, profilePath string) error {
	psid, err := syscall.UTF16PtrFromString(sid)
	if err != nil {
		return err
	}
	ppath, err := syscall.UTF16PtrFromString(profilePath)
	if err != nil {
		return err
	}
	r1, _, e1 := syscall.Syscall(procDeleteProfile.Addr(), 3,
		uintptr(unsafe.Pointer(psid)),
		uintptr(unsafe.Pointer(ppath)),
		0,
	)
	if r1 == 0 {
		if e1 == 0 {
			return os.NewSyscallError("DeleteProfile", syscall.EINVAL)
		}
		return os.NewSyscallError("DeleteProfile", e1)
	}
	return nil
}

func isSpecial(c byte) bool {
	return ('!' <= c && c <= '/') || (':' <= c && c <= '@') ||
		('[' <= c && c <= '`') || ('{' <= c && c <= '~')
}

// validPassword, checks if password s meets the Windows complexity
// requirements defined here:
//
//   https://technet.microsoft.com/en-us/library/hh994562(v=ws.11).aspx
//
func validPassword(s string) bool {
	var (
		digits    bool
		special   bool
		alphaLow  bool
		alphaHigh bool
	)
	if len(s) < 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case '0' <= c && c <= '9':
			digits = true
		case 'a' <= c && c <= 'z':
			alphaLow = true
		case 'A' <= c && c <= 'Z':
			alphaHigh = true
		case isSpecial(c):
			special = true
		}
	}
	var n int
	if digits {
		n++
	}
	if special {
		n++
	}
	if alphaLow {
		n++
	}
	if alphaHigh {
		n++
	}
	return n >= 3
}

// generatePassword, returns a 14 char ascii85 encoded password.
func generatePassword() (string, error) {
	const Length = 14

	in := make([]byte, ascii85.MaxEncodedLen(Length))
	if _, err := io.ReadFull(rand.Reader, in); err != nil {
		return "", err
	}

	out := make([]byte, ascii85.MaxEncodedLen(len(in)))
	if n := ascii85.Encode(out, in); n < Length {
		return "", errors.New("short password")
	}
	return string(out[:Length]), nil
}

// randomPassword, returns a ascii85 encoded 14 char password
// if the password is longer than 14 chars NET.exe will ask
// for confirmation due to backwards compatibility issues with
// Windows prior to Windows 2000.
func randomPassword() (string, error) {
	limit := 100
	for ; limit >= 0; limit-- {
		s, err := generatePassword()
		if err != nil {
			return "", err
		}
		if validPassword(s) {
			return s, nil
		}
	}
	return "", errors.New("failed to generate valid Windows password")
}

func userExists(name string) bool {
	_, _, t, err := syscall.LookupSID("", name)
	return err == nil && t == syscall.SidTypeUser
}

func createUserProfile(username string) error {
	if userExists(username) {
		return fmt.Errorf("user account already exists: %s", username)
	}

	// Create local user
	password, err := randomPassword()
	if err != nil {
		return err
	}
	createCmd := exec.Command("NET.exe", "USER", username, password, "/ADD")
	createOut, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating user (%s): %s", err, string(createOut))
	}

	// Add to Administrators group
	groupCmd := exec.Command("NET.exe", "LOCALGROUP", "Administrators", username, "/ADD")
	groupOut, err := groupCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error adding user to Administrator group (%s): %s",
			err, string(groupOut))
	}

	sid, _, _, err := syscall.LookupSID("", username)
	if err != nil {
		return err
	}
	ssid, err := sid.String()
	if err != nil {
		return err
	}
	_, err = createProfile(ssid, username)
	return err
}

func deleteUserProfile(username string) error {
	sid, _, _, err := syscall.LookupSID("", username)
	if err != nil {
		return err
	}
	ssid, err := sid.String()
	if err != nil {
		return err
	}
	drive, ok := os.LookupEnv("SYSTEMDRIVE")
	if !ok {
		return errors.New("missing SYSTEMDRIVE environment variable")
	}
	drive += "\\"
	profilePath := filepath.Join(drive, "Users", username)
	if err := deleteProfile(ssid, profilePath); err != nil {
		return err
	}

	cmd := exec.Command("NET.exe", "USER", username, "/DELETE")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error deleting user (%s): %s", err, string(out))
	}

	return nil
}
