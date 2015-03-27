package auth

import (
	"crypto/x509/pkix"

	"github.com/cloudfoundry/bosh-agent/errors"
)

type SSLConfig struct {
	CertFile  string
	KeyFile   string
	CACertPem string
	PkixNames []pkix.Name
}

func NewSSLConfig(certFile, keyFile, certPem string, allowedNames []string) (SSLConfig, error) {
	pkixNames, err := parseNames(allowedNames)
	if err != nil {
		return SSLConfig{}, err
	}

	return SSLConfig{
		CertFile:  certFile,
		KeyFile:   keyFile,
		CACertPem: certPem,
		PkixNames: pkixNames,
	}, nil
}

func parseNames(allowedNames []string) ([]pkix.Name, error) {
	if len(allowedNames) == 0 {
		return nil, errors.Error("AllowedNames must be specified")
	}

	var pkixNames []pkix.Name
	parser := NewDistinguishedNamesParser()
	for _, dn := range allowedNames {
		pkixName, err := parser.Parse(dn)
		if err != nil {
			return nil, errors.WrapError(err, "Invalid AllowedNames")
		}
		pkixNames = append(pkixNames, *pkixName)
	}

	return pkixNames, nil
}
