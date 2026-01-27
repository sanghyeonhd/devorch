package provider

import "fmt"

type Error struct {
	Code    string
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func Err(code, msg string) error {
	return Error{Code: code, Message: msg}
}

var ErrProviderNotFound = Error{Code: "provider_not_found", Message: "provider not found"}
