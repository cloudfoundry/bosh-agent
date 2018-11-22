package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"gopkg.in/yaml.v2"
	"path"

	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

type CertExpiration struct {
	PropertyName string `json:"property"`
	Expires      time.Time `json:"expires"`
	ErrorString  string `json:"error_string"`
}

type GetCertInfoAction struct {
	spec         applyspec.V1Service
	boshfs 		boshsys.FileSystem
	jobDir       string
	certFileName string
}

func NewGetCertInfoTask(spec applyspec.V1Service, boshsf boshsys.FileSystem) (getCertInfo GetCertInfoAction) {
	getCertInfo.spec = spec
	getCertInfo.boshfs = boshsf
	getCertInfo.jobDir = "/var/vcap/jobs"
	getCertInfo.certFileName = "validate_certificate.yml"
	return
}

// TODO change it to true and make is async
func (g GetCertInfoAction) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (g GetCertInfoAction) IsPersistent() bool {
	return false
}

func (g GetCertInfoAction) IsLoggable() bool {
	return true
}

func (g GetCertInfoAction) Run() (map[string][]CertExpiration, error) {
	v1Spec, err := g.spec.Get()
	if err != nil {
		return nil, bosherr.WrapError(err, "Failed get jobsSpecs")
	}

	jobList := make(map[string][]CertExpiration)

	for _, job := range v1Spec.Jobs() {
		certExpiration := []CertExpiration{}
		certExpirationInfo := CertExpiration{}

		certs :=  make(map[string]string)
		certFilePath := path.Join(g.jobDir, job.Name, "/config", g.certFileName)

		if g.boshfs.FileExists(certFilePath) {
			data, err := g.boshfs.ReadFile(certFilePath)
			fmt.Printf("\ndata: %v", string(data))
			if err != nil {
				return nil, bosherr.WrapError(err, "not able to readfile")
			}

			err = yaml.Unmarshal(data, &certs)
			if err != nil {
				return nil, bosherr.WrapError(err, fmt.Sprintf("loading %s file failed", certFilePath))
			}

			for propertyName, cert := range certs {
				expires, err := g.validateCert(fmt.Sprintf("%v",cert))
				certExpirationInfo.PropertyName = fmt.Sprintf("%v",propertyName)
				if err != nil {
					certExpirationInfo.ErrorString =  err.Error()
				} else {
					certExpirationInfo.Expires = expires
				}
				certExpiration = append(certExpiration, certExpirationInfo)
			}

			jobList[job.Name] = certExpiration
		}else{
			jobList[job.Name] = certExpiration
		}

	}

	return jobList, nil
}

func (g GetCertInfoAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (g GetCertInfoAction) Cancel() error {
	return errors.New("not supported")
}

func (g GetCertInfoAction) validateCert(cert string) (time.Time, error) {
	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		return time.Time{}, bosherr.WrapError(nil, "failed to decode certificate")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, bosherr.WrapError(err, "failed to parse certificate")
	}

	return parsedCert.NotAfter, nil
}
