package tarpath

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

type PrefixDetector struct{}

//go:generate counterfeiter . Detector

type Detector interface {
	Detect(tgz string, path string) (bool, error)
}

func NewPrefixDetector() *PrefixDetector {
	return &PrefixDetector{}
}

func (n *PrefixDetector) Detect(tgz string, path string) (bool, error) {
	f, err := os.Open(tgz)
	if err != nil {
		return false, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return false, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		if strings.HasPrefix(header.Name, "./"+path+"/") {
			return true, nil
		}

		if strings.HasPrefix(header.Name, path+"/") {
			return false, nil
		}
	}

	return false, fmt.Errorf("no file with prefix '%s' found", path)
}
