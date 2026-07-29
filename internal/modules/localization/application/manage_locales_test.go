package application

import (
	"context"
	"testing"
	"time"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/stretchr/testify/require"
)

func TestManageLocalesCreatesDisabledLocaleWithSystemPermission(t *testing.T) {
	repository := &localeManagementRepositoryStub{}
	useCase, err := NewManageLocales(ManageLocaleDependencies{
		Authorizer: localeAuthorizerStub{allowed: true},
		Repository: repository,
		NewID:      func() (string, error) { return "locale-id", nil },
		Now: func() time.Time {
			return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		},
	})
	require.NoError(t, err)

	created, err := useCase.Create(
		context.Background(),
		"system-user",
		CreateLocaleInput{Tag: "TH", Name: "Thai", Direction: "ltr"},
	)

	require.NoError(t, err)
	require.Equal(t, "th", created.Tag)
	require.False(t, created.Enabled)
	require.False(t, created.UserSelectable)
	require.Equal(t, int64(1), created.Version)
	require.Equal(t, created, repository.created.Locale)
}

func TestManageLocalesPreservesStatusDuringMetadataUpdate(t *testing.T) {
	repository := &localeManagementRepositoryStub{found: localizationdomain.Locale{
		ID: "locale-id", Tag: "ar", Name: "Arabic",
		Direction: localizationdomain.DirectionRTL,
		Enabled:   true, UserSelectable: true, FallbackID: "en-id", Version: 4,
	}}
	useCase := newLocaleManagerForTest(t, repository)

	version, err := useCase.UpdateMetadata(
		context.Background(),
		"system-user",
		"locale-id",
		UpdateLocaleMetadataInput{
			Name: "Arabic (العربية)", Direction: "rtl", ExpectedVersion: 4,
		},
	)

	require.NoError(t, err)
	require.Equal(t, int64(5), version)
	require.True(t, repository.updated.Enabled)
	require.True(t, repository.updated.UserSelectable)
	require.Equal(t, "en-id", repository.updated.FallbackID)
}

func TestManageLocalesRejectsMutationWithoutSystemPermission(t *testing.T) {
	useCase, err := NewManageLocales(ManageLocaleDependencies{
		Authorizer: localeAuthorizerStub{allowed: false},
		Repository: &localeManagementRepositoryStub{},
		NewID:      func() (string, error) { return "locale-id", nil },
		Now:        time.Now,
	})
	require.NoError(t, err)

	_, err = useCase.Create(
		context.Background(),
		"user-id",
		CreateLocaleInput{Tag: "fr", Name: "French", Direction: "ltr"},
	)

	require.ErrorIs(t, err, ErrLocaleManagementDenied)
}

func newLocaleManagerForTest(
	t *testing.T,
	repository *localeManagementRepositoryStub,
) *ManageLocales {
	t.Helper()
	useCase, err := NewManageLocales(ManageLocaleDependencies{
		Authorizer: localeAuthorizerStub{allowed: true},
		Repository: repository,
		NewID:      func() (string, error) { return "locale-id", nil },
		Now:        time.Now,
	})
	require.NoError(t, err)
	return useCase
}

type localeAuthorizerStub struct {
	allowed bool
}

func (stub localeAuthorizerStub) Allowed(
	context.Context,
	string,
	authorizationdomain.PermissionKey,
) (bool, error) {
	return stub.allowed, nil
}

type localeManagementRepositoryStub struct {
	byTag   map[string]localizationdomain.Locale
	found   localizationdomain.Locale
	created ports.CreateLocaleCommand
	updated ports.UpdateLocaleCommand
}

func (stub *localeManagementRepositoryStub) FindByTag(
	_ context.Context,
	tag string,
) (localizationdomain.Locale, error) {
	locale, ok := stub.byTag[tag]
	if !ok {
		return localizationdomain.Locale{}, ports.ErrLocaleNotFound
	}
	return locale, nil
}

func (stub *localeManagementRepositoryStub) FindByID(
	context.Context,
	string,
) (localizationdomain.Locale, error) {
	return stub.found, nil
}

func (stub *localeManagementRepositoryStub) Create(
	_ context.Context,
	command ports.CreateLocaleCommand,
) error {
	stub.created = command
	return nil
}

func (stub *localeManagementRepositoryStub) Update(
	_ context.Context,
	command ports.UpdateLocaleCommand,
) (int64, error) {
	stub.updated = command
	return command.ExpectedVersion + 1, nil
}

func (stub *localeManagementRepositoryStub) SetDefault(
	context.Context,
	string,
	int64,
	time.Time,
) (ports.DefaultLocaleResult, error) {
	return ports.DefaultLocaleResult{}, nil
}
