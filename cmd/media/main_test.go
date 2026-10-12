package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/fluentra/fluentra/internal/generated/resource/sqlc"
	"github.com/fluentra/fluentra/internal/platform/media/rendition"
	"github.com/fluentra/fluentra/internal/platform/storage"
)

const (
	flagAll   = "-all"
	flagLimit = "-limit"
)

// The filter narrows the claim, so it must name concrete kinds: a category
// expands, a kind stands for itself, nothing repeats, and no filter is nil.
func TestExpandKindsFilter(t *testing.T) {
	assert.Nil(t, expandKindsFilter(""))
	assert.Equal(t,
		[]string{rendition.KindThumbnail, rendition.KindDisplay, rendition.KindPreview},
		expandKindsFilter("image, pdf"))
	assert.Equal(t, []string{rendition.KindAudioWeb}, expandKindsFilter("audio_web,AUDIO"))
}

func TestParseOptions_RefusesALimitOutsideTheBatchBounds(t *testing.T) {
	_, err := parseOptions([]string{flagAll, flagLimit, "0"})
	assert.Error(t, err)
	_, err = parseOptions([]string{flagAll, flagLimit, "5000"})
	assert.Error(t, err)

	opts, err := parseOptions([]string{flagAll, flagLimit, "10", "-kinds", "video"})
	assert.NoError(t, err)
	assert.Equal(t, int32(10), opts.limit)
	assert.Len(t, opts.kinds, 4)
}

type renditionSourceStore struct {
	storage.Store
	data   []byte
	getErr error
}

func (s renditionSourceStore) Get(_ context.Context, _, _ string) (io.ReadCloser, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func TestProcessRendition_PreservesCallerWorkspace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		getErr error
	}{
		{name: "HP-01 rendered PNG remains readable after return"},
		{name: "ERR-01 download failure leaves caller sentinel intact", getErr: storage.ErrObjectNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			sentinelPath := filepath.Join(tmpDir, "caller-owned.txt")
			sentinel := []byte("caller owns this workspace")
			require.NoError(t, os.WriteFile(sentinelPath, sentinel, 0600))
			resourceID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(t.Name()))
			objectKey := "original.png"
			item := sqlc.ResourceRendition{ResourceID: resourceID, Kind: rendition.KindThumbnail}
			res := sqlc.ResourceResource{ID: resourceID, ObjectKey: &objectKey, DetectedMime: "image/png"}
			img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
			pixel := color.NRGBA{R: 30, G: 60, B: 90, A: 255}
			img.SetNRGBA(0, 0, pixel)
			var source bytes.Buffer
			require.NoError(t, png.Encode(&source, img))
			store := renditionSourceStore{data: source.Bytes(), getErr: tc.getErr}

			result, err := processRendition(context.Background(), store, item, res, mediaOptions{}, tmpDir)

			if tc.getErr != nil {
				require.ErrorIs(t, err, tc.getErr)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.False(t, result.Skipped)
				require.Equal(t, "image/png", result.MIMEType)
				require.Equal(t, tmpDir, filepath.Dir(result.OutputPath))
				require.FileExists(t, result.OutputPath)
				data, readErr := os.ReadFile(result.OutputPath)
				require.NoError(t, readErr)
				require.NotEmpty(t, data)
				require.NotNil(t, result.ByteSize)
				require.Equal(t, int64(len(data)), *result.ByteSize)
				rendered, decodeErr := png.Decode(bytes.NewReader(data))
				require.NoError(t, decodeErr)
				require.Equal(t, img.Bounds(), rendered.Bounds())
				require.Equal(t, pixel, color.NRGBAModel.Convert(rendered.At(0, 0)))
			}
			data, readErr := os.ReadFile(sentinelPath) //nolint:gosec // fixed sentinel name inside t.TempDir, not user input
			require.NoError(t, readErr)
			require.Equal(t, sentinel, data)
		})
	}
}
