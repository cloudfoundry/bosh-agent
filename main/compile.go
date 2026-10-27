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
		compiledReleaseTarballPath, err := releasetarball.Compile(compiler, releaseTarballPath, dirProvider.BlobsDir(), options.OutputDirectory, compiledReleaseFileSuffix, releasetarball.CompileOptions{ExcludeCompileTimePackages: options.ExcludeCompileTimePackages})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Finished archiving compiled tarball %s", compiledReleaseTarballPath)
	}
}

type CompileTarballOptions struct {
	OutputDirectory            string
	SourceReleases             []string
	ExcludeCompileTimePackages bool
}

func newCompileTarballOptions(command string, args []string) (CompileTarballOptions, error) {
	var options CompileTarballOptions
	flags := newCompileTarballFlagSet(command, &options)
	err := flags.Parse(args)
	options.SourceReleases = flags.Args()
	return options, err
}

func newCompileTarballFlagSet(command string, options *CompileTarballOptions) *flag.FlagSet {
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	flags.StringVar(&options.OutputDirectory, "output-directory", "/tmp", "the directory to put the compiled release tarball")
	flags.BoolVar(&options.ExcludeCompileTimePackages, "exclude-compile-time-packages", false, "omit compiled packages that no job lists (e.g. Go/Rust toolchains); see notes below")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), //nolint:errcheck
			`The BOSH Agent %[1]s command creates BOSH Release tarballs with compiled packages from tarballs with source packages.

Usage:

	%[1]s [FLAGS] [TARBALL...]

Flags:
`, command)
		flags.PrintDefaults()
		_, _ = fmt.Fprintln(flags.Output(), //nolint:errcheck
			`Notes on --exclude-compile-time-packages:
	- Compiled output gets a distinct version: +trimmed is appended, or .trimmed if the version already contains +; the filename changes to match. If nothing is excluded, the version and output are unchanged.
	- Every package listed by any job's job.MF is kept; others are recorded under compile_time_packages. Keep the untrimmed release to roll back. Export trimmed releases only on the stemcell they are compiled for.`)
	}
	return flags
}
