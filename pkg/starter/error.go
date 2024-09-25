package starter

import "errors"

type Error struct {
	kind internalError
	err  error
}

func (e Error) Error() string {
	return e.kind.String() + ": " + e.err.Error()
}

func (e Error) Unwrap() error {
	return e.err
}

type internalError int

const (
	errorInvalidLabel internalError = iota
)

func (i internalError) String() string {
	switch i {
	case errorInvalidLabel:
		return "invalid label"
	default:
		return "unknown error"
	}
}

var (
	ErrInvalidLabel = Error{kind: errorInvalidLabel, err: nil}
)

func NewInvalidLabel(err error) error {
	e := ErrInvalidLabel
	e.err = err
	return e
}

func (e Error) Is(target error) bool {
	var t Error
	ok := errors.As(target, &t)
	if !ok {
		return false
	}
	return e.kind == t.kind
}
