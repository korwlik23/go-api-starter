package domain

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/text/language"
)

var ErrInvalidLocaleTag = errors.New("invalid locale tag")

type Direction string

const (
	DirectionLTR Direction = "ltr"
	DirectionRTL Direction = "rtl"
)

type Locale struct {
	ID             string
	Tag            string
	Name           string
	Direction      Direction
	Enabled        bool
	UserSelectable bool
	Default        bool
	FallbackID     string
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NormalizeLocaleTag(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" ||
		strings.Contains(value, "_") ||
		len(value) > 63 {
		return "", ErrInvalidLocaleTag
	}
	tag, err := language.Parse(value)
	if err != nil || tag == language.Und {
		return "", ErrInvalidLocaleTag
	}
	normalized := strings.ToLower(tag.String())
	if normalized == "und" {
		return "", ErrInvalidLocaleTag
	}
	return normalized, nil
}
