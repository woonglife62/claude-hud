//go:build ignore

// mkicon generates icon.ico matching the Claude HUD tray icon design
// (purple circle with white "C" letter).
// Run: go run tools/mkicon.go
package main

import (
	"encoding/binary"
	"math"
	"os"
)

func main() {
	sizes := []int{16, 32, 48, 256}
	type imgEntry struct {
		size   int
		pixels []byte // BGRA, bottom-up
		mask   []byte // 1bpp AND mask, bottom-up
	}

	var entries []imgEntry
	for _, sz := range sizes {
		rgba := renderIcon(sz)
		px, msk := toBGRABottomUp(rgba, sz)
		entries = append(entries, imgEntry{sz, px, msk})
	}

	f, err := os.Create("icon.ico")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// ICO header
	writeU16(f, 0)                 // reserved
	writeU16(f, 1)                 // type: icon
	writeU16(f, uint16(len(sizes))) // image count

	// Calculate data offset (header=6 + directory=16*N)
	offset := 6 + 16*len(sizes)

	// Directory entries
	for _, e := range entries {
		w, h := byte(e.size), byte(e.size)
		if e.size >= 256 {
			w, h = 0, 0
		}
		dataSize := 40 + len(e.pixels) + len(e.mask)
		f.Write([]byte{w, h, 0, 0}) // width, height, colors, reserved
		writeU16(f, 1)               // planes
		writeU16(f, 32)              // bit count
		writeU32(f, uint32(dataSize))
		writeU32(f, uint32(offset))
		offset += dataSize
	}

	// Image data
	for _, e := range entries {
		// BITMAPINFOHEADER (40 bytes)
		bih := make([]byte, 40)
		binary.LittleEndian.PutUint32(bih[0:], 40)
		binary.LittleEndian.PutUint32(bih[4:], uint32(e.size))
		binary.LittleEndian.PutUint32(bih[8:], uint32(e.size*2)) // height doubled for ICO
		binary.LittleEndian.PutUint16(bih[12:], 1)               // planes
		binary.LittleEndian.PutUint16(bih[14:], 32)              // bits per pixel
		f.Write(bih)
		f.Write(e.pixels)
		f.Write(e.mask)
	}
}

// renderIcon creates an NRGBA pixel array for the icon at the given size.
// Returns row-major top-down [R,G,B,A] per pixel.
func renderIcon(size int) []byte {
	px := make([]byte, size*size*4)
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 0.5

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			fy := float64(y) + 0.5
			dist := math.Sqrt((fx-cx)*(fx-cx) + (fy-cy)*(fy-cy))
			idx := (y*size + x) * 4

			if dist <= radius {
				// Anti-alias circle edge
				alpha := 1.0
				if dist > radius-1.0 {
					alpha = radius - dist + 1.0
					if alpha > 1 {
						alpha = 1
					}
					if alpha < 0 {
						alpha = 0
					}
				}
				// Purple circle background (160, 120, 255)
				px[idx+0] = 160
				px[idx+1] = 120
				px[idx+2] = 255
				px[idx+3] = uint8(alpha * 255)
			}
			// else: transparent (0,0,0,0)
		}
	}

	// Draw white "C" letter as an arc (ring with gap on right)
	cOuterR := float64(size) * 0.38
	cInnerR := float64(size) * 0.18
	gapHalf := 50.0 * math.Pi / 180.0 // half-angle of gap on right side

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5 - cx
			fy := float64(y) + 0.5 - cy
			dist := math.Sqrt(fx*fx + fy*fy)

			if dist < cInnerR-1 || dist > cOuterR+1 {
				continue
			}

			angle := math.Atan2(fy, fx)
			absAngle := math.Abs(angle)

			if absAngle <= gapHalf {
				continue // inside gap
			}

			// Ring edge anti-aliasing
			ringAlpha := 1.0
			if dist < cInnerR {
				ringAlpha = 1.0 - (cInnerR - dist)
			} else if dist > cOuterR {
				ringAlpha = 1.0 - (dist - cOuterR)
			}

			// Gap edge anti-aliasing
			gapEdge := 0.15 // radians of soft edge
			if absAngle < gapHalf+gapEdge {
				ga := (absAngle - gapHalf) / gapEdge
				if ga < ringAlpha {
					ringAlpha = ga
				}
			}

			if ringAlpha <= 0 {
				continue
			}
			if ringAlpha > 1 {
				ringAlpha = 1
			}

			idx := (y*size + x) * 4
			// Blend white "C" over the purple background
			bgR, bgG, bgB, bgA := px[idx+0], px[idx+1], px[idx+2], px[idx+3]
			if bgA > 0 {
				// Alpha composite: white over purple
				srcA := ringAlpha
				dstA := float64(bgA) / 255.0
				outA := srcA + dstA*(1-srcA)
				if outA > 0 {
					px[idx+0] = uint8((255.0*srcA + float64(bgR)*dstA*(1-srcA)) / outA)
					px[idx+1] = uint8((255.0*srcA + float64(bgG)*dstA*(1-srcA)) / outA)
					px[idx+2] = uint8((255.0*srcA + float64(bgB)*dstA*(1-srcA)) / outA)
					px[idx+3] = uint8(outA * 255)
				}
			}
		}
	}

	return px
}

// toBGRABottomUp converts top-down RGBA to bottom-up BGRA + AND mask.
func toBGRABottomUp(rgba []byte, size int) (pixels, mask []byte) {
	pixels = make([]byte, size*size*4)
	maskRowBytes := ((size + 31) / 32) * 4
	mask = make([]byte, maskRowBytes*size)

	for y := 0; y < size; y++ {
		srcY := size - 1 - y // flip vertically
		for x := 0; x < size; x++ {
			si := (srcY*size + x) * 4
			di := (y*size + x) * 4
			pixels[di+0] = rgba[si+2] // B
			pixels[di+1] = rgba[si+1] // G
			pixels[di+2] = rgba[si+0] // R
			pixels[di+3] = rgba[si+3] // A

			if rgba[si+3] == 0 {
				// Transparent → set AND mask bit
				byteIdx := y*maskRowBytes + x/8
				bitIdx := uint(7 - x%8)
				mask[byteIdx] |= 1 << bitIdx
			}
		}
	}
	return
}

func writeU16(f *os.File, v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	f.Write(buf[:])
}

func writeU32(f *os.File, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	f.Write(buf[:])
}
