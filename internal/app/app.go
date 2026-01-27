package app

import (
	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/router"
	"devorch/internal/session"
	"devorch/internal/storage/sqlite"
)

type Deps struct {
	DB               *sqlite.DB
	ProviderRegistry *provider.Registry
	OkAONStore       *okaon.Store
	Router           *router.Router
	SessionCaller    *session.Caller
}
