package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/platform/media/rendition"
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
