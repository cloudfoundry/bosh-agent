package ini

import (
	"bytes"
	"fmt"
	"io"
)

type File struct {
	Comment  string
	sections []*Section
}

type Section struct {
	Name string
	Keys []*Key
}

type Key struct {
	Name  string
	Value string
}

func Empty() *File {
	return &File{
		Comment:  "",
		sections: make([]*Section, 0, 10),
	}
}

func (f *File) AppendSection(section *Section) {
	f.sections = append(f.sections, section)
}

func (f *File) WriteTo(w io.Writer) (int64, error) {
	buf := bytes.NewBuffer(nil)

	if f.Comment != "" {
		buf.WriteString(f.Comment + "\n")
	}

	for _, section := range f.sections {
		buf.WriteString("[" + section.Name + "]\n")

		for _, key := range section.Keys {
			buf.WriteString(fmt.Sprintf("%s=%s\n", key.Name, key.Value))
		}

		buf.WriteString("\n")
	}

	return buf.WriteTo(w)
}

func (s *Section) AddKey(name, value string) {
	s.Keys = append(s.Keys, &Key{Name: name, Value: value})
}
