package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"

	"golang.org/x/image/draw"
)

// processGIF crops and resizes every frame of a (possibly animated) GIF,
// keeping its timing and loop count.
//
// GIF frames are often small patches drawn over the previous picture, so
// each frame is first composed onto a full canvas following its disposal
// method (GIF89a §23), and the whole canvas is then scaled.
func processGIF(data []byte, square image.Rectangle) (Result, error) {
	animation, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil || len(animation.Image) == 0 {
		return Result{}, ErrDamaged
	}
	width, height := animation.Config.Width, animation.Config.Height
	if len(animation.Image) > maximumFrames || len(animation.Image)*width*height > maximumFramePixels {
		return Result{}, ErrTooLarge
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	output := &gif.GIF{LoopCount: animation.LoopCount, Config: image.Config{Width: Size, Height: Size}}
	for index, frame := range animation.Image {
		before := cloneRGBA(canvas)
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		output.Image = append(output.Image, quantize(scale(canvas, square), frame.Palette))
		output.Delay = append(output.Delay, animation.Delay[index])
		output.Disposal = append(output.Disposal, gif.DisposalNone)
		disposeFrame(canvas, before, frame.Bounds(), disposal(animation, index))
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, output); err != nil {
		return Result{}, ErrDamaged
	}
	return Result{Data: encoded.Bytes(), Format: FormatGIF}, nil
}

func disposal(animation *gif.GIF, index int) byte {
	if index < len(animation.Disposal) {
		return animation.Disposal[index]
	}
	return gif.DisposalNone
}

// disposeFrame prepares the canvas for the next frame.
func disposeFrame(canvas *image.RGBA, before *image.RGBA, area image.Rectangle, method byte) {
	switch method {
	case gif.DisposalBackground:
		draw.Draw(canvas, area, image.Transparent, image.Point{}, draw.Src)
	case gif.DisposalPrevious:
		draw.Draw(canvas, canvas.Bounds(), before, image.Point{}, draw.Src)
	}
}

func cloneRGBA(source *image.RGBA) *image.RGBA {
	copied := image.NewRGBA(source.Bounds())
	copy(copied.Pix, source.Pix)
	return copied
}

// quantize maps a scaled frame back to the frame's palette, with
// Floyd–Steinberg dithering so gradients stay smooth. A transparent entry
// is added when the palette has room, so transparent areas stay
// transparent.
func quantize(frame *image.RGBA, palette color.Palette) *image.Paletted {
	colors := append(color.Palette{}, palette...)
	if len(colors) < 256 && !hasTransparent(colors) {
		colors = append(colors, color.Transparent)
	}
	paletted := image.NewPaletted(frame.Bounds(), colors)
	draw.FloydSteinberg.Draw(paletted, frame.Bounds(), frame, image.Point{})
	return paletted
}

func hasTransparent(palette color.Palette) bool {
	for _, entry := range palette {
		if _, _, _, alpha := entry.RGBA(); alpha == 0 {
			return true
		}
	}
	return false
}
