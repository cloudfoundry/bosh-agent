package bundlecollection

// BundleDefinition uniquely identifies an asset within a BundleCollection (e.g. Job, Package)
type BundleDefinition interface {
	BundleName() string
	BundleVersion() string
}

type BundleCollection interface {
	Get(defintion BundleDefinition) (bundle Bundle, err error)
	List() ([]Bundle, error)
}

type Bundle interface {
	Install(sourcePath, pathInBundle string) (path string, err error)
	InstallWithoutContents() (path string, err error)
	Uninstall() (err error)

	IsInstalled() (bool, error)
	GetInstallPath() (path string, err error)

	Enable() (path string, err error)
	Disable() (err error)
}
