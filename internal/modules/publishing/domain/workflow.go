package domain

import "errors"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusReview    Status = "review"
	StatusPublished Status = "published"
	StatusScheduled Status = "scheduled"
	StatusArchived  Status = "archived"
)

var ErrInvalidWorkflowTransition = errors.New("publishing: invalid workflow transition")

func ValidateStatus(status Status) error {
	switch status {
	case StatusDraft, StatusReview, StatusPublished, StatusScheduled, StatusArchived:
		return nil
	default:
		return ErrInvalidWorkflowTransition
	}
}

func ValidateTransition(from, to Status) error {
	if ValidateStatus(from) != nil || ValidateStatus(to) != nil {
		return ErrInvalidWorkflowTransition
	}
	allowed := map[Status][]Status{
		StatusDraft:     {StatusReview, StatusArchived},
		StatusReview:    {StatusPublished, StatusScheduled, StatusArchived},
		StatusScheduled: {StatusPublished},
		StatusPublished: {StatusArchived},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return ErrInvalidWorkflowTransition
}

func Transition(from, to Status) (Status, error) {
	if err := ValidateTransition(from, to); err != nil {
		return "", err
	}
	return to, nil
}
