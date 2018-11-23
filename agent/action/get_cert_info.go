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
)

type CertExpirationInfo struct {
	PropertyName string `json:"property"`
	Expires      int64  `json:"expires"`
	ErrorString  string `json:"error_string"`
}

type GetCertInfoAction struct {
	spec         applyspec.V1Service
	boshfs       boshsys.FileSystem
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

func (g GetCertInfoAction) Run() (map[string][]CertExpirationInfo, error) {
	v1Spec, err := g.spec.Get()
	if err != nil {
		return nil, bosherr.WrapError(err, "Failed get jobsSpecs")
	}

	jobList := make(map[string][]CertExpirationInfo)

	for _, job := range v1Spec.Jobs() {
		jobCertExpirationInfo := []CertExpirationInfo{}
		jobCerts := make(map[string]string)

		certFilePath := path.Join(g.jobDir, job.Name, "/config", g.certFileName)
		if g.boshfs.FileExists(certFilePath) {
			data, err := g.boshfs.ReadFile(certFilePath)
			if err != nil {
				return nil, bosherr.WrapError(err, "unable to read file")
			}

			err = yaml.Unmarshal(data, &jobCerts)
			if err != nil {
				return nil, bosherr.WrapError(err, fmt.Sprintf("Unmarshaling YAML for %s file failed", certFilePath))
			}

			for propertyName, cert := range jobCerts {
				certExpirationInfo := CertExpirationInfo{}

				expires, err := g.getCertExpiryDate(fmt.Sprintf("%v", cert))

				certExpirationInfo.PropertyName = fmt.Sprintf("%v", propertyName)
				certExpirationInfo.Expires = expires

				if err != nil {
					certExpirationInfo.ErrorString = err.Error()
				}

				jobCertExpirationInfo = append(jobCertExpirationInfo, certExpirationInfo)
			}

			jobList[job.Name] = jobCertExpirationInfo
		} else {
			return nil, bosherr.Errorf("%s not found", certFilePath)
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

func (g GetCertInfoAction) getCertExpiryDate(cert string) (int64, error) {
	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		return 0, errors.New("failed to decode certificate")
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, bosherr.WrapError(err, "failed to parse certificate")
	}

	return parsedCert.NotAfter.Unix(), nil
}
