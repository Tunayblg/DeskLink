package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
)

// maxCoverSide is the longest side of a cover sent to the phone. The card shows
// it at a few hundred pixels at most, and a smaller picture decodes and slides
// without stutter on an old phone.
const maxCoverSide = 512

// prepareCover turns what a player hands over into the picture the phone
// should show: a branded card (the cover in the middle with the player's logo
// under it, as Spotify sends) is cut down to the cover, and a large picture is
// shrunk. Anything it does not understand is returned as it came.
func prepareCover(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	changed := false
	if r, ok := coverRect(img); ok {
		img = crop(img, r)
		changed = true
	}
	b := img.Bounds()
	if m := max(b.Dx(), b.Dy()); m > maxCoverSide {
		img = shrink(img, maxCoverSide)
		changed = true
	}
	if !changed {
		return data
	}
	var out bytes.Buffer
	if jpeg.Encode(&out, img, &jpeg.Options{Quality: 88}) != nil {
		return data
	}
	return out.Bytes()
}

const bgTol = 14 // how far a pixel may be from the margin color and still count as margin

func near(a, b color.RGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= bgTol && d(a.G, b.G) <= bgTol && d(a.B, b.B) <= bgTol
}

func rgbaAt(img image.Image, x, y int) color.RGBA {
	r, g, b, _ := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
}

// coverRect finds the cover inside a card: a plain margin all round, then the
// cover, then (maybe) a logo under it. It answers only when it is sure: the
// whole outer ring is one color and what it finds is a big, almost square
// block clearly smaller than the picture.
func coverRect(img image.Image) (image.Rectangle, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 64 || h < 64 {
		return image.Rectangle{}, false
	}
	bg := rgbaAt(img, b.Min.X+1, b.Min.Y+1)
	// The ring: every pixel of the outer 2 px edge must match the corner.
	for x := 0; x < w; x++ {
		for _, y := range []int{0, 1, h - 2, h - 1} {
			if !near(rgbaAt(img, b.Min.X+x, b.Min.Y+y), bg) {
				return image.Rectangle{}, false
			}
		}
	}
	for y := 0; y < h; y++ {
		for _, x := range []int{0, 1, w - 2, w - 1} {
			if !near(rgbaAt(img, b.Min.X+x, b.Min.Y+y), bg) {
				return image.Rectangle{}, false
			}
		}
	}
	// Rows and columns that hold anything but margin.
	rowInk := make([]int, h)
	colInk := make([]int, w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !near(rgbaAt(img, b.Min.X+x, b.Min.Y+y), bg) {
				rowInk[y]++
				colInk[x]++
			}
		}
	}
	// The tallest run of inked rows is the cover (a logo is a short run under it).
	bestFrom, bestTo := -1, -1
	for y := 0; y < h; {
		if rowInk[y] < 3 {
			y++
			continue
		}
		from := y
		for y < h && rowInk[y] >= 3 {
			y++
		}
		if bestFrom < 0 || y-from > bestTo-bestFrom {
			bestFrom, bestTo = from, y
		}
	}
	if bestFrom < 0 {
		return image.Rectangle{}, false
	}
	// Its columns: the span of inked columns within those rows.
	left, right := -1, -1
	for x := 0; x < w; x++ {
		n := 0
		for y := bestFrom; y < bestTo; y++ {
			if !near(rgbaAt(img, b.Min.X+x, b.Min.Y+y), bg) {
				n++
			}
		}
		if n >= 3 {
			if left < 0 {
				left = x
			}
			right = x + 1
		}
	}
	if left < 0 {
		return image.Rectangle{}, false
	}
	cw, ch := right-left, bestTo-bestFrom
	ratio := float64(cw) / float64(ch)
	// A real cover: big, square-ish, and the picture really had a margin.
	if cw < w/2 || ch < h/2 || ratio < 0.9 || ratio > 1.1 {
		return image.Rectangle{}, false
	}
	if cw*100 > w*96 && ch*100 > h*96 {
		return image.Rectangle{}, false
	}
	return image.Rect(b.Min.X+left, b.Min.Y+bestFrom, b.Min.X+right, b.Min.Y+bestTo), true
}

func crop(img image.Image, r image.Rectangle) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}
	return out
}

// shrink scales a picture down so its longest side is side, averaging the
// source pixels under each new pixel.
func shrink(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := side, side*h/w
	if h > w {
		nw, nh = side*w/h, side
	}
	if nw < 1 || nh < 1 {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, max((y+1)*h/nh, y*h/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, max((x+1)*w/nw, x*w/nw+1)
			var r, g, bl, n uint32
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					pr, pg, pb, _ := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					r += pr >> 8
					g += pg >> 8
					bl += pb >> 8
					n++
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
		}
	}
	return out
}
