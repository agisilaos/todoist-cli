package api

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// ResponseState separates facts not returned from explicit null and invalid data.
type ResponseState uint8

const (
	ResponseAbsent ResponseState = iota
	ResponseNull
	ResponseValue
	ResponseInvalid
)

// ResponseFact is an immutable fact from a response, independent of Go defaults.
type ResponseFact struct {
	State ResponseState
	raw   string
}

func (f ResponseFact) Text() (string, bool) {
	var value string
	err := json.Unmarshal([]byte(f.raw), &value)
	return value, f.State == ResponseValue && err == nil
}

func (f ResponseFact) Bool() (bool, bool) {
	var value bool
	err := json.Unmarshal([]byte(f.raw), &value)
	return value, f.State == ResponseValue && err == nil
}

func (f ResponseFact) Int() (int, bool) {
	value, err := strconv.Atoi(f.raw)
	return value, f.State == ResponseValue && err == nil
}

// JSON returns owned bytes; callers cannot change retained response facts.
func (f ResponseFact) JSON() json.RawMessage { return json.RawMessage(f.raw) }

type ResponseIssue struct {
	Path     string
	Expected string
	Actual   string
}

// Maps and slices are private and never modified after decoding. Task copies
// safely share this snapshot, without sharing mutable resource data with callers.
type responseFacts struct {
	resource string
	facts    map[string]ResponseFact
	issues   []ResponseIssue
}

type factType struct {
	kind    string
	members map[string]factType
}

var dueFactTypes = scalarFactTypes("date datetime string lang timezone", "", "is_recurring")
var taskFactTypes = func() map[string]factType {
	types := scalarFactTypes(
		"user_id id project_id section_id parent_id added_by_uid assigned_by_uid responsible_uid added_at completed_at completed_by_uid updated_at order_key content description",
		"priority child_order note_count day_order completed_count postponed_count",
		"is_collapsed checked is_deleted is_uncompletable",
	)
	types["labels"] = factType{kind: "string array"}
	types["due"] = factType{kind: "object", members: dueFactTypes}
	types["deadline"] = factType{kind: "object", members: scalarFactTypes("date lang", "", "")}
	types["duration"] = factType{kind: "object", members: scalarFactTypes("unit", "amount", "")}
	return types
}()

func scalarFactTypes(text, integers, booleans string) map[string]factType {
	result := make(map[string]factType)
	for kind, names := range map[string]string{"string": text, "integer": integers, "boolean": booleans} {
		for _, name := range strings.Fields(names) {
			result[name] = factType{kind: kind}
		}
	}
	return result
}

func decodeResponseFacts(data []byte, types map[string]factType) *responseFacts {
	facts := &responseFacts{facts: make(map[string]ResponseFact)}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields) // The established decoder already checks JSON.
	resource := facts.collect(fields, types, "")
	encoded, _ := json.Marshal(resource)
	facts.resource = string(encoded)
	return facts
}

func (f *responseFacts) collect(fields map[string]json.RawMessage, types map[string]factType, prefix string) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage)
	var names []string
	for name := range types {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, present := fields[name]
		if !present {
			continue
		}
		raw = bytes.TrimSpace(raw)
		path := prefix + name
		typeOf := types[name]
		fact := ResponseFact{State: ResponseValue, raw: string(raw)}
		if bytes.Equal(raw, []byte("null")) {
			fact.State = ResponseNull
		} else if !matchesFactType(raw, typeOf.kind) {
			fact.State = ResponseInvalid
			fact.raw = ""
			f.issues = append(f.issues, ResponseIssue{Path: path, Expected: typeOf.kind, Actual: jsonValueType(raw)})
		} else if typeOf.members != nil {
			var nested map[string]json.RawMessage
			_ = json.Unmarshal(raw, &nested)
			cleaned := f.collect(nested, typeOf.members, path+".")
			raw, _ = json.Marshal(cleaned)
			fact.raw = string(raw)
		}
		f.facts[path] = fact
		if fact.State != ResponseInvalid {
			result[name] = raw
		}
	}
	return result
}

func jsonValueType(raw []byte) string {
	if len(raw) == 0 {
		return "missing"
	}
	switch raw[0] {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

func matchesFactType(raw []byte, kind string) bool {
	actual := jsonValueType(raw)
	switch kind {
	case "integer":
		return actual == "number" && !bytes.ContainsAny(raw, ".eE")
	case "string array":
		if actual != "array" {
			return false
		}
		var elements []json.RawMessage
		_ = json.Unmarshal(raw, &elements)
		for _, element := range elements {
			if jsonValueType(bytes.TrimSpace(element)) != "string" {
				return false
			}
		}
		return true
	default:
		return actual == kind
	}
}

func factAt(snapshot *responseFacts, path string) ResponseFact {
	if snapshot == nil {
		return ResponseFact{}
	}
	return snapshot.facts[path]
}

func (t Task) ResponseFact(path string) ResponseFact { return factAt(t.response, path) }
func (d Due) ResponseFact(path string) ResponseFact  { return factAt(d.response, path) }

func (t Task) ResponseIssues() []ResponseIssue {
	if t.response == nil {
		return nil
	}
	return append([]ResponseIssue(nil), t.response.issues...)
}

// FaithfulResource projects only returned, supported facts. It does not infer
// presence from typed fields or change the legacy Task serialization. The map is
// owned by the caller; json.Number preserves integer values without float rounding.
func (t Task) FaithfulResource() map[string]any {
	resource := make(map[string]any)
	if t.response != nil {
		decoder := json.NewDecoder(strings.NewReader(t.response.resource))
		decoder.UseNumber()
		_ = decoder.Decode(&resource)
	}
	if content, ok := t.ResponseFact("content").Text(); ok {
		resource["reference_item"] = map[string]any{
			"is_reference": strings.HasPrefix(content, "* "), "source": "content_prefix",
		}
	}
	return resource
}
