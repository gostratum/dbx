package dbx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDSN_WithFullDSN(t *testing.T) {
	dc := &DatabaseConfig{DSN: "postgres://user:pass@db.example:5432/appdb?sslmode=require"}
	got := dc.GetDSN()
	require.Equal(t, "postgres://user:pass@db.example:5432/appdb?sslmode=require", got)
}

func TestGetDSN_BuildFromComponents(t *testing.T) {
	dc := &DatabaseConfig{
		User:     "alice",
		Password: "secret",
		Host:     "db.local",
		Port:     5433,
		DBName:   "mydb",
		SSLMode:  "disable",
	}
	got := dc.GetDSN()
	// order and formatting must match BuildDSN
	require.Equal(t, "postgres://alice:secret@db.local:5433/mydb?sslmode=disable", got)
}

func TestGetDSN_MinimalDBNameOnly(t *testing.T) {
	dc := &DatabaseConfig{
		DBName: "onlydb",
	}
	got := dc.GetDSN()
	require.Equal(t, "postgres://localhost:5432/onlydb?sslmode=disable", got)
}
