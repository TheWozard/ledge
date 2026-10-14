// Package schema checks JSON against a set of draft 2020-12 JSON Schemas, and reports
// the one error that best explains why a value was rejected.
package schema

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const suffix = ".schema.json"

var printer = message.NewPrinter(language.English)

// Set is the schemas in one directory, by name ("event" for event.schema.json). A
// schema can $ref another in the set by its $id.
type Set struct {
	sources  map[string][]byte
	compiled map[string]*jsonschema.Schema
	docs     map[string]any // parsed schemas by $id, for reading descriptions
}

// Error is a value a schema rejected: where in the value, and why.
type Error struct {
	Path    []string
	Message string
}

func (e *Error) Error() string {
	if len(e.Path) == 0 {
		return e.Message
	}
	return strings.Join(e.Path, "/") + ": " + e.Message
}

// Load compiles every *.schema.json at the top of fsys. Each needs an $id.
func Load(fsys fs.FS) (*Set, error) {
	names, err := fs.Glob(fsys, "*"+suffix)
	if err != nil {
		return nil, err
	}
	set := &Set{sources: map[string][]byte{}, compiled: map[string]*jsonschema.Schema{}, docs: map[string]any{}}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	ids := map[string]string{}
	for _, file := range names {
		source, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(source))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		obj, _ := doc.(map[string]any)
		id, _ := obj["$id"].(string)
		if id == "" {
			return nil, fmt.Errorf("%s: no $id", file)
		}
		if err := compiler.AddResource(id, doc); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		name := strings.TrimSuffix(path.Base(file), suffix)
		set.sources[name] = source
		set.docs[id] = doc
		ids[name] = id
	}
	for name, id := range ids {
		compiled, err := compiler.Compile(id)
		if err != nil {
			return nil, fmt.Errorf("%s%s: %w", name, suffix, err)
		}
		set.compiled[name] = compiled
	}
	return set, nil
}

// Names lists the schemas, sorted.
func (s *Set) Names() []string {
	names := make([]string, 0, len(s.sources))
	for name := range s.sources {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Source is the schema file as written, or false if there's no schema by that name.
func (s *Set) Source(name string) ([]byte, bool) {
	source, ok := s.sources[name]
	return source, ok
}

// Decode parses JSON the way the schemas expect it: numbers stay json.Number, and
// anything after the value is an error.
func Decode(data []byte) (any, error) {
	return jsonschema.UnmarshalJSON(bytes.NewReader(data))
}

// Validate checks a decoded value against the named schema. A rejected value gives
// an *Error.
func (s *Set) Validate(name string, value any) error {
	compiled, ok := s.compiled[name]
	if !ok {
		return fmt.Errorf("no schema %q", name)
	}
	err := compiled.Validate(value)
	var invalid *jsonschema.ValidationError
	if !errors.As(err, &invalid) {
		return err
	}
	cause := best(invalid)
	message := cause.ErrorKind.LocalizedString(printer)
	// A failed `not` only says the value shouldn't have matched. The schemas give
	// those rules a description to report instead.
	if _, ok := cause.ErrorKind.(*kind.Not); ok {
		if text := s.notDescription(cause.SchemaURL); text != "" {
			message = text
		}
	}
	return &Error{Path: cause.InstanceLocation, Message: message}
}

// best is the error to report: of the innermost errors, the one nearest the top of
// the value, since that's where the most is wrong. Under an anyOf or oneOf it's the
// one deepest in the value instead, from the alternative that matched furthest
// before failing. Ties go to the first.
func best(err *jsonschema.ValidationError) *jsonschema.ValidationError {
	if len(err.Causes) == 0 {
		return err
	}
	deepest := false
	switch err.ErrorKind.(type) {
	case *kind.AnyOf, *kind.OneOf:
		deepest = true
	}
	var found *jsonschema.ValidationError
	for _, cause := range err.Causes {
		leaf := best(cause)
		if found == nil ||
			(!deepest && len(leaf.InstanceLocation) < len(found.InstanceLocation)) ||
			(deepest && len(leaf.InstanceLocation) > len(found.InstanceLocation)) {
			found = leaf
		}
	}
	return found
}

// notDescription is the description of the `not` in the schema at location
// ("<$id>#<json pointer>").
func (s *Set) notDescription(location string) string {
	id, pointer, _ := strings.Cut(location, "#")
	node := s.docs[id]
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if token == "" {
			continue
		}
		token = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
		switch value := node.(type) {
		case map[string]any:
			node = value[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(value) {
				return ""
			}
			node = value[index]
		default:
			return ""
		}
	}
	obj, _ := node.(map[string]any)
	not, _ := obj["not"].(map[string]any)
	text, _ := not["description"].(string)
	return text
}
