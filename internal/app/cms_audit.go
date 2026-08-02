package app

import (
	"context"
	"errors"
	"strings"

	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	redirectsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/redirects"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/settings"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	menusapp "github.com/korwlik23/go-api-starter/internal/modules/navigation/application/menus"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/application/revisions"
	schedulesapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/schedules"
	workflowapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/workflow"
)

var errInvalidCMSAuditAdapter = errors.New("app: invalid CMS audit adapter")

// cmsAuditAdapter keeps module-specific audit ports behind one application
// boundary. It intentionally records account-scoped events only; the worker
// path may use a system actor while interactive requests use the authenticated
// user ID.
type cmsAuditAdapter struct {
	appender authorizationAuditAppender
}

func (adapter cmsAuditAdapter) record(
	ctx context.Context,
	accountID string,
	actorUserID string,
	requestID string,
	action string,
	data map[string]any,
) error {
	action = normalizeCMSAuditAction(action)
	if adapter.appender == nil || ctx == nil || accountID == "" || action == "" || data == nil {
		return errInvalidCMSAuditAdapter
	}
	actor := auditdomain.Actor{Type: auditdomain.ActorSystem}
	if actorUserID != "" {
		actor = auditdomain.Actor{Type: auditdomain.ActorUser, ID: actorUserID}
	}
	_, err := adapter.appender.Execute(ctx, auditapp.AppendCommand{
		Scope: auditdomain.ScopeAccount, AccountID: accountID, Actor: actor,
		Action: action, RequestID: requestID, Data: data,
	})
	return err
}

func normalizeCMSAuditAction(action string) string {
	return strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(action))
}

func (adapter cmsAuditAdapter) RecordContentChange(ctx context.Context, event contentapp.ContentAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"content_id": event.ContentID, "translation_id": event.TranslationID, "version": event.Version,
	})
}

func (adapter cmsAuditAdapter) RecordWorkflowChange(ctx context.Context, event workflowapp.WorkflowAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"content_id": event.ContentID, "translation_id": event.TranslationID,
		"from_status": event.FromStatus, "to_status": event.ToStatus, "version": event.Version,
	})
}

func (adapter cmsAuditAdapter) RecordRevisionChange(ctx context.Context, event revisions.RevisionAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"content_id": event.ContentID, "translation_id": event.TranslationID,
		"revision_id": event.RevisionID, "version": event.Version, "data": event.Data,
	})
}

func (adapter cmsAuditAdapter) RecordScheduleChange(ctx context.Context, event schedulesapp.ScheduleAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"schedule_id": event.ScheduleID, "translation_id": event.TranslationID,
		"state": event.State, "attempts": event.Attempts,
	})
}

func (adapter cmsAuditAdapter) RecordMediaChange(ctx context.Context, event mediaapp.MediaAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"upload_id": event.UploadID, "asset_id": event.AssetID, "version": event.Version, "data": event.Data,
	})
}

func (adapter cmsAuditAdapter) RecordMenuChange(ctx context.Context, event menusapp.MenuAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"menu_id": event.MenuID, "version": event.Version, "data": event.Data,
	})
}

func (adapter cmsAuditAdapter) RecordRedirectChange(ctx context.Context, event redirectsapp.RedirectAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"redirect_id": event.RedirectID, "version": event.Version, "data": event.Data,
	})
}

func (adapter cmsAuditAdapter) RecordSEOChange(ctx context.Context, event settingsapp.SEOAuditEvent) error {
	return adapter.record(ctx, event.AccountID, event.ActorUserID, event.RequestID, event.Action, map[string]any{
		"locale_id": event.LocaleID, "version": event.Version, "data": event.Data,
	})
}

var _ contentapp.ContentAuditor = cmsAuditAdapter{}
var _ workflowapp.WorkflowAuditor = cmsAuditAdapter{}
var _ revisions.RevisionAuditor = cmsAuditAdapter{}
var _ schedulesapp.ScheduleAuditor = cmsAuditAdapter{}
var _ mediaapp.MediaAuditor = cmsAuditAdapter{}
var _ menusapp.MenuAuditor = cmsAuditAdapter{}
var _ redirectsapp.RedirectAuditor = cmsAuditAdapter{}
var _ settingsapp.SEOAuditor = cmsAuditAdapter{}
