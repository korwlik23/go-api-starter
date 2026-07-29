package databaseid

import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalid = errors.New("invalid database UUID")

type ID string

func Encode(dialect, value string) (driver.Value, error) {
	decoded, canonical, err := parse(value)
	if err != nil {
		return nil, err
	}
	switch dialect {
	case "postgres":
		return canonical, nil
	case "mysql":
		return decoded, nil
	default:
		return nil, ErrInvalid
	}
}

func (id *ID) Scan(source any) error {
	switch value := source.(type) {
	case string:
		_, canonical, err := parse(value)
		if err != nil {
			return err
		}
		*id = ID(canonical)
	case []byte:
		if len(value) == 16 {
			*id = ID(format(value))
			return nil
		}
		_, canonical, err := parse(string(value))
		if err != nil {
			return err
		}
		*id = ID(canonical)
	default:
		return ErrInvalid
	}
	return nil
}

func (id ID) String() string {
	return string(id)
}

func parse(value string) ([]byte, string, error) {
	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return nil, "", ErrInvalid
	}
	decoded, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	if err != nil || len(decoded) != 16 {
		return nil, "", ErrInvalid
	}
	return decoded, format(decoded), nil
}

func format(value []byte) string {
	encoded := hex.EncodeToString(value)
	return encoded[0:8] + "-" +
		encoded[8:12] + "-" +
		encoded[12:16] + "-" +
		encoded[16:20] + "-" +
		encoded[20:32]
}
