package sls

import (
	"encoding/json"
)

// GraphStoreLayoutType selects a logical vertex or edge collection.
type GraphStoreLayoutType string

const (
	GraphStoreLayoutVertex GraphStoreLayoutType = "vertex"
	GraphStoreLayoutEdge   GraphStoreLayoutType = "edge"
)

// GraphStoreFieldType is the property type; INT is a signed 64-bit integer.
type GraphStoreFieldType string

const (
	GraphStoreFieldString GraphStoreFieldType = "STRING"
	GraphStoreFieldInt    GraphStoreFieldType = "INT"
	GraphStoreFieldDouble GraphStoreFieldType = "DOUBLE"
	GraphStoreFieldBool   GraphStoreFieldType = "BOOL"
)

// GraphStoreFullTextIndex describes logical schema configuration, without physical storage metadata.
type GraphStoreFullTextIndex struct {
	// Required nonempty token characters.
	Token string `json:"token"`
	// Optional nonempty subset of token characters; nil omits the field.
	OptionToken *string `json:"optionToken,omitempty"`
	// Required case sensitivity setting.
	CaseSensitive bool `json:"caseSensitive"`
	// Required Chinese tokenization setting.
	Chn bool `json:"chn"`
}

// GraphStoreRangeIndex describes logical schema configuration, without physical storage metadata.
type GraphStoreRangeIndex struct {
}

// GraphStoreIndexConfig describes logical schema configuration, without physical storage metadata.
type GraphStoreIndexConfig struct {
	// Required; true without a specialized index selects Exact.
	Enable bool `json:"enable"`
	// Optional FullText configuration.
	FullTextIndex *GraphStoreFullTextIndex `json:"fullTextIndex,omitempty"`
	// Optional Range configuration; the service validates index combinations.
	RangeIndex *GraphStoreRangeIndex `json:"rangeIndex,omitempty"`
}

// GraphStoreFieldSchema describes logical schema configuration, without physical storage metadata.
type GraphStoreFieldSchema struct {
	// Required nonempty property field name.
	Name string `json:"name"`
	// Required property type; INT is signed int64.
	Type GraphStoreFieldType `json:"type"`
	// Required field index configuration.
	IndexConfig GraphStoreIndexConfig `json:"indexConfig"`
}

// GraphStoreCollectionKeyValue describes logical schema configuration, without physical storage metadata.
type GraphStoreCollectionKeyValue struct {
	// Required nonempty CK field name.
	Name string `json:"name"`
	// Required logical string constant; an empty string is allowed.
	Value string `json:"value"`
}

// GraphStoreEndpoint describes logical schema configuration, without physical storage metadata.
type GraphStoreEndpoint struct {
	// Legacy shorthand for a single component. LabelComponents takes precedence.
	Label string `json:"-"`
	// Ordered label field names for the source or target.
	LabelComponents []string `json:"-"`
	// Required source or target ID field name.
	ID string `json:"id"`
}

// LabelFields returns the endpoint label field names in binding order.
func (s GraphStoreEndpoint) LabelFields() []string {
	return graphStoreLabelFields(s.Label, s.LabelComponents)
}

func (s GraphStoreEndpoint) MarshalJSON() ([]byte, error) {
	type plain GraphStoreEndpoint
	return json.Marshal(struct {
		plain
		Label []string `json:"label"`
	}{plain(s), s.LabelFields()})
}

func (s *GraphStoreEndpoint) UnmarshalJSON(data []byte) error {
	type plain GraphStoreEndpoint
	var endpoint GraphStoreEndpoint
	value := struct {
		*plain
		Label graphStoreLabel `json:"label"`
	}{plain: (*plain)(&endpoint)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	endpoint.Label, endpoint.LabelComponents = value.Label.layoutFields()
	*s = endpoint
	return nil
}

// GraphStoreCollectionSchema describes logical schema configuration, without physical storage metadata.
type GraphStoreCollectionSchema struct {
	// Required nonempty logical collection name; no numeric collection ID is exposed.
	Name string `json:"name"`
	// Required nonempty description.
	Description string `json:"description"`
	// Required fixed vertex or edge layout.
	GraphLayout GraphStoreLayoutType `json:"graphLayout"`
	// Required complete user CK values; system CK values belong to the service.
	CollectionKeyValues []GraphStoreCollectionKeyValue `json:"collectionKeyValues"`
	// Nil inherits BaseSchema; explicit TTL must be positive seconds.
	TTLSeconds *int64 `json:"ttlSeconds,omitempty"`
	// Required property array; an empty array is valid.
	Fields []GraphStoreFieldSchema `json:"fields"`
	// Read-only logical labels returned by Get/List; omitted from Create requests.
	Graph *GraphStoreCollectionGraph `json:"graph,omitempty"`
}

// GraphStoreCollectionGraph contains the effective labels for a collection.
type GraphStoreCollectionGraph struct {
	// Label is []string for vertices and string for edges.
	Label       interface{} `json:"label"`
	SourceLabel []string    `json:"sourceLabel,omitempty"`
	TargetLabel []string    `json:"targetLabel,omitempty"`
}

func (s *GraphStoreCollectionGraph) UnmarshalJSON(data []byte) error {
	type plain GraphStoreCollectionGraph
	var graph GraphStoreCollectionGraph
	value := struct {
		*plain
		Label graphStoreLabel `json:"label"`
	}{plain: (*plain)(&graph)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	graph.Label = value.Label.value
	*s = graph
	return nil
}

// GraphStoreCollectionSchemaUpdate describes logical schema configuration, without physical storage metadata.
type GraphStoreCollectionSchemaUpdate struct {
	// Required nonempty name, identical to the request path.
	Name string `json:"name"`
	// Required nonempty replacement description.
	Description string `json:"description"`
	// Nil restores inheritance; explicit TTL must be positive seconds. Replacement, not patch.
	TTLSeconds *int64 `json:"ttlSeconds,omitempty"`
	// Required complete replacement property array; an empty array is valid.
	Fields []GraphStoreFieldSchema `json:"fields"`
}

// GraphStoreVertexLayout names key fields. Nil CK/PK/RouteKey use server defaults;
// non-nil empty slices are encoded as [] and are distinct from omitted keys.
type GraphStoreVertexLayout struct {
	// Legacy shorthand for a single component. LabelComponents takes precedence.
	Label string `json:"-"`
	// Ordered vertex label field names.
	LabelComponents []string `json:"-"`
	// Required vertex ID field name.
	ID string `json:"id"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	CK []string `json:"ck,omitempty"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	PK []string `json:"pk,omitempty"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	RouteKey []string `json:"routeKey,omitempty"`
	// Optional typed key index configuration; nil omits the field.
	KeyIndexConfigs map[string]GraphStoreIndexConfig `json:"keyIndexConfigs,omitempty"`
}

// LabelFields returns the vertex label field names in binding order.
func (s GraphStoreVertexLayout) LabelFields() []string {
	return graphStoreLabelFields(s.Label, s.LabelComponents)
}

func graphStoreLabelFields(label string, components []string) []string {
	if components != nil {
		return components
	}
	if label != "" {
		return []string{label}
	}
	return nil
}

type graphStoreLabel struct {
	value interface{}
}

func (s *graphStoreLabel) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		var label string
		if err := json.Unmarshal(data, &label); err != nil {
			return err
		}
		s.value = label
		return nil
	}
	var components []string
	if err := json.Unmarshal(data, &components); err != nil {
		return err
	}
	s.value = nil
	if components != nil {
		s.value = components
	}
	return nil
}

func (s graphStoreLabel) layoutFields() (string, []string) {
	switch label := s.value.(type) {
	case string:
		return label, nil
	case []string:
		if len(label) == 1 && label[0] != "" {
			return label[0], nil
		}
		return "", label
	default:
		return "", nil
	}
}

// Name is the fixed wire discriminator.
func (GraphStoreVertexLayout) Name() string { return "vertex" }

// GraphStoreEdgeLayout names the source, label and target fields. Optional keys
// have the same omitted/empty semantics as GraphStoreVertexLayout.
type GraphStoreEdgeLayout struct {
	// Required source field names.
	Source GraphStoreEndpoint `json:"source"`
	// Required edge label field name.
	Label string `json:"label"`
	// Required target field names.
	Target GraphStoreEndpoint `json:"target"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	CK []string `json:"ck,omitempty"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	PK []string `json:"pk,omitempty"`
	// Nil omits the key; an explicit empty list is preserved. Defaults and key rules belong to the service.
	RouteKey []string `json:"routeKey,omitempty"`
	// Optional typed key index configuration; nil omits the field.
	KeyIndexConfigs map[string]GraphStoreIndexConfig `json:"keyIndexConfigs,omitempty"`
}

func (GraphStoreEdgeLayout) Name() string { return "edge" }

func (s GraphStoreVertexLayout) MarshalJSON() ([]byte, error) {
	type plain GraphStoreVertexLayout
	value := struct {
		*plain
		Name            string                            `json:"name"`
		Label           []string                          `json:"label"`
		CK              *[]string                         `json:"ck,omitempty"`
		PK              *[]string                         `json:"pk,omitempty"`
		RouteKey        *[]string                         `json:"routeKey,omitempty"`
		KeyIndexConfigs *map[string]GraphStoreIndexConfig `json:"keyIndexConfigs,omitempty"`
	}{plain: (*plain)(&s), Name: s.Name(), Label: s.LabelFields()}
	if s.CK != nil {
		value.CK = &s.CK
	}
	if s.PK != nil {
		value.PK = &s.PK
	}
	if s.RouteKey != nil {
		value.RouteKey = &s.RouteKey
	}
	if s.KeyIndexConfigs != nil {
		value.KeyIndexConfigs = &s.KeyIndexConfigs
	}
	return json.Marshal(value)
}

func (s *GraphStoreVertexLayout) UnmarshalJSON(data []byte) error {
	type plain GraphStoreVertexLayout
	var layout GraphStoreVertexLayout
	value := struct {
		*plain
		Label graphStoreLabel `json:"label"`
	}{plain: (*plain)(&layout)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	layout.Label, layout.LabelComponents = value.Label.layoutFields()
	*s = layout
	return nil
}

func (s GraphStoreEdgeLayout) MarshalJSON() ([]byte, error) {
	type plain GraphStoreEdgeLayout
	value := struct {
		*plain
		Name            string                            `json:"name"`
		CK              *[]string                         `json:"ck,omitempty"`
		PK              *[]string                         `json:"pk,omitempty"`
		RouteKey        *[]string                         `json:"routeKey,omitempty"`
		KeyIndexConfigs *map[string]GraphStoreIndexConfig `json:"keyIndexConfigs,omitempty"`
	}{plain: (*plain)(&s), Name: s.Name()}
	if s.CK != nil {
		value.CK = &s.CK
	}
	if s.PK != nil {
		value.PK = &s.PK
	}
	if s.RouteKey != nil {
		value.RouteKey = &s.RouteKey
	}
	if s.KeyIndexConfigs != nil {
		value.KeyIndexConfigs = &s.KeyIndexConfigs
	}
	return json.Marshal(value)
}

// GraphStoreGraphLayoutUpdate replaces the explicit CK/PK index map for one existing layout.
type GraphStoreGraphLayoutUpdate struct {
	Name GraphStoreLayoutType `json:"name"`
	// Required map; use an empty non-nil map to clear explicit key settings.
	KeyIndexConfigs map[string]GraphStoreIndexConfig `json:"keyIndexConfigs"`
}

// GraphStoreBaseSchemaUpdate replaces mutable settings; key structure remains service owned.
type GraphStoreBaseSchemaUpdate struct {
	TTLSeconds    int64                    `json:"ttlSeconds"`
	IndexEnable   bool                     `json:"indexEnable"`
	FullTextIndex *GraphStoreFullTextIndex `json:"fullTextIndex,omitempty"`
	// Required vertex and edge entries, including empty index maps.
	GraphLayouts []GraphStoreGraphLayoutUpdate `json:"graphLayouts"`
}

// GraphStoreBaseSchema owns both layouts; the wire representation remains graphLayouts.
type GraphStoreBaseSchema struct {
	// Read-only Store version assigned by Master; omitted from create requests.
	Version uint32 `json:"version,omitempty"`
	// Read-only CMS mode returned by Get; Create controls it through request headers.
	CMSMode bool `json:"cmsMode,omitempty"`
	// Read-only time state setting returned by Get; Create controls it through request headers.
	TimeStateEnable bool `json:"timeStateEnable"`
	// Required positive seconds; validated by the service.
	TTLSeconds int64 `json:"ttlSeconds"`
	// Optional common property names; the service defaults omitted values to an empty array.
	CommonFieldNames []string `json:"commonFieldNames,omitempty"`
	// Required global index setting; configuration combinations are validated by the service.
	IndexEnable bool `json:"indexEnable"`
	// Optional global FullText configuration.
	FullTextIndex *GraphStoreFullTextIndex `json:"fullTextIndex,omitempty"`
	// Required vertex layout; encoded within graphLayouts.
	VertexLayout GraphStoreVertexLayout `json:"-"`
	// Required edge layout; encoded within graphLayouts.
	EdgeLayout GraphStoreEdgeLayout `json:"-"`
}

type graphStoreBaseSchemaFields GraphStoreBaseSchema

type graphStoreBaseSchemaWire struct {
	graphStoreBaseSchemaFields
	GraphLayouts [2]interface{} `json:"graphLayouts"`
}

func (s GraphStoreBaseSchema) wire() graphStoreBaseSchemaWire {
	return graphStoreBaseSchemaWire{graphStoreBaseSchemaFields(s), [2]interface{}{s.VertexLayout, s.EdgeLayout}}
}

func (s GraphStoreBaseSchema) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.wire())
}

func (s *GraphStoreBaseSchema) UnmarshalJSON(data []byte) error {
	type plain GraphStoreBaseSchema
	var candidate GraphStoreBaseSchema
	value := struct {
		*plain
		GraphLayouts []json.RawMessage `json:"graphLayouts"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	for _, layout := range value.GraphLayouts {
		var discriminator struct{ Name string }
		if err := json.Unmarshal(layout, &discriminator); err != nil {
			return err
		}
		switch discriminator.Name {
		case "vertex":
			if err := json.Unmarshal(layout, &candidate.VertexLayout); err != nil {
				return err
			}
		case "edge":
			if err := json.Unmarshal(layout, &candidate.EdgeLayout); err != nil {
				return err
			}
		}
	}
	*s = candidate
	return nil
}
