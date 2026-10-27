//go:build ignore

// Run from the repository root: go run ./packaging/make-icon.go
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/omriharel/deej/pkg/deej/icon"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	f, err := os.Open("assets/logo-512.png")
	must(err)
	source, err := png.Decode(f)
	must(err)
	f.Close()
	b := source.Bounds()
	logo := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		t := float64(y) / float64(b.Dy()-1)
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(source.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			// Recover the white slider artwork from the original blue-purple gradient.
			base := 71.0 + 71.0*t
			white := math.Max(0, math.Min(1, (float64(c.R)-base)/(255-base)))
			red := 238 * (1 - t)
			cx, cy := float64(x)+0.5-float64(b.Dx())/2, float64(y)+0.5-float64(b.Dy())/2
			coverage := math.Max(0, math.Min(1, float64(b.Dx())/2-math.Hypot(cx, cy)+0.5))
			c.A = uint8(float64(c.A) * coverage)
			logo.SetNRGBA(x, y, color.NRGBA{uint8(red*(1-white) + 255*white), uint8(255 * white), uint8(255 * white), c.A})
		}
	}
	out, err := os.Create("assets/deej-red-black.png")
	must(err)
	must(png.Encode(out, logo))
	must(out.Close())
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	var images [][]byte
	for _, size := range sizes {
		var data bytes.Buffer
		write := func(v interface{}) { must(binary.Write(&data, binary.LittleEndian, v)) }
		maskStride := ((size + 31) / 32) * 4
		write(uint32(40))
		write(int32(size))
		write(int32(size * 2))
		write(uint16(1))
		write(uint16(32))
		write(uint32(0))
		write(uint32(size * size * 4))
		write(int32(0))
		write(int32(0))
		write(uint32(0))
		write(uint32(0))
		mask := make([]byte, maskStride*size)
		for y := size - 1; y >= 0; y-- {
			for x := 0; x < size; x++ {
				var r, g, bl, a float64
				for sy := 0; sy < 8; sy++ {
					for sx := 0; sx < 8; sx++ {
						px := int((float64(x) + (float64(sx)+0.5)/8) * float64(b.Dx()) / float64(size))
						py := int((float64(y) + (float64(sy)+0.5)/8) * float64(b.Dy()) / float64(size))
						c := logo.NRGBAAt(px, py)
						alpha := float64(c.A) / 255
						r += float64(c.R) * alpha
						g += float64(c.G) * alpha
						bl += float64(c.B) * alpha
						a += alpha
					}
				}
				if a > 0 {
					data.Write([]byte{uint8(bl / a), uint8(g / a), uint8(r / a), uint8(a / 64 * 255)})
				} else {
					data.Write([]byte{0, 0, 0, 0})
					mask[(size-1-y)*maskStride+x/8] |= 1 << uint(7-x%8)
				}
			}
		}
		data.Write(mask)
		images = append(images, data.Bytes())
	}
	var ico bytes.Buffer
	write := func(v interface{}) { must(binary.Write(&ico, binary.LittleEndian, v)) }
	write(uint16(0))
	write(uint16(1))
	write(uint16(len(sizes)))
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		write(uint8(size % 256))
		write(uint8(size % 256))
		write(uint8(0))
		write(uint8(0))
		write(uint16(1))
		write(uint16(32))
		write(uint32(len(images[i])))
		write(uint32(offset))
		offset += len(images[i])
	}
	for _, data := range images {
		ico.Write(data)
	}
	must(os.WriteFile("assets/deej-red-black.ico", ico.Bytes(), 0644))
	must(os.WriteFile("pkg/deej/icon/app.ico", ico.Bytes(), 0644))
	must(os.WriteFile("pkg/deej/icon/settings-red.ico", redMenuIcon(icon.EditConfig), 0644))
	must(os.WriteFile("pkg/deej/icon/refresh-sessions-red.ico", redMenuIcon(icon.RefreshSessions), 0644))
}

// Retain the original menu artwork, alpha channel, and mask; replace its
// purple hue with red without recoloring white and gray highlights.
func redMenuIcon(source []byte) []byte {
	data := append([]byte(nil), source...)
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	for i := 0; i < count; i++ {
		entry := 6 + 16*i
		offset := int(binary.LittleEndian.Uint32(data[entry+12:]))
		header := data[offset:]
		if binary.LittleEndian.Uint32(header) != 40 || binary.LittleEndian.Uint16(header[14:]) != 32 {
			panic("menu icon must contain 32-bit DIB images")
		}
		width := int(binary.LittleEndian.Uint32(header[4:]))
		height := int(binary.LittleEndian.Uint32(header[8:])) / 2
		for pixel := offset + 40; pixel < offset+40+width*height*4; pixel += 4 {
			b, g, r := data[pixel], data[pixel+1], data[pixel+2]
			high, low := max(r, g, b), min(r, g, b)
			data[pixel], data[pixel+1], data[pixel+2] = low, low, high
		}
	}
	return data
}
