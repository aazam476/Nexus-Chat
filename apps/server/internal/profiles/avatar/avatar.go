// Package avatar turns an uploaded profile picture into the stored one:
// cropped to the square the user chose, resized to 512×512 and encoded
// again (spec §4.1).
//
// Encoding again matters for privacy as much as for size: the output keeps
// no EXIF data, so a phone photo's GPS position never reaches other users.
// Images are checked against size limits before they are decoded, so a
// small file that claims enormous dimensions cannot exhaust memory.
//
// Start reading at [Process].
package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// Limits on uploads (spec §4.1) and on what we agree to decode.
const (
	// MaximumUploadBytes is the largest accepted file.
	MaximumUploadBytes = 8 << 20
	// Size is the width and height of every stored avatar.
	Size = 512
	// maximumSide and maximumPixels bound the decoded image: 8192×8192 at
	// four bytes a pixel is 256 MiB, the most one upload may cost.
	maximumSide   = 8192
	maximumPixels = maximumSide * maximumSide
	// maximumFrames and maximumFramePixels bound animated GIFs.
	maximumFrames      = 300
	maximumFramePixels = 100_000_000
	jpegQuality        = 90
)

// Format is how a stored avatar is encoded.
type Format string

// Stored formats. WebP uploads are stored as PNG: Go can read WebP but has
// no WebP encoder in its standard or x/ libraries.
const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
	FormatGIF  Format = "gif"
)

// ContentType is the MIME type to serve the format with.
func (format Format) ContentType() string {
	return "image/" + string(format)
}

// Errors for uploads that cannot become an avatar. Their messages are shown
// to the user.
var (
	ErrUnsupported = errors.New("the picture must be a PNG, JPEG, WebP or GIF image")
	ErrTooLarge    = fmt.Errorf("the picture may be at most %d MB and %d pixels on each side", MaximumUploadBytes>>20, maximumSide)
	ErrBadCrop     = errors.New("the crop square must lie inside the picture")
	ErrDamaged     = errors.New("the picture could not be read; it may be damaged")
)

// Crop is the square, in the uploaded image's pixels, that the user chose.
type Crop struct {
	X    int
	Y    int
	Size int
}

// Result is a processed avatar.
type Result struct {
	Data   []byte
	Format Format
}

// Process crops and resizes data. A nil crop takes the largest centered
// square.
func Process(data []byte, crop *Crop) (Result, error) {
	if len(data) > MaximumUploadBytes {
		return Result{}, ErrTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrUnsupported
	}
	if config.Width > maximumSide || config.Height > maximumSide || config.Width*config.Height > maximumPixels {
		return Result{}, ErrTooLarge
	}
	square, err := cropSquare(config.Width, config.Height, crop)
	if err != nil {
		return Result{}, err
	}
	if format == "gif" {
		return processGIF(data, square)
	}
	return processStill(data, format, square)
}

// cropSquare checks the user's square, or picks the centered one.
func cropSquare(width int, height int, crop *Crop) (image.Rectangle, error) {
	if crop == nil {
		side := min(width, height)
		x, y := (width-side)/2, (height-side)/2
		return image.Rect(x, y, x+side, y+side), nil
	}
	if crop.Size < 1 || crop.X < 0 || crop.Y < 0 || crop.X+crop.Size > width || crop.Y+crop.Size > height {
		return image.Rectangle{}, ErrBadCrop
	}
	return image.Rect(crop.X, crop.Y, crop.X+crop.Size, crop.Y+crop.Size), nil
}

func processStill(data []byte, format string, square image.Rectangle) (Result, error) {
	var decoded image.Image
	var err error
	switch format {
	case "png":
		decoded, err = png.Decode(bytes.NewReader(data))
	case "jpeg":
		decoded, err = jpeg.Decode(bytes.NewReader(data))
	case "webp":
		decoded, err = webp.Decode(bytes.NewReader(data))
	default:
		return Result{}, ErrUnsupported
	}
	if err != nil {
		return Result{}, ErrDamaged
	}
	resized := scale(decoded, square)
	var output bytes.Buffer
	if format == "jpeg" {
		// Photos stay JPEG: as PNG they would be several times larger.
		// Encoding to a buffer cannot fail.
		_ = jpeg.Encode(&output, resized, &jpeg.Options{Quality: jpegQuality})
		return Result{Data: output.Bytes(), Format: FormatJPEG}, nil
	}
	_ = png.Encode(&output, resized)
	return Result{Data: output.Bytes(), Format: FormatPNG}, nil
}

// scale draws the square of source into a new 512×512 image. CatmullRom
// is a high-quality filter for both shrinking and enlarging.
func scale(source image.Image, square image.Rectangle) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, Size, Size))
	draw.CatmullRom.Scale(target, target.Bounds(), source, square.Add(source.Bounds().Min), draw.Src, nil)
	return target
}
