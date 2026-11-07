// Package validation provides the error type every feature returns for
// invalid user input, so the API can report each bad field the same way.
//
// Build an [Errors] with [Errors.Add] while checking fields, then return
// [Errors.Err], which is nil when nothing was wrong.
package validation

import "strings"

// FieldError describes one invalid input field, in words a user can act on.
type FieldError struct {
	// Field is the field's path in the request, for example "admin.email".
	Field string
	// Message says what is wrong, for example "must be 1–32 characters".
	Message string
}

// Error lists every invalid field of a request.
type Error struct {
	Fields []FieldError
}

func (validationError *Error) Error() string {
	messages := make([]string, 0, len(validationError.Fields))
	for _, field := range validationError.Fields {
		messages = append(messages, field.Field+": "+field.Message)
	}
	return "invalid input: " + strings.Join(messages, "; ")
}

// Errors collects field problems while a request is checked.
type Errors struct {
	fields []FieldError
}

// Add records err for field. A nil err is ignored, so callers can write
// errors.Add("name", checkName(name)) for every field.
func (errors *Errors) Add(field string, err error) {
	if err != nil {
		errors.fields = append(errors.fields, FieldError{Field: field, Message: err.Error()})
	}
}

// Merge adds every field of other, prefixing each field name, for example
// turning "email" into "admin.email". A nil other is ignored.
func (errors *Errors) Merge(prefix string, other *Error) {
	if other == nil {
		return
	}
	for _, field := range other.Fields {
		errors.fields = append(errors.fields, FieldError{Field: prefix + field.Field, Message: field.Message})
	}
}

// Err returns an *Error listing the recorded problems, or nil if there were
// none.
func (errors *Errors) Err() error {
	if len(errors.fields) == 0 {
		return nil
	}
	return &Error{Fields: errors.fields}
}
