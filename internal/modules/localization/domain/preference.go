package domain

type LocalePreference struct {
	AllLanguages bool
	LocaleIDs    []string
	Version      int64
}
