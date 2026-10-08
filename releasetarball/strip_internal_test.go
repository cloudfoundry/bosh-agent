package releasetarball

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

var _ = Describe("selectPackages", func() {
	logCache := manifest.Manifest{
		Packages: []manifest.PackageRef{
			{Name: "golang"},
			{Name: "log-cache", Dependencies: []string{"golang"}},
			{Name: "orphan", Dependencies: []string{"golang"}},
		},
	}

	It("compiles a build dependency but does not ship it", func() {
		compile, ship, err := selectPackages(logCache, map[string]bool{"log-cache": true}, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(compile).To(HaveKey("golang"))
		Expect(compile).To(HaveKey("log-cache"))
		Expect(ship).To(HaveKey("log-cache"))
		Expect(ship).NotTo(HaveKey("golang"))
	})

	It("neither compiles nor ships a package nothing reaches", func() {
		compile, ship, err := selectPackages(logCache, map[string]bool{"log-cache": true}, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(compile).NotTo(HaveKey("orphan"))
		Expect(ship).NotTo(HaveKey("orphan"))
	})

	It("keeps everything when stripping is not requested", func() {
		compile, ship, err := selectPackages(logCache, map[string]bool{"log-cache": true}, false)
		Expect(err).NotTo(HaveOccurred())

		Expect(compile).To(HaveLen(3))
		Expect(ship).To(HaveLen(3))
	})

	It("keeps everything when no job declares a package", func() {
		compile, ship, err := selectPackages(logCache, nil, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(compile).To(HaveLen(3))
		Expect(ship).To(HaveLen(3))
	})

	It("selects nothing for an already compiled release", func() {
		compile, ship, err := selectPackages(manifest.Manifest{}, map[string]bool{"log-cache": true}, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(compile).To(BeEmpty())
		Expect(ship).To(BeEmpty())
	})

	It("returns an error when a job declares an unknown package", func() {
		_, _, err := selectPackages(logCache, map[string]bool{"missing": true}, true)
		Expect(err).To(MatchError(ContainSubstring(`package "missing" required by a job spec is not in the release manifest`)))
	})

	It("returns an error when a dependency is unknown", func() {
		m := manifest.Manifest{Packages: []manifest.PackageRef{{Name: "log-cache", Dependencies: []string{"missing"}}}}

		_, _, err := selectPackages(m, map[string]bool{"log-cache": true}, true)
		Expect(err).To(MatchError(ContainSubstring(`package "missing" required by package "log-cache" is not in the release manifest`)))
	})
})

var _ = Describe("shippedDependencies", func() {
	It("drops names that are not shipped", func() {
		Expect(shippedDependencies([]string{"golang", "zlib"}, map[string]bool{"zlib": true})).To(Equal([]string{"zlib"}))
	})

	It("returns an empty list rather than nil", func() {
		Expect(shippedDependencies([]string{"golang"}, map[string]bool{})).To(BeEmpty())
	})
})

var _ = Describe("jobPackageNames", func() {
	It("reads the package list from each job.MF", func() {
		names, err := jobPackageNames("testdata/log-cache-release-3.0.9.tgz")
		Expect(err).NotTo(HaveOccurred())

		Expect(names).To(Equal(map[string]bool{
			"log-cache":               true,
			"log-cache-cf-auth-proxy": true,
			"log-cache-gateway":       true,
			"log-cache-syslog-server": true,
		}))
	})
})
