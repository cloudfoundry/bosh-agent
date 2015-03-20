package auth

import (
	"bytes"
	"crypto/x509/pkix"
	"github.com/cloudfoundry/bosh-agent/errors"
	"strings"
)

type DistinguishedNamesParser interface {
	Parse(string string) (*pkix.Name, error)
}

type distinguishedNamesParser struct{}

func NewDistinguishedNamesParser() DistinguishedNamesParser {
	return distinguishedNamesParser{}
}

func (parser distinguishedNamesParser) Parse(dn string) (*pkix.Name, error) {
	name := &pkix.Name{}

	if dn == "*" {
		return name, nil
	}

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

	value := buf.String()
	err := populate(ident, value)
	if err != nil {
		return nil, err
	}

	return name, nil
}
