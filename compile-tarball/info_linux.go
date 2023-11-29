package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func readStemcellSlug() (string, error) {
	stemcellVersionBuf, err := os.ReadFile("/var/vcap/bosh/etc/stemcell_version")
	if err != nil {
		return "", err
	}
	stemcellVersion := string(stemcellVersionBuf)
	osReleaseBuf, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	osReleaseEnv, err := parseEnvFile(bytes.NewReader(osReleaseBuf))
	if err != nil {
		return "", err
	}
	stemcellCodename, ok := osReleaseEnv["VERSION_CODENAME"]
	if !ok {
		return "", fmt.Errorf("VERSION_CODENAME not found in %s", osReleaseBuf)
	}
	stemcellOS, ok := osReleaseEnv["ID"]
	if !ok {
		return "", fmt.Errorf("VERSION_CODENAME not found in %s", osReleaseBuf)
	}
	stemcellOS = strings.TrimSpace(stemcellOS)
	stemcellCodename = strings.TrimSpace(stemcellCodename)
	stemcellVersion = strings.TrimSpace(stemcellVersion)
	return fmt.Sprintf("%s-%s/%s", stemcellOS, stemcellCodename, stemcellVersion), nil
}

func parseEnvFile(r io.Reader) (map[string]string, error) {
	br := bufio.NewReader(r)
	env := make(map[string]string)
	for i := 1; ; i++ {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return env, nil
			}
			return nil, err
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if len(value) > 0 && value[0] == '"' && value[len(value)-1] == '"' {
			value, err = strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("failed to unquote line %d: %q", i, line)
			}
		}
		env[key] = value
	}
}
