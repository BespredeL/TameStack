package tools

// strParam builds a JSON-schema string property.
func strParam(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// intParam builds a JSON-schema integer property.
func intParam(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

// schemaBuilder accumulates the properties and required list of a tool's
// object parameter schema, replacing the hand-written nested map literals.
type schemaBuilder struct {
	properties map[string]any
	required   []string
}

// newSchema starts an object parameter schema.
func newSchema() *schemaBuilder {
	return &schemaBuilder{
		properties: map[string]any{},
		required:   []string{},
	}
}

// str adds a string property.
func (b *schemaBuilder) str(name, description string) *schemaBuilder {
	b.properties[name] = strParam(description)
	return b
}

// integer adds an integer property.
func (b *schemaBuilder) integer(name, description string) *schemaBuilder {
	b.properties[name] = intParam(description)
	return b
}

// req sets the required property names.
func (b *schemaBuilder) req(names ...string) *schemaBuilder {
	b.required = names
	return b
}

// build renders the JSON-schema object expected by provider function calling.
func (b *schemaBuilder) build() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": b.properties,
		"required":   b.required,
	}
}
