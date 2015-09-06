package agent_test

import (
	"encoding/json"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
)

func init() {
	Describe("Heartbeat", func() {
		Context("when all information is available to the heartbeat", func() {
			It("serializes heartbeat with all fields", func() {
				name := "foo"
				index := 0

				hb := Heartbeat{
					Job:      &name,
					Index:    &index,
					JobState: "running",
					Vitals: boshvitals.Vitals{
						Disk: boshvitals.DiskVitals{
							"system":     boshvitals.SpecificDiskVitals{},
							"ephemeral":  boshvitals.SpecificDiskVitals{},
							"persistent": boshvitals.SpecificDiskVitals{},
						},
					},
					NodeID: "node-id",
				}

				expectedJSON := `{"job":"foo","index":0,"job_state":"running","vitals":{"cpu":{},"disk":{"ephemeral":{},"persistent":{},"system":{}},"mem":{},"swap":{}},"node_id":"node-id"}`

				hbBytes, err := json.Marshal(hb)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(hbBytes)).To(Equal(expectedJSON))
			})
		})

		Context("when job name, index are not available", func() {
			It("serializes job name and index as nulls to indicate that there is no job assigned to this agent", func() {
				hb := Heartbeat{
					JobState: "running",
					Vitals: boshvitals.Vitals{
						Disk: boshvitals.DiskVitals{
							"system":     boshvitals.SpecificDiskVitals{},
							"ephemeral":  boshvitals.SpecificDiskVitals{},
							"persistent": boshvitals.SpecificDiskVitals{},
						},
					},
					NodeID: "node-id",
				}

				expectedJSON := `{"job":null,"index":null,"job_state":"running","vitals":{"cpu":{},"disk":{"ephemeral":{},"persistent":{},"system":{}},"mem":{},"swap":{}},"node_id":"node-id"}`

				hbBytes, err := json.Marshal(hb)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(hbBytes)).To(Equal(expectedJSON))
			})
		})
	})
}
