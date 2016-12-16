package models

type Package struct {
	Name    string
	Version string
	Source  Source
}

func (s Package) BundleName() string {
	return s.Name
}

func (s Package) BundleVersion() string {
	var digest string

	if s.Source.Sha1 != nil {
		digest = s.Source.Sha1.String()
	}

	return s.Version + "-" + digest
}
