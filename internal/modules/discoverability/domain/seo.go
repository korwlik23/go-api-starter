package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	RobotsIndexFollow     = "index,follow"
	RobotsNoIndexFollow   = "noindex,follow"
	RobotsIndexNoFollow   = "index,nofollow"
	RobotsNoIndexNoFollow = "noindex,nofollow"
)

var ErrInvalidSEODefaults = errors.New("discoverability: invalid SEO defaults")

type SEODefaults struct {
	ID            string
	AccountID     string
	LocaleID      string
	Title         string
	Description   string
	CanonicalBase string
	Robots        string
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SEODefaultsInput struct {
	ID            string
	AccountID     string
	LocaleID      string
	Title         string
	Description   string
	CanonicalBase string
	Robots        string
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewSEODefaults(input SEODefaultsInput) (SEODefaults, error) {
	title := strings.TrimSpace(input.Title)
	description := strings.TrimSpace(input.Description)
	canonical, err := normalizeCanonicalBase(input.CanonicalBase)
	if err != nil || title == "" || len(title) > 255 || len(description) > 320 || !validRobots(input.Robots) || input.Version < 1 || !redirectIDPattern.MatchString(input.ID) || !redirectIDPattern.MatchString(input.AccountID) || !redirectIDPattern.MatchString(input.LocaleID) || (!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return SEODefaults{}, ErrInvalidSEODefaults
	}
	return SEODefaults{ID: input.ID, AccountID: input.AccountID, LocaleID: input.LocaleID, Title: title, Description: description, CanonicalBase: canonical, Robots: input.Robots, Version: input.Version, CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC()}, nil
}

func normalizeCanonicalBase(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidSEODefaults
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path != "" && strings.Contains(path, "//") {
		return "", ErrInvalidSEODefaults
	}
	parsed.Path = path
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validRobots(value string) bool {
	switch value {
	case RobotsIndexFollow, RobotsNoIndexFollow, RobotsIndexNoFollow, RobotsNoIndexNoFollow:
		return true
	default:
		return false
	}
}
