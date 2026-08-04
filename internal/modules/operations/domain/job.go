package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidJob = errors.New("operations: invalid job")
var ErrInvalidJobTransition = errors.New("operations: invalid job transition")

type State string

const (
	StateQueued  State = "queued"
	StateRunning State = "running"
	StateFailed  State = "failed"
	StateDone    State = "done"
)

type Job struct {
	ID        string
	Name      string
	Payload   []byte
	State     State
	Attempts  int
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewJob(id, name string, payload []byte, createdAt time.Time) (Job, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" || len(payload) == 0 || createdAt.IsZero() {
		return Job{}, ErrInvalidJob
	}
	return Job{ID: id, Name: strings.TrimSpace(name), Payload: append([]byte(nil), payload...), State: StateQueued, CreatedAt: createdAt.UTC(), UpdatedAt: createdAt.UTC()}, nil
}

func (job *Job) Fail(message string, at time.Time) error {
	if job == nil || job.ID == "" || strings.TrimSpace(message) == "" || at.IsZero() || (job.State != StateQueued && job.State != StateRunning) {
		return ErrInvalidJobTransition
	}
	job.State = StateFailed
	job.Attempts++
	job.LastError = strings.TrimSpace(message)
	job.UpdatedAt = at.UTC()
	return nil
}

func (job *Job) Retry(at time.Time) error {
	if job == nil || job.ID == "" || at.IsZero() || job.State != StateFailed {
		return ErrInvalidJobTransition
	}
	job.State = StateQueued
	job.LastError = ""
	job.UpdatedAt = at.UTC()
	return nil
}
