package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry/bosh-agent/v2/app"
	"github.com/cloudfoundry/bosh-agent/v2/releasetarball"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	"github.com/cloudfoundry/bosh-agent/v2/stemcellmetadata"
)

func compileTarball(command string, args []string) {
	options, err := newCompileTarballOptions(command, args)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(options.OutputDirectory, 0o700); err != nil {
		log.Fatal(err)
	}

	stemcellOS, stemcellName, stemcellVersion, err := stemcellmetadata.SlugParts()
	if err != nil {
		log.Fatal(err)
	}
	compiledReleaseFileSuffix := fmt.Sprintf("%s-%s/%s", stemcellOS, stemcellName, stemcellVersion)

	log.Printf("Compiling with stemcell %s", compiledReleaseFileSuffix)

	dirProvider := directories.NewProvider(app.DefaultBaseDirectory)
	// see platform.blobsDirPermissions
	if err := os.MkdirAll(dirProvider.BlobsDir(), 0x0700); err != nil {
		log.Fatal(err)
	}

	compiler, err := releasetarball.NewCompiler(dirProvider)
	if err != nil {
		log.Fatal(err)
	}
	for _, releaseTarballPath := range options.SourceReleases {
		compiledReleaseTarballPath, err := releasetarball.Compile(compiler, releaseTarballPath, dirProvider.BlobsDir(), options.OutputDirectory, compiledReleaseFileSuffix)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Finished archiving compiled tarball %s", compiledReleaseTarballPath)
	}
}

type CompileTarballOptions struct {
	OutputDirectory string
	SourceReleases  []string
}

func newCompileTarballOptions(command string, args []string) (CompileTarballOptions, error) {
	var options CompileTarballOptions
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	flags.StringVar(&options.OutputDirectory, "output-directory", "/tmp", "the directory to put the compiled release tarball")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), //nolint:errcheck
			`The BOSH Agent %[1]s command creates BOSH Release tarballs with compiled packages from tarballs with source packages.

Usage:

	%[1]s [FLAGS] [TARBALL...]

Flags:
`, command)
		flags.PrintDefaults()
	}
	err := flags.Parse(args)
	options.SourceReleases = flags.Args()
	return options, err
}
