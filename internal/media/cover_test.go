package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// card draws what Spotify hands over: a plain margin, the cover, a logo bar under it.
func card(withLogo bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 640, 640))
	bg := color.RGBA{200, 200, 205, 255}
	for y := 0; y < 640; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := 40; y < 480; y++ {
		for x := 100; x < 540; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 3), uint8(y / 3), 90, 255})
		}
	}
	if withLogo {
		for y := 540; y < 600; y++ {
			for x := 220; x < 420; x++ {
				img.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 95})
	return b.Bytes()
}

func TestPrepareCoverCutsLogoCard(t *testing.T) {
	out := prepareCover(card(true))
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 400 || b.Dx() > 450 || b.Dy() < 400 || b.Dy() > 450 {
		t.Fatalf("expected the ~440 px cover, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestPrepareCoverLeavesPlainCoverAlone(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 50, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	if out := prepareCover(b.Bytes()); !bytes.Equal(out, b.Bytes()) {
		t.Fatal("a plain cover must come back untouched")
	}
}

func TestPrepareCoverShrinksBig(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 1200))
	for y := 0; y < 1200; y += 1 {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 5), uint8(y / 5), 50, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	out, _, err := image.Decode(bytes.NewReader(prepareCover(b.Bytes())))
	if err != nil || out.Bounds().Dx() != maxCoverSide {
		t.Fatalf("expected %d px, got %v %v", maxCoverSide, out.Bounds(), err)
	}
}

func TestPrepareCoverGarbage(t *testing.T) {
	in := []byte("not an image")
	if out := prepareCover(in); !bytes.Equal(out, in) {
		t.Fatal("garbage must come back as is")
	}
}
