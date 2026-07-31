package integration_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("nats firewall", func() {
	AfterEach(func() {
		fileSettings := settings.Settings{
			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data",
				},
			},
		}
		err := testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())
		err = testEnvironment.UpdateAgentConfig(testEnvironment.GetSettingsFile(""))
		Expect(err).ToNot(HaveOccurred())
	})

	Context("ipv4", func() {
		var directorIP = "192.0.2.100" // RFC 5737 TEST-NET-1

		BeforeEach(func() {
			_, err := testEnvironment.RunCommand(fmt.Sprintf("sudo ip addr del %s/32 dev lo || true", directorIP))
			Expect(err).ToNot(HaveOccurred())

			fileSettings := settings.Settings{
				AgentID: "fake-agent-id",
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Mbus: fmt.Sprintf("nats://%s:4222", directorIP),
				Disks: settings.Disks{
					Ephemeral: "/dev/sdh",
				},
			}

			err = testEnvironment.CreateSettingsFile(fileSettings)
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.UpdateAgentConfig(testEnvironment.GetSettingsFile(""))
			Expect(err).ToNot(HaveOccurred())

			_, _ = testEnvironment.RunCommand("sudo nft flush chain inet bosh_agent nats_access") //nolint:errcheck

			// Flush legacy iptables mangle rules left over from the initial agent deploy.
			// The old agent used iptables cgroup-based rules in the mangle table; these
			// conflict with the new nftables UID-based firewall and would drop traffic
			// that doesn't match the old cgroup.
			_, _ = testEnvironment.RunCommand("sudo iptables -t mangle -F")  //nolint:errcheck
			_, _ = testEnvironment.RunCommand("sudo ip6tables -t mangle -F") //nolint:errcheck

			err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_, err := testEnvironment.RunCommand("sudo pkill -f 'nc -l -p 4222' || true")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip addr del %s/32 dev lo || true", directorIP))
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo nft flush chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
		})

		It("sets up the outgoing nats firewall", func() {
			format.MaxLength = 0

			Eventually(func() bool {
				output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
				return err == nil && strings.Contains(output, directorIP)
			}, 300).Should(BeTrue())

			output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
			Expect(output).To(ContainSubstring("ct state established,related accept"))
			Expect(output).To(MatchRegexp(`meta skuid 0 ip daddr %s tcp dport 4222 accept`, directorIP))
			Expect(output).To(MatchRegexp(`ip daddr %s tcp dport 4222 drop`, directorIP))

			// check that non-root cannot access the director nats, -w2 == timeout 2 seconds
			// The drop rule silently drops packets, causing a timeout. We add the dummy IP to the loopback interface
			// before testing so that the routing table has a valid route. Without a valid route, nc would fail instantly
			// with "Network is unreachable" rather than sending the packet through the firewall to be dropped.
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip addr add %s/32 dev lo || true", directorIP))
			Expect(err).To(BeNil())
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", directorIP))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("timed out"))

			// root (UID 0) is allowed through the nftables firewall
			// Set up a quick dummy listener using nc so we can prove root can connect
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo nohup nc -l -p 4222 -s %s >/dev/null 2>&1 &", directorIP))
			Expect(err).To(BeNil())

			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo nc -z -w2 %v 4222 -v", directorIP))
			Expect(err).To(BeNil())

			// cleanup
			_, err = testEnvironment.RunCommand("sudo pkill -f 'nc -l -p 4222' || true")
			Expect(err).To(BeNil())
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip addr del %s/32 dev lo || true", directorIP))
			Expect(err).To(BeNil())
		})
	})

	Context("multi-url", func() {
		var unreachableIP = "192.0.2.1" // RFC 5737 TEST-NET-1 — unreachable by design
		var directorIP = "192.0.2.100"

		BeforeEach(func() {
			fileSettings := settings.Settings{
				AgentID: "fake-agent-id",
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Env: settings.Env{
					Bosh: settings.BoshEnv{
						Mbus: settings.MBus{
							URLs: []string{
								fmt.Sprintf("nats://%s:4222", unreachableIP),
								fmt.Sprintf("nats://%s:4222", directorIP),
							},
						},
					},
				},
				Disks: settings.Disks{
					Ephemeral: "/dev/sdh",
				},
			}

			err := testEnvironment.CreateSettingsFile(fileSettings)
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.UpdateAgentConfig(testEnvironment.GetSettingsFile(""))
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
			Expect(err).ToNot(HaveOccurred())

			// Flush legacy iptables mangle rules as in the ipv4 case.
			_, _ = testEnvironment.RunCommand("sudo iptables -t mangle -F")  //nolint:errcheck
			_, _ = testEnvironment.RunCommand("sudo ip6tables -t mangle -F") //nolint:errcheck
		})

		AfterEach(func() {
			err := testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo nft flush chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
		})

		It("adds allow/block rules for both NATS server IPs", func() {
			format.MaxLength = 0

			Eventually(func() bool {
				output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
				return err == nil && strings.Contains(output, unreachableIP) && strings.Contains(output, directorIP)
			}, 300).Should(BeTrue())

			output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
			Expect(output).To(ContainSubstring("ct state established,related accept"))

			// Rules for the real NATS server IP
			Expect(output).To(MatchRegexp(`meta skuid 0 ip daddr %s tcp dport 4222 accept`, directorIP))
			Expect(output).To(MatchRegexp(`ip daddr %s tcp dport 4222 drop`, directorIP))

			// Rules for the dummy/unreachable IP
			Expect(output).To(MatchRegexp(`meta skuid 0 ip daddr %s tcp dport 4222 accept`, unreachableIP))
			Expect(output).To(MatchRegexp(`ip daddr %s tcp dport 4222 drop`, unreachableIP))
		})
	})

	Context("ipv6", func() {
		var directorIP = "2001:db8::1"

		BeforeEach(func() {
			_, err := testEnvironment.RunCommand("sudo nft flush chain inet bosh_agent nats_access")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip -6 addr del %s/128 dev lo || true", directorIP))
			Expect(err).ToNot(HaveOccurred())

			fileSettings := settings.Settings{
				AgentID: "fake-agent-id",
				Blobstore: settings.Blobstore{
					Type: "local",
					Options: map[string]interface{}{
						"blobstore_path": "/var/vcap/data",
					},
				},
				Mbus: fmt.Sprintf("nats://[%s]:4222", directorIP),
				Disks: settings.Disks{
					Ephemeral: "/dev/sdh",
				},
			}

			err = testEnvironment.CreateSettingsFile(fileSettings)
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.UpdateAgentConfig(testEnvironment.GetSettingsFile(""))
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_, err := testEnvironment.RunCommand("sudo pkill -f 'nc -6 -l -p 4222' || true")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip -6 addr del %s/128 dev lo || true", directorIP))
			Expect(err).ToNot(HaveOccurred())
			err = testEnvironment.DetachDevice("/dev/sdh")
			Expect(err).ToNot(HaveOccurred())
			_, err = testEnvironment.RunCommand("sudo nft flush chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
		})

		It("sets up the outgoing nats for firewall ipv6", func() {
			format.MaxLength = 0

			Eventually(func() bool {
				output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
				return err == nil && strings.Contains(output, directorIP)
			}, 300).Should(BeTrue())

			output, err := testEnvironment.RunCommand("sudo nft list chain inet bosh_agent nats_access")
			Expect(err).To(BeNil())
			Expect(output).To(ContainSubstring("ct state established,related accept"))
			Expect(output).To(MatchRegexp(`meta skuid 0 ip6 daddr %s tcp dport 4222 accept`, directorIP))
			Expect(output).To(MatchRegexp(`ip6 daddr %s tcp dport 4222 drop`, directorIP))

			// check that non-root cannot access the director nats, -w2 == timeout 2 seconds
			// We add the dummy IP to the loopback interface before testing so that the routing table has a valid route.
			// Without a valid route, nc would fail instantly with "Network is unreachable" rather than sending the packet to the firewall.
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip -6 addr add %s/128 dev lo || true", directorIP))
			Expect(err).To(BeNil())
			out, err := testEnvironment.RunCommand(fmt.Sprintf("nc -6 %v 4222 -w2 -v", directorIP))
			Expect(err).NotTo(BeNil())
			Expect(out).To(ContainSubstring("timed out"))

			// root (UID 0) is allowed through the nftables firewall
			// Set up a quick dummy listener using nc so we can prove root can connect
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo nohup nc -6 -l -p 4222 -s %s >/dev/null 2>&1 &", directorIP))
			Expect(err).To(BeNil())

			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo nc -6 -z -w2 %v 4222 -v", directorIP))
			Expect(err).To(BeNil())

			// cleanup
			_, err = testEnvironment.RunCommand("sudo pkill -f 'nc -6 -l -p 4222' || true")
			Expect(err).To(BeNil())
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo ip -6 addr del %s/128 dev lo || true", directorIP))
			Expect(err).To(BeNil())
		})
	})
})
