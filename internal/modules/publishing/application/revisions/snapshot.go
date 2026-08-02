package revisions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
)

const maxRevisionSnapshotBytes = 1 << 20

var (
	ErrInvalidRevisionSnapshot  = errors.New("publishing: invalid revision snapshot")
	ErrRevisionSnapshotMismatch = errors.New("publishing: revision snapshot mismatch")
)

type Snapshot struct {
	Kind       publishingdomain.Kind    `json:"kind"`
	ContentKey string                   `json:"content_key"`
	LocaleID   string                   `json:"locale_id"`
	Slug       string                   `json:"slug"`
	Path       string                   `json:"path"`
	Title      string                   `json:"title"`
	Excerpt    string                   `json:"excerpt,omitempty"`
	Blocks     []publishingdomain.Block `json:"blocks"`
	SEO        SnapshotSEO              `json:"seo"`
	GEO        SnapshotGEO              `json:"geo"`
	AEO        SnapshotAEO              `json:"aeo"`
	Status     publishingdomain.Status  `json:"status"`
}

type SnapshotSEO struct {
	Title          string         `json:"title,omitempty"`
	Description    string         `json:"description,omitempty"`
	CanonicalURL   string         `json:"canonical_url,omitempty"`
	Robots         string         `json:"robots,omitempty"`
	StructuredData map[string]any `json:"structured_data,omitempty"`
}

type SnapshotGEO struct {
	Region   string `json:"region,omitempty"`
	Locality string `json:"locality,omitempty"`
}

type SnapshotAEO struct {
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

func SnapshotFromContent(detail ports.ContentDetail) Snapshot {
	return Snapshot{
		Kind: detail.Kind, ContentKey: detail.ContentKey, LocaleID: detail.LocaleID,
		Slug: detail.Slug, Path: detail.Path, Title: detail.Title, Excerpt: detail.Excerpt,
		Blocks: detail.Blocks,
		SEO: SnapshotSEO{
			Title: detail.SEO.Title, Description: detail.SEO.Description,
			CanonicalURL: detail.SEO.CanonicalURL, Robots: detail.SEO.Robots,
			StructuredData: detail.SEO.StructuredData,
		},
		GEO:    SnapshotGEO{Region: detail.GEO.Region, Locality: detail.GEO.Locality},
		AEO:    SnapshotAEO{Question: detail.AEO.Question, Answer: detail.AEO.Answer},
		Status: detail.TranslationStatus,
	}
}

func snapshotDetail(snapshot Snapshot, current ports.ContentDetail, updatedAt time.Time) ports.ContentDetail {
	current.Kind = snapshot.Kind
	current.ContentKey = snapshot.ContentKey
	current.Slug = snapshot.Slug
	current.Path = snapshot.Path
	current.Title = snapshot.Title
	current.Excerpt = snapshot.Excerpt
	current.Blocks = snapshot.Blocks
	current.SEO = publishingdomain.SEOFields{
		Title: snapshot.SEO.Title, Description: snapshot.SEO.Description,
		CanonicalURL: snapshot.SEO.CanonicalURL, Robots: snapshot.SEO.Robots,
		StructuredData: snapshot.SEO.StructuredData,
	}
	current.GEO = publishingdomain.GEOFields{Region: snapshot.GEO.Region, Locality: snapshot.GEO.Locality}
	current.AEO = publishingdomain.AEOFields{Question: snapshot.AEO.Question, Answer: snapshot.AEO.Answer}
	current.ContentStatus = snapshot.Status
	current.TranslationStatus = snapshot.Status
	current.ContentVersion++
	current.TranslationVersion++
	current.UpdatedAt = updatedAt.UTC()
	return current
}

func encodeSnapshot(snapshot Snapshot) ([]byte, string, error) {
	if err := validateSnapshot(snapshot); err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil || len(raw) > maxRevisionSnapshotBytes {
		return nil, "", ErrInvalidRevisionSnapshot
	}
	hash := sha256.Sum256(raw)
	return raw, hex.EncodeToString(hash[:]), nil
}

func decodeSnapshot(raw []byte, expectedHash string) (Snapshot, error) {
	if len(raw) == 0 || len(raw) > maxRevisionSnapshotBytes {
		return Snapshot{}, ErrInvalidRevisionSnapshot
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, ErrInvalidRevisionSnapshot
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Snapshot{}, ErrInvalidRevisionSnapshot
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	_, actualHash, err := encodeSnapshot(snapshot)
	if err != nil || !strings.EqualFold(actualHash, expectedHash) {
		return Snapshot{}, ErrRevisionSnapshotMismatch
	}
	return snapshot, nil
}

func validateSnapshot(snapshot Snapshot) error {
	if snapshot.Kind != publishingdomain.KindPage && snapshot.Kind != publishingdomain.KindPost {
		return ErrInvalidRevisionSnapshot
	}
	if strings.TrimSpace(snapshot.LocaleID) == "" || strings.TrimSpace(snapshot.ContentKey) == "" ||
		strings.TrimSpace(snapshot.Title) == "" || strings.TrimSpace(snapshot.Path) == "" {
		return ErrInvalidRevisionSnapshot
	}
	if publishingdomain.ValidateStatus(snapshot.Status) != nil {
		return ErrInvalidRevisionSnapshot
	}
	if err := contentapp.ValidateBlocks(snapshot.Blocks); err != nil {
		return ErrInvalidRevisionSnapshot
	}
	return nil
}
