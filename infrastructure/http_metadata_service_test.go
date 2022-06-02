package infrastructure_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("HTTPMetadataService", describeHTTPMetadataService)

func describeHTTPMetadataService() { //nolint:funlen
	var (
		metadataHeaders map[string]string
		dnsResolver     *fakeinf.FakeDNSResolver
		platform        *platformfakes.FakePlatform
		logger          boshlog.Logger
		metadataService MetadataService
	)

	BeforeEach(func() {
		metadataHeaders = make(map[string]string)
		metadataHeaders["key"] = "value"
		dnsResolver = &fakeinf.FakeDNSResolver{}
		platform = &platformfakes.FakePlatform{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		metadataService = NewHTTPMetadataService("fake-metadata-host", metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "", platform, logger)
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
				networks, _ := platform.SetupNetworkingArgsForCall(0)
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

	Describe("IsAvailable", func() {
		It("returns true", func() {
			Expect(metadataService.IsAvailable()).To(BeTrue())
		})
	})

	Describe("GetPublicKey", func() {
		var (
			ts          *httptest.Server
			sshKeysPath string
		)

		Context("when using IMDSv1", func() {
			BeforeEach(func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/ssh-keys"))
					Expect(r.Header.Get("key")).To(Equal("value"))

					_, err := w.Write([]byte("fake-public-key"))
					Expect(err).NotTo(HaveOccurred())
				})
				ts = httptest.NewServer(handler)
			})

			AfterEach(func() {
				ts.Close()
			})

			Context("when the ssh keys path is present", func() {
				BeforeEach(func() {
					sshKeysPath = "/ssh-keys"
					metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, "", platform, logger)
				})

				It("returns fetched public key", func() {
					publicKey, err := metadataService.GetPublicKey()
					Expect(err).NotTo(HaveOccurred())
					Expect(publicKey).To(Equal("fake-public-key"))
				})

				ItEnsuresMinimalNetworkSetup(func() (string, error) {
					return metadataService.GetPublicKey()
				})
			})

			Context("when the ssh keys path is not present", func() {
				BeforeEach(func() {
					sshKeysPath = ""
					metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, "", platform, logger)
				})

				It("returns an empty ssh key", func() {
					publicKey, err := metadataService.GetPublicKey()
					Expect(err).NotTo(HaveOccurred())
					Expect(publicKey).To(BeEmpty())
				})
			})
		})

		Context("when IMDSv2 is required", func() {
			var tokenCalls int
			BeforeEach(func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()

					// requests for new tokens use the PUT HTTP verb
					if r.Method == "PUT" {
						Expect(r.URL.Path).To(Equal("/token"))
						Expect(r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds")).To(Equal("300"))
						tokenCalls++

						_, err := w.Write([]byte("this-is-a-token"))
						Expect(err).NotTo(HaveOccurred())
						return
					}

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/ssh-keys"))
					Expect(r.Header.Get("key")).To(Equal("value"))
					Expect(r.Header.Get("X-aws-ec2-metadata-token")).To(Equal("this-is-a-token"))

					_, err := w.Write([]byte("fake-public-key"))
					Expect(err).NotTo(HaveOccurred())
				})
				ts = httptest.NewServer(handler)

				metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "/token", platform, logger)
			})

			AfterEach(func() {
				ts.Close()
			})
			It("returns fetched public key", func() {
				publicKey, err := metadataService.GetPublicKey()
				Expect(err).NotTo(HaveOccurred())
				Expect(tokenCalls).NotTo(BeZero())
				Expect(publicKey).To(Equal("fake-public-key"))
			})
		})
	})

	Describe("GetEmptyPublicKey", func() {
		var (
			ts          *httptest.Server
			sshKeysPath string
		)

		BeforeEach(func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()

				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/ssh-keys"))
				Expect(r.Header.Get("key")).To(Equal("value"))
			})
			ts = httptest.NewServer(handler)
		})

		AfterEach(func() {
			ts.Close()
		})

		Context("when the ssh keys path is present but key value is empty", func() {
			BeforeEach(func() {
				sshKeysPath = "/ssh-keys"
				metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", sshKeysPath, "", platform, logger)
			})

			It("returns empty public key", func() {
				publicKey, err := metadataService.GetPublicKey()
				Expect(err).NotTo(HaveOccurred())
				Expect(publicKey).To(BeEmpty())
			})

			ItEnsuresMinimalNetworkSetup(func() (string, error) {
				return metadataService.GetPublicKey()
			})
		})
	})

	Describe("GetInstanceID", func() {
		var (
			ts             *httptest.Server
			instanceIDPath string
		)

		Context("when using IMDSv1", func() {
			BeforeEach(func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/instanceid"))
					Expect(r.Header.Get("key")).To(Equal("value"))

					_, err := w.Write([]byte("fake-instance-id"))
					Expect(err).NotTo(HaveOccurred())
				})
				ts = httptest.NewServer(handler)
			})

			AfterEach(func() {
				ts.Close()
			})

			Context("when the instance ID path is present", func() {
				BeforeEach(func() {
					instanceIDPath = "/instanceid"
					metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", instanceIDPath, "/ssh-keys", "", platform, logger)
				})

				It("returns fetched instance id", func() {
					instanceID, err := metadataService.GetInstanceID()
					Expect(err).NotTo(HaveOccurred())
					Expect(instanceID).To(Equal("fake-instance-id"))
				})

				ItEnsuresMinimalNetworkSetup(func() (string, error) {
					return metadataService.GetInstanceID()
				})
			})

			Context("when the instance ID path is not present", func() {
				BeforeEach(func() {
					instanceIDPath = ""
					metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", instanceIDPath, "/ssh-keys", "", platform, logger)
				})

				It("returns an empty instance ID", func() {
					instanceID, err := metadataService.GetInstanceID()
					Expect(err).NotTo(HaveOccurred())
					Expect(instanceID).To(BeEmpty())
				})
			})
		})

		Context("when IMDSv2 is required", func() {
			var tokenCalls int
			BeforeEach(func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()

					// requests for new tokens use the PUT HTTP verb
					if r.Method == "PUT" {
						Expect(r.URL.Path).To(Equal("/token"))
						Expect(r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds")).To(Equal("300"))
						tokenCalls++

						_, err := w.Write([]byte("this-is-a-token"))
						Expect(err).NotTo(HaveOccurred())
						return
					}

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/instanceid"))
					Expect(r.Header.Get("key")).To(Equal("value"))
					Expect(r.Header.Get("X-aws-ec2-metadata-token")).To(Equal("this-is-a-token"))

					_, err := w.Write([]byte("fake-instance-id"))
					Expect(err).NotTo(HaveOccurred())
				})
				ts = httptest.NewServer(handler)

				metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "/token", platform, logger)
			})

			AfterEach(func() {
				ts.Close()
			})
			It("returns fetched instance id", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(tokenCalls).NotTo(BeZero())
				Expect(instanceID).To(Equal("fake-instance-id"))
			})
		})

		Context("when a tokenPath is set, but the region does not support IMDSv2 (which could be a thing that could happen, we don't know we can't verify)", func() {
			var tokenCalls int
			BeforeEach(func() {
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()

					// requests for new tokens use the PUT HTTP verb
					if r.Method == "PUT" {
						Expect(r.URL.Path).To(Equal("/token"))
						Expect(r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds")).To(Equal("300"))
						tokenCalls++

						w.WriteHeader(500)
						_, err := w.Write([]byte("ceci-nest-pas-une-token"))
						Expect(err).NotTo(HaveOccurred())
						return
					}

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/instanceid"))
					Expect(r.Header.Get("key")).To(Equal("value"))
					Expect(r.Header.Get("X-aws-ec2-metadata-token")).To(Equal(""))

					_, err := w.Write([]byte("fake-instance-id"))
					Expect(err).NotTo(HaveOccurred())
				})
				ts = httptest.NewServer(handler)

				metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "/token", platform, logger)
			})

			AfterEach(func() {
				ts.Close()
			})
			It("returns fetched instance id", func() {
				instanceID, err := metadataService.GetInstanceID()
				Expect(err).NotTo(HaveOccurred())
				Expect(tokenCalls).NotTo(BeZero())
				Expect(instanceID).To(Equal("fake-instance-id"))
			})
		})
	})

	Describe("GetServerName", func() {
		var (
			ts         *httptest.Server
			serverName *string
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

			_, err := w.Write([]byte(jsonStr))
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			serverName = nil

			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "", platform, logger)
		})

		AfterEach(func() {
			ts.Close()
		})

		Context("when the server name is present in the JSON", func() {
			BeforeEach(func() {
				name := "fake-server-name"
				serverName = &name
			})

			It("returns the server name", func() {
				name, err := metadataService.GetServerName()
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("fake-server-name"))
			})

			ItEnsuresMinimalNetworkSetup(func() (string, error) {
				return metadataService.GetServerName()
			})
		})

		Context("when the server name is not present in the JSON", func() {
			BeforeEach(func() {
				serverName = nil
			})

			It("returns an error", func() {
				name, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(name).To(BeEmpty())
			})
		})
	})

	Describe("GetRegistryEndpoint", func() {
		var (
			ts          *httptest.Server
			registryURL *string
			dnsServer   *string
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

			_, err := w.Write([]byte(jsonStr))
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			url := "http://fake-registry.com"
			registryURL = &url
			dnsServer = nil

			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "", platform, logger)
		})

		AfterEach(func() {
			ts.Close()
		})

		//	ItEnsuresMinimalNetworkSetup(func() (string, error) {
		//		return metadataService.GetRegistryEndpoint()
		//	})

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

				//			It("returns the successfully resolved registry endpoint", func() {
				//				endpoint, err := metadataService.GetRegistryEndpoint()
				//				Expect(err).ToNot(HaveOccurred())
				//				Expect(endpoint).To(Equal("http://fake-registry-ip"))
				//			})
			})

			Context("when registry endpoint is not successfully resolved", func() {
				BeforeEach(func() {
					dnsResolver.LookupHostErr = errors.New("fake-lookup-host-err")
				})

				//			It("returns error because it failed to resolve registry endpoint", func() {
				//				endpoint, err := metadataService.GetRegistryEndpoint()
				//				Expect(err).To(HaveOccurred())
				//				Expect(err.Error()).To(ContainSubstring("fake-lookup-host-err"))
				//				Expect(endpoint).To(BeEmpty())
				//			})
			})
		})

		//	Context("when metadata does not contain dns servers", func() {
		//		It("returns fetched registry endpoint", func() {
		//			endpoint, err := metadataService.GetRegistryEndpoint()
		//			Expect(err).NotTo(HaveOccurred())
		//			Expect(endpoint).To(Equal("http://fake-registry.com"))
		//		})
		//	})
	})

	Describe("GetNetworks", func() {
		It("returns nil networks, since you don't need them for bootstrapping since your network must be set up before you can get the metadata", func() {
			Expect(metadataService.GetNetworks()).To(BeNil())
		})
	})

	Describe("Retryable Metadata Service Request", func() {
		var (
			ts *httptest.Server
			//		registryURL *string
			//		dnsServer   *string
		)

		//	createHandlerFunc := func(count int) func(http.ResponseWriter, *http.Request) {
		//		initialCount := 0
		//		return func(w http.ResponseWriter, r *http.Request) {
		//			if initialCount < count {
		//				initialCount++
		//				http.Error(w, http.StatusText(500), 500)
		//				return
		//			}

		//			var jsonStr string
		//			if dnsServer == nil {
		//				jsonStr = fmt.Sprintf(`{"registry":{"endpoint":"%s"}}`, *registryURL)
		//			} else {
		//				jsonStr = fmt.Sprintf(`{
		//				"registry":{"endpoint":"%s"},
		//				"dns":{"nameserver":["%s"]}
		//			}`, *registryURL, *dnsServer)
		//			}
		//			_, err := w.Write([]byte(jsonStr))
		//			Expect(err).NotTo(HaveOccurred())
		//		}
		//	}

		BeforeEach(func() {
			//		url := "http://fake-registry.com"
			//		registryURL = &url
			//		dnsServer = nil
		})

		AfterEach(func() {
			ts.Close()
		})

		Context("when server returns an HTTP Response with status code ==2xx (as defined by the request retryable) within 10 retries", func() {
			BeforeEach(func() {
				dnsResolver.RegisterRecord(fakeinf.FakeDNSRecord{
					DNSServers: []string{"fake-dns-server-ip"},
					Host:       "http://fake-registry.com",
					IP:         "http://fake-registry-ip",
				})
			})

			//		It("returns the successfully resolved registry endpoint", func() {
			//			handler := http.HandlerFunc(createHandlerFunc(9))
			//			ts = httptest.NewServer(handler)
			//			metadataService = NewHTTPMetadataServiceWithCustomRetryDelay(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys",  platform, logger, 0*time.Second)

			//			endpoint, err := metadataService.GetRegistryEndpoint()
			//			Expect(err).ToNot(HaveOccurred())
			//			Expect(endpoint).To(Equal("http://fake-registry.com"))
			//		})
		})

		//	Context("when server returns an HTTP Response with status code !=2xx (as defined by the request retryable) more than 10 times", func() {
		//		It("returns an error containing the HTTP Response", func() {
		//			handler := http.HandlerFunc(createHandlerFunc(10))
		//			ts = httptest.NewServer(handler)
		//			metadataService = NewHTTPMetadataServiceWithCustomRetryDelay(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys",  platform, logger, 0*time.Second)

		//			_, err := metadataService.GetRegistryEndpoint()
		//			Expect(err).To(MatchError(fmt.Sprintf("Getting user data: invalid status from url %s/user-data: 500", ts.URL)))
		//		})
		//	})
	})

	Describe("GetServerName from url encoded user data", func() {
		var (
			ts      *httptest.Server
			jsonStr *string
		)

		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			Expect(r.Method).To(Equal("GET"))
			Expect(r.URL.Path).To(Equal("/user-data"))
			Expect(r.Header.Get("key")).To(Equal("value"))
			_, err := w.Write([]byte(*jsonStr))
			Expect(err).NotTo(HaveOccurred())
		}

		BeforeEach(func() {
			handler := http.HandlerFunc(handlerFunc)
			ts = httptest.NewServer(handler)
			metadataService = NewHTTPMetadataService(ts.URL, metadataHeaders, "/user-data", "/instanceid", "/ssh-keys", "", platform, logger)
		})

		AfterEach(func() {
			ts.Close()
		})

		Context("when the server name is present in the JSON", func() {
			BeforeEach(func() {
				encodedJSON := base64.RawURLEncoding.EncodeToString([]byte(`{"server":{"name":"fake-server-name"}}`))
				jsonStr = &encodedJSON
			})

			It("returns the server name", func() {
				name, err := metadataService.GetServerName()
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("fake-server-name"))
			})

			ItEnsuresMinimalNetworkSetup(func() (string, error) {
				return metadataService.GetServerName()
			})
		})

		Context("when the URL encoding is corrupt", func() {
			BeforeEach(func() {
				// This is std base64 encoding, not url encoding. This should cause a decode err.
				encodedJSON := base64.StdEncoding.EncodeToString([]byte(`{"server":{"name":"fake-server-name"}}`))
				jsonStr = &encodedJSON
			})

			It("returns an error", func() {
				_, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Decoding url encoded user data"))
			})
		})

		Context("when the JSON is malformed", func() {
			BeforeEach(func() {
				encodedJSON := base64.RawURLEncoding.EncodeToString([]byte(`{"server bad json]`))
				jsonStr = &encodedJSON
			})

			It("returns an error", func() {
				_, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unmarshalling url decoded user data '{\"server bad json]'"))
			})
		})

		Context("when the server name is not present in the JSON", func() {
			BeforeEach(func() {
				encodedJSON := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
				jsonStr = &encodedJSON
			})

			It("returns an error", func() {
				name, err := metadataService.GetServerName()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Empty server name"))
				Expect(name).To(BeEmpty())
			})
		})
	})
}
