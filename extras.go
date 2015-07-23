package jsonapi

type ApiExtras struct {
	Links     []*LinkConfiguration
	LinkPaths map[string][]*LinkConfiguration
}

func (extras *ApiExtras) AddLink(name string, recordType string, parentRecordTypes []string, format string) {
	extras.Links = append(extras.Links, &LinkConfiguration{
		Name:              name,
		RecordType:        recordType,
		ParentRecordTypes: parentRecordTypes,
		Format:            format,
	})
}

func (lc *LinkConfiguration) PathKey() string {
	k := ""

	for _, recordType := range lc.ParentRecordTypes {
		k = k + recordType + "."
	}

	return k + lc.RecordType
}

func (extras *ApiExtras) Build() error {
	extras.LinkPaths = make(map[string][]*LinkConfiguration)

	for _, lc := range extras.Links {
		k := lc.PathKey()

		if extras.LinkPaths[k] == nil {
			extras.LinkPaths[k] = []*LinkConfiguration{lc}
		} else {
			extras.LinkPaths[k] = append(extras.LinkPaths[k], lc)
		}
	}

	return nil
}

type LinkConfiguration struct {
	Name              string
	RecordType        string
	ParentRecordTypes []string
	Format            string
}
