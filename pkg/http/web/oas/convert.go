package oas

import (
	"reflect"
	"strings"
)

const (
	schemaTypeString = "string"
	schemaTypeBool   = "boolean"
	schemaTypeInt    = "integer"
	schemaTypeNumber = "number"
	schemaTypeObject = "object"
	schemaTypeArray  = "array"

	formatInt32    = "int32"
	formatInt64    = "int64"
	formatFloat    = "float"
	formatDateTime = "date-time"
)

// Schema represents an OpenAPI Schema Object
//
// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.3.md#schema-object
type Schema struct {
	Type                 string            `json:"type"`
	Required             []string          `json:"required,omitempty"`
	Description          string            `json:"description,omitempty"`
	Format               string            `json:"format,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	AdditionalProperties *bool             `json:"additionalProperties,omitempty"`
	Example              interface{}       `json:"example,omitempty"`
}

func parseDeep(v reflect.Value, name, tag string, out map[string]Schema) map[string]Schema {
	switch v.Kind() {
	case reflect.Ptr:
		if !v.IsNil() {
			return parseDeep(v.Elem(), name, tag, out)
		}
		return parseDeep(reflect.New(v.Type().Elem()), name, tag, out)
	case reflect.String:
		out[name] = Schema{Type: schemaTypeString}
	case reflect.Bool:
		out[name] = Schema{Type: schemaTypeBool}
	case reflect.Int, reflect.Int8, reflect.Int16:
		out[name] = Schema{Type: schemaTypeInt}
	case reflect.Int32:
		out[name] = Schema{Type: schemaTypeInt, Format: formatInt32}
	case reflect.Int64:
		out[name] = Schema{Type: schemaTypeInt, Format: formatInt64}
	case reflect.Float32, reflect.Float64:
		out[name] = Schema{Type: schemaTypeNumber, Format: formatFloat}
	case reflect.Struct:
		switch v.Type().String() {
		// RFC3339
		case "time.Time":
			out[name] = Schema{Type: schemaTypeString, Format: formatDateTime}
		default:
			p := Schema{Type: schemaTypeObject, Properties: map[string]Schema{}, Required: make([]string, 0)}
			for i := 0; i < v.NumField(); i++ {
				vtyp := v.Type().Field(i)
				if vtyp.Anonymous {
					parseDeep(reflect.New(vtyp.Type), "_anonymous", tag, p.Properties)
					xxx := p.Properties["_anonymous"]
					for k, v := range xxx.Properties {
						p.Properties[k] = v
					}
					p.Required = append(p.Required, xxx.Required...)
					delete(p.Properties, "_anonymous")
				} else {
					x, ok := vtyp.Tag.Lookup(tag)
					if ok {
						x = strings.TrimSpace(strings.Split(x, ",")[0])
						if x == "-" || x == "" {
							continue
						}
						p.Properties = parseDeep(v.Field(i), x, tag, p.Properties)
						// 是否有注释？
						if comment := vtyp.Tag.Get("comment"); comment != "" {
							pp := p.Properties[x]
							pp.Description = comment
							p.Properties[x] = pp
							// p.Description = comment
						}
						// 是否必填？
						if xc := strings.Split(vtyp.Tag.Get("binding"), ","); xc[0] == "required" {
							p.Required = append(p.Required, x)
						}
					}
				}

			}
			if len(p.Properties) > 0 {
				out[name] = p
			}
		}
	case reflect.Slice, reflect.Array:
		p := Schema{Type: schemaTypeArray}
		v2 := reflect.New(v.Type().Elem())
		dummy := parseDeep(v2, "dummy", tag, map[string]Schema{})
		d := dummy["dummy"]
		p.Items = &d

		out[name] = p
	case reflect.Map:
		additionalProps := true
		p := Schema{
			Type:                 schemaTypeObject,
			Properties:           map[string]Schema{},
			AdditionalProperties: &additionalProps,
		}

		v3 := reflect.New(v.Type().Elem())
		p.Properties = parseDeep(v3, "example", tag, p.Properties)
		out[name] = p

	case reflect.Interface:
		p := Schema{Type: schemaTypeObject, Properties: map[string]Schema{}, Required: make([]string, 0)}
		out[name] = p
	}

	return out
}

func Generate(input reflect.Value, tag string) map[string]Schema {
	response := map[string]Schema{}
	response = parseDeep(input, "schema", tag, response)
	return response
}
