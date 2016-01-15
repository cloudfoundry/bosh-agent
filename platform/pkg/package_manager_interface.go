package pkg

type Manager interface {
	RemovePackage(packageName string) error
}
