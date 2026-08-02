package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

type AuditStatus string

const (
	AuditStatusPass    AuditStatus = "pass"
	AuditStatusWarning AuditStatus = "warning"
	AuditStatusError   AuditStatus = "error"
)

type AuditSeverity string

const (
	AuditSeverityInfo    AuditSeverity = "info"
	AuditSeverityWarning AuditSeverity = "warning"
	AuditSeverityError   AuditSeverity = "error"
)

var (
	ErrInvalidContentAudit = errors.New("discoverability: invalid content audit")
	ErrInvalidAuditSubject = errors.New("discoverability: invalid audit subject")
)

// AuditCheck is a deterministic observation. ObservedValue is deliberately a
// bounded projection and never contains generated claims, credentials, or
// citation text.
type AuditCheck struct {
	RuleKey       string        `json:"rule_key"`
	Status        AuditStatus   `json:"status"`
	Severity      AuditSeverity `json:"severity"`
	ObservedValue string        `json:"observed_value"`
	Message       string        `json:"message"`
}

type AuditSubject struct {
	ID                 string
	AccountID          string
	ContentID          string
	LocaleID           string
	Kind               publishingdomain.Kind
	Version            int64
	TranslationVersion int64
	Title              string
	Excerpt            string
	Blocks             []publishingdomain.Block
	SEO                publishingdomain.SEOFields
	GEO                publishingdomain.GEOFields
	AEO                publishingdomain.AEOFields
}

type ContentAuditReport struct {
	Status AuditStatus
	Checks []AuditCheck
}

type ContentAudit struct {
	ID                 string
	AccountID          string
	ContentID          string
	TranslationID      string
	LocaleID           string
	ContentVersion     int64
	TranslationVersion int64
	Status             AuditStatus
	Checks             []AuditCheck
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ContentAuditInput struct {
	ID                 string
	AccountID          string
	ContentID          string
	TranslationID      string
	LocaleID           string
	ContentVersion     int64
	TranslationVersion int64
	Status             AuditStatus
	Checks             []AuditCheck
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewContentAudit(input ContentAuditInput) (ContentAudit, error) {
	if !redirectIDPattern.MatchString(input.ID) ||
		!redirectIDPattern.MatchString(input.AccountID) ||
		!redirectIDPattern.MatchString(input.ContentID) ||
		!redirectIDPattern.MatchString(input.TranslationID) ||
		!redirectIDPattern.MatchString(input.LocaleID) ||
		input.ContentVersion < 1 || input.TranslationVersion < 1 ||
		!validAuditStatus(input.Status) || len(input.Checks) == 0 ||
		(!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return ContentAudit{}, ErrInvalidContentAudit
	}
	seen := make(map[string]struct{}, len(input.Checks))
	checks := make([]AuditCheck, 0, len(input.Checks))
	for _, check := range input.Checks {
		if strings.TrimSpace(check.RuleKey) == "" || len(check.RuleKey) > 128 ||
			len(check.ObservedValue) > 512 || len(check.Message) > 1024 ||
			!validAuditStatus(check.Status) || !validAuditSeverity(check.Severity) ||
			(check.Status == AuditStatusPass && check.Severity != AuditSeverityInfo) ||
			(check.Status == AuditStatusWarning && check.Severity != AuditSeverityWarning) ||
			(check.Status == AuditStatusError && check.Severity != AuditSeverityError) {
			return ContentAudit{}, ErrInvalidContentAudit
		}
		if _, exists := seen[check.RuleKey]; exists {
			return ContentAudit{}, ErrInvalidContentAudit
		}
		seen[check.RuleKey] = struct{}{}
		checks = append(checks, check)
	}
	return ContentAudit{
		ID: input.ID, AccountID: input.AccountID, ContentID: input.ContentID,
		TranslationID: input.TranslationID, LocaleID: input.LocaleID,
		ContentVersion: input.ContentVersion, TranslationVersion: input.TranslationVersion,
		Status: input.Status, Checks: checks, CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC(),
	}, nil
}

func RunContentAudit(subject AuditSubject) (ContentAuditReport, error) {
	if !validSubject(subject) {
		return ContentAuditReport{}, ErrInvalidAuditSubject
	}
	checks := []AuditCheck{
		directAnswerCheck(subject), definitionCheck(subject), stepsOrComparisonCheck(subject), keyFactsCheck(subject),
		authorCredentialsCheck(subject), citationsCheck(subject), datesCheck(subject), provenanceCheck(subject),
	}
	status := AuditStatusPass
	for _, check := range checks {
		if check.Status == AuditStatusError {
			status = AuditStatusError
			break
		}
		if check.Status == AuditStatusWarning {
			status = AuditStatusWarning
		}
	}
	return ContentAuditReport{Status: status, Checks: checks}, nil
}

func validSubject(subject AuditSubject) bool {
	return redirectIDPattern.MatchString(subject.ID) && redirectIDPattern.MatchString(subject.AccountID) &&
		redirectIDPattern.MatchString(subject.ContentID) && redirectIDPattern.MatchString(subject.LocaleID) &&
		(subject.Kind == publishingdomain.KindPage || subject.Kind == publishingdomain.KindPost) &&
		subject.Version > 0 && subject.TranslationVersion > 0
}

func directAnswerCheck(subject AuditSubject) AuditCheck {
	question, answer := strings.TrimSpace(subject.AEO.Question) != "", strings.TrimSpace(subject.AEO.Answer) != ""
	if question && answer {
		return passCheck("aeo.direct_answer", "question_and_answer_present", "A direct question and answer are present.")
	}
	if !question && !answer {
		return warningCheck("aeo.direct_answer", "question_and_answer_missing", "No direct question and answer were provided; no answer was invented.")
	}
	return errorCheck("aeo.direct_answer", "question_or_answer_missing", "A provided direct-answer field is incomplete.")
}

func definitionCheck(subject AuditSubject) AuditCheck {
	if blockHasKey(subject.Blocks, "definition", "term_definition") {
		return passCheck("geo.definition", "definition_observed", "A definition signal is present in visible block data.")
	}
	return warningCheck("geo.definition", "definition_missing", "No definition signal was observed in visible block data.")
}

func stepsOrComparisonCheck(subject AuditSubject) AuditCheck {
	for _, block := range subject.Blocks {
		if block.Type == publishingdomain.BlockSteps || block.Type == publishingdomain.BlockComparison {
			return passCheck("geo.steps_or_comparison", "structured_block_observed", "A steps or comparison block is present.")
		}
	}
	return warningCheck("geo.steps_or_comparison", "structured_block_missing", "No steps or comparison block was observed.")
}

func keyFactsCheck(subject AuditSubject) AuditCheck {
	if structuredValuePresent(subject.SEO.StructuredData, "key_facts", "keyFacts") || blockHasKey(subject.Blocks, "key_facts", "keyFacts", "facts") {
		return passCheck("geo.key_facts", "key_facts_observed", "Key-fact data is present in supplied content metadata or blocks.")
	}
	return warningCheck("geo.key_facts", "key_facts_missing", "No key-fact signal was observed; no facts were generated.")
}

func authorCredentialsCheck(subject AuditSubject) AuditCheck {
	author, exists := lookupValue(subject.SEO.StructuredData, "author")
	if !exists || !hasVisibleValue(author) {
		return warningCheck("authority.author_credentials", "author_missing", "No author credentials were supplied.")
	}
	authorMap, ok := author.(map[string]any)
	if !ok || !hasVisibleValue(authorMap["name"]) {
		return errorCheck("authority.author_credentials", "author_name_missing", "An author was supplied without a usable name.")
	}
	if !hasVisibleValue(firstValueOnly(authorMap, "credentials", "credential", "qualifications")) {
		return warningCheck("authority.author_credentials", "credentials_missing", "An author name is present but credentials were not supplied.")
	}
	return passCheck("authority.author_credentials", "author_and_credentials_observed", "Author name and credentials are present in supplied metadata.")
}

func citationsCheck(subject AuditSubject) AuditCheck {
	value, exists := firstLookup(subject.SEO.StructuredData, "citations", "citation")
	if !exists {
		for _, block := range subject.Blocks {
			if candidate, blockExists := firstValue(block.Data, "citations", "citation"); blockExists {
				value, exists = candidate, true
				break
			}
		}
	}
	if !exists {
		return warningCheck("authority.citations", "citations_missing", "No citation was supplied; no citation was invented.")
	}
	if !hasVisibleValue(value) {
		return errorCheck("authority.citations", "citations_empty", "A citation field was supplied but is empty.")
	}
	return passCheck("authority.citations", "citations_observed", "At least one supplied citation signal is present.")
}

func datesCheck(subject AuditSubject) AuditCheck {
	published, publishedOK := firstLookup(subject.SEO.StructuredData, "datePublished", "date_published")
	modified, modifiedOK := firstLookup(subject.SEO.StructuredData, "dateModified", "date_modified")
	if publishedOK && !validDateValue(published) || modifiedOK && !validDateValue(modified) {
		return errorCheck("content.dates", "date_invalid", "A supplied publication date is not a valid RFC3339 timestamp.")
	}
	if publishedOK && modifiedOK {
		return passCheck("content.dates", "publication_dates_observed", "Publication and modification dates are present.")
	}
	return warningCheck("content.dates", "publication_dates_incomplete", "Publication and modification dates were not both supplied.")
}

func provenanceCheck(subject AuditSubject) AuditCheck {
	value, exists := firstLookup(subject.SEO.StructuredData, "provenance", "source")
	if !exists {
		for _, block := range subject.Blocks {
			if candidate, blockExists := firstValue(block.Data, "provenance", "source"); blockExists {
				value, exists = candidate, true
				break
			}
		}
	}
	if !exists {
		return warningCheck("content.provenance", "provenance_missing", "No provenance signal was supplied.")
	}
	if !hasVisibleValue(value) {
		return errorCheck("content.provenance", "provenance_empty", "A provenance field was supplied but is empty.")
	}
	return passCheck("content.provenance", "provenance_observed", "A supplied provenance signal is present.")
}

func passCheck(ruleKey, observed, message string) AuditCheck {
	return AuditCheck{RuleKey: ruleKey, Status: AuditStatusPass, Severity: AuditSeverityInfo, ObservedValue: observed, Message: message}
}
func warningCheck(ruleKey, observed, message string) AuditCheck {
	return AuditCheck{RuleKey: ruleKey, Status: AuditStatusWarning, Severity: AuditSeverityWarning, ObservedValue: observed, Message: message}
}
func errorCheck(ruleKey, observed, message string) AuditCheck {
	return AuditCheck{RuleKey: ruleKey, Status: AuditStatusError, Severity: AuditSeverityError, ObservedValue: observed, Message: message}
}

func blockHasKey(blocks []publishingdomain.Block, keys ...string) bool {
	for _, block := range blocks {
		if value, exists := firstValue(block.Data, keys...); exists && hasVisibleValue(value) {
			return true
		}
	}
	return false
}

func structuredValuePresent(values map[string]any, keys ...string) bool {
	value, exists := firstLookup(values, keys...)
	return exists && hasVisibleValue(value)
}

func firstLookup(values map[string]any, keys ...string) (any, bool) {
	return firstValue(values, keys...)
}

func firstValue(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		for actual, value := range values {
			if strings.EqualFold(actual, key) {
				return value, true
			}
		}
	}
	return nil, false
}

func lookupValue(values map[string]any, key string) (any, bool) {
	return firstValue(values, key)
}

func hasVisibleValue(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case []any:
		for _, item := range typed {
			if hasVisibleValue(item) {
				return true
			}
		}
		return false
	case map[string]any:
		for _, item := range typed {
			if hasVisibleValue(item) {
				return true
			}
		}
		return false
	default:
		return value != nil && fmt.Sprint(value) != ""
	}
}

func firstValueOnly(values map[string]any, keys ...string) any {
	value, _ := firstValue(values, keys...)
	return value
}

func validDateValue(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	_, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
	return err == nil
}

func validAuditStatus(value AuditStatus) bool {
	return value == AuditStatusPass || value == AuditStatusWarning || value == AuditStatusError
}
func validAuditSeverity(value AuditSeverity) bool {
	return value == AuditSeverityInfo || value == AuditSeverityWarning || value == AuditSeverityError
}
