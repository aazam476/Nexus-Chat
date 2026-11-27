package avatar

import (
	"bytes"
	"errors"
	"image"
)

// Small pictures that keep their shape, such as custom emoji (spec §4.5),
// go through the same checks and encoding as avatars: decoded only after
// their size is known, encoded again without metadata.

// ErrNotAllowed means the picture's format is not one of the allowed ones.
var ErrNotAllowed = errors.New("this kind of picture is not accepted here")

// Limits are what [Fit] accepts.
type Limits struct {
	// MaximumBytes is the largest accepted file.
	MaximumBytes int
	// MaximumSide is the largest width or height kept; larger pictures are
	// scaled down to fit, keeping their shape. Smaller ones keep their size.
	MaximumSide int
	// Formats are the accepted formats as image.DecodeConfig names them:
	// "png", "gif", "webp", "jpeg".
	Formats []string
}

// Fit checks data against limits and encodes it again, scaled down to fit
// MaximumSide. WebP becomes PNG, as for avatars.
func Fit(data []byte, limits Limits) (Result, error) {
	if len(data) > limits.MaximumBytes {
		return Result{}, ErrTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrUnsupported
	}
	if !allowed(format, limits.Formats) {
		return Result{}, ErrNotAllowed
	}
	if config.Width < 1 || config.Height < 1 {
		return Result{}, ErrDamaged
	}
	if config.Width > maximumSide || config.Height > maximumSide || config.Width*config.Height > maximumPixels {
		return Result{}, ErrTooLarge
	}
	whole := image.Rect(0, 0, config.Width, config.Height)
	size := fitted(config.Width, config.Height, limits.MaximumSide)
	if format == "gif" {
		return processGIF(data, whole, size)
	}
	return processStill(data, format, whole, size)
}

func allowed(format string, formats []string) bool {
	for _, candidate := range formats {
		if candidate == format {
			return true
		}
	}
	return false
}

// fitted is width×height scaled down, if needed, so neither side passes
// side; each side stays at least one pixel.
func fitted(width int, height int, side int) image.Point {
	if width <= side && height <= side {
		return image.Pt(width, height)
	}
	if width >= height {
		return image.Pt(side, max(1, height*side/width))
	}
	return image.Pt(max(1, width*side/height), side)
}
