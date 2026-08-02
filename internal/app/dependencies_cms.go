package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	discoverabilitymodule "github.com/korwlik23/go-api-starter/internal/modules/discoverability"
	discoverabilitygorm "github.com/korwlik23/go-api-starter/internal/modules/discoverability/adapters/gorm"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/audit"
	redirectsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/redirects"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/settings"
	discoverabilityhttp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/transport/http"
	localizationgorm "github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/gorm"
	mediamodule "github.com/korwlik23/go-api-starter/internal/modules/media"
	mediagorm "github.com/korwlik23/go-api-starter/internal/modules/media/adapters/gorm"
	medialocal "github.com/korwlik23/go-api-starter/internal/modules/media/adapters/storage/local"
	mediaapp "github.com/korwlik23/go-api-starter/internal/modules/media/application"
	mediahttp "github.com/korwlik23/go-api-starter/internal/modules/media/transport/http"
	navigationmodule "github.com/korwlik23/go-api-starter/internal/modules/navigation"
	navigationgorm "github.com/korwlik23/go-api-starter/internal/modules/navigation/adapters/gorm"
	menusapp "github.com/korwlik23/go-api-starter/internal/modules/navigation/application/menus"
	navigationhttp "github.com/korwlik23/go-api-starter/internal/modules/navigation/transport/http"
	publishingmodule "github.com/korwlik23/go-api-starter/internal/modules/publishing"
	publishinggorm "github.com/korwlik23/go-api-starter/internal/modules/publishing/adapters/gorm"
	publishingapplication "github.com/korwlik23/go-api-starter/internal/modules/publishing/application"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	previewapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/preview"
	revisionsapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/revisions"
	schedulesapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/schedules"
	workflowapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/workflow"
	publishinghttp "github.com/korwlik23/go-api-starter/internal/modules/publishing/transport/http"
	"github.com/korwlik23/go-api-starter/internal/platform/cache"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

var errCMSComposition = errors.New("app: CMS composition failed")

const (
	defaultCMSCacheEntries  = 2048
	defaultCMSCacheBytes    = 16 << 20
	defaultCMSCacheTTL      = time.Minute
	defaultCMSStaleWindow   = 30 * time.Second
	defaultPreviewTTL       = 5 * time.Minute
	defaultScheduleLease    = 2 * time.Minute
	defaultScheduleAttempts = 5
)

type workflowPermissionAuthorizer struct {
	evaluator *authorizationapp.EvaluatePermission
}

func (authorizer workflowPermissionAuthorizer) Allowed(
	ctx context.Context,
	userID string,
	accountID string,
	permission authorizationdomain.PermissionKey,
	resourceID string,
) (bool, error) {
	if authorizer.evaluator == nil {
		return false, errCMSComposition
	}
	if permission.Scope == authorizationdomain.PermissionScopeOwn {
		// Publishing content has no separate owner column. The own scope is
		// therefore evaluated against the authenticated account member, just
		// like the protected CMS route guard.
		resourceID = userID
	}
	return authorizer.evaluator.Execute(ctx, authorizationapp.PermissionRequest{
		UserID: userID, AccountID: accountID, Permission: permission, ResourceID: resourceID,
	})
}

// buildCMSRuntime composes the bounded CMS modules. The public account is
// opt-in through PUBLIC_ACCOUNT_ID so a multi-tenant API never accidentally
// exposes whichever account happened to be bootstrapped first.
func buildCMSRuntime(
	db *gormdb.DB,
	cfg config.Config,
	authorization authorizationRuntime,
) (cmsRouteRegistration, error) {
	if db == nil || authorization.transactions == nil || authorization.auditor == nil ||
		authorization.accounts == nil || authorization.evaluator == nil {
		return cmsRouteRegistration{}, errCMSComposition
	}
	transactions := authorization.transactions
	auditor := cmsAuditAdapter{appender: authorization.auditor}
	invalidator, err := newCMSCacheInvalidator()
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	contentRepository, err := publishinggorm.NewContentRepositoryWithDependencies(db, transactions)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	contentMutations, err := contentapp.NewContentMutations(contentapp.ContentMutationDependencies{
		Repository: contentRepository, Transactions: transactions, Auditor: auditor,
		NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	contentHandler, err := publishinghttp.NewContentHandler(contentMutations, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	workflowRepository, err := publishinggorm.NewWorkflowRepositoryWithDependencies(db, transactions)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	workflowService, err := workflowapp.NewService(workflowapp.Dependencies{
		Repository:   workflowRepository,
		Authorizer:   workflowPermissionAuthorizer{evaluator: authorization.evaluator},
		Transactions: transactions, Auditor: auditor, CacheInvalidator: invalidator,
		NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	workflowHandler, err := publishinghttp.NewWorkflowHandler(workflowService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	scheduleRepository, err := publishinggorm.NewScheduleRepositoryWithDependencies(db, transactions)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	scheduleExecutor := newSchedulePublicationExecutor(db, workflowRepository, transactions, invalidator)
	scheduleService, err := schedulesapp.NewService(schedulesapp.Dependencies{
		Repository: scheduleRepository, Transactions: transactions, Auditor: auditor,
		Executor: scheduleExecutor, NewID: newRuntimeID, Now: time.Now,
		LeaseDuration: defaultScheduleLease, MaxAttempts: defaultScheduleAttempts,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	scheduleHandler, err := publishinghttp.NewScheduleHandler(scheduleService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	revisionRepository, err := publishinggorm.NewRevisionRepositoryWithDependencies(db, transactions)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	revisionService, err := revisionsapp.NewService(revisionsapp.RevisionDependencies{
		Repository: revisionRepository, Content: contentRepository, Transactions: transactions,
		Auditor: auditor, NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	revisionHandler, err := publishinghttp.NewRevisionHandler(revisionService, revisionService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	previewHandler, err := buildPreviewHandler(db, cfg, revisionRepository, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	navigationRepository := navigationgorm.NewMenuRepositoryWithDependencies(db, transactions)
	if navigationRepository == nil {
		return cmsRouteRegistration{}, errCMSComposition
	}
	menuService, err := menusapp.NewService(menusapp.Dependencies{
		Repository: navigationRepository, Transactions: transactions, Auditor: auditor,
		CacheInvalidator: invalidator, NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	menuHandler, err := navigationhttp.NewMenuHandler(menuService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	redirectRepository := discoverabilitygorm.NewRedirectRepositoryWithDependencies(db, transactions)
	if redirectRepository == nil {
		return cmsRouteRegistration{}, errCMSComposition
	}
	redirectService, err := redirectsapp.NewService(redirectsapp.Dependencies{
		Repository: redirectRepository, Transactions: transactions, Auditor: auditor,
		CacheInvalidator: invalidator, NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	redirectHandler, err := discoverabilityhttp.NewRedirectHandler(redirectService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	settingsService, err := settingsapp.NewService(settingsapp.Dependencies{
		Repository: redirectRepository, Transactions: transactions, Auditor: auditor,
		NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	settingsHandler, err := discoverabilityhttp.NewSettingsHandler(settingsService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	auditRepository := discoverabilitygorm.NewContentAuditRepositoryWithDependencies(db, transactions)
	if auditRepository == nil {
		return cmsRouteRegistration{}, errCMSComposition
	}
	auditService, err := auditapp.NewService(auditapp.Dependencies{
		Content: contentRepository, Audits: auditRepository, NewID: newRuntimeID, Now: time.Now,
	})
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	auditHandler, err := discoverabilityhttp.NewAuditHandler(auditService, authorization.accounts)
	if err != nil {
		return cmsRouteRegistration{}, err
	}

	registration := cmsRouteRegistration{
		publishing: publishinghttp.RouteHandlers{
			Content: contentHandler, Revisions: revisionHandler, Workflow: workflowHandler,
			Schedules: scheduleHandler, Preview: previewHandler,
		},
		preview: previewHandler, menus: menuHandler, redirects: redirectHandler,
		settings: settingsHandler, audits: auditHandler,
	}
	if strings.TrimSpace(cfg.Media.StorageRoot) != "" {
		mediaRegistration, mediaErr := buildMediaRegistration(cfg, transactions, auditor, authorization.accounts)
		if mediaErr != nil {
			return cmsRouteRegistration{}, mediaErr
		}
		registration.uploads, registration.assets = mediaRegistration.uploads, mediaRegistration.assets
	}
	if strings.TrimSpace(cfg.Publishing.PublicAccountID) != "" {
		publicHandler, publicErr := buildPublicCMSHandler(db, cfg.Publishing.PublicAccountID, contentRepository, navigationRepository)
		if publicErr != nil {
			return cmsRouteRegistration{}, publicErr
		}
		registration.public = publicHandler
	}
	registration.guards, err = buildCMSRouteGuards(authorization)
	if err != nil {
		return cmsRouteRegistration{}, err
	}
	return registration, nil
}

func newCMSCacheInvalidator() (*publishingapplication.CacheInvalidationService, error) {
	memory, err := cache.NewMemory(cache.MemoryOptions{
		MaxEntries: defaultCMSCacheEntries, MaxBytes: defaultCMSCacheBytes,
		DefaultTTL: defaultCMSCacheTTL, DefaultStaleIfError: defaultCMSStaleWindow,
		Now: time.Now,
	})
	if err != nil {
		return nil, err
	}
	return publishingapplication.NewCacheInvalidationService(memory)
}

type mediaRouteRegistration struct {
	uploads *mediahttp.UploadHandler
	assets  *mediahttp.AssetHandler
}

func buildMediaRegistration(
	cfg config.Config,
	transactions *database.TransactionManager,
	auditor cmsAuditAdapter,
	accounts *accountsapp.ResolveAccount,
) (mediaRouteRegistration, error) {
	repository, err := mediagorm.NewTransactionalAssetRepository(transactions, cfg.Database.Driver)
	if err != nil {
		return mediaRouteRegistration{}, err
	}
	storage, err := medialocal.NewProvider(cfg.Media.StorageRoot)
	if err != nil {
		return mediaRouteRegistration{}, err
	}
	service, err := mediaapp.NewService(mediaapp.Dependencies{
		Repository: repository, Storage: storage, Auditor: auditor,
		NewID: newRuntimeID, Now: time.Now,
		UploadTTL: 15 * time.Minute, MaxSize: 32 << 20,
	})
	if err != nil {
		return mediaRouteRegistration{}, err
	}
	uploads, err := mediahttp.NewUploadHandler(service, accounts)
	if err != nil {
		return mediaRouteRegistration{}, err
	}
	assets, err := mediahttp.NewAssetHandler(service, accounts)
	if err != nil {
		return mediaRouteRegistration{}, err
	}
	return mediaRouteRegistration{uploads: uploads, assets: assets}, nil
}

func buildPreviewHandler(
	db *gormdb.DB,
	cfg config.Config,
	revisions *publishinggorm.RevisionRepository,
	accounts *accountsapp.ResolveAccount,
) (*publishinghttp.PreviewHandler, error) {
	previewRepository, err := publishinggorm.NewPreviewRepository(db)
	if err != nil {
		return nil, err
	}
	keyring, err := config.ParseRecoveryTokenKeyring(
		cfg.Authentication.Recovery.TokenActiveKeyVersion,
		cfg.Authentication.Recovery.TokenHMACKeys,
	)
	if err != nil {
		return nil, err
	}
	codec, err := platformcrypto.NewBoundTokenCodec(keyring)
	if err != nil {
		return nil, err
	}
	issue, err := previewapp.NewIssue(previewapp.IssueDependencies{
		Repository: previewRepository, Revisions: revisions, Codec: codec,
		NewID: newRuntimeID, Now: time.Now, TTL: defaultPreviewTTL,
	})
	if err != nil {
		return nil, err
	}
	exchange, err := previewapp.NewExchange(previewapp.ExchangeDependencies{
		Repository: previewRepository, Revisions: revisions, Codec: codec, Now: time.Now,
	})
	if err != nil {
		return nil, err
	}
	return publishinghttp.NewPreviewHandler(issue, exchange, accounts)
}

func buildPublicCMSHandler(
	db *gormdb.DB,
	accountID string,
	contentRepository *publishinggorm.ContentRepository,
	menuRepository *navigationgorm.MenuRepository,
) (*publishinghttp.PublicHandler, error) {
	content, err := contentapp.NewPublicResolver(contentRepository)
	if err != nil {
		return nil, err
	}
	posts, err := contentapp.NewPublicPosts(contentRepository)
	if err != nil {
		return nil, err
	}
	transactions, err := database.NewTransactionManager(db)
	if err != nil {
		return nil, err
	}
	locales, err := localizationgorm.NewLocaleRepositoryWithDependencies(db, transactions, transactions)
	if err != nil {
		return nil, err
	}
	return publishinghttp.NewPublicHandlerWithMenus(content, posts, locales, menuRepository, strings.TrimSpace(accountID))
}

func buildCMSRouteGuards(authorization authorizationRuntime) (cmsRouteGuards, error) {
	own := currentPrincipalResourceID
	guards := cmsRouteGuards{}
	var err error
	makeGuard := func(raw string, resource authorizationhttp.ResourceIDResolver) gin.HandlerFunc {
		guard, guardErr := newPermissionGuard(authorization.accounts, authorization.evaluator, raw, resource)
		if guardErr != nil {
			err = guardErr
		}
		return guard
	}
	guards.pagesRead = makeGuard(publishingmodule.PermissionPagesReadOwn, own)
	guards.pagesWrite = makeGuard(publishingmodule.PermissionPagesWriteOwn, own)
	guards.pagesReview = makeGuard(publishingmodule.PermissionPagesReviewAny, nil)
	guards.pagesPublish = makeGuard(publishingmodule.PermissionPagesPublishAny, nil)
	guards.revisionsRead = makeGuard(publishingmodule.PermissionRevisionsReadOwn, own)
	guards.revisionsRollback = makeGuard(publishingmodule.PermissionRevisionsRollbackOwn, own)
	guards.mediaUploadsWrite = makeGuard(mediamodule.PermissionUploadsWriteOwn, own)
	guards.mediaAssetsRead = makeGuard(mediamodule.PermissionAssetsReadOwn, own)
	guards.mediaAssetsWrite = makeGuard(mediamodule.PermissionAssetsWriteOwn, own)
	guards.menusRead = makeGuard(navigationmodule.PermissionMenusReadOwn, own)
	guards.menusWrite = makeGuard(navigationmodule.PermissionMenusWriteOwn, own)
	guards.redirectsRead = makeGuard(discoverabilitymodule.PermissionRedirectsReadOwn, own)
	guards.redirectsWrite = makeGuard(discoverabilitymodule.PermissionRedirectsWriteOwn, own)
	guards.seoRead = makeGuard(discoverabilitymodule.PermissionSEOReadOwn, own)
	guards.seoWrite = makeGuard(discoverabilitymodule.PermissionSEOWriteOwn, own)
	guards.auditsRead = makeGuard(discoverabilitymodule.PermissionAuditReadOwn, own)
	guards.auditsRun = makeGuard(discoverabilitymodule.PermissionAuditRunOwn, own)
	if err != nil {
		return cmsRouteGuards{}, err
	}
	return guards, nil
}
