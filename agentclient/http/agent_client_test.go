package http_test

import (
	"encoding/json"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/cloudfoundry/bosh-agent/v2/agentclient"
	"github.com/cloudfoundry/bosh-agent/v2/agentclient/applyspec"
	. "github.com/cloudfoundry/bosh-agent/v2/agentclient/http"
)

var _ = Describe("AgentClient", func() {
	var (
		server      *ghttp.Server
		agentClient agentclient.AgentClient

		agentAddress        string
		replyToAddress      string
		toleratedErrorCount int
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		logger := boshlog.NewLogger(boshlog.LevelNone)
		httpClient := httpclient.NewHTTPClient(httpclient.DefaultClient, logger)

		agentAddress = server.URL()
		replyToAddress = "fake-reply-to-uuid"

		getTaskDelay := time.Duration(0)
		toleratedErrorCount = 2

		agentClient = NewAgentClient(agentAddress, replyToAddress, getTaskDelay, toleratedErrorCount, httpClient, logger)
	})

	disconnectingRequestHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		Expect(err).NotTo(HaveOccurred())

		conn.Close() //nolint:errcheck
	})

	Describe("get_task", func() {
		Context("when the http client errors", func() {
			It("should retry", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					disconnectingRequestHandler,
					disconnectingRequestHandler,
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":"stopped"}`),
					),
				)

				err := agentClient.Stop()
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the http client errors more times than the error retry count", func() {
				It("should return the error", func() {
					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/agent"),
							ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						),
						disconnectingRequestHandler,
						disconnectingRequestHandler,
						disconnectingRequestHandler,
					)

					err := agentClient.Stop()
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("Post \"%s/agent\": EOF", server.URL())))
					Expect(server.ReceivedRequests()).To(HaveLen(4))
				})
			})

			Context("when the https client errors, recovers, and begins erroring again", func() {
				It("should reset the error count when a successful call goes through", func() {
					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/agent"),
							ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						),
						disconnectingRequestHandler,
						disconnectingRequestHandler,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/agent"),
							ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						),
						disconnectingRequestHandler,
						disconnectingRequestHandler,
						disconnectingRequestHandler,
					)

					err := agentClient.Stop()
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("Post \"%s/agent\": EOF", server.URL())))
					Expect(server.ReceivedRequests()).To(HaveLen(7))
				})
			})
		})
	})

	Describe("Ping", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":"pong"}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "ping",
						Arguments: []interface{}{},
						ReplyTo:   replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.Ping()
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns the value", func() {
				responseValue, err := agentClient.Ping()
				Expect(err).ToNot(HaveOccurred())
				Expect(responseValue).To(Equal("pong"))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				_, err := agentClient.Ping()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				_, err := agentClient.Ping()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("Stop", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "stop",
							Arguments: []interface{}{},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":"stopped"}`),
					),
				)
			})

			It("makes a POST request to the endpoint", func() {
				err := agentClient.Stop()
				Expect(err).ToNot(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})

			It("waits for the task to be finished", func() {
				err := agentClient.Stop()
				Expect(err).ToNot(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				err := agentClient.Stop()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				err := agentClient.Stop()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("Drain", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "drain",
							Arguments: []interface{}{"shutdown", map[string]interface{}{}},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":15}`),
					),
				)
			})

			It("makes a POST request to the endpoint and waits for the task to be finished", func() {
				response, err := agentClient.Drain("shutdown")
				Expect(err).ToNot(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(4))
				Expect(response).To(Equal(int64(15)))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				_, err := agentClient.Drain("shutdown")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				_, err := agentClient.Drain("shutdown")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("Apply", func() {
		var (
			spec applyspec.ApplySpec
		)

		BeforeEach(func() {
			spec = applyspec.ApplySpec{
				Deployment: "fake-deployment-name",
			}
			var err error
			_, err = json.Marshal(spec)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "apply",
						Arguments: []interface{}{spec},
						ReplyTo:   replyToAddress,
					}),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "get_task",
						Arguments: []interface{}{"fake-agent-task-id"},
						ReplyTo:   replyToAddress,
					}),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":"stopped"}`),
				))
			})

			It("makes a POST request to the endpoint", func() {
				err := agentClient.Apply(spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})

			It("waits for the task to be finished", func() {
				err := agentClient.Apply(spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				err := agentClient.Apply(spec)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				err := agentClient.Apply(spec)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("Start", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":"started"}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "start",
						Arguments: []interface{}{},
						ReplyTo:   replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				err := agentClient.Start()
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				err := agentClient.Start()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				err := agentClient.Start()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("GetState", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"job_state":"running","networks":{"private":{"ip":"192.0.2.10"},"public":{"ip":"192.0.3.11"}}}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "get_state",
						Arguments: []interface{}{},
						ReplyTo:   replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				stateResponse, err := agentClient.GetState()
				Expect(err).ToNot(HaveOccurred())
				Expect(stateResponse).To(Equal(agentclient.AgentState{
					JobState: "running",
					NetworkSpecs: map[string]agentclient.NetworkSpec{
						"private": {
							IP: "192.0.2.10",
						},
						"public": {
							IP: "192.0.3.11",
						},
					},
				}))

				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.RespondWith(http.StatusInternalServerError, ""),
					ghttp.RespondWith(http.StatusInternalServerError, ""),
					ghttp.RespondWith(http.StatusInternalServerError, ""),
				)
			})

			It("returns an error", func() {
				stateResponse, err := agentClient.GetState()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
				Expect(stateResponse).To(Equal(agentclient.AgentState{}))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				stateResponse, err := agentClient.GetState()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
				Expect(stateResponse).To(Equal(agentclient.AgentState{}))
			})
		})

		Context("when agent client errors sending the http request less times than the sendErrorCount", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					disconnectingRequestHandler,
					disconnectingRequestHandler,
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"job_state":"running"}}`),
					),
				)
			})

			It("retries the up to error count specified", func() {
				stateResponse, err := agentClient.GetState()
				Expect(err).ToNot(HaveOccurred())
				Expect(stateResponse).To(Equal(agentclient.AgentState{JobState: "running"}))
			})
		})

		Context("when agent client errors sending the http request more times than the sendErrorCount", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					disconnectingRequestHandler,
					disconnectingRequestHandler,
					disconnectingRequestHandler,
				)
			})

			It("returns the error", func() {
				_, err := agentClient.GetState()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Post \"%s/agent\": EOF", server.URL())))
				Expect(server.ReceivedRequests()).To(HaveLen(3))
			})
		})
	})

	Describe("MountDisk", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "mount_disk",
							Arguments: []interface{}{"fake-disk-cid"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{}}`),
					),
				)
			})

			It("makes a POST request to the endpoint and waits for the task to be finished", func() {
				err := agentClient.MountDisk("fake-disk-cid")
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})
		})

		Describe("RemovePersistentDisk", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "remove_persistent_disk",
						Arguments: []interface{}{"fake-disk-cid"},
						ReplyTo:   replyToAddress,
					}),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "get_task",
						Arguments: []interface{}{"fake-agent-task-id"},
						ReplyTo:   replyToAddress,
					}),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
				))
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{}}`),
				))
			})
			It("makes a POST request to the endpoint", func() {
				err := agentClient.RemovePersistentDisk("fake-disk-cid")
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})
		})

		Describe("UnmountDisk", func() {
			Context("when agent responds with a value", func() {
				BeforeEach(func() {
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "unmount_disk",
							Arguments: []interface{}{"fake-disk-cid"},
							ReplyTo:   replyToAddress,
						}),
					))
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					))
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					))
					server.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{}}`),
					))
				})

				It("makes a POST request to the endpoint", func() {
					err := agentClient.UnmountDisk("fake-disk-cid")
					Expect(err).ToNot(HaveOccurred())
					Expect(server.ReceivedRequests()).To(HaveLen(4))
				})
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				err := agentClient.MountDisk("fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				err := agentClient.MountDisk("fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("ListDisk", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":["fake-disk-1", "fake-disk-2"]}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "list_disk",
						Arguments: []interface{}{},
						ReplyTo:   replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.ListDisk()
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns disks", func() {
				disks, err := agentClient.ListDisk()
				Expect(err).ToNot(HaveOccurred())
				Expect(disks).To(Equal([]string{"fake-disk-1", "fake-disk-2"}))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				_, err := agentClient.ListDisk()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				_, err := agentClient.ListDisk()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("MigrateDisk", func() {
		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "migrate_disk",
							Arguments: []interface{}{},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{}}`),
					),
				)
			})

			It("makes a POST request to the endpoint", func() {
				err := agentClient.MigrateDisk()
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})
		})
	})

	Describe("CompilePackage", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method: "compile_package",
						Arguments: []interface{}{
							"fake-package-blobstore-id",
							"fake-package-sha1",
							"fake-package-name",
							"fake-package-version",
							map[string]interface{}{
								"fake-compiled-package-dep-name": map[string]interface{}{
									"name":         "fake-compiled-package-dep-name",
									"version":      "fake-compiled-package-dep-version",
									"sha1":         "fake-compiled-package-dep-sha1",
									"blobstore_id": "fake-compiled-package-dep-blobstore-id",
								},
							},
						},
						ReplyTo: replyToAddress,
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWithJSONEncoded(200, map[string]interface{}{
						"value": map[string]interface{}{
							"result": map[string]string{
								"sha1":         "fake-compiled-package-sha1",
								"blobstore_id": "fake-compiled-package-blobstore-id",
							},
						},
					}),
				))
		})

		It("makes a compile_package request and waits for the task to be done", func() {
			packageSource := agentclient.BlobRef{
				Name:        "fake-package-name",
				Version:     "fake-package-version",
				SHA1:        "fake-package-sha1",
				BlobstoreID: "fake-package-blobstore-id",
			}
			dependencies := []agentclient.BlobRef{
				{
					Name:        "fake-compiled-package-dep-name",
					Version:     "fake-compiled-package-dep-version",
					SHA1:        "fake-compiled-package-dep-sha1",
					BlobstoreID: "fake-compiled-package-dep-blobstore-id",
				},
			}
			_, err := agentClient.CompilePackage(packageSource, dependencies)
			Expect(err).ToNot(HaveOccurred())
			Expect(server.ReceivedRequests()).To(HaveLen(4))
		})
	})

	Describe("DeleteARPEntries", func() {
		var (
			ips []string
		)

		Context("when agent responds with a value", func() {
			BeforeEach(func() {
				ips = []string{"10.0.0.1", "10.0.0.2"}
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "delete_arp_entries",
						Arguments: []interface{}{map[string]interface{}{"ips": []interface{}{ips[0], ips[1]}}},
						ReplyTo:   replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				err := agentClient.DeleteARPEntries(ips)
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				err := agentClient.DeleteARPEntries(ips)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				err := agentClient.DeleteARPEntries(ips)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("RunScript", func() {
		It("sends a run_script message to the agent", func() {
			server.AppendHandlers(
				// run_script
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "run_script",
						Arguments: []interface{}{"the-script", map[string]interface{}{}},
						ReplyTo:   replyToAddress,
					}),
				),
				// get_task
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":{}}`),
				),
			)

			err := agentClient.RunScript("the-script", map[string]interface{}{})
			Expect(err).ToNot(HaveOccurred())
			Expect(server.ReceivedRequests()).To(HaveLen(2))
		})

		It("returns an error if an error occurs", func() {
			server.AppendHandlers(disconnectingRequestHandler)

			err := agentClient.RunScript("the-script", map[string]interface{}{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Post \"%s/agent\": EOF", server.URL())))
		})

		It("does not return an error if the error is 'unknown message'", func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/agent"),
				ghttp.RespondWith(200, `{"exception":{"message":"Agent responded with error: unknown message run_script"}}`),
			))

			err := agentClient.RunScript("the-script", map[string]interface{}{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SyncDNS", func() {
		Context("when agent successfully executes the sync_dns", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":"synced"}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "sync_dns",
						Arguments: []interface{}{"fake-blob-store-id", "fake-blob-store-id-sha1", float64(42)}, // JSON unmarshals to float64
						ReplyTo:   "fake-reply-to-uuid",
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.SyncDNS("fake-blob-store-id", "fake-blob-store-id-sha1", 42)
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns the synced value", func() {
				responseValue, err := agentClient.SyncDNS("fake-blob-store-id", "fake-blob-store-id-sha1", 42)
				Expect(err).ToNot(HaveOccurred())
				Expect(responseValue).To(Equal("synced"))
			})
		})

		Context("when agent does not respond with 200", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("returns an error", func() {
				_, err := agentClient.SyncDNS("fake-blob-store-id", "fake-blob-store-id-sha1", 42)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("status code: 500")))
			})
		})

		Context("when agent responds with exception", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"exception":{"message":"bad request"}}`),
				))
			})

			It("returns an error", func() {
				_, err := agentClient.SyncDNS("fake-blob-store-id", "fake-blob-store-id-sha1", 42)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("bad request")))
			})
		})
	})

	Describe("AddPersistentDisk", func() {
		Context("when agent adds persistent disk successfully", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "add_persistent_disk",
							Arguments: []interface{}{"fake-disk-cid", "/dev/sdf"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
						ghttp.VerifyJSONRepresenting(AgentRequestMessage{
							Method:    "get_task",
							Arguments: []interface{}{"fake-agent-task-id"},
							ReplyTo:   replyToAddress,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{"agent_task_id":"fake-agent-task-id","state":"running"}}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/agent"),
						ghttp.RespondWith(200, `{"value":{}}`),
					),
				)
			})

			It("responds with success", func() {
				err := agentClient.AddPersistentDisk("fake-disk-cid", "/dev/sdf")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when agent cannot add persistent disk", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(http.StatusInternalServerError, ""))
			})

			It("responds with error", func() {
				err := agentClient.AddPersistentDisk("meow", "somewhere")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("SetUpSSH", func() {
		Context("when the agent responds with a successful response", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value": {"command":"setup","status":"success","ip":"1.2.3.4","host_public_key":"some-key"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method: "ssh",
						Arguments: []interface{}{
							"setup", map[string]interface{}{
								"user":       "user",
								"public_key": "user-key",
							},
						},
						ReplyTo: replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns the value", func() {
				responseValue, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).ToNot(HaveOccurred())
				Expect(responseValue.Command).To(Equal("setup"))
				Expect(responseValue.Status).To(Equal("success"))
				Expect(responseValue.Ip).To(Equal("1.2.3.4"))
				Expect(responseValue.HostPublicKey).To(Equal("some-key"))
			})
		})

		Context("when agent does not respond with success", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(200, `{"value": {"command":"setup","status":"failed for some reason"}}`))
			})

			It("returns an error", func() {
				_, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Unable to setup SSH account with the agent, status was: failed for some reason")))
			})
		})

		Context("when agent request results in an error", func() {
			BeforeEach(func() {
				server.AppendHandlers(disconnectingRequestHandler)
			})

			It("returns an error", func() {
				_, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Performing request")))
			})
		})
	})

	Describe("CleanUpSSH", func() {
		Context("when the agent responds with a successful response", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value": {"command":"cleanup","status":"success"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method: "ssh",
						Arguments: []interface{}{
							"cleanup", map[string]interface{}{
								"user_regex": "^user",
							},
						},
						ReplyTo: replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.CleanUpSSH("user")
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns the value", func() {
				responseValue, err := agentClient.CleanUpSSH("user")
				Expect(err).ToNot(HaveOccurred())
				Expect(responseValue.Command).To(Equal("cleanup"))
				Expect(responseValue.Status).To(Equal("success"))
			})
		})

		Context("when agent does not respond with success", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(200, `{"value": {"command":"cleanup","status":"failed for some reason"}}`))
			})

			It("returns an error", func() {
				_, err := agentClient.CleanUpSSH("user")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Unable to cleanup SSH account with the agent, status was: failed for some reason")))
			})
		})

		Context("when agent request results in an error", func() {
			BeforeEach(func() {
				server.AppendHandlers(disconnectingRequestHandler)
			})

			It("returns an error", func() {
				_, err := agentClient.CleanUpSSH("user")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Performing request")))
			})
		})
	})

	Describe("BundleLogs", func() {
		Context("when the agent responds with a successful response", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value": {"logs_tar_path":"/tmp/good-logs-here.tgz","sha512":"goodSHA"}}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method: "bundle_logs",
						Arguments: []interface{}{
							map[string]interface{}{
								"owning_user": "bosh-user",
								"log_type":    "job",
								"filters":     []interface{}{"foo", "bar"},
							},
						},
						ReplyTo: replyToAddress,
					}),
				))
			})

			It("makes a POST request to the endpoint", func() {
				_, err := agentClient.BundleLogs("bosh-user", "job", []string{"foo", "bar"})
				Expect(err).ToNot(HaveOccurred())
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("returns the value", func() {
				responseValue, err := agentClient.BundleLogs("bosh-user", "job", []string{"foo", "bar"})
				Expect(err).ToNot(HaveOccurred())
				Expect(responseValue.LogsTarPath).To(Equal("/tmp/good-logs-here.tgz"))
				Expect(responseValue.SHA512Digest).To(Equal("goodSHA"))
			})
		})

		Context("when agent does not respond with success", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.RespondWith(200, `{"value": {"command":"setup","status":"failed for some reason"}}`))
			})

			It("returns an error", func() {
				_, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Unable to setup SSH account with the agent, status was: failed for some reason")))
			})
		})

		Context("when agent request results in an error", func() {
			BeforeEach(func() {
				server.AppendHandlers(disconnectingRequestHandler)
			})

			It("returns an error", func() {
				_, err := agentClient.SetUpSSH("user", "user-key")
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Performing request")))
			})
		})
	})

	Describe("RemoveFile", func() {
		It("sends a remove_file message to the agent", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/agent"),
					ghttp.RespondWith(200, `{"value":""}`),
					ghttp.VerifyJSONRepresenting(AgentRequestMessage{
						Method:    "remove_file",
						Arguments: []interface{}{"/tmp/here-is/the.file"},
						ReplyTo:   replyToAddress,
					}),
				),
			)

			err := agentClient.RemoveFile("/tmp/here-is/the.file")
			Expect(err).ToNot(HaveOccurred())
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		It("returns an error if an error occurs", func() {
			server.AppendHandlers(disconnectingRequestHandler)

			err := agentClient.RemoveFile("/tmp/here-is/the.file")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Post \"%s/agent\": EOF", server.URL())))
		})
	})
})
