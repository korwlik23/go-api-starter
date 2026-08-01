package locales

import "testing"

func TestPublicSiteCategoriesAreBundledForEveryStarterLocale(t *testing.T) {
	t.Parallel()

	for _, locale := range []string{"en", "th"} {
		for _, category := range []string{"common", "forms", "marketing", "navigation"} {
			locale := locale
			category := category
			t.Run(locale+"/"+category, func(t *testing.T) {
				t.Parallel()

				entries, err := Category(locale, category)
				if err != nil {
					t.Fatalf("Category(%q, %q) error = %v", locale, category, err)
				}
				if len(entries) == 0 {
					t.Fatalf("Category(%q, %q) returned no entries", locale, category)
				}
			})
		}
	}
}
