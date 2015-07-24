package jsonapi

import "fmt"

type ApiExtras struct {
	RootLinks              map[string]string
	RelationshipLinks      []*LinkConfiguration
	KeyedRelationshipLinks map[string][]*LinkConfiguration
}

func (extras *ApiExtras) AddRootLink(linkName string, format string) {
	if extras.RootLinks == nil {
		extras.RootLinks = make(map[string]string)
	}

	extras.RootLinks[linkName] = format
}

func (extras *ApiExtras) AddRelationshipLink(linkName string, recordType string, relationshipName string, parentRecordType string, format string) {
	extras.RelationshipLinks = append(extras.RelationshipLinks, &LinkConfiguration{
		Name:             linkName,
		RecordType:       recordType,
		ParentRecordType: parentRecordType,
		RelationshipName: relationshipName,
		Format:           format,
	})
}

func (extras *ApiExtras) Build() error {
	extras.KeyedRelationshipLinks = make(map[string][]*LinkConfiguration)

	for _, lc := range extras.RelationshipLinks {
		k := fmt.Sprintf("%s,%s,%s", lc.RecordType, lc.ParentRecordType, lc.RelationshipName)

		if extras.KeyedRelationshipLinks[k] == nil {
			extras.KeyedRelationshipLinks[k] = []*LinkConfiguration{lc}
		} else {
			extras.KeyedRelationshipLinks[k] = append(extras.KeyedRelationshipLinks[k], lc)
		}
	}

	return nil
}

type LinkConfiguration struct {
	Name             string
	RecordType       string
	ParentRecordType string
	RelationshipName string
	Format           string
}
