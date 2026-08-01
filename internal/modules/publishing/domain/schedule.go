package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type ScheduleState string

const (
	SchedulePending   ScheduleState = "pending"
	ScheduleLeased    ScheduleState = "leased"
	ScheduleSucceeded ScheduleState = "succeeded"
	ScheduleFailed    ScheduleState = "failed"
	ScheduleCanceled  ScheduleState = "canceled"
)

var (
	ErrInvalidPublicationSchedule = errors.New("publishing: invalid publication schedule")
	scheduleKeyPattern            = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type PublicationSchedule struct {
	ID              string
	AccountID       string
	TranslationID   string
	PublishAt       time.Time
	UnpublishAt     *time.Time
	ExpectedVersion int64
	IdempotencyKey  string
	State           ScheduleState
	LeaseUntil      *time.Time
	Attempts        int
	FailureReason   string
}

type PublicationScheduleInput struct {
	ID              string
	AccountID       string
	TranslationID   string
	PublishAt       time.Time
	UnpublishAt     *time.Time
	ExpectedVersion int64
	IdempotencyKey  string
	State           ScheduleState
	LeaseUntil      *time.Time
	Attempts        int
	FailureReason   string
}

func NewPublicationSchedule(input PublicationScheduleInput) (PublicationSchedule, error) {
	state := input.State
	if state == "" {
		state = SchedulePending
	}
	if !validIdentity(input.ID) ||
		!validIdentity(input.AccountID) ||
		!validIdentity(input.TranslationID) ||
		input.PublishAt.IsZero() ||
		(input.UnpublishAt != nil && !input.UnpublishAt.After(input.PublishAt)) ||
		input.ExpectedVersion <= 0 ||
		!scheduleKeyPattern.MatchString(input.IdempotencyKey) ||
		!validScheduleState(state) || input.Attempts < 0 ||
		len(input.FailureReason) > 2000 ||
		strings.ContainsAny(input.FailureReason, "\r\n") {
		return PublicationSchedule{}, ErrInvalidPublicationSchedule
	}
	return PublicationSchedule{
		ID:              strings.TrimSpace(input.ID),
		AccountID:       strings.TrimSpace(input.AccountID),
		TranslationID:   strings.TrimSpace(input.TranslationID),
		PublishAt:       input.PublishAt.UTC(),
		UnpublishAt:     cloneTime(input.UnpublishAt),
		ExpectedVersion: input.ExpectedVersion,
		IdempotencyKey:  input.IdempotencyKey,
		State:           state,
		LeaseUntil:      cloneTime(input.LeaseUntil),
		Attempts:        input.Attempts,
		FailureReason:   input.FailureReason,
	}, nil
}

func validScheduleState(state ScheduleState) bool {
	switch state {
	case SchedulePending, ScheduleLeased, ScheduleSucceeded, ScheduleFailed, ScheduleCanceled:
		return true
	default:
		return false
	}
}
