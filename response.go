package jsonapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Wrties a jsonapi response with one, with related records sideloaded, into "included" array.
// This method encodes a response for a single record only. Hence, data will be a single record rather
// than an array of records.  If you want to serialize many records, see, MarshalManyPayload.
//
// See UnmarshalPayload for usage example.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayload(w io.Writer, model interface{}) error {
	rootNode, included, err := visitModelNode(model, true)
	if err != nil {
		return err
	}
	payload := &OnePayload{Data: rootNode}

	payload.Included, _ = uniqueByTypeAndId(included)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

func MarshalOnePayloadWithExtras(w io.Writer, model interface{}, c func(*ApiExtras)) error {
	extras := new(ApiExtras)

	c(extras)

	return MarshalOnePayloadWithExtrasConfig(w, model, extras)
}

func MarshalOnePayloadWithExtrasConfig(w io.Writer, model interface{}, extras *ApiExtras) error {
	if extras != nil {
		if err := extras.Build(); err != nil {
			return err
		}
	}

	rootNode, included, err := visitModelNode(model, true)
	if err != nil {
		return err
	}

	payload := &OnePayload{Data: rootNode}
	uniqueIncluded, includedMap := uniqueByTypeAndId(included)
	payload.Included = uniqueIncluded

	if extras != nil {
		rootNodeLinks := make(map[string]string)
		nodeStack := []*Node{rootNode}

		for linkName, format := range extras.RootLinks {
			rootNodeLinks[linkName] = sprintfLink(format, nodeStack, includedMap)
		}

		payload.Links = rootNodeLinks

		if rootNode.Relationships != nil && len(rootNode.Relationships) > 0 {
			visitRelationshipsAddExtras(rootNode.Relationships, extras.KeyedRelationshipLinks, nodeStack, includedMap)
		}
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

// Wrties a jsonapi response with many records, with related records sideloaded, into "included" array.
// This method encodes a response for a slice of records, hence data will be an array of
// records rather than a single record.  To serialize a single record, see MarshalOnePayload
//
// For example you could pass it, w, your http.ResponseWriter, and, models, a slice of Blog
// struct instance pointers as interface{}'s to write to the response,
//
//	 func ListBlogs(w http.ResponseWriter, r *http.Request) {
//		 // ... fetch your blogs and filter, offset, limit, etc ...
//
//		 blogs := testBlogsForList()
//
//		 w.WriteHeader(200)
//		 w.Header().Set("Content-Type", "application/vnd.api+json")
//		 if err := jsonapi.MarshalManyPayload(w, blogs); err != nil {
//			 http.Error(w, err.Error(), 500)
//		 }
//	 }
//
//
// Visit https://github.com/shwoodard/jsonapi#list for more info.
//
// models []interface{} should be a slice of struct pointers.
func MarshalManyPayload(w io.Writer, models []interface{}) error {
	modelsValues := reflect.ValueOf(models)
	data := make([]*Node, 0, modelsValues.Len())

	incl := make([]*Node, 0)

	for i := 0; i < modelsValues.Len(); i += 1 {
		model := modelsValues.Index(i).Interface()

		node, included, err := visitModelNode(model, true)
		if err != nil {
			return err
		}
		data = append(data, node)
		incl = append(incl, included...)
	}

	uniqueIncluded, _ := uniqueByTypeAndId(incl)

	payload := &ManyPayload{
		Data:     data,
		Included: uniqueIncluded,
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

// This method not meant to for use in implementation code, although feel
// free.  The purpose of this method is for use in tests.  In most cases, your
// request payloads for create will be embedded rather than sideloaded for related records.
// This method will serialize a single struct pointer into an embedded json
// response.  In other words, there will be no, "included", array in the json
// all relationships will be serailized inline in the data.
//
// However, in tests, you may want to construct payloads to post to create methods
// that are embedded to most closely resemble the payloads that will be produced by
// the client.  This is what this method is intended for.
//
// model interface{} should be a pointer to a struct.
func MarshalOnePayloadEmbedded(w io.Writer, model interface{}) error {
	rootNode, _, err := visitModelNode(model, false)
	if err != nil {
		return err
	}

	payload := &OnePayload{Data: rootNode}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return err
	}

	return nil
}

func visitModelNode(model interface{}, sideload bool) (*Node, []*Node, error) {
	node := new(Node)

	var er error
	var included []*Node

	modelType := reflect.TypeOf(model).Elem()
	modelValue := reflect.ValueOf(model).Elem()

	var i = 0
	modelType.FieldByNameFunc(func(name string) bool {
		if er != nil {
			return false
		}

		structField := modelType.Field(i)
		tag := structField.Tag.Get("jsonapi")
		if tag == "" {
			i += 1
			return false
		}

		fieldValue := modelValue.Field(i)

		i += 1

		args := strings.Split(tag, ",")

		if len(args) != 2 {
			er = errors.New(fmt.Sprintf("jsonapi tag, on %s, had two few arguments", structField.Name))
			return false
		}

		if len(args) >= 1 && args[0] != "" {
			annotation := args[0]

			if annotation == "primary" {
				node.Id = fmt.Sprintf("%v", fieldValue.Interface())
				node.Type = args[1]
			} else if annotation == "attr" {
				if node.Attributes == nil {
					node.Attributes = make(map[string]interface{})
				}

				if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
					isZeroMethod := fieldValue.MethodByName("IsZero")
					isZero := isZeroMethod.Call(make([]reflect.Value, 0))[0].Interface().(bool)
					if isZero {
						return false
					}

					unix := fieldValue.MethodByName("Unix")
					val := unix.Call(make([]reflect.Value, 0))[0]
					node.Attributes[args[1]] = val.Int()
				} else {
					node.Attributes[args[1]] = fieldValue.Interface()
				}
			} else if annotation == "relation" {
				isSlice := fieldValue.Type().Kind() == reflect.Slice

				if (isSlice && fieldValue.Len() < 1) || (!isSlice && fieldValue.IsNil()) {
					return false
				}

				if node.Relationships == nil {
					node.Relationships = make(map[string]interface{})
				}

				if sideload && included == nil {
					included = make([]*Node, 0)
				}

				if isSlice {
					relationship, incl, err := visitModelNodeRelationships(args[1], fieldValue, sideload)
					d := relationship.Data

					if err == nil {
						if sideload {
							included = append(included, incl...)
							shallowNodes := make([]*Node, 0)
							for _, node := range d {
								shallowNodes = append(shallowNodes, toShallowNode(node))
							}

							node.Relationships[args[1]] = &RelationshipManyNode{Data: shallowNodes}
						} else {
							node.Relationships[args[1]] = relationship
						}
					} else {
						er = err
						return false
					}
				} else {
					relationship, incl, err := visitModelNode(fieldValue.Interface(), sideload)
					if err == nil {
						if sideload {
							included = append(included, incl...)
							included = append(included, relationship)
							node.Relationships[args[1]] = &RelationshipOneNode{Data: toShallowNode(relationship)}
						} else {
							node.Relationships[args[1]] = &RelationshipOneNode{Data: relationship}
						}
					} else {
						er = err
						return false
					}
				}

			} else if annotation != "links" {
				er = errors.New(fmt.Sprintf("Unsupported jsonapi tag annotation, %s", annotation))
				return false
			}
		}

		return false
	})

	if er != nil {
		return nil, nil, er
	}

	return node, included, nil
}

func toShallowNode(node *Node) *Node {
	return &Node{
		Id:   node.Id,
		Type: node.Type,
	}
}

func visitModelNodeRelationships(relationName string, models reflect.Value, sideload bool) (*RelationshipManyNode, []*Node, error) {
	nodes := make([]*Node, 0)

	var included []*Node
	if sideload {
		included = make([]*Node, 0)
	}

	for i := 0; i < models.Len(); i++ {
		node, incl, err := visitModelNode(models.Index(i).Interface(), sideload)
		if err != nil {
			return nil, nil, err
		}

		nodes = append(nodes, node)
		included = append(included, incl...)
	}

	included = append(included, nodes...)

	n := &RelationshipManyNode{Data: nodes}

	return n, included, nil
}

func uniqueByTypeAndId(nodes []*Node) ([]*Node, map[string]*Node) {
	uniqueIncluded := make(map[string]*Node)

	for i := 0; i < len(nodes); i += 1 {
		n := nodes[i]
		k := fmt.Sprintf("%s,%s", n.Type, n.Id)
		if uniqueIncluded[k] == nil {
			uniqueIncluded[k] = n
		} else {
			nodes = append(nodes[:i], nodes[i+1:]...)
			i--
		}
	}

	return nodes, uniqueIncluded
}

func visitRelationshipsAddExtras(relationships map[string]interface{}, relationshipExtras map[string][]*LinkConfiguration, nodeStack []*Node, included map[string]*Node) {
	for relationshipName, r := range relationships {
		var recordType string

		withRelationship(r, func(rel *RelationshipOneNode) {
			recordType = rel.Data.Type
		}, func(rel *RelationshipManyNode) {
			if rel.Data == nil || len(rel.Data) == 0 {
				return
			} else {
				recordType = rel.Data[0].Type
			}
		})

		if recordType == "" {
			continue
		}

		k := fmt.Sprintf("%s,%s,%s", recordType, nodeStack[len(nodeStack)-1].Type, relationshipName)
		if relationshipExtras[k] == nil {
			return
		}

		links := relationshipLinks(relationshipExtras[k], nodeStack, included)

		withRelationship(r, func(rel *RelationshipOneNode) {
			rel.Links = links

			if rel.Data.Relationships == nil || len(rel.Data.Relationships) == 0 {
				includedKey := fmt.Sprintf("%s,%s", rel.Data.Type, rel.Data.Id)
				incl := included[includedKey]

				if incl == nil || incl.Relationships == nil || len(incl.Relationships) == 0 {
					return
				} else {
					nodeStack = append(nodeStack, incl)
				}
			} else {
				nodeStack = append(nodeStack, rel.Data)
			}

			visitRelationshipsAddExtras(rel.Data.Relationships, relationshipExtras, nodeStack, included)

			nodeStack = nodeStack[:len(nodeStack)-1]
		}, func(rel *RelationshipManyNode) {
			rel.Links = links

			for _, n := range rel.Data {
				if n.Relationships == nil || len(n.Relationships) == 0 {
					includedKey := fmt.Sprintf("%s,%s", n.Type, n.Id)
					incl := included[includedKey]

					if incl == nil || incl.Relationships == nil || len(incl.Relationships) == 0 {
						return
					} else {
						nodeStack = append(nodeStack, incl)
					}
				} else {
					nodeStack = append(nodeStack, n)
				}

				visitRelationshipsAddExtras(n.Relationships, relationshipExtras, nodeStack, included)

				nodeStack = nodeStack[:len(nodeStack)-1]
			}
		})

	}
}

func relationshipLinks(linkConfigs []*LinkConfiguration, nodeStack []*Node, included map[string]*Node) map[string]string {
	links := make(map[string]string)

	for _, lc := range linkConfigs {
		link := sprintfLink(lc.Format, nodeStack, included)
		links[lc.Name] = link
	}

	return links
}

func findNodeByType(nodes []*Node, recordType string) *Node {
	for _, n := range nodes {
		if n.Type == recordType {
			return n
		}
	}

	return nil
}

func sprintfLink(format string, nodeStack []*Node, included map[string]*Node) string {
	link := format
	regex := regexp.MustCompile(`(?:\{((?:\w+\.?)+)\})`)

	for _, m := range regex.FindAll([]byte(format), -1) {
		match := string(m)
		parts := strings.Split(match[1:len(match)-1], ".")
		field := parts[len(parts)-1]
		recordType := parts[0]

		node := findNodeByType(nodeStack, recordType)

		if node == nil {
			link = strings.Replace(link, match, "", -1)
			continue
		}

		var value interface{}

		if field == "id" {
			value = node.Id
		} else {
			if node.Attributes == nil || len(node.Attributes) == 0 {
				includedKey := fmt.Sprintf("%s,%s", node.Type, node.Id)
				if r := included[includedKey]; r != nil {
					value = r.Attributes[field]
				} else {
					return ""
				}
			} else {
				value = node.Attributes[field]
			}
		}

		link = strings.Replace(link, match, fmt.Sprintf("%v", value), -1)
	}

	uri, _ := url.Parse(link)
	uri.RawQuery = url.QueryEscape(uri.RawQuery)

	return uri.String()
}

func withRelationship(relationship interface{}, oneCallback func(r *RelationshipOneNode), manyCallback func(r *RelationshipManyNode)) {
	r, isRelationshipOneNode := relationship.(*RelationshipOneNode)

	if isRelationshipOneNode {
		oneCallback(r)
	} else {
		rel := relationship.(*RelationshipManyNode)
		manyCallback(rel)
	}
}
