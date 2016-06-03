package compiler

import (
	"os"
	"path"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func (c concreteCompiler) runPackagingCommand(compilePath, enablePath string, pkg Package) error {
	packagingScript := path.Join(compilePath, PackagingScriptName)
	file, err := c.fs.OpenFile(packagingScript, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	command := boshsys.Command{
		Name:  "powershell",
		Args:  []string{"-command", "'$input | iex'"},
		Stdin: file,
		Env: map[string]string{
			"BOSH_COMPILE_TARGET":  compilePath,
			"BOSH_INSTALL_TARGET":  enablePath,
			"BOSH_PACKAGE_NAME":    pkg.Name,
			"BOSH_PACKAGE_VERSION": pkg.Version,
		},
		WorkingDir: compilePath,
	}

	_, err = c.runner.RunCommand("compilation", PackagingScriptName, command)
	if err != nil {
		return bosherr.WrapError(err, "Running packaging script")
	}
	return nil
}
