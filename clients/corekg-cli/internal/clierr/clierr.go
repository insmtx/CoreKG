package clierr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	ExitRuntime  = 1
	ExitUsage    = 2
	ExitAuth     = 3
	ExitNetwork  = 4
	ExitInternal = 5
	ExitConfirm  = 10
)

type Error struct {
	Code     string
	Message  string
	ExitCode int
	Hint     string
	Details  any
}

func (e *Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(code, message string) error {
	return &Error{Code: code, Message: message, ExitCode: ExitRuntime}
}

func WithDetails(code, message string, details any) error {
	return &Error{Code: code, Message: message, ExitCode: ExitRuntime, Details: details}
}

func Wrap(code string, err error) error {
	if err == nil {
		return nil
	}
	var detailer interface{ CLIErrorDetails() any }
	if errors.As(err, &detailer) {
		return WithDetails(code, err.Error(), detailer.CLIErrorDetails())
	}
	return New(code, err.Error())
}

func Usage(code, message string) error {
	return &Error{Code: code, Message: message, ExitCode: ExitUsage}
}

func Confirm(code, message string) error {
	return &Error{Code: code, Message: message, ExitCode: ExitConfirm}
}

type errorEnvelope struct {
	OK    bool       `json:"ok"`
	Error errorValue `json:"error"`
}

type errorValue struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Details any    `json:"details,omitempty"`
}

func WriteJSON(w io.Writer, err error) error {
	value := errorValue{Type: "runtime", Message: err.Error()}
	var cliError *Error
	if errors.As(err, &cliError) {
		value.Code = cliError.Code
		value.Message = cliError.Message
		value.Hint = cliError.Hint
		value.Details = cliError.Details
		switch cliError.ExitCode {
		case ExitUsage:
			value.Type = "validation"
		case ExitAuth:
			value.Type = "auth"
		case ExitNetwork:
			value.Type = "network"
		case ExitInternal:
			value.Type = "internal"
		case ExitConfirm:
			value.Type = "confirmation"
		}
	}
	return json.NewEncoder(w).Encode(errorEnvelope{OK: false, Error: value})
}

func ExitCode(err error) int {
	var cliError *Error
	if errors.As(err, &cliError) && cliError.ExitCode != 0 {
		return cliError.ExitCode
	}
	return ExitRuntime
}
