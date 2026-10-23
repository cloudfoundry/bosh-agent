package releasetarball_test

import (
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/releasetarball"
)

type expectedIdentity struct {
	dependencyKey string
	rewritten     bool
	fingerprint   string
	dependencies  []string
}

type identityCase struct {
	name     string
	shipped  []string
	packages []releasetarball.IdentityPkg
	expected map[string]expectedIdentity
}

func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s)) //nolint:gosec
	return hex.EncodeToString(sum[:])
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func pkg(name, fingerprint string, dependencies ...string) releasetarball.IdentityPkg {
	return releasetarball.IdentityPkg{Name: name, Version: fingerprint, Fingerprint: fingerprint, Dependencies: append([]string{}, dependencies...)}
}

// Dependency keys follow the director's KeyGenerator#dependency_key_from_manifest, unchanged since 2016:
// https://github.com/cloudfoundry/bosh/blob/0410768ff620384100a277ff0683274970bc2c56/src/bosh-director/lib/bosh/director/key_generator.rb
var identityCases = []identityCase{
	{
		name:    "nested-unsorted-dependencies",
		shipped: []string{"app", "mid", "zeta", "alpha"},
		packages: []releasetarball.IdentityPkg{
			pkg("app", sha1Hex("app-1"), "mid"),
			pkg("mid", sha1Hex("mid-1"), "zeta", "alpha"),
			pkg("zeta", sha1Hex("zeta-1")),
			pkg("alpha", sha1Hex("alpha-1")),
		},
		expected: map[string]expectedIdentity{
			"app": {
				dependencyKey: `[["mid","1f025b96e35b4d12eef99c82c4ac07216c0a92a8",[["alpha","125c151dba18c0df4ed43a1804ab7f66c877e275"],["zeta","6e3322cfd905416a22af7020684833ade7f31ab1"]]]]`,
				rewritten:     false,
				fingerprint:   "8d91d7c662e62b772ff8483400d2a665eb0cab3c",
				dependencies:  []string{"mid"},
			},
			"mid": {
				dependencyKey: `[["alpha","125c151dba18c0df4ed43a1804ab7f66c877e275"],["zeta","6e3322cfd905416a22af7020684833ade7f31ab1"]]`,
				rewritten:     false,
				fingerprint:   "1f025b96e35b4d12eef99c82c4ac07216c0a92a8",
				dependencies:  []string{"zeta", "alpha"},
			},
			"zeta": {
				dependencyKey: `[]`,
				rewritten:     false,
				fingerprint:   "6e3322cfd905416a22af7020684833ade7f31ab1",
				dependencies:  []string{},
			},
			"alpha": {
				dependencyKey: `[]`,
				rewritten:     false,
				fingerprint:   "125c151dba18c0df4ed43a1804ab7f66c877e275",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "leaf-nothing-dropped",
		shipped: []string{"leaf"},
		packages: []releasetarball.IdentityPkg{
			pkg("leaf", sha1Hex("leaf-1")),
		},
		expected: map[string]expectedIdentity{
			"leaf": {
				dependencyKey: `[]`,
				rewritten:     false,
				fingerprint:   "1847f16a41619cdd974904f49d7a781c5638ba62",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "chain-sha1",
		shipped: []string{"app"},
		packages: []releasetarball.IdentityPkg{
			pkg("app", sha1Hex("app-1"), "golang"),
			pkg("golang", sha1Hex("golang-1")),
		},
		expected: map[string]expectedIdentity{
			"app": {
				dependencyKey: `[["golang","e958a258ac0a3a0998e87b76083f71ed70f7dcb4"]]`,
				rewritten:     true,
				fingerprint:   "13a411a02f9f3e86f55ed8308e9bc848fde2fcad",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "log-cache-shape",
		shipped: []string{"log-cache", "log-cache-cf-auth-proxy", "log-cache-gateway", "log-cache-syslog-server"},
		packages: []releasetarball.IdentityPkg{
			pkg("golang-1.20-linux", sha1Hex("golang-1.20-linux")),
			pkg("log-cache", sha1Hex("log-cache-1"), "golang-1.20-linux"),
			pkg("log-cache-cf-auth-proxy", sha1Hex("log-cache-cf-auth-proxy-1"), "golang-1.20-linux"),
			pkg("log-cache-gateway", sha1Hex("log-cache-gateway-1"), "golang-1.20-linux"),
			pkg("log-cache-syslog-server", sha1Hex("log-cache-syslog-server-1"), "golang-1.20-linux"),
		},
		expected: map[string]expectedIdentity{
			"log-cache": {
				dependencyKey: `[["golang-1.20-linux","e4add25e4496b774fc172e5cade2462e065a7a88"]]`,
				rewritten:     true,
				fingerprint:   "3ed36d4bfd2b34426edee6422c1dd2494180348c",
				dependencies:  []string{},
			},
			"log-cache-cf-auth-proxy": {
				dependencyKey: `[["golang-1.20-linux","e4add25e4496b774fc172e5cade2462e065a7a88"]]`,
				rewritten:     true,
				fingerprint:   "754e8b3c278659e912114361e067f9f4cf20ff40",
				dependencies:  []string{},
			},
			"log-cache-gateway": {
				dependencyKey: `[["golang-1.20-linux","e4add25e4496b774fc172e5cade2462e065a7a88"]]`,
				rewritten:     true,
				fingerprint:   "3e61538090d7147724eadf2f8acc5971886a505d",
				dependencies:  []string{},
			},
			"log-cache-syslog-server": {
				dependencyKey: `[["golang-1.20-linux","e4add25e4496b774fc172e5cade2462e065a7a88"]]`,
				rewritten:     true,
				fingerprint:   "4bffe6bd7a8ad393626511e40f36f9e227d77666",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "diamond",
		shipped: []string{"P", "A", "B"},
		packages: []releasetarball.IdentityPkg{
			pkg("P", sha1Hex("P-1"), "A", "B"),
			pkg("A", sha1Hex("A-1"), "C"),
			pkg("B", sha1Hex("B-1"), "C"),
			pkg("C", sha1Hex("C-1")),
		},
		expected: map[string]expectedIdentity{
			"P": {
				dependencyKey: `[["A","04807be371964d6005a11a740bc53a2c6bbb2ce5",[["C","10b3dea1b985e3f405094d085341c07815537de0"]]],["B","002fb32c9c51cd4b0ef81ed60239be57413ebf4b",[["C","10b3dea1b985e3f405094d085341c07815537de0"]]]]`,
				rewritten:     true,
				fingerprint:   "c189f527eff56e6e6a9f008149b93c322003a9f7",
				dependencies:  []string{"A", "B"},
			},
			"A": {
				dependencyKey: `[["C","10b3dea1b985e3f405094d085341c07815537de0"]]`,
				rewritten:     true,
				fingerprint:   "d5a9d59faed3b9b8254931b48a19f86d4ab1daf5",
				dependencies:  []string{},
			},
			"B": {
				dependencyKey: `[["C","10b3dea1b985e3f405094d085341c07815537de0"]]`,
				rewritten:     true,
				fingerprint:   "21aa5cd736031587804d6cd53eca3fbbfb2eca6b",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "transitive-drop",
		shipped: []string{"A", "B", "C"},
		packages: []releasetarball.IdentityPkg{
			pkg("A", sha1Hex("A-1"), "B"),
			pkg("B", sha1Hex("B-1"), "C"),
			pkg("C", sha1Hex("C-1"), "toolchain"),
			pkg("toolchain", sha1Hex("toolchain-1")),
		},
		expected: map[string]expectedIdentity{
			"A": {
				dependencyKey: `[["B","002fb32c9c51cd4b0ef81ed60239be57413ebf4b",[["C","10b3dea1b985e3f405094d085341c07815537de0",[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]]]]]`,
				rewritten:     true,
				fingerprint:   "3811a6952bbd7778e3e711a7d35c296044466b6e",
				dependencies:  []string{"B"},
			},
			"B": {
				dependencyKey: `[["C","10b3dea1b985e3f405094d085341c07815537de0",[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]]]`,
				rewritten:     true,
				fingerprint:   "c01bad11cb0b6a05d39e842b5708391aad32de18",
				dependencies:  []string{"C"},
			},
			"C": {
				dependencyKey: `[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]`,
				rewritten:     true,
				fingerprint:   "89a02d2c317646ac1c9f81df49af97329ff9b702",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "transitive-drop-mid",
		shipped: []string{"A", "B"},
		packages: []releasetarball.IdentityPkg{
			pkg("A", sha1Hex("A-1"), "B"),
			pkg("B", sha1Hex("B-1"), "C"),
			pkg("C", sha1Hex("C-1"), "toolchain"),
			pkg("toolchain", sha1Hex("toolchain-1")),
		},
		expected: map[string]expectedIdentity{
			"A": {
				dependencyKey: `[["B","002fb32c9c51cd4b0ef81ed60239be57413ebf4b",[["C","10b3dea1b985e3f405094d085341c07815537de0",[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]]]]]`,
				rewritten:     true,
				fingerprint:   "3811a6952bbd7778e3e711a7d35c296044466b6e",
				dependencies:  []string{"B"},
			},
			"B": {
				dependencyKey: `[["C","10b3dea1b985e3f405094d085341c07815537de0",[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]]]`,
				rewritten:     true,
				fingerprint:   "66a907e2360d9a2f81220136e95ebf911a79abcc",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "kept-set-change-a",
		shipped: []string{"P"},
		packages: []releasetarball.IdentityPkg{
			pkg("P", sha1Hex("P-1"), "A", "B"),
			pkg("A", sha1Hex("A-1")),
			pkg("B", sha1Hex("B-1")),
		},
		expected: map[string]expectedIdentity{
			"P": {
				dependencyKey: `[["A","04807be371964d6005a11a740bc53a2c6bbb2ce5"],["B","002fb32c9c51cd4b0ef81ed60239be57413ebf4b"]]`,
				rewritten:     true,
				fingerprint:   "9a17778a24f9263f3bae31f8b3cd71bb53b1108c",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "kept-set-change-b",
		shipped: []string{"P", "A"},
		packages: []releasetarball.IdentityPkg{
			pkg("P", sha1Hex("P-1"), "A", "B"),
			pkg("A", sha1Hex("A-1")),
			pkg("B", sha1Hex("B-1")),
		},
		expected: map[string]expectedIdentity{
			"P": {
				dependencyKey: `[["A","04807be371964d6005a11a740bc53a2c6bbb2ce5"],["B","002fb32c9c51cd4b0ef81ed60239be57413ebf4b"]]`,
				rewritten:     true,
				fingerprint:   "4a95195604071dc1e5bb6d241f95d2be51c5e511",
				dependencies:  []string{"A"},
			},
			"A": {
				dependencyKey: `[]`,
				rewritten:     false,
				fingerprint:   "04807be371964d6005a11a740bc53a2c6bbb2ce5",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "sha256",
		shipped: []string{"app"},
		packages: []releasetarball.IdentityPkg{
			pkg("app", sha256Hex("app-1"), "golang"),
			pkg("golang", sha256Hex("golang-1")),
		},
		expected: map[string]expectedIdentity{
			"app": {
				dependencyKey: `[["golang","e6ec14b0aaccbf2c0ae42aee77f84342030605783450f22f8469476481be3045"]]`,
				rewritten:     true,
				fingerprint:   "fa4b51a808dd30da4896af567ad729d6ea262bbc85ec5d504efc24b3e7e5482c",
				dependencies:  []string{},
			},
		},
	},
	{
		name:    "partial-closure",
		shipped: []string{"X", "Y", "Z"},
		packages: []releasetarball.IdentityPkg{
			pkg("X", sha1Hex("X-1"), "Y", "Z"),
			pkg("Y", sha1Hex("Y-1")),
			pkg("Z", sha1Hex("Z-1"), "toolchain"),
			pkg("toolchain", sha1Hex("toolchain-1")),
		},
		expected: map[string]expectedIdentity{
			"X": {
				dependencyKey: `[["Y","1b7e63d6572f466d8eebac1436ae6e7bb188e7fe"],["Z","1fa5d2694124a2ceaef3a18bbeeb032dff504b77",[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]]]`,
				rewritten:     true,
				fingerprint:   "3f03d6e5e286aeab75e08ac96595f9e73b360685",
				dependencies:  []string{"Y", "Z"},
			},
			"Y": {
				dependencyKey: `[]`,
				rewritten:     false,
				fingerprint:   "1b7e63d6572f466d8eebac1436ae6e7bb188e7fe",
				dependencies:  []string{},
			},
			"Z": {
				dependencyKey: `[["toolchain","39c5f824fcbb990276f7eb62daccd1f17dde037f"]]`,
				rewritten:     true,
				fingerprint:   "41b730e2e3507cf8f1d2884e72e65b1c63af8986",
				dependencies:  []string{},
			},
		},
	},
}

func shippedSet(names []string) map[string]bool {
	shipped := make(map[string]bool, len(names))
	for _, name := range names {
		shipped[name] = true
	}
	return shipped
}

func caseNamed(name string) identityCase {
	for _, c := range identityCases {
		if c.name == name {
			return c
		}
	}
	panic("unknown identity case " + name)
}

var _ = Describe("package identities", func() {
	for _, c := range identityCases {
		It("computes identities for "+c.name, func() {
			shipped := shippedSet(c.shipped)
			result, err := releasetarball.RewriteIdentitiesForTest(c.packages, shipped)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(len(c.expected)))
			for name, expected := range c.expected {
				actual, exists := result[name]
				Expect(exists).To(BeTrue(), "package %q should be included", name)
				key, err := releasetarball.DependencyKeyForTest(name, c.packages)
				Expect(err).NotTo(HaveOccurred())
				Expect(key).To(Equal(expected.dependencyKey), name)
				Expect(actual.Rewritten).To(Equal(expected.rewritten), name)
				Expect(actual.Fingerprint).To(Equal(expected.fingerprint), name)
				Expect(actual.Dependencies).To(Equal(expected.dependencies), name)
			}
			for _, p := range c.packages {
				if !shipped[p.Name] {
					Expect(result).NotTo(HaveKey(p.Name), "dropped package %q should not be included", p.Name)
				}
			}
		})
	}

	It("keeps the fingerprint of a package that loses no dependencies", func() {
		c := caseNamed("leaf-nothing-dropped")
		result, err := releasetarball.RewriteIdentitiesForTest(c.packages, shippedSet(c.shipped))
		Expect(err).NotTo(HaveOccurred())
		Expect(result["leaf"].Fingerprint).To(Equal(c.packages[0].Fingerprint))
	})

	It("gives a package a different fingerprint when a different set of its dependencies is kept", func() {
		a, b := caseNamed("kept-set-change-a"), caseNamed("kept-set-change-b")
		resultA, err := releasetarball.RewriteIdentitiesForTest(a.packages, shippedSet(a.shipped))
		Expect(err).NotTo(HaveOccurred())
		resultB, err := releasetarball.RewriteIdentitiesForTest(b.packages, shippedSet(b.shipped))
		Expect(err).NotTo(HaveOccurred())
		Expect(resultA["P"].Fingerprint).NotTo(Equal(resultB["P"].Fingerprint))
	})

	It("rejects unsupported fingerprint lengths for rewritten packages", func() {
		packages := []releasetarball.IdentityPkg{
			{Name: "app", Version: "1", Fingerprint: "12345678901234567890123456789012345678901234567890", Dependencies: []string{"dropped"}},
			{Name: "dropped", Version: "2", Fingerprint: "1234567890123456789012345678901234567890"},
		}
		_, err := releasetarball.RewriteIdentitiesForTest(packages, map[string]bool{"app": true})
		Expect(err).To(MatchError(ContainSubstring("app")))
	})

	It("returns an error for an unknown dependency", func() {
		packages := []releasetarball.IdentityPkg{{Name: "app", Version: "1", Dependencies: []string{"missing"}}}
		_, err := releasetarball.DependencyKeyForTest("app", packages)
		Expect(err).To(MatchError(ContainSubstring("missing")))
	})

	It("does not HTML-escape package names in dependency keys", func() {
		packages := []releasetarball.IdentityPkg{
			{Name: "app", Version: "1", Dependencies: []string{"<dep"}},
			{Name: "<dep", Version: "2"},
		}
		key, err := releasetarball.DependencyKeyForTest("app", packages)
		Expect(err).NotTo(HaveOccurred())
		Expect(key).To(Equal(`[["<dep","2"]]`))
	})
})
