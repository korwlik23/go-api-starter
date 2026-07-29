package gorm

import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"strings"
)

var errInvalidAccountDatabaseID = errors.New("invalid account database UUID")

type databaseID string

func encodeDatabaseID(dialect, value string) (driver.Value, error) {
	decoded, canonical, err := parseDatabaseID(value)
	if err != nil {
		return nil, err
	}
	switch dialect {
	case "postgres":
		return canonical, nil
	case "mysql":
		return decoded, nil
	default:
		return nil, errInvalidAccountDatabaseID
	}
}

func (id *databaseID) Scan(source any) error {
	switch value := source.(type) {
	case string:
		_, canonical, err := parseDatabaseID(value)
		if err != nil {
			return err
		}
		*id = databaseID(canonical)
	case []byte:
		if len(value) == 16 {
			*id = databaseID(formatDatabaseID(value))
			return nil
		}
		_, canonical, err := parseDatabaseID(string(value))
		if err != nil {
			return err
		}
		*id = databaseID(canonical)
	default:
		return errInvalidAccountDatabaseID
	}
	return nil
}

func (id databaseID) String() string {
	return string(id)
}

func parseDatabaseID(value string) ([]byte, string, error) {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return nil, "", errInvalidAccountDatabaseID
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	if err != nil || len(decoded) != 16 {
		return nil, "", errInvalidAccountDatabaseID
	}
	return decoded, formatDatabaseID(decoded), nil
}

func formatDatabaseID(value []byte) string {
	encoded := hex.EncodeToString(value)
	return encoded[0:8] + "-" +
		encoded[8:12] + "-" +
		encoded[12:16] + "-" +
		encoded[16:20] + "-" +
		encoded[20:32]
}
