package gorm

import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidDatabaseID = errors.New("invalid database UUID")

type databaseID string

func encodeDatabaseID(dialect, value string) (driver.Value, error) {
	bytes, canonical, err := parseDatabaseID(value)
	if err != nil {
		return nil, err
	}

	switch dialect {
	case "postgres":
		return canonical, nil
	case "mysql":
		return bytes, nil
	default:
		return nil, fmt.Errorf("%w: unsupported dialect", ErrInvalidDatabaseID)
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
		return nil
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
		return nil
	default:
		return ErrInvalidDatabaseID
	}
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
		return nil, "", ErrInvalidDatabaseID
	}

	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return nil, "", ErrInvalidDatabaseID
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
