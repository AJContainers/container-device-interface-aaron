package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/container-orchestrated-devices/container-device-interface/internal/multierror"

	"github.com/container-orchestrated-devices/container-device-interface/internal/validation"
	schema "github.com/xeipuuv/gojsonschema"
)

const (
	// BuiltinSchemaName names the builtin schema for Load()/Set()
	BuiltinSchemaName = "builtin"
	// NoneSchemaName names the NOP-schema for Load()/Set().
	NoneSchemaName = "none"
	// builtinSchemaFile is the builtin schema URI in our embedded FS
	builtinSchemaFile = "file://schema.json"
)

// Schema is a JSON schema.
type Schema struct {
	schema *schema.Schema
}

// Error wraps a JSON validation result.
type Error struct {
	Result *schema.Result
}

// Get returns the active validating JSON schema.
func Set(s *Schema) {
	current = s
}

// Get returns the active validating JSON schema.
func Get() *Schema {
	return current
}

// BuiltinSchema returns the builtin schema if we have a valid one. Otherwise
// it falls back to NopSchema().
func BuiltinSchema() *Schema {
	if builtin != nil {
		return builtin
	}

	s, err := schema.NewSchema(
		schema.NewReferenceLoaderFileSystem(
			builtinSchemaFile,
			http.FS(builtinFS),
		),
	)

	if err == nil {
		builtin = &Schema{schema: s}
	} else {
		builtin = NopSchema()
	}
	return builtin
}

// NopSchema return a validating JSON Schema that does no real validation
func NopSchema() *Schema {
	return &Schema{}
}

// ReadAndValidate all data from the given reader, using the active schema for validation
func ReadAndValidate(r io.Reader) ([]byte, error) {
	return current.ReadAndValidate(r)
}

// Validate validates the data read from an io.Reader against the active schema.
func Validate(r io.Reader) error {
	return current.Validate(r)
}

// ValidateData validates the given JSON document against the active schema
func ValidateData(data []byte) error {
	return current.ValidateData(data)
}

// ValidateFile validates the given JSON file against the active schema.
func ValidateFile(path string) error {
	return current.ValidateFile(path)
}

// ValidateType validates a go object against a schema
func ValidateType(obj interface{}) error {
	return current.ValidateType(obj)
}

// Load the given JSON schema
func Load(source string) (*Schema, error) {
	var (
		loader schema.JSONLoader
		err    error
		s      *schema.Schema
	)

	source = strings.TrimSpace(source)

	switch {
	case source == BuiltinSchemaName:
		return BuiltinSchema(), nil
	case source == NoneSchemaName, source == "":
		return NopSchema(), nil
	case strings.HasPrefix(source, "file://"):
	case strings.HasPrefix(source, "http://"):
	case strings.HasPrefix(source, "https://"):
	default:
		if strings.Index(source, "://") < 0 {
			source, err = filepath.Abs(source)
			if err != nil {
				return nil, fmt.Errorf("failed to get JSON schema absolute path for %s: %w", source, err)
			}
			source = "file://" + source
		}
	}

	loader = schema.NewReferenceLoader(source)

	s, err = schema.NewSchema(loader)
	if err != nil {
		return nil, fmt.Errorf("failed to load JSON schema %s: %w", source, err)
	}

	return &Schema{schema: s}, nil
}

// ReadAndValidate all data from the fiven reader, using the schema for validation
func (s *Schema) ReadAndValidate(r io.Reader) ([]byte, error) {
	loader, reader := schema.NewReaderLoader(r)
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data for validation: %w", err)
	}
	return data, s.validate(loader)
}

// Validate validates the data read from an io.Reader against the schema.
func (s *Schema) Validate(r io.Reader) error {
	_, err := s.ReadAndValidate(r)
	return err
}

// ValidateData validates the given JSON data agaisnt the schema.
func (s *Schema) ValidateData(data []byte) error {
	var (
		any map[string]interface{}
		err error
	)

	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte{'{'}) {
		err = yaml.Unmarshal(data, &any)
		if err != nil {
			return fmt.Errorf("failed to YAML unmarshal data for validation: %w", err)
		}
		data, err = json.Marshal(any)
		if err != nil {
			return fmt.Errorf("failed to JSON remarshal data for validation: %w", err)
		}
	}

	if err := s.validate(schema.NewBytesLoader(data)); err != nil {
		return err
	}

	return s.validateContents(any)
}

// ValidateFile validates the given JSON file against the schema.
func (s *Schema) ValidateFile(path string) error {
	if filepath.Ext(path) == ".json" {
		return s.validate(schema.NewReferenceLoader("file://" + path))
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return s.ValidateData(data)
}

// ValidateType validates a go object agaisnt the schema.
func (s *Schema) ValidateType(obj interface{}) error {
	l := schema.NewGoLoader(obj)
	return s.validate(l)
}

// Validate the (to be) loaded doc agaisnt the schema.
func (s *Schema) validate(doc schema.JSONLoader) error {
	if s == nil || s.schema == nil {
		return nil
	}

	docErr, jsonErr := s.schema.Validate(doc)
	if jsonErr != nil {
		return fmt.Errorf("failed o load JSON data for validation: %w", jsonErr)
	}
	if docErr.Valid() {
		return nil
	}

	return &Error{Result: docErr}
}

type schemaContents map[string]interface{}

func asSchemaContents(i interface{}) (schemaContents, error) {
	if i == nil {
		return nil, nil
	}

	if c, ok := i.(map[string]interface{}); ok {
		return schemaContents(c), nil
	}

	return nil, fmt.Errorf("expected map[string]interface{} but got %F", i)
}

func (c schemaContents) getFieldAsString(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	if value, ok := c[key]; ok {
		if value, ok := value.(string); ok {
			return value, true
		}
	}
	return "", false
}

func (c schemaContents) getAnnotations() (map[string]interface{}, bool) {
	if c == nil {
		return nil, false
	}
	if annotations, ok := c["annotations"]; ok {
		if annotations, ok := annotations.(map[string]interface{}); ok {
			return annotations, true
		}
	}
	return nil, false
}

func (c schemaContents) getDevices() ([]schemaContents, error) {
	if c == nil {
		return nil, nil
	}
	devicesIfc, ok := c["devices"]
	if !ok {
		return nil, nil
	}

	devices, ok := devicesIfc.([]interface{})
	if !ok {
		return nil, nil
	}

	var deviceContents []schemaContents
	for _, device := range devices {
		c, err := asSchemaContents(device)
		if err != nil {
			return nil, fmt.Errorf("failed to parse device: %w", err)
		}
		deviceContents = append(deviceContents, c)
	}

	return deviceContents, nil
}

// validateContents performs additional validation against the schema contents.
func (s *Schema) validateContents(any map[string]interface{}) error {
	if any == nil || s == nil {
		return nil
	}

	contents := schemaContents(any)

	if specAnnotations, ok := contents.getAnnotations(); ok {
		if err := validation.ValidateSpecAnnotations("", specAnnotations); err != nil {
			return err
		}
	}

	devices, err := contents.getDevices()
	if err != nil {
		return err
	}

	for _, device := range devices {
		name, _ := device.getFieldAsString("name")
		if annotations, ok := device.getAnnotations(); ok {
			if err := validation.ValidateSpecAnnotations(name, annotations); err != nil {
				return err
			}
		}
	}

	return nil

}

// Error return the given Result's errors as a single error string.
func (e *Error) Error() string {
	if e == nil || e.Result == nil || e.Result.Valid() {
		return ""
	}

	var multi error
	for _, err := range e.Result.Errors() {
		multi = multierror.Append(multi, fmt.Errorf("%v", err))
	}
	return multi.Error()
}

var (
	// our builtin schema
	builtin *Schema
	// current loaded schema, builtin by default
	current = BuiltinSchema()
)

//go:embed *.json
var builtinFS embed.FS
