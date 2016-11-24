package crypto

import (
	"fmt"
	"io"
	"os"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type digestProviderImpl struct {
	fs boshsys.FileSystem
}

func NewDigestProvider(fs boshsys.FileSystem) DigestProvider {
	return digestProviderImpl{
		fs: fs,
	}
}

func (f digestProviderImpl) CreateFromFile(filePath string, algorithm DigestAlgorithm) (Digest, error) {
	hash, err := CreateHashFromAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}

	file, err := f.fs.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, bosherr.WrapError(err, "Opening file for digest calculation")
	}

	defer file.Close()

	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, bosherr.WrapError(err, "Copying file for digest calculation")
	}

	return NewDigest(algorithm, fmt.Sprintf("%x", hash.Sum(nil))), nil
}
