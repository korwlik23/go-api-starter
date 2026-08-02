package ports

import (
	"context"
	"errors"
	"strings"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
)

var (
	ErrInvalidScheduleRepository = errors.New("invalid schedule repository")
	ErrScheduleNotFound          = errors.New("schedule not found")
	ErrScheduleConflict          = errors.New("schedule conflict")
)

type ScheduleCreateCommand struct {
	Schedule publishingdomain.PublicationSchedule
}

type ScheduleCancelCommand struct {
	AccountID  string
	ScheduleID string
	CanceledAt time.Time
}

type ScheduleFinishCommand struct {
	AccountID        string
	ScheduleID       string
	ExpectedAttempts int
	State            publishingdomain.ScheduleState
	FailureReason    string
	NextAttemptAt    *time.Time
	FinishedAt       time.Time
}

type ScheduleListQuery struct {
	AccountID string
	Limit     int
}

type SchedulePage struct {
	Items []publishingdomain.PublicationSchedule
}

type ScheduleRepository interface {
	Create(context.Context, ScheduleCreateCommand) error
	FindByIdempotency(context.Context, string, string) (publishingdomain.PublicationSchedule, error)
	List(context.Context, ScheduleListQuery) (SchedulePage, error)
	Cancel(context.Context, ScheduleCancelCommand) error
	ClaimDue(context.Context, time.Time, time.Time, int) ([]publishingdomain.PublicationSchedule, error)
	Finish(context.Context, ScheduleFinishCommand) error
}

func ValidateScheduleFinish(command ScheduleFinishCommand) error {
	if strings.TrimSpace(command.AccountID) == "" || strings.TrimSpace(command.ScheduleID) == "" ||
		command.ExpectedAttempts < 1 || command.FinishedAt.IsZero() ||
		(command.State != publishingdomain.SchedulePending &&
			command.State != publishingdomain.ScheduleSucceeded &&
			command.State != publishingdomain.ScheduleFailed) ||
		(command.State == publishingdomain.ScheduleFailed && strings.TrimSpace(command.FailureReason) == "") {
		return ErrInvalidScheduleRepository
	}
	return nil
}
