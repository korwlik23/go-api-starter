package app

import (
	"context"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	"github.com/stretchr/testify/require"
)

func TestBootstrapCreatesDynamicPermissionDrivenFoundationOnce(t *testing.T) {
	t.Parallel()

	store := &bootstrapStoreFake{
		state: BootstrapState{Status: BootstrapAbsent},
	}
	auditor := &bootstrapAuditorFake{}
	bootstrap := newBootstrapForTest(t, store, auditor)

	result, err := bootstrap.Execute(context.Background(), BootstrapCommand{
		Email:           "Owner@Example.Test",
		Password:        "correct horse battery staple",
		AccountSlug:     "personal",
		AccountRoleName: "Account Manager",
		SystemRoleName:  "System Manager",
	})

	require.NoError(t, err)
	require.True(t, result.Created)
	require.True(t, store.created)
	require.Len(t, store.record.Permissions, 45)
	require.NotEmpty(t, store.record.AccountRoleID)
	require.NotEmpty(t, store.record.SystemRoleID)
	require.NotEmpty(t, store.record.MembershipRoleAssignmentID)
	require.NotEmpty(t, store.record.SystemRoleAssignmentID)
	require.NotEqual(t, store.record.AccountRoleID, store.record.SystemRoleID)
	require.Equal(t, "owner@example.test", store.record.Email)
	require.Equal(t, "hashed-password", store.record.PasswordHash)
	require.Equal(t, 1, auditor.calls)
	require.Equal(t, auditdomain.ScopeSystem, auditor.command.Scope)
	require.Equal(t, "bootstrap.foundation.created", auditor.command.Action)
}

func TestBootstrapRerunVerifiesIDsAndMappingsWithoutAuditing(t *testing.T) {
	t.Parallel()

	store := &bootstrapStoreFake{
		state: BootstrapState{
			Status:    BootstrapPresent,
			AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			UserID:    "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		},
	}
	auditor := &bootstrapAuditorFake{}
	bootstrap := newBootstrapForTest(t, store, auditor)

	result, err := bootstrap.Execute(context.Background(), BootstrapCommand{
		Email:           "owner@example.test",
		Password:        "correct horse battery staple",
		AccountSlug:     "personal",
		AccountRoleName: "Renamed Account Role",
		SystemRoleName:  "Renamed System Role",
	})

	require.NoError(t, err)
	require.False(t, result.Created)
	require.True(t, store.verified)
	require.False(t, store.created)
	require.Zero(t, auditor.calls)
}

func TestBootstrapRejectsPartialExistingFoundation(t *testing.T) {
	t.Parallel()

	store := &bootstrapStoreFake{
		state: BootstrapState{Status: BootstrapPartial},
	}
	bootstrap := newBootstrapForTest(t, store, &bootstrapAuditorFake{})

	_, err := bootstrap.Execute(context.Background(), BootstrapCommand{
		Email:           "owner@example.test",
		Password:        "correct horse battery staple",
		AccountSlug:     "personal",
		AccountRoleName: "Account Manager",
		SystemRoleName:  "System Manager",
	})

	require.ErrorIs(t, err, ErrBootstrapConflict)
	require.False(t, store.created)
}

type bootstrapStoreFake struct {
	state    BootstrapState
	record   BootstrapRecord
	created  bool
	verified bool
}

func (store *bootstrapStoreFake) WithinBootstrap(
	ctx context.Context,
	operation func(context.Context, BootstrapSession) error,
) error {
	return operation(ctx, store)
}

func (store *bootstrapStoreFake) Inspect(
	context.Context,
	string,
	string,
) (BootstrapState, error) {
	return store.state, nil
}

func (store *bootstrapStoreFake) Create(
	_ context.Context,
	record BootstrapRecord,
) error {
	store.created = true
	store.record = record
	return nil
}

func (store *bootstrapStoreFake) Verify(
	_ context.Context,
	record BootstrapRecord,
) error {
	store.verified = true
	store.record = record
	return nil
}

type bootstrapAuditorFake struct {
	command auditapp.AppendCommand
	calls   int
}

func (auditor *bootstrapAuditorFake) Execute(
	_ context.Context,
	command auditapp.AppendCommand,
) (auditdomain.Event, error) {
	auditor.calls++
	auditor.command = command
	return auditdomain.Event{}, nil
}

type bootstrapPasswordHasherFake struct{}

func (bootstrapPasswordHasherFake) Hash(string) (string, error) {
	return "hashed-password", nil
}

func newBootstrapForTest(
	t *testing.T,
	store BootstrapStore,
	auditor bootstrapAuditor,
) *Bootstrap {
	t.Helper()

	sequence := 0
	bootstrap, err := NewBootstrap(BootstrapDependencies{
		Store:     store,
		Auditor:   auditor,
		Passwords: bootstrapPasswordHasherFake{},
		Modules: func() (*modular.Registry, error) {
			return Modules()
		},
		NewID: func() (string, error) {
			sequence++
			return []string{
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
				"018f1f64-7b2a-7c10-8bf2-3c40d5f67589",
			}[sequence-1], nil
		},
	})
	require.NoError(t, err)
	return bootstrap
}
