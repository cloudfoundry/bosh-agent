package infrastructure_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakeboshsys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("HTTPMetadataService", describeCachingMetadataService)

func describeCachingMetadataService() {
	var (
		metadataHeaders     map[string]string
		dnsResolver         *fakeinf.FakeDNSResolver
		platform            *platformfakes.FakePlatform
		logger              boshlog.Logger
		sshKeysPath         string
		httpMetadataService HTTPMetadataService
		metadataService     MetadataService
		fs                  *fakeboshsys.FakeFileSystem
	)

	BeforeEach(func() {
		metadataHeaders = make(map[string]string)
		metadataHeaders["key"] = "value"
		dnsResolver = &fakeinf.FakeDNSResolver{}
		platform = &platformfakes.FakePlatform{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fs = fakeboshsys.NewFakeFileSystem()

		sshKeysPath = "/ssh-keys"
		httpMetadataService = NewHTTPMetadataServiceInstance("fake-metadata-host", metadataHeaders, "/user-data", "/instanceid", sshKeysPath, dnsResolver, platform, logger)
		metadataService = NewCachingMetadataService("/fake-user-data-cache-path", dnsResolver, fs, logger, httpMetadataService)
	})

	ItEnsuresMinimalNetworkSetup := func(subject func() (string, error)) {
		Context("when no networks are configured", func() {
			BeforeEach(func() {
				platform.GetConfiguredNetworkInterfacesReturns([]string{}, nil)
			})

			It("sets up DHCP network", func() {
				_, err := subject()
				Expect(err).ToNot(HaveOccurred())

				Expect(platform.SetupNetworkingCallCount()).To(Equal(1))
				networks := platform.SetupNetworkingArgsForCall(0)
				Expect(networks).To(Equal(boshsettings.Networks{
					"eth0": boshsettings.Network{
						Type: "dynamic",
					},
				}))
			})

			Context("when setting up DHCP fails", func() {
				BeforeEach(func() {
					platform.SetupNetworkingReturns(errors.New("fake-network-error"))
				})

				It("returns an error", func() {
					_, err := subject()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-network-error"))
				})
			})
		})
	}

	Describe("GetServerName", func() {
		var (
			ts                  *httptest.Server
			serverName          *string
			serverNameFromCache *string

			userdataCachePath    *string
			userdataCacheContent string
		)

		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			Expect(r.Method).To(Equal("GET"))
			Expect(r.URL.Path).To(Equal("/user-data"))
			Expect(r.Header.Get("key")).To(Equal("value"))

			var jsonStr string

			if serverName == nil {
				jsonStr = `{}`
			} else {
				jsonStr = fmt.Sprintf(`{"server":{"name":"%s"}}`, *serverName)
			}

			w.Write([]byte(jsonStr))
		}

		BeforeEach(func() {
			name := "fake-server-name"
			serverName = &name
			nameFromCache := "fake-server-name-from-cache"
			serverNameFromCache = &nameFromCache
			httpRegistryCachePath := "/fake-user-data-cache-path"
			userdataCachePath = &httpRegistryCachePath
			userdataCacheContent = fmt.Sprintf("{\"server\":{\"name\":\"%s\"}}", nameFromCache)

			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			httpMetadataService = NewHTTPMetadataServiceInstance(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, dnsResolver, platform, logger)
			metadataService = NewCachingMetadataService(httpRegistryCachePath, dnsResolver, fs, logger, httpMetadataService)
		})

		AfterEach(func() {
			ts.Close()
		})

		Context("when the server name is present in the JSON", func() {
			It("returns the server name", func() {
				name, err := metadataService.GetServerName()
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal(*serverName))
			})

			It("returns the server name from local cache", func() {
				fs.WriteFileString(*userdataCachePath, userdataCacheContent)

				name, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(name).To(Equal(*serverNameFromCache))
			})

			It("returns the server name when cache file could not be read", func() {
				fs.ReadFileError = errors.New("fake-read-error")

				name, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(name).To(Equal(*serverName))
			})

			It("returns the server name when cache contains invalid json", func() {
				badUserdataCacheContent := "{\"\"registry\":{\"endpoint\":\"http://fake-registry-from-cache\"}}"
				fs.WriteFileString(*userdataCachePath, badUserdataCacheContent)

				name, err := metadataService.GetServerName()
				Expect(err).NotTo(HaveOccurred())
				Expect(name).To(Equal(*serverName))
			})

			It("returns error because cache file could not be written", func() {
				fs.WriteFileError = errors.New("fake-write-error")

				name, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-write-error"))
				Expect(name).To(BeEmpty())
				Expect(fs.FileExists(*userdataCachePath)).To(BeFalse())
			})

			ItEnsuresMinimalNetworkSetup(func() (string, error) {
				return metadataService.GetServerName()
			})
		})

	})

	Describe("GetRegistryEndpoint", func() {
		var (
			ts                   *httptest.Server
			registryURL          *string
			dnsServer            *string
			userdataCachePath    *string
			userdataCacheContent string
		)

		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			Expect(r.Method).To(Equal("GET"))
			Expect(r.URL.Path).To(Equal("/user-data"))
			Expect(r.Header.Get("key")).To(Equal("value"))

			var jsonStr string

			if dnsServer == nil {
				jsonStr = fmt.Sprintf(`{"registry":{"endpoint":"%s"}}`, *registryURL)
			} else {
				jsonStr = fmt.Sprintf(`{
					"registry":{"endpoint":"%s"},
					"dns":{"nameserver":["%s"]}
				}`, *registryURL, *dnsServer)
			}

			w.Write([]byte(jsonStr))
		}

		BeforeEach(func() {
			url := "http://fake-registry.com"
			registryURL = &url
			dnsServer = nil
			httpRegistryCachePath := "/fake-user-data-cache-path"
			userdataCachePath = &httpRegistryCachePath
			userdataCacheContent = "{\"registry\":{\"endpoint\":\"http://fake-registry-ip-from-cache\"}}"

			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			httpMetadataService = NewHTTPMetadataServiceInstance(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, dnsResolver, platform, logger)
			metadataService = NewCachingMetadataService(httpRegistryCachePath, dnsResolver, fs, logger, httpMetadataService)
		})

		AfterEach(func() {
			ts.Close()
		})

		ItEnsuresMinimalNetworkSetup(func() (string, error) {
			return metadataService.GetRegistryEndpoint()
		})

		Context("when metadata contains a dns server", func() {
			BeforeEach(func() {
				server := "fake-dns-server-ip"
				dnsServer = &server
			})

			Context("when registry endpoint is successfully resolved", func() {
				BeforeEach(func() {
					dnsResolver.RegisterRecord(fakeinf.FakeDNSRecord{
						DNSServers: []string{"fake-dns-server-ip"},
						Host:       "http://fake-registry.com",
						IP:         "http://fake-registry-ip",
					})
				})

				It("returns successfully resolved registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).ToNot(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip"))
				})

				It("returns successfully registry endpoint from local cache", func() {
					fs.WriteFileString(*userdataCachePath, userdataCacheContent)

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).NotTo(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip-from-cache"))
				})

				It("returns successfully resolved registry endpoint when cache file could not be read", func() {
					fs.ReadFileError = errors.New("fake-read-error")

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).NotTo(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip"))
				})

				It("returns successfully resolved registry endpoint when cache contains invalid json", func() {
					badUserdataCacheContent := "{\"\"registry\":{\"endpoint\":\"http://fake-registry-from-cache\"}}"
					fs.WriteFileString(*userdataCachePath, badUserdataCacheContent)

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).NotTo(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip"))
				})

				It("returns error because cache file could not be written", func() {
					fs.WriteFileError = errors.New("fake-write-error")

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-write-error"))
					Expect(endpoint).To(BeEmpty())
					Expect(fs.FileExists(*userdataCachePath)).To(BeFalse())
				})
			})

			Context("when registry endpoint is not successfully resolved", func() {
				BeforeEach(func() {
					dnsResolver.LookupHostErr = errors.New("fake-lookup-host-err")
				})

				It("returns error because it failed to resolve registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-lookup-host-err"))
					Expect(endpoint).To(BeEmpty())
				})
			})
		})

		Context("when metadata does not contain dns servers", func() {
			It("returns fetched registry endpoint", func() {
				endpoint, err := metadataService.GetRegistryEndpoint()
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).To(Equal("http://fake-registry.com"))
			})

			Context("when enabling local cache", func() {
				BeforeEach(func() {
					sshKeysPath = "/ssh-keys"
					httpMetadataService = NewHTTPMetadataServiceInstance(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, dnsResolver, platform, logger)
					metadataService = NewCachingMetadataService(*userdataCachePath, dnsResolver, fs, logger, httpMetadataService)
				})

				It("returns the successfully resolved registry endpoint from metadata host and stores in local cache", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).NotTo(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry.com"))
					Expect(fs.ReadFileString(*userdataCachePath)).To(Equal(fmt.Sprintf(`{"registry":{"endpoint":"%s"},"server":{},"dns":{}}`, *registryURL)))
					Expect(fs.FileExists(*userdataCachePath)).To(BeTrue())
				})

				It("returns the successfully resolved registry endpoint from local cache", func() {
					fs.WriteFileString(*userdataCachePath, userdataCacheContent)

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).NotTo(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip-from-cache"))
					Expect(fs.FileExists(*userdataCachePath)).To(BeTrue())
				})

				It("returns error because cache file could not be written", func() {
					fs.WriteFileError = errors.New("fake-write-error")

					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-write-error"))
					Expect(endpoint).To(BeEmpty())
					Expect(fs.FileExists(*userdataCachePath)).To(BeFalse())
				})
			})
		})
	})
}
