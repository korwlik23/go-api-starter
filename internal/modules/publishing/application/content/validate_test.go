package content

import (
	"encoding/json"
	"testing"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A3ValidatorAcceptsDeclaredBlocksAndMetadata(t *testing.T) {
	t.Parallel()

	err := Validate(ContentValidationInput{
		Locale: "th",
		Slug:   "getting-started",
		Path:   "/docs/getting-started",
		Title:  "เริ่มต้นใช้งาน",
		Blocks: []publishingdomain.Block{
			{Type: publishingdomain.BlockText, Data: map[string]any{"text": "อ่านคู่มือ"}},
			{Type: publishingdomain.BlockAnswer, Data: map[string]any{"answer": "ทำตามขั้นตอนนี้"}},
		},
		SEO: publishingdomain.SEOFields{
			Title:       "คู่มือเริ่มต้นใช้งาน",
			Description: "คำอธิบายสำหรับผลการค้นหา",
		},
		AEO: publishingdomain.AEOFields{
			Question: "เริ่มต้นอย่างไร",
			Answer:   "ทำตามขั้นตอนในหน้านี้",
		},
	})
	require.NoError(t, err)
}

func TestM3A3ValidatorRejectsUnknownBlockUnsafeURLAndNoVisibleText(t *testing.T) {
	t.Parallel()

	base := ContentValidationInput{
		Locale: "en",
		Slug:   "safe",
		Path:   "/safe",
		Title:  "Safe",
	}
	for _, test := range []struct {
		name  string
		input ContentValidationInput
		err   error
	}{
		{
			name: "unknown block",
			input: withBlocks(base, publishingdomain.Block{
				Type: "video",
				Data: map[string]any{"url": "https://example.test/video"},
			}),
			err: ErrUnknownBlockType,
		},
		{
			name: "unsafe url",
			input: withBlocks(base, publishingdomain.Block{
				Type: publishingdomain.BlockImage,
				Data: map[string]any{
					"asset_id": "asset-1",
					"alt":      "ภาพ",
					"url":      "javascript:alert(1)",
				},
			}),
			err: ErrUnsafeURL,
		},
		{
			name: "no visible text",
			input: withBlocks(base, publishingdomain.Block{
				Type: publishingdomain.BlockImage,
				Data: map[string]any{"asset_id": "asset-1", "alt": ""},
			}),
			err: ErrNoVisibleText,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.input)
			require.ErrorIs(t, err, test.err)
		})
	}
}

func TestM3A3DecodeBlocksIsStrictAndRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	valid, err := DecodeBlocks(json.RawMessage(`[{"type":"text","data":{"text":"hello"}}]`))
	require.NoError(t, err)
	require.Len(t, valid, 1)

	_, err = DecodeBlocks(json.RawMessage(`[{"type":"text","data":{"text":"hello"},"extra":true}]`))
	require.ErrorIs(t, err, ErrInvalidBlockJSON)
	_, err = DecodeBlocks(json.RawMessage(`[{"type":"text","data":{"text":"hello"}}] trailing`))
	require.ErrorIs(t, err, ErrInvalidBlockJSON)
}

func withBlocks(input ContentValidationInput, blocks ...publishingdomain.Block) ContentValidationInput {
	input.Blocks = blocks
	return input
}
