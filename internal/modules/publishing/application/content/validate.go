package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"reflect"
	"strings"
	"unicode"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

var (
	ErrInvalidContent   = errors.New("publishing: invalid content")
	ErrInvalidBlock     = errors.New("publishing: invalid block")
	ErrInvalidBlockJSON = errors.New("publishing: invalid block JSON")
	ErrUnknownBlockType = errors.New("publishing: unknown block type")
	ErrUnsafeURL        = errors.New("publishing: unsafe URL")
	ErrNoVisibleText    = errors.New("publishing: no visible text")
	ErrInvalidLocale    = errors.New("publishing: invalid locale")
	ErrFieldTooLong     = errors.New("publishing: field too long")
)

const maxBlocksJSONBytes = 1 << 20

type ContentValidationInput struct {
	Locale  string
	Slug    string
	Path    string
	Title   string
	Excerpt string
	Blocks  []publishingdomain.Block
	SEO     publishingdomain.SEOFields
	GEO     publishingdomain.GEOFields
	AEO     publishingdomain.AEOFields
}

func Validate(input ContentValidationInput) error {
	if _, err := publishingdomain.NormalizeLocale(input.Locale); err != nil {
		return invalid(ErrInvalidLocale)
	}
	if _, err := publishingdomain.NormalizeSlug(input.Slug); err != nil {
		return invalid(publishingdomain.ErrInvalidContentTranslation)
	}
	if _, err := publishingdomain.NormalizePath(input.Path); err != nil {
		return invalid(publishingdomain.ErrInvalidContentTranslation)
	}
	if input.Title == "" || input.Title != strings.TrimSpace(input.Title) {
		return invalid(publishingdomain.ErrInvalidContentTranslation)
	}
	if len(input.Title) > 255 || len(input.Excerpt) > 2000 {
		return invalid(ErrFieldTooLong)
	}
	if err := validateMetadata(input.SEO, input.GEO, input.AEO); err != nil {
		return err
	}
	if err := ValidateBlocks(input.Blocks); err != nil {
		return err
	}
	return nil
}

func DecodeBlocks(raw []byte) ([]publishingdomain.Block, error) {
	if len(raw) == 0 || len(raw) > maxBlocksJSONBytes {
		return nil, ErrInvalidBlockJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var blocks []publishingdomain.Block
	if err := decoder.Decode(&blocks); err != nil {
		return nil, ErrInvalidBlockJSON
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidBlockJSON
	}
	if err := ValidateBlocks(blocks); err != nil {
		return nil, errors.Join(ErrInvalidBlockJSON, err)
	}
	return blocks, nil
}

func ValidateBlocks(blocks []publishingdomain.Block) error {
	if len(blocks) == 0 || len(blocks) > 100 {
		return invalid(ErrNoVisibleText)
	}
	visible := false
	for _, block := range blocks {
		if block.Data == nil {
			return invalid(ErrInvalidBlock)
		}
		if err := validateBlockURLs(block.Data); err != nil {
			return err
		}
		blockVisible, err := validateBlock(block)
		if err != nil {
			return err
		}
		visible = visible || blockVisible
	}
	if !visible {
		return invalid(ErrNoVisibleText)
	}
	return nil
}

func validateBlock(block publishingdomain.Block) (bool, error) {
	textFields := func(keys ...string) bool {
		for _, key := range keys {
			if visibleString(block.Data[key]) {
				return true
			}
		}
		return false
	}
	switch block.Type {
	case publishingdomain.BlockText:
		if !textFields("text") || containsMarkup(stringValue(block.Data["text"])) {
			return false, invalid(ErrNoVisibleText)
		}
		return true, nil
	case publishingdomain.BlockImage:
		if !visibleString(block.Data["asset_id"]) ||
			!visibleString(block.Data["alt"]) {
			return false, invalid(ErrNoVisibleText)
		}
		for _, key := range []string{"width", "height"} {
			if value, exists := block.Data[key]; exists {
				number, ok := numericValue(value)
				if !ok || number <= 0 || number > 100000 {
					return false, invalid(ErrInvalidBlock)
				}
			}
		}
		return true, nil
	case publishingdomain.BlockCallout:
		if !visibleString(block.Data["heading"]) ||
			!visibleString(block.Data["body"]) {
			return false, invalid(ErrNoVisibleText)
		}
		return true, nil
	case publishingdomain.BlockAnswer:
		if !textFields("answer") {
			return false, invalid(ErrNoVisibleText)
		}
		return true, nil
	case publishingdomain.BlockSteps:
		if !collectionHasVisibleText(block.Data["steps"]) {
			return false, invalid(ErrNoVisibleText)
		}
		return true, nil
	case publishingdomain.BlockComparison:
		if !collectionHasVisibleText(block.Data["rows"]) &&
			!collectionHasVisibleText(block.Data["columns"]) {
			return false, invalid(ErrNoVisibleText)
		}
		return true, nil
	default:
		return false, invalid(ErrUnknownBlockType)
	}
}

func validateMetadata(
	seo publishingdomain.SEOFields,
	geo publishingdomain.GEOFields,
	aeo publishingdomain.AEOFields,
) error {
	if len(seo.Title) > 255 || len(seo.Description) > 320 ||
		len(seo.CanonicalURL) > 2048 || len(seo.Robots) > 64 ||
		len(geo.Region) > 128 || len(geo.Locality) > 128 ||
		len(aeo.Question) > 255 || len(aeo.Answer) > 20000 {
		return invalid(ErrFieldTooLong)
	}
	if seo.CanonicalURL != "" && !isSafeURL(seo.CanonicalURL) {
		return invalid(ErrUnsafeURL)
	}
	if seo.StructuredData != nil {
		if err := validateStructuredData(seo.StructuredData, 0); err != nil {
			return err
		}
	}
	return nil
}

func validateStructuredData(value map[string]any, depth int) error {
	if depth > 4 || len(value) > 64 {
		return invalid(ErrInvalidContent)
	}
	for key, item := range value {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") {
			return invalid(ErrInvalidContent)
		}
		if strings.Contains(strings.ToLower(key), "url") && item != nil {
			if stringValue(item) != "" && !isSafeURL(stringValue(item)) {
				return invalid(ErrUnsafeURL)
			}
		}
		switch typed := item.(type) {
		case map[string]any:
			if err := validateStructuredData(typed, depth+1); err != nil {
				return err
			}
		case []any:
			for _, child := range typed {
				if nested, ok := child.(map[string]any); ok {
					if err := validateStructuredData(nested, depth+1); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validateBlockURLs(value map[string]any) error {
	for key, item := range value {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "url") || lowerKey == "href" || lowerKey == "src" {
			if candidate := stringValue(item); candidate != "" && !isSafeURL(candidate) {
				return invalid(ErrUnsafeURL)
			}
		}
		switch typed := item.(type) {
		case map[string]any:
			if err := validateBlockURLs(typed); err != nil {
				return err
			}
		case []any:
			for _, child := range typed {
				if nested, ok := child.(map[string]any); ok {
					if err := validateBlockURLs(nested); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func collectionHasVisibleText(value any) bool {
	if visibleString(value) {
		return true
	}
	valueOf := reflect.ValueOf(value)
	if !valueOf.IsValid() {
		return false
	}
	if valueOf.Kind() == reflect.Slice || valueOf.Kind() == reflect.Array {
		for index := 0; index < valueOf.Len(); index++ {
			if collectionHasVisibleText(valueOf.Index(index).Interface()) {
				return true
			}
		}
	}
	if typed, ok := value.(map[string]any); ok {
		for _, item := range typed {
			if collectionHasVisibleText(item) {
				return true
			}
		}
	}
	return false
}

func visibleString(value any) bool {
	return strings.TrimSpace(stringValue(value)) != ""
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func containsMarkup(value string) bool {
	return strings.ContainsAny(value, "<>")
}

func isSafeURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "vbscript:") {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != ""
}

func invalid(reason error) error {
	return errors.Join(ErrInvalidContent, reason)
}
