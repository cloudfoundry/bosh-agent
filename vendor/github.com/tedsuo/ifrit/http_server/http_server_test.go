package http_server_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
)

var _ = Describe("HttpServer", func() {
	var (
		address            string
		server             ifrit.Runner
		startedRequestChan chan struct{}
		finishRequestChan  chan struct{}

		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedRequestChan <- struct{}{}
			<-finishRequestChan
			w.Write([]byte("yo"))
		})
	)

	BeforeEach(func() {
		startedRequestChan = make(chan struct{}, 1)
		finishRequestChan = make(chan struct{}, 1)
		port := 8000 + GinkgoParallelNode()
		address = fmt.Sprintf("127.0.0.1:%d", port)
	})

	Describe("Invoke", func() {
		var process ifrit.Process

		Context("when the server starts successfully", func() {
			BeforeEach(func() {
				server = http_server.New(address, handler)
				process = ifrit.Invoke(server)
			})

			AfterEach(func() {
				process.Signal(syscall.SIGINT)
				Eventually(process.Wait()).Should(Receive())
			})

			Context("and a request is in flight", func() {
				type httpResponse struct {
					response *http.Response
					err      error
				}
				var responses chan httpResponse

				BeforeEach(func() {
					responses = make(chan httpResponse, 1)
					go func() {
						response, err := httpGet("http://" + address)
						responses <- httpResponse{response, err}
						close(responses)
					}()
					<-startedRequestChan
				})

				AfterEach(func() {
					Eventually(responses).Should(BeClosed())
				})

				It("serves http requests with the given handler", func() {
					finishRequestChan <- struct{}{}

					var resp httpResponse
					Eventually(responses).Should(Receive(&resp))

					Ω(resp.err).ShouldNot(HaveOccurred())

					body, err := ioutil.ReadAll(resp.response.Body)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(body)).Should(Equal("yo"))
				})

				Context("and it receives a signal", func() {
					BeforeEach(func() {
						process.Signal(syscall.SIGINT)
					})

					It("stops serving new http requests", func() {
						_, err := httpGet("http://" + address)
						Ω(err).Should(HaveOccurred())

						// make sure we exit
						finishRequestChan <- struct{}{}
					})

					It("does not return an error", func() {
						finishRequestChan <- struct{}{}
						err := <-process.Wait()
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("does not exit until all outstanding requests are complete", func() {
						Consistently(process.Wait()).ShouldNot(Receive())
						finishRequestChan <- struct{}{}
						Eventually(process.Wait()).Should(Receive())
					})
				})
			})
		})

		Context("when the server fails to start", func() {
			BeforeEach(func() {
				address = fmt.Sprintf("127.0.0.1:80")
				server = http_server.New(address, handler)
			})

			It("returns an error", func() {
				process = ifrit.Invoke(server)
				err := <-process.Wait()
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("and it starts a server with TLS", func() {
			var tlsConfig *tls.Config
			type httpResponse struct {
				response *http.Response
				err      error
			}
			var responses chan httpResponse

			BeforeEach(func() {
				basePath := path.Join(os.Getenv("GOPATH"), "src", "github.com", "tedsuo", "ifrit", "http_server", "test_certs")
				certFile := path.Join(basePath, "server.crt")
				keyFile := path.Join(basePath, "server.key")

				tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
				Expect(err).NotTo(HaveOccurred())

				tlsConfig = &tls.Config{
					Certificates:       []tls.Certificate{tlsCert},
					InsecureSkipVerify: true,
				}

				server = http_server.NewTLSServer(address, handler, tlsConfig)
				process = ifrit.Invoke(server)
			})

			AfterEach(func() {
				process.Signal(syscall.SIGINT)
				Eventually(process.Wait()).Should(Receive())
			})

			Context("and a valid, secure request is in flight", func() {
				BeforeEach(func() {
					responses = make(chan httpResponse, 1)
					go func() {
						response, err := httpTLSGet("https://"+address, tlsConfig)
						responses <- httpResponse{response, err}
						close(responses)
					}()
					Eventually(startedRequestChan).Should(Receive())
				})

				AfterEach(func() {
					Eventually(responses).Should(BeClosed())
				})

				It("serves tls-secured http requests with the given handler", func() {
					finishRequestChan <- struct{}{}

					var resp httpResponse
					Eventually(responses).Should(Receive(&resp))

					Ω(resp.err).ShouldNot(HaveOccurred())

					body, err := ioutil.ReadAll(resp.response.Body)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(body)).Should(Equal("yo"))
				})
			})

			Context("and an insecure request is in flight", func() {
				BeforeEach(func() {
					responses = make(chan httpResponse, 1)
					go func() {
						response, err := httpGet("http://" + address)
						responses <- httpResponse{response, err}
						close(responses)
					}()
					Consistently(startedRequestChan).ShouldNot(Receive())
				})

				AfterEach(func() {
					Eventually(responses).Should(BeClosed())
				})

				It("rejects insecure http requests and recieves an error", func() {
					finishRequestChan <- struct{}{}

					var resp httpResponse
					Eventually(responses).Should(Receive(&resp))

					Ω(resp.err).Should(HaveOccurred())
				})
			})
		})
	})
})

func httpGet(url string) (*http.Response, error) {
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}
	return client.Get(url)
}

func httpTLSGet(url string, tlsConfig *tls.Config) (*http.Response, error) {
	client := http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}
	return client.Get(url)
}
