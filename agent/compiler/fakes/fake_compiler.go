package fakes

import (
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"

	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	boshcomp "github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
)

type FakeCompiler struct {
	CompilePkg    boshcomp.Package
	CompileDeps   []boshmodels.Package
	CompileBlobID string
	CompileDigest boshcrypto.Digest
	CompileErr    error
}

func NewFakeCompiler() (c *FakeCompiler) {
	c = new(FakeCompiler)
	return
}

func (c *FakeCompiler) Compile(pkg boshcomp.Package, deps []boshmodels.Package) (blobID string, digest boshcrypto.Digest, err error) {
	c.CompilePkg = pkg
	c.CompileDeps = deps
	blobID = c.CompileBlobID
	digest = c.CompileDigest
	err = c.CompileErr
	return
}
