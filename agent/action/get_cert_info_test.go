package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	"fmt"
)

var _ = FDescribe("GetCertInfo", func() {
	var (
		action       GetCertInfoAction
		fileSystem   *fakesys.FakeFileSystem
		platform     *platformfakes.FakePlatform
		specService  *fakeas.FakeV1Service
		certsFileContent = ""
		certFilePath = "/var/vcap/jobs/fake-job/config/validate_certificate.yml"
	)

	BeforeEach(func() {
		fmt.Printf("Before:outside")
		platform = &platformfakes.FakePlatform{}

		fileSystem = fakesys.NewFakeFileSystem()
		err := fileSystem.MkdirAll("/var/vcap/jobs/fake-job", 0700)
		Expect(err).NotTo(HaveOccurred())

		err = fileSystem.WriteFileString(certFilePath, certsFileContent)
		Expect(err).NotTo(HaveOccurred())

		platform.GetFsReturns(fileSystem)

		specService = fakeas.NewFakeV1Service()
		action = NewGetCertInfoTask(specService, fileSystem)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("when certificate file for validation exists in job", func() {

		BeforeEach(func() {
			fmt.Printf("Before:Inside:top")
			specService.Spec = boshas.V1ApplySpec{
				RenderedTemplatesArchiveSpec: &boshas.RenderedTemplatesArchiveSpec{},
				JobSpec: boshas.JobSpec{
					JobTemplateSpecs: []boshas.JobTemplateSpec{
						{Name: "fake-job"},
					},
				},
			}
		})

		Context("when certs are valid", func() {
			BeforeEach(func() {
				fmt.Printf("Before:Inside")
				certsFileContent =`
nats.tls.client_ca.certificate : |
  -----BEGIN CERTIFICATE-----
  MIIEijCCAvKgAwIBAgIRAKlv5BEguA9GrlrfUVeWwAcwDQYJKoZIhvcNAQELBQAw
  TjEMMAoGA1UEBhMDVVNBMRYwFAYDVQQKEw1DbG91ZCBGb3VuZHJ5MSYwJAYDVQQD
  Ex1kZWZhdWx0Lm5hdHMtY2EuYm9zaC1pbnRlcm5hbDAeFw0xODExMjEyMTQzNTha
  Fw0xOTExMjEyMTQzNThaME4xDDAKBgNVBAYTA1VTQTEWMBQGA1UEChMNQ2xvdWQg
  Rm91bmRyeTEmMCQGA1UEAxMdZGVmYXVsdC5uYXRzLWNhLmJvc2gtaW50ZXJuYWww
  ggGiMA0GCSqGSIb3DQEBAQUAA4IBjwAwggGKAoIBgQDTM7eDeiesG1zZKGHWZdSd
  ZQMun/LmVwRCVlLFoutJj+78xoujrh0hMzQ1nHXsvI7kEmlvQfo1KmYTmWpiIgG9
  pVXHcsZgwDU+9ZCf4zrl0bTVHLLpkUX1c7FW2ptu1CxLdS8tp9Shk1OMqKL1oYcz
  63rVww1nso5nHZDt0Ew81fBdWLk34GPST9RlEUXh7r7IetInA9V1p/65hljj1gsG
  wIoqOdpdw3xj9BFt3TxUGtYdeC4PfVyxBl2I7w4w9PDTY84LSnGo6HDSBW43iU4k
  x1Cu922G265IMf4w2be51ZyoCkZnHOjb+Wo66ePfJ0Qg7bPHhZuNoqY4df6HAGyn
  MPQWJPORT3+/Ri6LLOTF1tghLGjBzWNaAkzfmAPHcCWgWc5WHwlTxmBPYtrts1Vg
  9ibOAdcaWz7S4n7FVk7Dh8Npi7RF0Ho8o6MDbcSDDowqlLqXYmieqzAjfCPKNtvk
  M5cJ4RCAtG5Po15JOE4HshwfE6gbc5yyLi8RcuWXacUCAwEAAaNjMGEwDgYDVR0P
  AQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFAgZx38UBXPQmtHU
  622eUCkz/97AMB8GA1UdIwQYMBaAFAgZx38UBXPQmtHU622eUCkz/97AMA0GCSqG
  SIb3DQEBCwUAA4IBgQDK6RJOG5AyaAi0VfPJiS1wX3J50mk6ui9krPUTrsE1pmSe
  jkluGVPtN66RWXggRjIvnV6C8ICKEOpkwvm2AHkWIxwjM9v76cWCoJs9iYX+BVr8
  IVOlkG/UY0rh6KIOEvS6dKgZbqSTtd1GB6iwini/BUSyIFQmYaDVrzjO/I6RAEnB
  HVWWM+yJ7uekKf55krQ85LuXIJYg/KugGyM3rnmiDu8unemSeUYDllJaPimxAsTO
  rZFz7paCLh5SF4ntNBsymO55vL2NTRE/D7PtUd41yQjGUlJmxzvEFdRUPo/1fcS4
  VluN6ZrYe5iS39c3o72T+dgLxWBo4XL8Ynfet6CD+BkZKTO8H0v2zKDhnq6tlvMu
  QqoEHFQ6x7sEn+SAACpV4Z+MMaWtrnzfG96DyyTtk1M1MLQowTjown4orABSuNn9
  5ka/AP3rwlh66oK1ktwmClpnNPkUumj9wPtyPS/AH04IjeIKfqO9JTPKg0VdEfOT
  LYlKT1StItAfXfZyfZs=
  -----END CERTIFICATE-----
`
				err := fileSystem.WriteFileString(certFilePath, certsFileContent)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns certificate details per job", func() {

				taskValue, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				// Check JSON key casing
				boshassert.MatchesJSONString(GinkgoT(), taskValue,
					`{"fake-job":[{"property":"nats.tls.client_ca.certificate","expires":"2019-11-21T21:43:58Z","error_string":""}]}`)
			})
		})

		Context("when certs are bad", func() {
			BeforeEach(func() {
				certsFileContent =`
nats.tls.client_ca.certificate : "BAD CERTIFICATE"
`
				err := fileSystem.WriteFileString(certFilePath, certsFileContent)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error details to the value", func() {
				taskValue, err := action.Run()
				Expect(err).ToNot(HaveOccurred())

				boshassert.MatchesJSONString(GinkgoT(), taskValue,
					`{"fake-job":[{"property":"nats.tls.client_ca.certificate","expires":"0001-01-01T00:00:00Z","error_string":"failed to decode certificate: \u003cnil cause\u003e"}]}`)
			})
		})

		Context("when jobs donot have `validate_certificate.yml`", func() {

		})

	})
})
