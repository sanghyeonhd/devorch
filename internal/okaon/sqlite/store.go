package sqlite

import (
	"database/sql"
)

// Store는 SQLite 기반 OkAON 저장소
type Store struct {
	db *sql.DB
}

// NewStore는 새 Store 인스턴스 생성
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}
