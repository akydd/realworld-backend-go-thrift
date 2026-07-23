package thrift

import (
	"errors"

	"realworld-backend-go/api/thrift/gen/thriftpb"
	"realworld-backend-go/internal/domain"
)

// domainErr maps domain error types to the Thrift exceptions declared in the
// IDL. Only errors that correspond to a declared exception are translated into
// a structured Thrift exception; anything else is returned unchanged and is
// serialized by the processor as a generic TApplicationException.
func domainErr(err error) error {
	var validationErr *domain.ValidationError
	var dupErr *domain.DuplicateError

	switch {
	case errors.As(err, &validationErr):
		return &thriftpb.ValidationError{
			Errors: map[string][]string{
				validationErr.Field: validationErr.Errors,
			},
		}

	case errors.As(err, &dupErr):
		return &thriftpb.ValidationError{
			Errors: map[string][]string{
				dupErr.Field: {dupErr.Msg},
			},
		}

	default:
		return err
	}
}
