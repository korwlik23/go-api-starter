package ports

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

var (
	ErrInvalidWorkflowRepository = errors.New("invalid workflow repository")
	ErrWorkflowNotFound          = errors.New("workflow content not found")
	ErrWorkflowConflict          = errors.New("workflow content version conflict")
)

type WorkflowEvent struct {
	ID            string
	AccountID     string
	ContentID     string
	TranslationID string
	ActorID       string
	FromStatus    publishingdomain.Status
	ToStatus      publishingdomain.Status
	RequestID     string
	OccurredAt    time.Time
}

type WorkflowTransitionCommand struct {
	AccountID       string
	ContentID       string
	TranslationID   string
	LocaleID        string
	ExpectedVersion int64
	FromStatus      publishingdomain.Status
	ToStatus        publishingdomain.Status
	UpdatedAt       time.Time
}

type WorkflowRepository interface {
	Get(context.Context, string, string, string) (ContentDetail, error)
	Transition(context.Context, WorkflowTransitionCommand) (ContentDetail, error)
	AppendEvent(context.Context, WorkflowEvent) error
}

var workflowIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func ValidateWorkflowEvent(event WorkflowEvent) error {
	if !workflowIDPattern.MatchString(event.ID) ||
		!workflowIDPattern.MatchString(event.AccountID) ||
		!workflowIDPattern.MatchString(event.ContentID) ||
		!workflowIDPattern.MatchString(event.TranslationID) ||
		!workflowIDPattern.MatchString(event.ActorID) ||
		publishingdomain.ValidateStatus(event.FromStatus) != nil ||
		publishingdomain.ValidateStatus(event.ToStatus) != nil ||
		event.FromStatus == event.ToStatus || strings.TrimSpace(event.RequestID) == "" ||
		event.OccurredAt.IsZero() {
		return ErrInvalidWorkflowRepository
	}
	return nil
}
