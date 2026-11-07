package validation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorsCollectsFields(t *testing.T) {
	var collected Errors
	collected.Add("name", errors.New("is required"))
	collected.Add("ignored", nil)
	collected.Merge("admin.", &Error{Fields: []FieldError{{Field: "email", Message: "is invalid"}}})
	collected.Merge("none.", nil)

	err := collected.Err()

	var validationError *Error
	require.ErrorAs(t, err, &validationError)
	assert.Equal(t, []FieldError{
		{Field: "name", Message: "is required"},
		{Field: "admin.email", Message: "is invalid"},
	}, validationError.Fields)
	assert.Equal(t, "invalid input: name: is required; admin.email: is invalid", err.Error())
}

func TestErrIsNilWithoutProblems(t *testing.T) {
	var collected Errors
	collected.Add("name", nil)

	assert.NoError(t, collected.Err())
}
