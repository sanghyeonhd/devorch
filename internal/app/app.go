package app

import (
	okaonsqlite "devorch/internal/okaon/sqlite"
	"devorch/internal/provider"
	"devorch/internal/router"
	"devorch/internal/session"
	"devorch/internal/storage/sqlite"
)

type Deps struct {
	DB               *sqlite.DB
	ProviderRegistry *provider.Registry
	OkAONStore       *okaonsqlite.Store
	Router           *router.Router
	SessionCaller    *session.Caller
}
