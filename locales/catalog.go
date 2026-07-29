package locales

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed en/*.json th/*.json
var bundled embed.FS

var ErrCategoryNotBundled = errors.New("locale category is not bundled")

func Category(
	locale string,
	category string,
) (map[string]string, error) {
	contents, err := bundled.ReadFile(
		fmt.Sprintf("%s/%s.json", locale, category),
	)
	if err != nil {
		return nil, ErrCategoryNotBundled
	}
	var entries map[string]string
	if err := json.Unmarshal(contents, &entries); err != nil {
		return nil, fmt.Errorf("decode bundled locale category: %w", err)
	}
	return entries, nil
}
