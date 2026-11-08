package avatar

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoColors is red on the left half and blue on the right.
func twoColors(width int, height int) *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			shade := color.RGBA{R: 220, A: 255}
			if x >= width/2 {
				shade = color.RGBA{B: 220, A: 255}
			}
			picture.Set(x, y, shade)
		}
	}
	return picture
}

func encodePNG(t *testing.T, picture image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, picture))
	return buffer.Bytes()
}

func decode(t *testing.T, result Result) image.Image {
	t.Helper()
	decoded, format, err := image.Decode(bytes.NewReader(result.Data))
	require.NoError(t, err)
	assert.Equal(t, string(result.Format), format)
	assert.Equal(t, image.Rect(0, 0, Size, Size), decoded.Bounds())
	return decoded
}

func isRed(shade color.Color) bool {
	r, _, b, _ := shade.RGBA()
	return r > 0xA000 && b < 0x3000
}

func isBlue(shade color.Color) bool {
	r, _, b, _ := shade.RGBA()
	return b > 0xA000 && r < 0x3000
}

func TestPNGIsCroppedAndResized(t *testing.T) {
	result, err := Process(encodePNG(t, twoColors(200, 100)), &Crop{X: 0, Y: 0, Size: 100})

	require.NoError(t, err)
	assert.Equal(t, FormatPNG, result.Format)
	assert.Equal(t, "image/png", result.Format.ContentType())
	decoded := decode(t, result)
	assert.True(t, isRed(decoded.At(10, 10)) && isRed(decoded.At(500, 500)), "only the red half was chosen")
}

func TestWithoutCropTheCenterSquareIsUsed(t *testing.T) {
	result, err := Process(encodePNG(t, twoColors(300, 100)), nil)

	require.NoError(t, err)
	decoded := decode(t, result)
	assert.True(t, isRed(decoded.At(100, 256)))
	assert.True(t, isBlue(decoded.At(400, 256)))

	tall, err := Process(encodePNG(t, twoColors(100, 300)), nil)
	require.NoError(t, err)
	decode(t, tall)
}

func TestJPEGStaysJPEGWithoutMetadata(t *testing.T) {
	var buffer bytes.Buffer
	require.NoError(t, jpeg.Encode(&buffer, twoColors(64, 64), nil))
	original := buffer.Bytes()
	// An APP1 (EXIF) segment with a GPS marker, right after SOI.
	exif := append([]byte("Exif\x00\x00"), []byte("GPS 51.5N 0.12W")...)
	segment := append([]byte{0xFF, 0xE1, 0, byte(len(exif) + 2)}, exif...) //nolint:gosec // G115: the segment is 23 bytes
	withEXIF := append(append(append([]byte{}, original[:2]...), segment...), original[2:]...)

	result, err := Process(withEXIF, nil)

	require.NoError(t, err)
	assert.Equal(t, FormatJPEG, result.Format)
	decode(t, result)
	assert.NotContains(t, string(result.Data), "Exif")
	assert.NotContains(t, string(result.Data), "GPS")
}

func TestWebPIsStoredAsPNG(t *testing.T) {
	data, err := os.ReadFile("testdata/still.webp")
	require.NoError(t, err)

	result, err := Process(data, &Crop{X: 20, Y: 0, Size: 20})

	require.NoError(t, err)
	assert.Equal(t, FormatPNG, result.Format)
	assert.True(t, isBlue(decode(t, result).At(256, 256)))
}

func TestAnimatedWebPIsRejected(t *testing.T) {
	data, err := os.ReadFile("testdata/animated.webp")
	require.NoError(t, err)

	_, err = Process(data, nil)

	assert.Error(t, err)
}

// animatedGIF has three 20×20 frames: a full red frame, a small blue
// patch drawn over it, and a green patch whose area is cleared afterwards.
func animatedGIF(t *testing.T, disposals []byte) []byte {
	t.Helper()
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	full := image.NewPaletted(image.Rect(0, 0, 20, 20), palette)
	patch := image.NewPaletted(image.Rect(10, 10, 20, 20), palette)
	for index := range patch.Pix {
		patch.Pix[index] = 1
	}
	green := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	for index := range green.Pix {
		green.Pix[index] = 2
	}
	var buffer bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buffer, &gif.GIF{
		Image: []*image.Paletted{full, patch, green}, Delay: []int{10, 20, 30}, LoopCount: 0,
		Disposal: disposals, Config: image.Config{Width: 20, Height: 20, ColorModel: palette},
	}))
	return buffer.Bytes()
}

func TestAnimatedGIFKeepsEveryFrameAndItsTiming(t *testing.T) {
	data := animatedGIF(t, []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalBackground})

	result, err := Process(data, nil)

	require.NoError(t, err)
	assert.Equal(t, FormatGIF, result.Format)
	animation, err := gif.DecodeAll(bytes.NewReader(result.Data))
	require.NoError(t, err)
	require.Len(t, animation.Image, 3)
	assert.Equal(t, []int{10, 20, 30}, animation.Delay)
	assert.Equal(t, image.Rect(0, 0, Size, Size), animation.Image[0].Bounds())
	second := animation.Image[1]
	assert.True(t, isBlue(second.At(500, 500)), "the patch is drawn over the first frame")
	assert.True(t, isRed(second.At(10, 10)))
	third := animation.Image[2]
	assert.True(t, isRed(third.At(500, 500)), "DisposalPrevious restored the red frame under the patch")
}

func TestGIFWithoutDisposalsAndFullPalette(t *testing.T) {
	palette := make(color.Palette, 256)
	for index := range palette {
		palette[index] = color.RGBA{R: uint8(index), A: 255}
	}
	palette[255] = color.Transparent
	frame := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)
	var buffer bytes.Buffer
	require.NoError(t, gif.Encode(&buffer, frame, nil))

	result, err := Process(buffer.Bytes(), nil)

	require.NoError(t, err)
	assert.Equal(t, FormatGIF, result.Format)
	assert.True(t, hasTransparent(palette))
	assert.Equal(t, byte(gif.DisposalNone), disposal(&gif.GIF{}, 0))
}

// withSize rewrites a PNG's declared dimensions, keeping its CRC valid.
func withSize(t *testing.T, data []byte, width uint32, height uint32) []byte {
	t.Helper()
	changed := append([]byte{}, data...)
	binary.BigEndian.PutUint32(changed[16:], width)
	binary.BigEndian.PutUint32(changed[20:], height)
	binary.BigEndian.PutUint32(changed[29:], crc32.ChecksumIEEE(changed[12:29]))
	return changed
}

func TestUploadsThatCannotBecomeAnAvatar(t *testing.T) {
	small := encodePNG(t, twoColors(10, 10))
	tooManyFrames := animatedGIFWithFrames(t, maximumFrames+1)
	tests := []struct {
		name string
		data []byte
		crop *Crop
		want error
	}{
		{"not an image", []byte("hello"), nil, ErrUnsupported},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), nil, ErrUnsupported},
		{"file too big", make([]byte, MaximumUploadBytes+1), nil, ErrTooLarge},
		{"claims huge width", withSize(t, small, 100_000, 10), nil, ErrTooLarge},
		{"claims huge area", withSize(t, small, 8192, 8192+1), nil, ErrTooLarge},
		{"truncated", withSize(t, small, 10, 10)[:60], nil, ErrDamaged},
		{"crop outside", small, &Crop{X: 5, Y: 5, Size: 6}, ErrBadCrop},
		{"crop negative", small, &Crop{X: -1, Y: 0, Size: 5}, ErrBadCrop},
		{"crop empty", small, &Crop{Size: 0}, ErrBadCrop},
		{"crop negative y", small, &Crop{Y: -1, Size: 5}, ErrBadCrop},
		{"crop too tall", small, &Crop{Y: 6, Size: 5}, ErrBadCrop},
		{"too many frames", tooManyFrames, nil, ErrTooLarge},
		{"damaged gif", tooManyFrames[:40], nil, ErrDamaged},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Process(test.data, test.crop)

			assert.ErrorIs(t, err, test.want)
		})
	}
}

func animatedGIFWithFrames(t *testing.T, count int) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	animation := &gif.GIF{Config: image.Config{Width: 2, Height: 2, ColorModel: palette}}
	for range count {
		animation.Image = append(animation.Image, image.NewPaletted(image.Rect(0, 0, 2, 2), palette))
		animation.Delay = append(animation.Delay, 1)
	}
	var buffer bytes.Buffer
	require.NoError(t, gif.EncodeAll(&buffer, animation))
	return buffer.Bytes()
}

func TestDecodersForOtherFormatsAreNotUsed(t *testing.T) {
	_, err := processStill(nil, "bmp", image.Rect(0, 0, 1, 1))

	assert.ErrorIs(t, err, ErrUnsupported)
}
