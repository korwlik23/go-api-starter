package databaseid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeSupportsPostgresAndMySQLUUIDStorage(t *testing.T) {
	id := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	postgres, err := Encode("postgres", id)
	require.NoError(t, err)
	require.Equal(t, id, postgres)
	mysql, err := Encode("mysql", id)
	require.NoError(t, err)
	require.Len(t, mysql, 16)

	var scanned ID
	require.NoError(t, scanned.Scan(mysql))
	require.Equal(t, id, scanned.String())
}
