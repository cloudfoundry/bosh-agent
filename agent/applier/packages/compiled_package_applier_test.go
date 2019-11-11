package packages_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	fakebc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
	. "github.com/cloudfoundry/bosh-agent/agent/applier/packages"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

func buildPkg(bc *fakebc.FakeBundleCollection) (models.Package, *fakebc.FakeBundle) {
	uuidGen := boshuuid.NewGenerator()
	uuid, err := uuidGen.Generate()
	Expect(err).ToNot(HaveOccurred())

	pkg := models.Package{
		Name:    "fake-package-name" + uuid,
		Version: "fake-package-name",
		Source: models.Source{
			SignedURL:        "fake-package/signed-url",
			Sha1:             boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-blob-sha1")),
			BlobstoreHeaders: map[string]string{"key": "value"},
			BlobstoreID:      "fake-blobstore-id",
		},
	}

	bundle := bc.FakeGet(pkg)

	return pkg, bundle
}

func init() {
	Describe("compiledPackageApplier", func() {
		var (
			packagesBc *fakebc.FakeBundleCollection
			blobstore  *fakeblobdelegator.FakeBlobstoreDelegator
			fs         *fakesys.FakeFileSystem
			logger     boshlog.Logger
			applier    Applier
		)

		BeforeEach(func() {
			packagesBc = fakebc.NewFakeBundleCollection()
			blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
			fs = fakesys.NewFakeFileSystem()
			logger = boshlog.NewLogger(boshlog.LevelNone)
			applier = NewCompiledPackageApplier(packagesBc, true, blobstore, fs, logger)
		})

		Describe("Prepare & Apply", func() {
			var (
				pkg    models.Package
				bundle *fakebc.FakeBundle
			)

			BeforeEach(func() {
				pkg, bundle = buildPkg(packagesBc)
			})

			ItInstallsPkg := func(act func() error) {
				It("returns error when installing package fails", func() {
					bundle.InstallError = errors.New("fake-install-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-install-error"))
				})

				It("downloads and later cleans up downloaded package blob", func() {
					blobstore.GetReturns("/fake-blobstore-file-name", nil)

					err := act()
					Expect(err).ToNot(HaveOccurred())
					fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
					Expect(signedURL).To(Equal("fake-package/signed-url"))
					Expect(blobID).To(Equal("fake-blobstore-id"))
					Expect(headers).To(Equal(map[string]string{"key": "value"}))
					Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-blob-sha1"))))

					// downloaded file is cleaned up
					_, cleanupArg := blobstore.CleanUpArgsForCall(0)
					Expect(cleanupArg).To(Equal("/fake-blobstore-file-name"))
				})

				It("returns error when downloading package blob fails", func() {
					blobstore.GetReturns("", errors.New("fake-get-error"))

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-error"))
				})

				It("can process sha1 checksums in the new format", func() {
					blobstore.GetReturns("/fake-blobstore-file-name", nil)
					pkg.Source.Sha1 = boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sha1:fake-blob-sha1"))

					err := act()
					Expect(err).ToNot(HaveOccurred())
					fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
					Expect(signedURL).To(Equal("fake-package/signed-url"))
					Expect(blobID).To(Equal("fake-blobstore-id"))
					Expect(headers).To(Equal(map[string]string{"key": "value"}))
					Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sha1:fake-blob-sha1"))))
				})

				It("can process sha2 checksums", func() {
					blobstore.GetReturns("/fake-blobstore-file-name", nil)
					pkg.Source.Sha1 = boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA256, "sha256:fake-blob-sha256"))

					err := act()
					Expect(err).ToNot(HaveOccurred())
					fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
					Expect(signedURL).To(Equal("fake-package/signed-url"))
					Expect(blobID).To(Equal("fake-blobstore-id"))
					Expect(headers).To(Equal(map[string]string{"key": "value"}))
					Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA256, "sha256:fake-blob-sha256"))))
				})

				It("installs bundle from archive", func() {
					blobstore.GetReturns("/fake-blobstore-file-name", nil)
					err := act()
					Expect(err).ToNot(HaveOccurred())

					// make sure that bundle install happened after decompression
					Expect(bundle.InstallSourcePath).To(Equal("/fake-blobstore-file-name"))
					Expect(bundle.InstallPathInBundle).To(Equal(""))
				})
			}

			Describe("Prepare", func() {
				act := func() error { return applier.Prepare(pkg) }

				It("return an error if getting file bundle fails", func() {
					packagesBc.GetErr = errors.New("fake-get-bundle-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-bundle-error"))
				})

				It("returns an error if checking for package installation fails", func() {
					bundle.IsInstalledErr = errors.New("fake-is-installed-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-installed-error"))
				})

				Context("when package is already installed", func() {
					BeforeEach(func() {
						bundle.Installed = true
					})

					It("does not install", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(bundle.ActionsCalled).To(Equal([]string{})) // no Install
					})

					It("does not download the package", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(blobstore.GetCallCount()).To(Equal(0))
					})
				})

				Context("when package is not installed", func() {
					BeforeEach(func() {
						bundle.Installed = false
					})

					It("installs package (but does not enable it)", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(bundle.ActionsCalled).To(Equal([]string{"Install"}))
					})

					ItInstallsPkg(act)
				})
			})

			Describe("Apply", func() {
				act := func() error { return applier.Apply(pkg) }

				It("return an error if getting file bundle fails", func() {
					packagesBc.GetErr = errors.New("fake-get-bundle-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-bundle-error"))
				})

				It("returns an error if checking for package installation fails", func() {
					bundle.IsInstalledErr = errors.New("fake-is-installed-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-installed-error"))
				})

				Context("when package is already installed", func() {
					BeforeEach(func() {
						bundle.Installed = true
					})

					It("does not install but only enables package", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(bundle.ActionsCalled).To(Equal([]string{"Enable"})) // no Install
					})

					It("returns error when package enable fails", func() {
						bundle.EnableError = errors.New("fake-enable-error")

						err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-enable-error"))
					})

					It("does not download the package", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(blobstore.GetCallCount()).To(Equal(0))
					})
				})

				Context("when package is not installed", func() {
					BeforeEach(func() {
						bundle.Installed = false
					})

					It("installs and enables package", func() {
						err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(bundle.ActionsCalled).To(Equal([]string{"Install", "Enable"}))
					})

					It("returns error when package enable fails", func() {
						bundle.EnableError = errors.New("fake-enable-error")

						err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-enable-error"))
					})

					ItInstallsPkg(act)
				})
			})
		})

		Describe("KeepOnly", func() {
			ItReturnsErrors := func() {
				It("returns error when bundle collection fails to return list of installed bundles", func() {
					packagesBc.ListErr = errors.New("fake-bc-list-error")

					err := applier.KeepOnly([]models.Package{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bc-list-error"))
				})

				It("returns error when bundle collection cannot retrieve bundle for keep-only package", func() {
					pkg1, bundle1 := buildPkg(packagesBc)

					packagesBc.ListBundles = []boshbc.Bundle{bundle1}
					packagesBc.GetErr = errors.New("fake-bc-get-error")

					err := applier.KeepOnly([]models.Package{pkg1})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bc-get-error"))
				})

				It("returns error when at least one bundle cannot be disabled", func() {
					_, bundle1 := buildPkg(packagesBc)

					packagesBc.ListBundles = []boshbc.Bundle{bundle1}
					bundle1.DisableErr = errors.New("fake-bc-disable-error")

					err := applier.KeepOnly([]models.Package{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bc-disable-error"))
				})
			}

			Context("when operating on packages as a package owner", func() {
				BeforeEach(func() {
					applier = NewCompiledPackageApplier(packagesBc, true, blobstore, fs, logger)
				})

				It("first disables and then uninstalls packages that are not in keeponly list", func() {
					_, bundle1 := buildPkg(packagesBc)
					pkg2, bundle2 := buildPkg(packagesBc)
					_, bundle3 := buildPkg(packagesBc)
					pkg4, bundle4 := buildPkg(packagesBc)

					packagesBc.ListBundles = []boshbc.Bundle{bundle1, bundle2, bundle3, bundle4}

					err := applier.KeepOnly([]models.Package{pkg4, pkg2})
					Expect(err).ToNot(HaveOccurred())

					Expect(bundle1.ActionsCalled).To(Equal([]string{"Disable", "Uninstall"}))
					Expect(bundle2.ActionsCalled).To(Equal([]string{}))
					Expect(bundle3.ActionsCalled).To(Equal([]string{"Disable", "Uninstall"}))
					Expect(bundle4.ActionsCalled).To(Equal([]string{}))
				})

				ItReturnsErrors()

				It("returns error when at least one bundle cannot be uninstalled", func() {
					_, bundle1 := buildPkg(packagesBc)

					packagesBc.ListBundles = []boshbc.Bundle{bundle1}
					bundle1.UninstallErr = errors.New("fake-bc-uninstall-error")

					err := applier.KeepOnly([]models.Package{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bc-uninstall-error"))
				})
			})

			Context("when operating on packages not as a package owner", func() {
				BeforeEach(func() {
					applier = NewCompiledPackageApplier(packagesBc, false, blobstore, fs, logger)
				})

				It("disables and but does not uninstall packages that are not in keeponly list", func() {
					_, bundle1 := buildPkg(packagesBc)
					pkg2, bundle2 := buildPkg(packagesBc)
					_, bundle3 := buildPkg(packagesBc)
					pkg4, bundle4 := buildPkg(packagesBc)

					packagesBc.ListBundles = []boshbc.Bundle{bundle1, bundle2, bundle3, bundle4}

					err := applier.KeepOnly([]models.Package{pkg4, pkg2})
					Expect(err).ToNot(HaveOccurred())

					Expect(bundle1.ActionsCalled).To(Equal([]string{"Disable"})) // no Uninstall
					Expect(bundle2.ActionsCalled).To(Equal([]string{}))
					Expect(bundle3.ActionsCalled).To(Equal([]string{"Disable"})) // no Uninstall
					Expect(bundle4.ActionsCalled).To(Equal([]string{}))
				})

				ItReturnsErrors()
			})

		})
	})
}
