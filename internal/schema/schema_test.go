package schema

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// thing has a required integer, a nested object, a `not` with a description, and no
// other properties; things is a list of them, by $ref.
var schemas = fstest.MapFS{
	"thing.schema.json": {Data: []byte(`{
		"$id": "urn:test:thing",
		"type": "object",
		"properties": {
			"n": {"type": "integer"},
			"inner": {"type": "object", "properties": {"x": {"type": "string"}}, "required": ["x"]},
			"bad": true
		},
		"required": ["n"],
		"not": {"required": ["bad"], "description": "bad isn't allowed"},
		"unevaluatedProperties": false
	}`)},
	"things.schema.json": {Data: []byte(`{"$id": "urn:test:things", "type": "array", "items": {"$ref": "urn:test:thing"}}`)},
}

func TestValidate(t *testing.T) {
	set, err := Load(schemas)
	if err != nil {
		t.Fatal(err)
	}
	if names := set.Names(); !slices.Equal(names, []string{"thing", "things"}) {
		t.Errorf("Names = %v", names)
	}
	if source, ok := set.Source("thing"); !ok || !strings.Contains(string(source), "urn:test:thing") {
		t.Errorf("Source(thing) = %.40s, %v", source, ok)
	}

	for _, c := range []struct {
		schema, value string
		path          string // "" for the top
		message       string // "" for valid
	}{
		{"thing", `{"n": 1, "inner": {"x": "y"}}`, "", ""},
		{"thing", `{}`, "", "missing property 'n'"},
		{"thing", `{"n": 1.5}`, "n", "got number, want integer"},
		{"thing", `{"n": 1, "inner": {"x": 2}}`, "inner/x", "got number, want string"},
		// The innermost error nearest the top: what's missing from the thing itself,
		// not what's wrong deeper in it.
		{"thing", `{"inner": {"x": 2}}`, "", "missing property 'n'"},
		// An unknown property fails `unevaluatedProperties` as a false schema.
		{"thing", `{"n": 1, "extra": 1}`, "extra", "not a property this object can have"},
		// A failed `not` reports its description, rather than the property it leaves
		// unevaluated.
		{"thing", `{"n": 1, "bad": 1}`, "", "bad isn't allowed"},
		// Across a $ref to another schema in the set.
		{"things", `[{"n": 1}, {"n": "x"}]`, "1/n", "got string, want integer"},
	} {
		value, err := Decode([]byte(c.value))
		if err != nil {
			t.Fatal(err)
		}
		err = set.Validate(c.schema, value)
		if c.message == "" {
			if err != nil {
				t.Errorf("%s %s: %v", c.schema, c.value, err)
			}
			continue
		}
		invalid, ok := err.(*Error)
		if !ok || strings.Join(invalid.Path, "/") != c.path || !strings.Contains(invalid.Message, c.message) {
			t.Errorf("%s %s: %#v; want %q at %q", c.schema, c.value, err, c.message, c.path)
		}
	}

	if err := set.Validate("nope", map[string]any{}); err == nil || !strings.Contains(err.Error(), `no schema "nope"`) {
		t.Errorf("an unknown schema = %v", err)
	}
}

func TestLoadNeedsAnID(t *testing.T) {
	_, err := Load(fstest.MapFS{"x.schema.json": {Data: []byte(`{"type": "object"}`)}})
	if err == nil || !strings.Contains(err.Error(), "x.schema.json: no $id") {
		t.Errorf("Load = %v", err)
	}
}

func TestDecode(t *testing.T) {
	if _, err := Decode([]byte(`{} {}`)); err == nil {
		t.Error("trailing data should be an error")
	}
	value, err := Decode([]byte(`{"n": 1.50}`))
	if err != nil || value.(map[string]any)["n"].(interface{ String() string }).String() != "1.50" {
		t.Errorf("Decode = %#v, %v; want the number kept as written", value, err)
	}
}
