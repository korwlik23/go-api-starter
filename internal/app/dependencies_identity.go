package app

import (
	"crypto/rand"
	"log/slog"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/korwlik23/go-api-starter/internal/modules/identity/adapters/gorm"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	authapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/auth"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	sessionapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/session"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/ratelimit"
	gormdb "gorm.io/gorm"
)

const (
	mfaPendingIdleTTL     = 5 * time.Minute
	mfaPendingAbsoluteTTL = 10 * time.Minute
	mfaEnrollmentTTL      = 10 * time.Minute
	passwordRecoveryTTL   = 30 * time.Minute
)

type identityRuntime struct {
	handlers     identityhttp.RouteHandlers
	profile      *identityhttp.ProfileHandler
	rateLimits   identityhttp.RateLimits
	authenticate gin.HandlerFunc
	csrf         gin.HandlerFunc
}

func buildIdentityRuntime(
	db *gormdb.DB,
	cfg config.Config,
	trustedProxies config.TrustedProxySet,
	limiter *ratelimit.Limiter,
	clientKey platformmiddleware.ClientKeyer,
	logger *slog.Logger,
) (identityRuntime, error) {
	repositories, err := newIdentityRepositories(db)
	if err != nil {
		return identityRuntime{}, err
	}
	security, err := newIdentitySecurity(cfg)
	if err != nil {
		return identityRuntime{}, err
	}
	useCases, err := newIdentityUseCases(cfg, repositories, security)
	if err != nil {
		return identityRuntime{}, err
	}
	cookie, err := identityhttp.NewSessionCookie(identityhttp.SessionCookieConfig{
		Name:       cfg.Session.CookieName,
		Production: strings.EqualFold(string(cfg.App.Environment), "production"),
		Secure:     cfg.Session.CookieSecure,
	})
	if err != nil {
		return identityRuntime{}, err
	}
	handlers, err := newIdentityHandlers(
		useCases,
		repositories,
		cookie,
		trustedProxies,
	)
	if err != nil {
		return identityRuntime{}, err
	}
	profileService, err := identityapp.NewProfileService(repositories.profiles, nil)
	if err != nil {
		return identityRuntime{}, err
	}
	profileHandler, err := identityhttp.NewProfileHandler(profileService)
	if err != nil {
		return identityRuntime{}, err
	}
	authenticate, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName:            cookie.Name(),
			Sessions:              repositories.sessions,
			AuthorizationVersions: repositories.users,
			SessionTokens:         security.sessionTokens,
			Now:                   time.Now,
		},
	)
	if err != nil {
		return identityRuntime{}, err
	}
	csrf, err := platformmiddleware.NewCSRF(platformmiddleware.CSRFConfig{
		TrustedOrigin: cfg.Origins.Admin.String(),
		CookieName:    cookie.Name(),
		Sessions:      repositories.sessions,
		SessionTokens: security.sessionTokens,
		CSRFTokens:    security.csrfTokens,
		Now:           time.Now,
	})
	if err != nil {
		return identityRuntime{}, err
	}
	rateLimits, err := identityhttp.NewRateLimits(identityhttp.RateLimitDependencies{
		Limiter: limiter, TrustedProxies: trustedProxies,
		ClientKey: clientKey, Logger: logger,
	})
	if err != nil {
		return identityRuntime{}, err
	}
	return identityRuntime{
		handlers: handlers, profile: profileHandler, rateLimits: rateLimits,
		authenticate: authenticate, csrf: csrf,
	}, nil
}

type identityRepositories struct {
	users     *identitygorm.UserRepository
	passwords *identitygorm.PasswordRepository
	sessions  *identitygorm.SessionRepository
	attempts  *identitygorm.AuthAttemptRepository
	mfa       *identitygorm.MFARepository
	oneTime   *identitygorm.OneTimeTokenRepository
	profiles  *identitygorm.ProfileRepository
}

func newIdentityRepositories(db *gormdb.DB) (identityRepositories, error) {
	users, err := identitygorm.NewUserRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	passwords, err := identitygorm.NewPasswordRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	sessions, err := identitygorm.NewSessionRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	attempts, err := identitygorm.NewAuthAttemptRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	mfa, err := identitygorm.NewMFARepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	oneTime, err := identitygorm.NewOneTimeTokenRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	profiles, err := identitygorm.NewProfileRepository(db)
	if err != nil {
		return identityRepositories{}, err
	}
	return identityRepositories{
		users: users, passwords: passwords, sessions: sessions,
		attempts: attempts, mfa: mfa, oneTime: oneTime, profiles: profiles,
	}, nil
}

type identitySecurity struct {
	sessionTokens    *platformcrypto.TokenCodec
	csrfTokens       *platformcrypto.TokenCodec
	passwords        *platformcrypto.PasswordHasher
	attemptSubject   *platformcrypto.Fingerprinter
	attemptIP        *platformcrypto.Fingerprinter
	totpCipher       *platformcrypto.EnvelopeCipher
	recoveryTokens   *platformcrypto.BoundTokenCodec
	recoveryDelivery *platformcrypto.EnvelopeCipher
	recoveryCodes    *platformcrypto.RecoveryCodeCodec
	dummyHash        string
}

func newIdentitySecurity(cfg config.Config) (identitySecurity, error) {
	sessionTokens, err := platformcrypto.NewTokenCodec(
		[]byte(cfg.Session.TokenPepper),
		"identity.session.v1",
	)
	if err != nil {
		return identitySecurity{}, err
	}
	csrfTokens, err := platformcrypto.NewTokenCodec(
		[]byte(cfg.Session.CSRFTokenPepper),
		"identity.csrf.v1",
	)
	if err != nil {
		return identitySecurity{}, err
	}
	passwords, err := platformcrypto.NewPasswordHasher(defaultPasswordPolicy())
	if err != nil {
		return identitySecurity{}, err
	}
	dummyHash, err := passwords.Hash("constant-time-dummy-password")
	if err != nil {
		return identitySecurity{}, err
	}
	attemptSubject, err := platformcrypto.NewFingerprinter(
		[]byte(cfg.Authentication.AttemptFingerprintKey),
		"identity.auth-attempt.subject.v1",
	)
	if err != nil {
		return identitySecurity{}, err
	}
	attemptIP, err := platformcrypto.NewFingerprinter(
		[]byte(cfg.Authentication.AttemptFingerprintKey),
		"identity.auth-attempt.ip.v1",
	)
	if err != nil {
		return identitySecurity{}, err
	}
	totpKeys, err := config.ParseTOTPKeyring(
		cfg.Session.TOTPActiveKeyVersion,
		cfg.Session.TOTPEncryptionKeys,
	)
	if err != nil {
		return identitySecurity{}, err
	}
	totpCipher, err := platformcrypto.NewEnvelopeCipher(totpKeys)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryTokenKeys, err := config.ParseRecoveryTokenKeyring(
		cfg.Authentication.Recovery.TokenActiveKeyVersion,
		cfg.Authentication.Recovery.TokenHMACKeys,
	)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryTokens, err := platformcrypto.NewBoundTokenCodec(recoveryTokenKeys)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryDeliveryKeys, err := config.ParseRecoveryDeliveryKeyring(
		cfg.Authentication.Recovery.DeliveryActiveKeyVersion,
		cfg.Authentication.Recovery.DeliveryEncryptionKeys,
	)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryDelivery, err := platformcrypto.NewEnvelopeCipher(recoveryDeliveryKeys)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryCodeKeys, err := config.ParseRecoveryCodeKeyring(
		cfg.Authentication.Recovery.CodeActiveKeyVersion,
		cfg.Authentication.Recovery.CodeHMACKeys,
	)
	if err != nil {
		return identitySecurity{}, err
	}
	recoveryCodes, err := platformcrypto.NewRecoveryCodeCodec(recoveryCodeKeys)
	if err != nil {
		return identitySecurity{}, err
	}
	return identitySecurity{
		sessionTokens: sessionTokens, csrfTokens: csrfTokens,
		passwords: passwords, attemptSubject: attemptSubject,
		attemptIP: attemptIP, totpCipher: totpCipher,
		recoveryTokens: recoveryTokens, recoveryDelivery: recoveryDelivery,
		recoveryCodes: recoveryCodes, dummyHash: dummyHash,
	}, nil
}

type identityUseCases struct {
	createAnonymous *sessionapp.CreateAnonymous
	login           *authapp.LoginFlow
	revoke          *sessionapp.Revoke
	enrollMFA       *mfaapp.Enroller
	confirmMFA      *mfaapp.Confirmer
	verifyMFA       *mfaapp.Verifier
	verifyRecovery  *mfaapp.RecoveryVerifier
	requestRecovery *authapp.RequestRecovery
	resetPassword   *authapp.ResetPassword
}

func newIdentityUseCases(
	cfg config.Config,
	repositories identityRepositories,
	security identitySecurity,
) (identityUseCases, error) {
	attempts, err := authapp.NewAttemptPolicy(authapp.AttemptPolicyDependencies{
		Repository:           repositories.attempts,
		SubjectFingerprinter: security.attemptSubject,
		IPFingerprinter:      security.attemptIP,
		Now:                  time.Now,
		Config: ports.AuthAttemptPolicy{
			Threshold:    cfg.Authentication.Attempts.Threshold,
			Window:       cfg.Authentication.Attempts.Window,
			LockDuration: cfg.Authentication.Attempts.LockDuration,
		},
	})
	if err != nil {
		return identityUseCases{}, err
	}
	passwordLogin, err := authapp.NewPasswordLogin(authapp.PasswordLoginDependencies{
		Users: repositories.users, Passwords: repositories.passwords,
		Verifier: security.passwords, Attempts: attempts,
		DummyHash: security.dummyHash,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	createAnonymous, err := sessionapp.NewCreateAnonymous(sessionapp.CreateAnonymousDependencies{
		Repository: repositories.sessions, SessionTokens: security.sessionTokens,
		CSRFTokens: security.csrfTokens, NewID: newRuntimeID, Now: time.Now,
		IdleTTL: cfg.Session.IdleTimeout, AbsoluteTTL: cfg.Session.AbsoluteTimeout,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	rotator, err := sessionapp.NewRotator(sessionapp.RotatorDependencies{
		Repository: repositories.sessions, SessionTokens: security.sessionTokens,
		CSRFTokens: security.csrfTokens, NewID: newRuntimeID, Now: time.Now,
		IdleTTL: cfg.Session.IdleTimeout, AbsoluteTTL: cfg.Session.AbsoluteTimeout,
		MFAPendingIdleTTL:     mfaPendingIdleTTL,
		MFAPendingAbsoluteTTL: mfaPendingAbsoluteTTL,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	login, err := authapp.NewLoginFlow(passwordLogin, repositories.mfa, rotator)
	if err != nil {
		return identityUseCases{}, err
	}
	revoke, err := sessionapp.NewRevoke(sessionapp.RevokeDependencies{
		Repository: repositories.sessions, Tokens: security.sessionTokens, Now: time.Now,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	enrollMFA, err := mfaapp.NewEnroller(mfaapp.EnrollerDependencies{
		Repository: repositories.mfa, Cipher: security.totpCipher,
		Random: rand.Reader, NewID: newRuntimeID, Now: time.Now,
		PendingTTL: mfaEnrollmentTTL, Issuer: cfg.App.Name,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	confirmMFA, err := mfaapp.NewConfirmer(mfaapp.ConfirmerDependencies{
		Repository: repositories.mfa, Cipher: security.totpCipher,
		Codes: security.recoveryCodes, Now: time.Now,
		CodeCount: mfaapp.DefaultRecoveryCodeCount,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	verifyMFA, err := mfaapp.NewVerifier(mfaapp.VerifierDependencies{
		Repository: repositories.mfa, Cipher: security.totpCipher,
		SessionTokens: security.sessionTokens, CSRFTokens: security.csrfTokens,
		NewID: newRuntimeID, Now: time.Now,
		IdleTTL: cfg.Session.IdleTimeout, AbsoluteTTL: cfg.Session.AbsoluteTimeout,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	verifyRecovery, err := mfaapp.NewRecoveryVerifier(mfaapp.RecoveryVerifierDependencies{
		Repository: repositories.mfa, Codes: security.recoveryCodes,
		SessionTokens: security.sessionTokens, CSRFTokens: security.csrfTokens,
		NewID: newRuntimeID, Now: time.Now,
		IdleTTL: cfg.Session.IdleTimeout, AbsoluteTTL: cfg.Session.AbsoluteTimeout,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	requestRecovery, err := authapp.NewRequestRecovery(authapp.RequestRecoveryDependencies{
		Users: repositories.users, Tokens: security.recoveryTokens,
		Delivery: security.recoveryDelivery, Repository: repositories.oneTime,
		NewID: newRuntimeID, Now: time.Now, TTL: passwordRecoveryTTL,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	resetPassword, err := authapp.NewResetPassword(authapp.ResetPasswordDependencies{
		Repository: repositories.oneTime, Tokens: security.recoveryTokens,
		Passwords: security.passwords, SessionTokens: security.sessionTokens,
		CSRFTokens: security.csrfTokens, NewID: newRuntimeID, Now: time.Now,
		IdleTTL: cfg.Session.IdleTimeout, AbsoluteTTL: cfg.Session.AbsoluteTimeout,
		MFAIdleTTL: mfaPendingIdleTTL, MFAAbsoluteTTL: mfaPendingAbsoluteTTL,
	})
	if err != nil {
		return identityUseCases{}, err
	}
	return identityUseCases{
		createAnonymous: createAnonymous, login: login, revoke: revoke,
		enrollMFA: enrollMFA, confirmMFA: confirmMFA,
		verifyMFA: verifyMFA, verifyRecovery: verifyRecovery,
		requestRecovery: requestRecovery, resetPassword: resetPassword,
	}, nil
}

func newIdentityHandlers(
	useCases identityUseCases,
	repositories identityRepositories,
	cookie *identityhttp.SessionCookie,
	trustedProxies config.TrustedProxySet,
) (identityhttp.RouteHandlers, error) {
	csrfHandler, err := identityhttp.NewCSRFHandler(useCases.createAnonymous, cookie.Write)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	loginHandler, err := identityhttp.NewLoginHandler(
		useCases.login,
		cookie.Write,
		func(request *nethttp.Request) (string, error) {
			return platformmiddleware.ResolveClientAddress(request, trustedProxies)
		},
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	sessionHandler, err := identityhttp.NewSessionHandler(
		useCases.revoke,
		func(ctx *gin.Context) (string, error) {
			return ctx.Cookie(cookie.Name())
		},
		cookie.Clear,
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	sessionQuery, err := identityhttp.NewSessionQueryHandler(
		repositories.users,
		repositories.sessions,
		time.Now,
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	mfaEnrollment, err := identityhttp.NewMFAEnrollmentHandler(
		useCases.enrollMFA,
		repositories.users,
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	mfaConfirmation, err := identityhttp.NewMFAConfirmationHandler(useCases.confirmMFA)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	mfaHandler, err := identityhttp.NewMFAHandler(
		useCases.verifyMFA,
		useCases.verifyRecovery,
		cookie.Write,
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	recoveryHandler, err := identityhttp.NewRecoveryHandler(
		useCases.requestRecovery,
		useCases.resetPassword,
		cookie.Write,
	)
	if err != nil {
		return identityhttp.RouteHandlers{}, err
	}
	return identityhttp.RouteHandlers{
		CSRF: csrfHandler, Login: loginHandler, MFA: mfaHandler,
		MFAConfirmation: mfaConfirmation, MFAEnrollment: mfaEnrollment,
		Recovery: recoveryHandler, Session: sessionHandler,
		SessionQuery: sessionQuery,
	}, nil
}
