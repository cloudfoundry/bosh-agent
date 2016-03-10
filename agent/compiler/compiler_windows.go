package compiler

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const PackagingScriptName = "packaging.ps1"

func (c concreteCompiler) RunPackagingCommand(compilePath, enablePath string, pkg Package) error {
	// Required to execute a script local to the working directory.
	const scriptPath = `.\` + PackagingScriptName

	command := boshsys.Command{
		Name: "powershell",
		Args: []string{"-NoProfile", "-NonInteractive", scriptPath},
		Env: map[string]string{
			"BOSH_COMPILE_TARGET":  compilePath,
			"BOSH_INSTALL_TARGET":  enablePath,
			"BOSH_PACKAGE_NAME":    pkg.Name,
			"BOSH_PACKAGE_VERSION": pkg.Version,
		},
		WorkingDir: compilePath,
	}

	_, err := c.runner.RunCommand("compilation", PackagingScriptName, command)
	if err != nil {
		return bosherr.WrapError(err, "Running packaging script")
	}
	return nil
}
