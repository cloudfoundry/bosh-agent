package system_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/cloudfoundry/bosh-agent/bootstrapper/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("osSystem", func() {
	var system System

	BeforeEach(func() {
		system = NewOsSystem()
	})

	Describe("Untar", func() {
		var (
			tmpDir    string
			targetDir string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "test-tmp")
			Expect(err).ToNot(HaveOccurred())

			targetDir, err = ioutil.TempDir("", "test-target-tmp")
			Expect(err).ToNot(HaveOccurred())
		})

		writeTarballWith := func(contents string, dir string) io.Reader {
			ioutil.WriteFile(path.Join(dir, "test.filename"), ([]byte)(contents), 0555)
			tarCmd := exec.Command("tar", "cfz", "tarball.tgz", "test.filename")
			tarCmd.Dir = dir

			_, err := tarCmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred())

			tarballPath := path.Join(dir, "tarball.tgz")
			tarball, err := os.Open(tarballPath)
			Expect(err).ToNot(HaveOccurred())

			return tarball
		}

		It("Untars the tarball into the target directory", func() {
			tarball := writeTarballWith("test file contents", tmpDir)

			system = NewOsSystem()

			_, err := system.Untar(tarball, targetDir)
			Expect(err).ToNot(HaveOccurred())

			files, err := ioutil.ReadDir(targetDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(files)).To(Equal(1))
			Expect(files[0].Name()).To(Equal("test.filename"))

			contents, err := ioutil.ReadFile(path.Join(targetDir, files[0].Name()))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(contents)).To(Equal("test file contents"))
		})

		It("Returns the command run and the exit status", func() {
			tarball := writeTarballWith("test file contents", tmpDir)

			system = NewOsSystem()

			result, err := system.Untar(tarball, targetDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.ExitStatus).To(Equal(0))
			Expect(result.CommandRun).To(Equal("tar xvfz -"))
		})

		It("Returns the exit code an no error when the tar command fails", func() {
			system = NewOsSystem()

			result, err := system.Untar(strings.NewReader("invalid tarball"), targetDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.ExitStatus).To(BeNumerically(">", 0))
			Expect(result.CommandRun).To(Equal("tar xvfz -"))
		})

		It("Returns an error if we can't even run the tar command (for example the directory doesn't exist)", func() {
			tarball := writeTarballWith("test file contents", tmpDir)

			system = NewOsSystem()

			_, err := system.Untar(tarball, "invalid-target-dir")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})
	})

	Describe("RunScript", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "test-tmp")
			Expect(err).ToNot(HaveOccurred())
		})

		writeScript := func(scriptContents, dir string) string {
			scriptPath := path.Join(dir, "test_script.sh")
			err := ioutil.WriteFile(scriptPath, ([]byte)(scriptContents), 0755)
			Expect(err).ToNot(HaveOccurred())
			return scriptPath
		}

		It("Runs the script in the correct directory", func() {
			scriptContents := fmt.Sprintf(`#!/bin/bash
			pwd > %s/working_dir.txt
			`, tmpDir)
			scriptPath := writeScript(scriptContents, tmpDir)

			_, err := system.RunScript(scriptPath, tmpDir)
			Expect(err).ToNot(HaveOccurred())

			contents, err := ioutil.ReadFile(path.Join(tmpDir, "working_dir.txt"))
			Expect(err).ToNot(HaveOccurred())

			// on osx the tmpdir seems to be under a symlinked dir
			realTmpDir, err := filepath.EvalSymlinks(tmpDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(strings.TrimSpace(string(contents))).To(Equal(realTmpDir))
		})

		It("returns a useful command result", func() {
			scriptContents := fmt.Sprintf(`#!/bin/bash
			pwd > %s/working_dir.txt
			`, tmpDir)
			scriptPath := writeScript(scriptContents, tmpDir)

			result, err := system.RunScript(scriptPath, tmpDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.ExitStatus).To(Equal(0))
			Expect(result.CommandRun).To(Equal(scriptPath))
		})

		It("returns the exit code and no error if the script fails", func() {
			scriptContents := `#!/bin/bash
			exit 1
			`
			scriptPath := writeScript(scriptContents, tmpDir)

			result, err := system.RunScript(scriptPath, tmpDir)
			Expect(err).ToNot(HaveOccurred())

			Expect(result.ExitStatus).To(Equal(1))
			Expect(result.CommandRun).To(Equal(scriptPath))
		})

		It("returns an error if we fail hard (like the script doesn't exist)", func() {
			_, err := system.RunScript("./not-the-script.sh", tmpDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})
	})

	Describe("FileExists", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "test-tmp")
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns whether or not the file exists", func() {
			filePath := path.Join(tmpDir, "test.file")
			err := ioutil.WriteFile(filePath, ([]byte)(""), 0755)
			Expect(err).ToNot(HaveOccurred())

			Expect(system.FileExists(filePath)).To(BeTrue())
			Expect(system.FileExists(path.Join(tmpDir, "nonexistant.file"))).To(BeFalse())
		})
	})

	Describe("FileIsExecutable", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "test-tmp")
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns whether or not the file is executable", func() {
			executableFilePath := path.Join(tmpDir, "executable.file")
			err := ioutil.WriteFile(executableFilePath, ([]byte)(""), 0755)
			Expect(err).ToNot(HaveOccurred())

			nonExecutableFilePath := path.Join(tmpDir, "non_executable.file")
			err = ioutil.WriteFile(nonExecutableFilePath, ([]byte)(""), 0655)
			Expect(err).ToNot(HaveOccurred())

			Expect(system.FileIsExecutable(executableFilePath)).To(BeTrue())
			Expect(system.FileIsExecutable(nonExecutableFilePath)).To(BeFalse())
		})
	})
})
