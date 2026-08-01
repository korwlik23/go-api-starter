package catalog

import (
	"context"
	"errors"
	"testing"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/stretchr/testify/require"
)

func TestCatalogUpdateRequiresSystemPermissionAndFixesEditorProvenance(
	t *testing.T,
) {
	repository := &catalogUpdateRepositoryStub{}
	useCase, err := NewUpdate(
		systemAuthorizerStub{allowed: true},
		catalogRepositoryStub{byTag: localizationdomain.Locale{
			ID: "th-id", Tag: "th", Enabled: true,
		}},
		repository,
	)
	require.NoError(t, err)

	version, err := useCase.Execute(
		context.Background(),
		"system-user",
		UpdateCommand{
			Locale: "th", Category: "common",
			Entries:         map[string]string{"actions.save": "จัดเก็บ"},
			ExpectedVersion: 2,
		},
	)

	require.NoError(t, err)
	require.Equal(t, int64(3), version)
	require.Equal(t, "editor", repository.source)
	require.Equal(t, "th-id", repository.localeID)
}

func TestCatalogUpdateAuditsMetadataWithoutTranslationBody(t *testing.T) {
	repository := &catalogUpdateRepositoryStub{}
	auditor := &catalogAuditorFake{}
	useCase, err := NewUpdateWithDependencies(
		systemAuthorizerStub{allowed: true},
		catalogRepositoryStub{byTag: localizationdomain.Locale{ID: "th-id", Tag: "th", Enabled: true}},
		repository, &catalogTransactorFake{}, auditor,
	)
	require.NoError(t, err)

	version, err := useCase.Execute(context.Background(), "system-user", UpdateCommand{
		Locale: "th", Category: "common", Entries: map[string]string{"actions.save": "บันทึก"},
		ExpectedVersion: 2, RequestID: "request-1",
	})

	require.NoError(t, err)
	require.Equal(t, int64(3), version)
	require.True(t, auditor.called)
	require.Equal(t, "common", auditor.event.Category)
	require.Equal(t, 1, auditor.event.EntryCount)
	require.Equal(t, "request-1", auditor.event.RequestID)
}

func TestCatalogUpdateRollsBackWhenAuditFails(t *testing.T) {
	transactor := &catalogTransactorFake{}
	useCase, err := NewUpdateWithDependencies(
		systemAuthorizerStub{allowed: true},
		catalogRepositoryStub{byTag: localizationdomain.Locale{ID: "th-id", Tag: "th", Enabled: true}},
		&catalogUpdateRepositoryStub{}, transactor,
		&catalogAuditorFake{err: errors.New("audit unavailable")},
	)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), "system-user", UpdateCommand{
		Locale: "th", Category: "common", Entries: map[string]string{"actions.save": "บันทึก"},
		ExpectedVersion: 2,
	})

	require.Error(t, err)
	require.True(t, transactor.rolledBack)
}

type systemAuthorizerStub struct {
	allowed bool
	err     error
}

func (stub systemAuthorizerStub) Allowed(
	context.Context,
	string,
	authorizationdomain.PermissionKey,
) (bool, error) {
	return stub.allowed, stub.err
}

type catalogUpdateRepositoryStub struct {
	localeID string
	source   string
}

type catalogTransactorFake struct {
	rolledBack bool
}

func (fake *catalogTransactorFake) WithinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	err := operation(ctx)
	fake.rolledBack = err != nil
	return err
}

type catalogAuditorFake struct {
	called bool
	event  CatalogAuditEvent
	err    error
}

func (fake *catalogAuditorFake) RecordCatalogUpdate(
	_ context.Context,
	event CatalogAuditEvent,
) error {
	fake.called = true
	fake.event = event
	return fake.err
}

func (stub *catalogUpdateRepositoryStub) ReplaceCategory(
	_ context.Context,
	localeID string,
	_ string,
	_ map[string]string,
	expectedVersion int64,
	source string,
) (int64, error) {
	stub.localeID = localeID
	stub.source = source
	return expectedVersion + 1, nil
}
