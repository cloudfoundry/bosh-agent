package packages

type ApplierProvider interface {
	Root() Applier
	JobSpecific(jobName string) Applier
}
