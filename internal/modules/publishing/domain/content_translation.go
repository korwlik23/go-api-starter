package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/language"
)

var (
	ErrInvalidContentTranslation = errors.New("publishing: invalid content translation")
	canonicalSlugPattern         = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	canonicalPathPattern         = regexp.MustCompile(`^/[a-z0-9][a-z0-9/_-]*$`)
)

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockImage      BlockType = "image"
	BlockCallout    BlockType = "callout"
	BlockAnswer     BlockType = "answer"
	BlockSteps      BlockType = "steps"
	BlockComparison BlockType = "comparison"
)

type Block struct {
	Type BlockType      `json:"type"`
	Data map[string]any `json:"data"`
}

type SEOFields struct {
	Title          string
	Description    string
	CanonicalURL   string
	Robots         string
	StructuredData map[string]any
}

type GEOFields struct {
	Region   string
	Locality string
}

type AEOFields struct {
	Question string
	Answer   string
}

type ContentTranslation struct {
	ID          string
	AccountID   string
	ContentID   string
	Locale      string
	Slug        string
	Path        string
	Title       string
	Excerpt     string
	Blocks      []Block
	SEO         SEOFields
	GEO         GEOFields
	AEO         AEOFields
	Status      Status
	Version     int64
	PublishedAt *time.Time
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContentTranslationInput struct {
	ID          string
	AccountID   string
	ContentID   string
	Locale      string
	Slug        string
	Path        string
	Title       string
	Excerpt     string
	Blocks      []Block
	SEO         SEOFields
	GEO         GEOFields
	AEO         AEOFields
	Status      Status
	Version     int64
	PublishedAt *time.Time
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewContentTranslation(input ContentTranslationInput) (ContentTranslation, error) {
	locale, err := NormalizeLocale(input.Locale)
	if err != nil {
		return ContentTranslation{}, ErrInvalidContentTranslation
	}
	slug, err := NormalizeSlug(input.Slug)
	if err != nil {
		return ContentTranslation{}, ErrInvalidContentTranslation
	}
	path, err := NormalizePath(input.Path)
	if err != nil ||
		!validIdentity(input.ID) ||
		!validIdentity(input.AccountID) ||
		!validIdentity(input.ContentID) ||
		input.Version <= 0 ||
		input.Title == "" || input.Title != strings.TrimSpace(input.Title) ||
		ValidateStatusOrDefault(input.Status) != nil ||
		(!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() &&
			input.UpdatedAt.Before(input.CreatedAt)) {
		return ContentTranslation{}, ErrInvalidContentTranslation
	}
	status := input.Status
	if status == "" {
		status = StatusDraft
	}
	return ContentTranslation{
		ID:          strings.TrimSpace(input.ID),
		AccountID:   strings.TrimSpace(input.AccountID),
		ContentID:   strings.TrimSpace(input.ContentID),
		Locale:      locale,
		Slug:        slug,
		Path:        path,
		Title:       input.Title,
		Excerpt:     input.Excerpt,
		Blocks:      cloneBlocks(input.Blocks),
		SEO:         cloneSEO(input.SEO),
		GEO:         input.GEO,
		AEO:         input.AEO,
		Status:      status,
		Version:     input.Version,
		PublishedAt: cloneTime(input.PublishedAt),
		DeletedAt:   cloneTime(input.DeletedAt),
		CreatedAt:   input.CreatedAt.UTC(),
		UpdatedAt:   input.UpdatedAt.UTC(),
	}, nil
}

func NormalizeLocale(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "_") || len(value) > 255 {
		return "", ErrInvalidContentTranslation
	}
	tag, err := language.Parse(value)
	if err != nil || tag == language.Und {
		return "", ErrInvalidContentTranslation
	}
	result := strings.ToLower(tag.String())
	if result == "und" {
		return "", ErrInvalidContentTranslation
	}
	return result, nil
}

func NormalizeSlug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 0 || len(value) > 160 || !canonicalSlugPattern.MatchString(value) {
		return "", ErrInvalidContentTranslation
	}
	return value, nil
}

func NormalizeContentKey(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 0 || len(value) > 128 || !canonicalSlugPattern.MatchString(value) {
		return "", ErrInvalidContentTranslation
	}
	return value, nil
}

func NormalizePath(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "/" {
		return value, nil
	}
	if len(value) == 0 || len(value) > 512 ||
		strings.Contains(value, "//") || !canonicalPathPattern.MatchString(value) {
		return "", ErrInvalidContentTranslation
	}
	return value, nil
}

func ValidateStatusOrDefault(status Status) error {
	if status == "" {
		return nil
	}
	return ValidateStatus(status)
}

func cloneBlocks(blocks []Block) []Block {
	if blocks == nil {
		return nil
	}
	result := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		data := make(map[string]any, len(block.Data))
		for key, value := range block.Data {
			data[key] = value
		}
		result = append(result, Block{Type: block.Type, Data: data})
	}
	return result
}

func cloneSEO(fields SEOFields) SEOFields {
	fields.StructuredData = cloneMap(fields.StructuredData)
	return fields
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
