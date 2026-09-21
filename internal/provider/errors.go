package provider

import (
	"fmt"
)

// ProviderError is the base error type for all provider failures.
type ProviderError struct {
	Provider string
	Op       string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s provider error (%s): %v", e.Provider, e.Op, e.Err)
	}
	return fmt.Sprintf("%s provider error (%s)", e.Provider, e.Op)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ModelResponseError indicates a malformed or unexpected model response.
type ModelResponseError struct {
	Provider string
	Raw      string
}

func (e *ModelResponseError) Error() string {
	return fmt.Sprintf("%s: unexpected model response: %s", e.Provider, e.Raw)
}

// NetworkError indicates a transient network or connectivity issue.
type NetworkError struct {
	Provider string
	Op       string
	Err      error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("%s network error (%s): %v", e.Provider, e.Op, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// ValidationError indicates invalid input or configuration.
type ValidationError struct {
	Provider string
	Field    string
	Msg      string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s validation error: %s: %s", e.Provider, e.Field, e.Msg)
}