package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudfoundry/bosh-agent/app"
	"github.com/cloudfoundry/bosh-agent/releasetarball"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
)

func compileTarball(command string, args []string) {
	options, err := newCompileTarballOptions(command, args)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(options.OutputDirectory, 0o700); err != nil {
		log.Fatal(err)
	}

	stemcellOS, stemcellName, stemcellVersion, err := readStemcellSlug()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Compiling with stemcell %s-%s/%s", stemcellOS, stemcellName, stemcellVersion)

	dirProvider := directories.NewProvider(app.DefaultBaseDirectory)

	compiler, err := releasetarball.NewCompiler(dirProvider)
	if err != nil {
		log.Fatal(err)
	}
	compiledReleaseFileSuffix := strings.Join([]string{stemcellOS, stemcellName, stemcellVersion}, "-")
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
		_, _ = fmt.Fprintf(flags.Output(), `The BOSH Agent %[1]s command creates BOSH Release tarballs with compiled packages from tarballs with source packages.

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
