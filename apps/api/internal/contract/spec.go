// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package contract loads the OpenAPI document (docs/api/openapi.yaml) and exposes the
// operations it declares. It is used by repository tooling (the authorization matrix)
// and by test harnesses. The API server does not depend on it.
package contract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SpecRelPath is the location of the contract relative to the repository root.
const SpecRelPath = "docs/api/openapi.yaml"

// Methods lists the HTTP methods used by the contract, in document order of precedence.
var Methods = []string{"get", "put", "post", "patch", "delete"}

// Document is the subset of an OpenAPI 3.1 document that the tooling needs.
type Document struct {
	// Operations are in document order.
	Operations []Operation
	// GlobalSecurity is the top-level security requirement list.
	GlobalSecurity []SecurityRequirement
	Schemas        map[string]*Schema
	parameters     map[string]Parameter
}

// SecurityRequirement maps a security scheme name to its scopes. All schemes in one
// requirement are required together; the requirements of an operation are alternatives.
type SecurityRequirement map[string][]string

// Operation is one HTTP method on one path.
type Operation struct {
	ID     string
	Method string // upper case
	Path   string // as written in the document, e.g. /dormitories/{dormitoryId}
	Tags   []string
	// Security is the effective security: the operation's own list, or the global one.
	// An empty, non-nil slice means the operation is public.
	Security     []SecurityRequirement
	RequiredRole string
	APIKeyScopes []string
	RateLimit    string
	Parameters   []Parameter
	// BodySchema is the request body schema (resolved), or nil.
	BodySchema *Schema
}

// Parameter is a path, query or header parameter with its schema resolved.
type Parameter struct {
	Ref      string  `yaml:"$ref"`
	Name     string  `yaml:"name"`
	In       string  `yaml:"in"`
	Required bool    `yaml:"required"`
	Schema   *Schema `yaml:"schema"`
}

// Schema is the subset of JSON Schema used by the tooling.
type Schema struct {
	Ref                  string             `yaml:"$ref"`
	Type                 TypeList           `yaml:"type"`
	Format               string             `yaml:"format"`
	Enum                 []any              `yaml:"enum"`
	Const                any                `yaml:"const"`
	Properties           map[string]*Schema `yaml:"properties"`
	Required             []string           `yaml:"required"`
	Items                *Schema            `yaml:"items"`
	OneOf                []*Schema          `yaml:"oneOf"`
	AnyOf                []*Schema          `yaml:"anyOf"`
	AllOf                []*Schema          `yaml:"allOf"`
	MinLength            *int               `yaml:"minLength"`
	MaxLength            *int               `yaml:"maxLength"`
	Minimum              *float64           `yaml:"minimum"`
	Maximum              *float64           `yaml:"maximum"`
	Pattern              string             `yaml:"pattern"`
	AdditionalProperties any                `yaml:"additionalProperties"`
}

// TypeList accepts both `type: string` and `type: [string, 'null']`.
type TypeList []string

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *TypeList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*t = TypeList{node.Value}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*t = list
		return nil
	default:
		return fmt.Errorf("line %d: unsupported type node", node.Line)
	}
}

// Has reports whether the list contains name.
func (t TypeList) Has(name string) bool {
	for _, v := range t {
		if v == name {
			return true
		}
	}
	return false
}

type rawDocument struct {
	Security   []SecurityRequirement `yaml:"security"`
	Paths      yaml.Node             `yaml:"paths"`
	Components struct {
		Parameters map[string]Parameter `yaml:"parameters"`
		Schemas    map[string]*Schema   `yaml:"schemas"`
	} `yaml:"components"`
}

type rawOperation struct {
	OperationID  string                 `yaml:"operationId"`
	Tags         []string               `yaml:"tags"`
	Security     *[]SecurityRequirement `yaml:"security"`
	Parameters   []Parameter            `yaml:"parameters"`
	RequiredRole string                 `yaml:"x-required-role"`
	APIKeyScopes []string               `yaml:"x-api-key-scopes"`
	RateLimit    string                 `yaml:"x-rate-limit"`
	RequestBody  *struct {
		Content map[string]struct {
			Schema *Schema `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"requestBody"`
}

// FindSpec walks up from dir until it finds docs/api/openapi.yaml and returns its path.
func FindSpec(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(abs, filepath.FromSlash(SpecRelPath))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", errors.New("contract: " + SpecRelPath + " not found in any parent directory")
		}
		abs = parent
	}
}

// Load reads and parses the OpenAPI document at path.
func Load(path string) (*Document, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("contract: read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses an OpenAPI document.
func Parse(data []byte) (*Document, error) {
	var raw rawDocument
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("contract: parse: %w", err)
	}
	doc := &Document{
		GlobalSecurity: raw.Security,
		Schemas:        raw.Components.Schemas,
		parameters:     raw.Components.Parameters,
	}
	if raw.Paths.Kind != yaml.MappingNode {
		return nil, errors.New("contract: paths is missing or not a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i+1 < len(raw.Paths.Content); i += 2 {
		path := raw.Paths.Content[i].Value
		item := raw.Paths.Content[i+1]
		var common struct {
			Parameters []Parameter `yaml:"parameters"`
		}
		if err := item.Decode(&common); err != nil {
			return nil, fmt.Errorf("contract: path %s: %w", path, err)
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			method := item.Content[j].Value
			if !isMethod(method) {
				continue
			}
			var ro rawOperation
			if err := item.Content[j+1].Decode(&ro); err != nil {
				return nil, fmt.Errorf("contract: %s %s: %w", method, path, err)
			}
			if ro.OperationID == "" {
				return nil, fmt.Errorf("contract: %s %s has no operationId", method, path)
			}
			if seen[ro.OperationID] {
				return nil, fmt.Errorf("contract: duplicate operationId %s", ro.OperationID)
			}
			seen[ro.OperationID] = true
			op, err := doc.buildOperation(path, method, common.Parameters, ro)
			if err != nil {
				return nil, err
			}
			doc.Operations = append(doc.Operations, op)
		}
	}
	return doc, nil
}

func isMethod(key string) bool {
	for _, m := range Methods {
		if key == m {
			return true
		}
	}
	return false
}

func (d *Document) buildOperation(path, method string, common []Parameter, ro rawOperation) (Operation, error) {
	op := Operation{
		ID:           ro.OperationID,
		Method:       strings.ToUpper(method),
		Path:         path,
		Tags:         ro.Tags,
		RequiredRole: ro.RequiredRole,
		APIKeyScopes: ro.APIKeyScopes,
		RateLimit:    ro.RateLimit,
		Security:     d.GlobalSecurity,
	}
	if ro.Security != nil {
		op.Security = *ro.Security
		if op.Security == nil {
			op.Security = []SecurityRequirement{}
		}
	}
	for _, p := range append(append([]Parameter{}, common...), ro.Parameters...) {
		resolved, err := d.resolveParameter(p)
		if err != nil {
			return Operation{}, fmt.Errorf("contract: %s: %w", ro.OperationID, err)
		}
		op.Parameters = append(op.Parameters, resolved)
	}
	if ro.RequestBody != nil {
		if media, ok := ro.RequestBody.Content["application/json"]; ok && media.Schema != nil {
			op.BodySchema = d.Resolve(media.Schema)
		}
	}
	return op, nil
}

func (d *Document) resolveParameter(p Parameter) (Parameter, error) {
	if p.Ref == "" {
		p.Schema = d.Resolve(p.Schema)
		return p, nil
	}
	const prefix = "#/components/parameters/"
	if !strings.HasPrefix(p.Ref, prefix) {
		return Parameter{}, fmt.Errorf("unsupported parameter reference %q", p.Ref)
	}
	target, ok := d.parameters[strings.TrimPrefix(p.Ref, prefix)]
	if !ok {
		return Parameter{}, fmt.Errorf("unknown parameter reference %q", p.Ref)
	}
	target.Schema = d.Resolve(target.Schema)
	return target, nil
}

// Resolve follows a chain of #/components/schemas references and returns the target.
// It returns s unchanged when s has no reference, and nil for unknown references.
func (d *Document) Resolve(s *Schema) *Schema {
	const prefix = "#/components/schemas/"
	for depth := 0; s != nil && s.Ref != "" && depth < 32; depth++ {
		if !strings.HasPrefix(s.Ref, prefix) {
			return nil
		}
		s = d.Schemas[strings.TrimPrefix(s.Ref, prefix)]
	}
	return s
}

// HasScheme reports whether any security requirement of the operation uses scheme.
func (o Operation) HasScheme(scheme string) bool {
	for _, req := range o.Security {
		if _, ok := req[scheme]; ok {
			return true
		}
	}
	return false
}

// IsPublic reports whether the operation needs no credentials.
func (o Operation) IsPublic() bool { return len(o.Security) == 0 }

// PathParams returns the names of the path parameters in path order.
func (o Operation) PathParams() []string {
	var names []string
	rest := o.Path
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			return names
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			return names
		}
		names = append(names, rest[start+1:start+end])
		rest = rest[start+end+1:]
	}
}
