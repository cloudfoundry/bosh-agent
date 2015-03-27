package auth

import (
	"crypto/x509/pkix"
	"path/filepath"

	"crypto/x509"

	"github.com/cloudfoundry/bosh-agent/errors"
)

type CertificateVerifier struct {
	AllowedNames []pkix.Name
}

func (p *CertificateVerifier) Verify(peerCertificates []*x509.Certificate) error {
	if len(peerCertificates) < 1 {
		return errors.Error("No peer certificates provided by client")
	}
	subject := peerCertificates[0].Subject
	for _, pattern := range p.AllowedNames {
		matched, err := matchName(&pattern, &subject)
		if err != nil {
			return err
		}
		if matched {
			return nil
		}
	}
	return errors.Errorf("Subject (%#v) didn't match allowed distinguished names", subject)
}

func compareStr(pattern, name string) (bool, error) {
	if pattern == "" {
		return true, nil
	}

	return filepath.Match(pattern, name)
}

func compareStrs(pattern, name []string) (bool, error) {
	if len(pattern) == 0 {
		return true, nil
	}

	if len(pattern) > 1 || len(name) > 1 {
		return false, errors.Error("Multiple entries in x509 records not supported")
	}

	if len(name) != 1 {
		return false, nil
	}

	return compareStr(pattern[0], name[0])
}

func matchName(pattern, name *pkix.Name) (matched bool, err error) {
	matched, err = compareStrs(pattern.Country, name.Country)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStrs(pattern.Organization, name.Organization)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStrs(pattern.OrganizationalUnit, name.OrganizationalUnit)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStrs(pattern.Locality, name.Locality)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStrs(pattern.Province, name.Province)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStr(pattern.SerialNumber, name.SerialNumber)
	if !matched || err != nil {
		return matched, err
	}
	matched, err = compareStr(pattern.CommonName, name.CommonName)
	if !matched || err != nil {
		return matched, err
	}

	return true, nil
}
