package agent_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/agent"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
)

func init() { //nolint:gochecknoinits
	Describe("Heartbeat", func() {
		Context("when all information is available to the heartbeat", func() {
			It("serializes heartbeat with all fields", func() {
				name := "foo"
				index := 0

				hb := Heartbeat{
					Deployment: "FakeDeployment",
					Job:        &name,
					Index:      &index,
					JobState:   "running",
					Vitals: boshvitals.Vitals{
						Disk: boshvitals.DiskVitals{
							"system":     boshvitals.SpecificDiskVitals{},
							"ephemeral":  boshvitals.SpecificDiskVitals{},
							"persistent": boshvitals.SpecificDiskVitals{},
						},
					},
					NodeID:            "node-id",
					NumberOfProcesses: 3,
				}

				expectedJSON := `{"deployment":"FakeDeployment","job":"foo","index":0,"job_state":"running","vitals":{"cpu":{},"disk":{"ephemeral":{},"persistent":{},"system":{}},"mem":{},"swap":{},"uptime":{}},"node_id":"node-id","number_of_processes":3}`

				hbBytes, err := json.Marshal(hb)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(hbBytes)).To(Equal(expectedJSON))
			})
		})

		Context("when job name, index are not available", func() {
			It("serializes job name and index as nulls to indicate that there is no job assigned to this agent", func() {
				hb := Heartbeat{
					Deployment: "FakeDeployment",
					JobState:   "running",
					Vitals: boshvitals.Vitals{
						Disk: boshvitals.DiskVitals{
							"system":     boshvitals.SpecificDiskVitals{},
							"ephemeral":  boshvitals.SpecificDiskVitals{},
							"persistent": boshvitals.SpecificDiskVitals{},
						},
					},
					NodeID:            "node-id",
					NumberOfProcesses: 0,
				}

				expectedJSON := `{"deployment":"FakeDeployment","job":null,"index":null,"job_state":"running","vitals":{"cpu":{},"disk":{"ephemeral":{},"persistent":{},"system":{}},"mem":{},"swap":{},"uptime":{}},"node_id":"node-id","number_of_processes":0}`

				hbBytes, err := json.Marshal(hb)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(hbBytes)).To(Equal(expectedJSON))
			})
		})
	})
}
