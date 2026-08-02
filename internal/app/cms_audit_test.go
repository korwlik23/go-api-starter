package app

import (
	"context"
	"testing"

	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	workflowapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/workflow"
	"github.com/stretchr/testify/require"
)

type cmsAuditAppenderFake struct {
	command auditapp.AppendCommand
}

func (fake *cmsAuditAppenderFake) Execute(_ context.Context, command auditapp.AppendCommand) (auditdomain.Event, error) {
	fake.command = command
	return auditdomain.Event{}, nil
}

func TestM3F6CMSAuditCanonicalizesWorkflowActionForAuditDomain(t *testing.T) {
	t.Parallel()

	fake := &cmsAuditAppenderFake{}
	adapter := cmsAuditAdapter{appender: fake}
	err := adapter.RecordWorkflowChange(context.Background(), workflowapp.WorkflowAuditEvent{
		ActorUserID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		RequestID:     "workflow-request",
		Action:        "publishing.workflow.submit_review",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		FromStatus:    "draft",
		ToStatus:      "review",
		Version:       2,
	})
	require.NoError(t, err)
	require.Equal(t, "publishing.workflow.submitreview", fake.command.Action)
	require.Equal(t, auditdomain.ScopeAccount, fake.command.Scope)
}
