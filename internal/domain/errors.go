package domain

import (
	"fmt"
	"strings"
)

const (
	blankFieldErrMsg = "can't be blank"
	// DuplicateErrMsg is the human-readable message used when a unique constraint is violated.
	DuplicateErrMsg = "has already been taken"
)

// ValidationError represents a domain validation failure for a specific field.
type ValidationError struct {
	Field  string
	Errors []string
}

// Error implements the error interface for ValidationError.
func (v *ValidationError) Error() string {
	return fmt.Sprintf("field %s, errors %s", v.Field, strings.Join(v.Errors, ","))
}

// NewValidationError creates a ValidationError for the given field with a single error message.
func NewValidationError(field string, err string) *ValidationError {
	return &ValidationError{
		Field:  field,
		Errors: []string{err},
	}
}

// DuplicateError represents a unique-constraint violation for a specific field.
type DuplicateError struct {
	Field string
	Msg   string
}

// Error implements the error interface for DuplicateError.
func (d *DuplicateError) Error() string {
	return fmt.Sprintf("field %s: %s", d.Field, d.Msg)
}

// NewDuplicateError creates a DuplicateError for the given field using the standard duplicate message.
func NewDuplicateError(field string) *DuplicateError {
	return &DuplicateError{
		Field: field,
		Msg:   DuplicateErrMsg,
	}
}

// CredentialsError indicates that the supplied authentication credentials are invalid.
type CredentialsError struct{}

// Error implements the error interface for CredentialsError.
func (c *CredentialsError) Error() string {
	return "invalid credentials"
}

// ProfileNotFoundError indicates that no user profile matched the requested username.
type ProfileNotFoundError struct{}

// Error implements the error interface for ProfileNotFoundError.
func (p *ProfileNotFoundError) Error() string {
	return "profile not found"
}

// ArticleNotFoundError indicates that no article matched the requested slug.
type ArticleNotFoundError struct{}

// Error implements the error interface for ArticleNotFoundError.
func (a *ArticleNotFoundError) Error() string {
	return "article not found"
}

// CommentNotFoundError indicates that no comment matched the requested identifier.
type CommentNotFoundError struct{}

// Error implements the error interface for CommentNotFoundError.
func (c *CommentNotFoundError) Error() string {
	return "comment not found"
}

// ForbiddenError indicates that the caller does not have permission to perform the operation.
type ForbiddenError struct{}

// Error implements the error interface for ForbiddenError.
func (f *ForbiddenError) Error() string {
	return "forbidden"
}
