package provider

import "errors"

var ErrUnauthorized = errors.New("unauthorized: oauth required")
