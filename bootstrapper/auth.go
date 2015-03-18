package bootstrapper

import (
	"bytes"
	"crypto/x509/pkix"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/bosh-agent/errors"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type handlerWrapper struct {
	Handler         http.Handler
	Logger          logger.Logger
	CertAuthHandler CertAuthHandler
}

func (h *handlerWrapper) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	err := h.CertAuthHandler.Verify(req)
	if err != nil {
		rw.WriteHeader(http.StatusUnauthorized)
		h.Logger.Error("CertAuthHandler", errors.WrapError(err, "Unauthorized access").Error())
		return
	}

	h.Handler.ServeHTTP(rw, req)
}

type CertAuthHandler struct {
	Patterns []pkix.Name
}

func (p *CertAuthHandler) WrapHandler(logger logger.Logger, h http.Handler) http.Handler {
	return &handlerWrapper{
		Handler:         h,
		Logger:          logger,
		CertAuthHandler: *p,
	}
}

func (p *CertAuthHandler) Verify(req *http.Request) error {
	if req.TLS == nil || len(req.TLS.PeerCertificates) < 1 {
		return errors.Error("No peer certificates provided by client")
	}
	subject := req.TLS.PeerCertificates[0].Subject
	for _, pattern := range p.Patterns {
		matched, err := MatchName(&pattern, &subject)
		if err != nil {
			return err
		}
		if matched {
			return nil
		}
	}
	return errors.Errorf("Subject (%#v) didn't match allowed DNs", subject)
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

func MatchName(pattern, name *pkix.Name) (matched bool, err error) {
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

func ParseDistinguishedNames(names []string) (*CertAuthHandler, error) {
	var pkixNames []pkix.Name

	for _, dn := range names {
		if dn == "*" {
			pkixNames = append(pkixNames, pkix.Name{})
		} else {
			pkixName, err := ParseDistinguishedName(dn)
			if err != nil {
				return nil, errors.WrapError(err, "Invalid AllowedDNs")
			}
			pkixNames = append(pkixNames, *pkixName)
		}
	}

	if len(pkixNames) == 0 {
		return nil, errors.Error("AllowedDNs must be specified")
	}

	return &CertAuthHandler{Patterns: pkixNames}, nil
}

func ParseDistinguishedName(dn string) (*pkix.Name, error) {
	name := &pkix.Name{}

	populate := func(ident, value string) error {
		switch strings.ToLower(ident) {
		case "c":
			name.Country = append(name.Country, value)
		case "o":
			name.Organization = append(name.Organization, value)
		case "ou":
			name.OrganizationalUnit = append(name.OrganizationalUnit, value)
		case "l":
			name.Locality = append(name.Locality, value)
		case "st":
			name.Province = append(name.Province, value)
		case "serialnumber":
			name.SerialNumber = value
		case "cn":
			name.CommonName = value
		default:
			return errors.Errorf("Unknown field '%s'", ident)
		}
		return nil
	}

	var (
		buf    bytes.Buffer
		ident  = ""
		escape = false
	)

	for _, c := range dn {
		if escape {
			buf.WriteRune(c)
			escape = false
		} else if c == '=' {
			ident = buf.String()
			buf.Truncate(0)
		} else if c == ',' {
			value := buf.String()
			buf.Truncate(0)
			err := populate(ident, value)
			if err != nil {
				return nil, err
			}
			ident = ""
		} else if c == '\\' {
			escape = true
		} else {
			buf.WriteRune(c)
		}
	}

	if ident != "" {
		value := buf.String()
		err := populate(ident, value)
		if err != nil {
			return nil, err
		}
	}

	return name, nil
}
