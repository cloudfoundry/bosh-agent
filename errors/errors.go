package errors

import (
	"errors"
	"fmt"
)

type ShortenableError interface {
	error
	ShortError() string
}

type ComplexError struct {
	Delegate error
	Cause    error
}

func (e ComplexError) Error() string {
	return fmt.Sprintf("%s: %s", e.Delegate.Error(), e.Cause.Error())
}

func (e ComplexError) ShortError() string {
	var delegateMessage string
	if typedDelegate, ok := e.Delegate.(ShortenableError); ok {
		delegateMessage = typedDelegate.ShortError()
	} else {
		delegateMessage = e.Delegate.Error()
	}

	var causeMessage string
	if typedCause, ok := e.Cause.(ShortenableError); ok {
		causeMessage = typedCause.ShortError()
	} else {
		causeMessage = e.Cause.Error()
	}

	return fmt.Sprintf("%s: %s", delegateMessage, causeMessage)
}

func Error(msg string) error {
	return errors.New(msg)
}

func Errorf(msg string, args ...interface{}) error {
	return fmt.Errorf(msg, args...)
}

func WrapError(cause error, msg string) error {
	return WrapComplexError(cause, Error(msg))
}

func WrapErrorf(cause error, msg string, args ...interface{}) error {
	return WrapComplexError(cause, Errorf(msg, args...))
}

func WrapComplexError(cause, delegate error) error {
	if cause == nil {
		cause = Error("<nil cause>")
	}

	return ComplexError{
		Delegate: delegate,
		Cause:    cause,
	}
}
