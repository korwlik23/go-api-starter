package domain

import (
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3B6AuditRichContentPassesDeterministicChecks(t *testing.T) {
	t.Parallel()

	subject := richAuditSubject()
	report, err := RunContentAudit(subject)
	require.NoError(t, err)
	require.Equal(t, AuditStatusPass, report.Status)
	require.Len(t, report.Checks, 8)
	for _, check := range report.Checks {
		require.Equal(t, AuditStatusPass, check.Status, check.RuleKey)
		require.Equal(t, AuditSeverityInfo, check.Severity, check.RuleKey)
	}
}

func TestM3B6AuditDoesNotInventMissingSignals(t *testing.T) {
	t.Parallel()

	report, err := RunContentAudit(AuditSubject{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", Kind: publishingdomain.KindPage,
		Version: 2, TranslationVersion: 3, Title: "A title",
		Blocks: []publishingdomain.Block{{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Visible text"}}},
	})
	require.NoError(t, err)
	require.Equal(t, AuditStatusWarning, report.Status)
	for _, check := range report.Checks {
		require.NotEqual(t, AuditStatusPass, check.Status, check.RuleKey)
		require.NotContains(t, check.ObservedValue, "generated")
	}
}

func TestM3B6AuditRejectsMalformedProvidedSignals(t *testing.T) {
	t.Parallel()

	subject := richAuditSubject()
	subject.AEO.Answer = ""
	subject.SEO.StructuredData["datePublished"] = "not-a-date"
	report, err := RunContentAudit(subject)
	require.NoError(t, err)
	require.Equal(t, AuditStatusError, report.Status)
	checks := map[string]AuditCheck{}
	for _, check := range report.Checks {
		checks[check.RuleKey] = check
	}
	require.Equal(t, AuditStatusError, checks["aeo.direct_answer"].Status)
	require.Equal(t, AuditStatusError, checks["content.dates"].Status)
}

func TestM3B6NewContentAuditRejectsDuplicateRuleKeys(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	_, err := NewContentAudit(ContentAuditInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67584", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583",
		ContentVersion: 2, TranslationVersion: 3, Status: AuditStatusPass,
		Checks:    []AuditCheck{{RuleKey: "same", Status: AuditStatusPass, Severity: AuditSeverityInfo}, {RuleKey: "same", Status: AuditStatusWarning, Severity: AuditSeverityWarning}},
		CreatedAt: now, UpdatedAt: now,
	})
	require.ErrorIs(t, err, ErrInvalidContentAudit)
}

func richAuditSubject() AuditSubject {
	return AuditSubject{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ContentID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", Kind: publishingdomain.KindPost,
		Version: 2, TranslationVersion: 3, Title: "A title", AEO: publishingdomain.AEOFields{Question: "What is it?", Answer: "A direct answer."},
		Blocks: []publishingdomain.Block{
			{Type: publishingdomain.BlockText, Data: map[string]any{"text": "Definition", "definition": "A clear definition.", "key_facts": []any{"fact"}, "citation": "https://example.test/source"}},
			{Type: publishingdomain.BlockSteps, Data: map[string]any{"steps": []any{"one", "two"}}},
			{Type: publishingdomain.BlockComparison, Data: map[string]any{"rows": []any{"one", "two"}}},
		},
		SEO: publishingdomain.SEOFields{StructuredData: map[string]any{
			"author":    map[string]any{"name": "Author", "credentials": "Editor"},
			"citations": []any{"https://example.test/source"}, "datePublished": "2026-08-01T00:00:00Z", "dateModified": "2026-08-02T00:00:00Z", "provenance": "editorial-review", "key_facts": []any{"fact"},
		}},
	}
}
