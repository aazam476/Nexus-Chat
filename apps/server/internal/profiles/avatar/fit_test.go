package avatar

import (
	"bytes"
	"image"
	"image/gif"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emojiLimits = Limits{MaximumBytes: 256 << 10, MaximumSide: 128, Formats: []string{"png", "gif", "webp"}}

func fitBounds(t *testing.T, result Result) image.Rectangle {
	t.Helper()
	decoded, format, err := image.Decode(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Equal(t, string(result.Format), format)
	return decoded.Bounds()
}

func TestLargePicturesAreScaledDownKeepingTheirShape(t *testing.T) {
	wide, err := Fit(encodePNG(t, twoColors(512, 256)), emojiLimits)
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 128, 64), fitBounds(t, wide))

	tall, err := Fit(encodePNG(t, twoColors(100, 400)), emojiLimits)
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 32, 128), fitBounds(t, tall))

	sliver, err := Fit(encodePNG(t, twoColors(1000, 2)), emojiLimits)
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 128, 1), fitBounds(t, sliver))
}

func TestSmallPicturesKeepTheirSize(t *testing.T) {
	result, err := Fit(encodePNG(t, twoColors(64, 48)), emojiLimits)

	require.NoError(t, err)
	assert.Equal(t, FormatPNG, result.Format)
	assert.Equal(t, image.Rect(0, 0, 64, 48), fitBounds(t, result))
}

func TestFittedWebPBecomesPNG(t *testing.T) {
	data, err := os.ReadFile("testdata/still.webp")
	require.NoError(t, err)

	result, err := Fit(data, emojiLimits)

	require.NoError(t, err)
	assert.Equal(t, FormatPNG, result.Format)
}

func TestFittedGIFKeepsItsFrames(t *testing.T) {
	data := animatedGIF(t, []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalBackground})

	result, err := Fit(data, Limits{MaximumBytes: 1 << 20, MaximumSide: 10, Formats: []string{"gif"}})

	require.NoError(t, err)
	assert.Equal(t, FormatGIF, result.Format)
	animation, err := gif.DecodeAll(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Len(t, animation.Image, 3)
	assert.Equal(t, []int{10, 20, 30}, animation.Delay)
	assert.Equal(t, image.Rect(0, 0, 10, 10), animation.Image[0].Bounds())
}

func TestPicturesThatCannotBeFitted(t *testing.T) {
	small := encodePNG(t, twoColors(10, 10))
	// A GIF's header gives its width as two little-endian bytes at offset 6.
	noWidth := animatedGIF(t, nil)
	noWidth[6], noWidth[7] = 0, 0
	tests := []struct {
		name   string
		data   []byte
		limits Limits
		want   error
	}{
		{"too many bytes", small, Limits{MaximumBytes: 10, MaximumSide: 128, Formats: []string{"png"}}, ErrTooLarge},
		{"not an image", []byte("hello"), emojiLimits, ErrUnsupported},
		{"format not allowed", small, Limits{MaximumBytes: 1 << 20, MaximumSide: 128, Formats: []string{"gif"}}, ErrNotAllowed},
		{"no width", noWidth, emojiLimits, ErrDamaged},
		{"claims huge width", withSize(t, small, 100_000, 10), emojiLimits, ErrTooLarge},
		{"claims huge area", withSize(t, small, 8192, 8192+1), emojiLimits, ErrTooLarge},
		{"truncated", withSize(t, small, 10, 10)[:60], emojiLimits, ErrDamaged},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Fit(test.data, test.limits)

			assert.ErrorIs(t, err, test.want)
		})
	}
}
