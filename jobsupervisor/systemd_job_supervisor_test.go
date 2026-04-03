//go:build !windows

package jobsupervisor_test

import (
	"path/filepath"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

var _ = Describe("systemdJobSupervisor", func() {
	var (
		fs          *fakesys.FakeFileSystem
		runner      *fakesys.FakeCmdRunner
		logger      boshlog.Logger
		dirProvider boshdir.Provider
		supervisor  JobSupervisor

		systemdDir string
	)

	const validProcessesYML = `processes:
  - name: hello-world
    executable: /bin/sleep
    args:
      - "infinity"
`

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		dirProvider = boshdir.NewProvider("/var/vcap")
		systemdDir = "/etc/systemd/system"

		supervisor = NewSystemdJobSupervisor(fs, runner, logger, dirProvider)
	})

	addJob := func(jobName string) {
		configPath := "/var/vcap/jobs/" + jobName + "/processes.yml"
		Expect(fs.WriteFileString(configPath, validProcessesYML)).To(Succeed())
		Expect(supervisor.AddJob(jobName, 0, configPath)).To(Succeed())
	}

	Describe("AddJob", func() {
		It("writes a systemd unit file for each process", func() {
			addJob("hello-world")

			unitPath := filepath.Join(systemdDir, "bosh-job-hello-world.service")
			Expect(fs.FileExists(unitPath)).To(BeTrue())

			contents, err := fs.ReadFileString(unitPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(ContainSubstring("bpm run hello-world"))
			Expect(contents).To(ContainSubstring("bpm stop hello-world"))
			Expect(contents).To(ContainSubstring("WantedBy=bosh-jobs.target"))
		})

		It("runs daemon-reload and enable after writing the unit", func() {
			addJob("hello-world")

			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "daemon-reload"},
			))
			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "enable", "bosh-job-hello-world.service"},
			))
		})

		It("creates bosh-jobs.target with the correct content if it does not exist", func() {
			addJob("hello-world")

			targetPath := filepath.Join(systemdDir, "bosh-jobs.target")
			Expect(fs.FileExists(targetPath)).To(BeTrue())

			contents, err := fs.ReadFileString(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal("[Unit]\nDescription=BOSH Jobs\nAllowIsolate=no\n"))
		})

		It("runs daemon-reload after writing bosh-jobs.target", func() {
			// Capture commands before addJob so we can see the order
			addJob("hello-world")

			// The first daemon-reload must be the one from ensureBoshJobsTarget,
			// before the unit files are written and enabled.
			Expect(runner.RunCommands[0]).To(Equal([]string{"systemctl", "daemon-reload"}))
		})

		It("does not run an extra daemon-reload when bosh-jobs.target already exists", func() {
			targetPath := filepath.Join(systemdDir, "bosh-jobs.target")
			Expect(fs.WriteFileString(targetPath, "[Unit]\nDescription=BOSH Jobs\nAllowIsolate=no\n")).To(Succeed())

			addJob("hello-world")

			// Only one daemon-reload: the one at the end of AddJob for unit registration.
			// The ensureBoshJobsTarget path is skipped.
			daemonReloads := 0
			for _, cmd := range runner.RunCommands {
				if len(cmd) == 2 && cmd[0] == "systemctl" && cmd[1] == "daemon-reload" {
					daemonReloads++
				}
			}
			Expect(daemonReloads).To(Equal(1))
		})

		It("does not overwrite an existing bosh-jobs.target", func() {
			targetPath := filepath.Join(systemdDir, "bosh-jobs.target")
			Expect(fs.WriteFileString(targetPath, "existing")).To(Succeed())

			addJob("hello-world")

			contents, err := fs.ReadFileString(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal("existing"))
		})

		It("removes a stale unmonitor drop-in from a previous drain", func() {
			unitName := "bosh-job-hello-world.service"
			dropInPath := filepath.Join(systemdDir, unitName+".d", "unmonitor.conf")
			Expect(fs.MkdirAll(filepath.Dir(dropInPath), 0755)).To(Succeed())
			Expect(fs.WriteFileString(dropInPath, "[Service]\nRestart=no\n")).To(Succeed())

			addJob("hello-world")

			Expect(fs.FileExists(dropInPath)).To(BeFalse())
		})

		It("returns an error if processes.yml has no processes", func() {
			configPath := "/var/vcap/jobs/empty/processes.yml"
			Expect(fs.WriteFileString(configPath, "processes: []")).To(Succeed())
			err := supervisor.AddJob("empty", 0, configPath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no processes"))
		})
	})

	Describe("Unmonitor", func() {
		It("does nothing when no jobs are registered", func() {
			Expect(supervisor.Unmonitor()).To(Succeed())
			Expect(runner.RunCommands).To(BeEmpty())
		})

		It("writes a Restart=no drop-in for each unit", func() {
			addJob("hello-world")
			runner.RunCommands = nil // reset after AddJob commands

			Expect(supervisor.Unmonitor()).To(Succeed())

			dropInPath := filepath.Join(systemdDir, "bosh-job-hello-world.service.d", "unmonitor.conf")
			Expect(fs.FileExists(dropInPath)).To(BeTrue())

			contents, err := fs.ReadFileString(dropInPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal("[Service]\nRestart=no\n"))
		})

		It("runs daemon-reload after writing drop-ins", func() {
			addJob("hello-world")
			runner.RunCommands = nil

			Expect(supervisor.Unmonitor()).To(Succeed())

			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "daemon-reload"},
			))
		})

		It("does NOT call systemctl stop", func() {
			addJob("hello-world")
			runner.RunCommands = nil

			Expect(supervisor.Unmonitor()).To(Succeed())

			for _, cmd := range runner.RunCommands {
				Expect(cmd).NotTo(ContainElement("stop"))
			}
		})

		It("writes drop-ins for multiple units", func() {
			multiProcessYML := `processes:
  - name: worker
    executable: /bin/sleep
    args: ["infinity"]
  - name: api
    executable: /bin/sleep
    args: ["infinity"]
`
			configPath := "/var/vcap/jobs/my-job/processes.yml"
			Expect(fs.WriteFileString(configPath, multiProcessYML)).To(Succeed())
			Expect(supervisor.AddJob("my-job", 0, configPath)).To(Succeed())
			runner.RunCommands = nil

			Expect(supervisor.Unmonitor()).To(Succeed())

			Expect(fs.FileExists(filepath.Join(systemdDir, "bosh-job-my-job-worker.service.d", "unmonitor.conf"))).To(BeTrue())
			Expect(fs.FileExists(filepath.Join(systemdDir, "bosh-job-my-job-api.service.d", "unmonitor.conf"))).To(BeTrue())
		})
	})

	Describe("RemoveAllJobs", func() {
		It("removes unit files and drop-in directories", func() {
			addJob("hello-world")
			Expect(supervisor.Unmonitor()).To(Succeed()) // creates drop-in
			runner.RunCommands = nil

			Expect(supervisor.RemoveAllJobs()).To(Succeed())

			unitPath := filepath.Join(systemdDir, "bosh-job-hello-world.service")
			dropInDir := filepath.Join(systemdDir, "bosh-job-hello-world.service.d")
			Expect(fs.FileExists(unitPath)).To(BeFalse())
			Expect(fs.FileExists(dropInDir)).To(BeFalse())
		})

		It("runs daemon-reload after removing units", func() {
			addJob("hello-world")
			runner.RunCommands = nil

			Expect(supervisor.RemoveAllJobs()).To(Succeed())

			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "daemon-reload"},
			))
		})

		It("does not run daemon-reload when no jobs were registered", func() {
			Expect(supervisor.RemoveAllJobs()).To(Succeed())
			Expect(runner.RunCommands).To(BeEmpty())
		})
	})

	Describe("Start", func() {
		It("does nothing when no jobs are registered", func() {
			Expect(supervisor.Start()).To(Succeed())
			Expect(runner.RunCommands).To(BeEmpty())
		})

		It("starts bosh-jobs.target when jobs are registered", func() {
			addJob("hello-world")
			runner.RunCommands = nil

			Expect(supervisor.Start()).To(Succeed())

			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "start", "bosh-jobs.target"},
			))
		})
	})

	Describe("Stop", func() {
		It("does nothing when no jobs are registered", func() {
			Expect(supervisor.Stop()).To(Succeed())
			Expect(runner.RunCommands).To(BeEmpty())
		})

		It("stops each unit individually", func() {
			addJob("hello-world")
			runner.RunCommands = nil

			Expect(supervisor.Stop()).To(Succeed())

			Expect(runner.RunCommands).To(ContainElement(
				[]string{"systemctl", "stop", "bosh-job-hello-world.service"},
			))
		})
	})

	Describe("Status", func() {
		It("returns running when no jobs are registered", func() {
			Expect(supervisor.Status()).To(Equal("running"))
		})

		It("returns running when all units are active", func() {
			addJob("hello-world")
			// Sticky=true because Status calls is-active twice per unit (two loops)
			runner.AddCmdResult("systemctl is-active bosh-job-hello-world.service",
				fakesys.FakeCmdResult{Stdout: "active\n", Sticky: true})

			Expect(supervisor.Status()).To(Equal("running"))
		})

		It("returns failing when any unit is not active", func() {
			addJob("hello-world")
			runner.AddCmdResult("systemctl is-active bosh-job-hello-world.service",
				fakesys.FakeCmdResult{Stdout: "failed\n", Sticky: true})

			Expect(supervisor.Status()).To(Equal("failing"))
		})

		It("returns starting when any unit is activating", func() {
			addJob("hello-world")
			// "activating" causes an early return from the first loop, so only one call
			runner.AddCmdResult("systemctl is-active bosh-job-hello-world.service",
				fakesys.FakeCmdResult{Stdout: "activating\n"})

			Expect(supervisor.Status()).To(Equal("starting"))
		})

		It("returns stopped when the stopped file exists", func() {
			addJob("hello-world")
			stoppedPath := filepath.Join(dirProvider.MonitDir(), "systemd_stopped")
			Expect(fs.WriteFileString(stoppedPath, "")).To(Succeed())

			Expect(supervisor.Status()).To(Equal("stopped"))
		})
	})
})
