package art

import (
	"errors"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	advapi32               = syscall.NewLazyDLL("advapi32.dll")
	procPrivateExtractIcon = user32.NewProc("PrivateExtractIconsW")
	procGetIconInfo        = user32.NewProc("GetIconInfo")
	procDestroyIcon        = user32.NewProc("DestroyIcon")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetObject          = gdi32.NewProc("GetObjectW")
	procGetDIBits          = gdi32.NewProc("GetDIBits")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procRegGetValue        = advapi32.NewProc("RegGetValueW")
)

type iconInfo struct {
	fIcon              int32
	xHotspot, yHotspot uint32
	hbmMask, hbmColor  uintptr
}

type bitmap struct {
	typ, width, height, widthBytes int32
	planes, bitsPixel              uint16
	bits                           uintptr
}

type bitmapInfoHeader struct {
	size                         uint32
	width, height                int32
	planes, bitCount             uint16
	compression, sizeImage       uint32
	xPelsPerMeter, yPelsPerMeter int32
	clrUsed, clrImportant        uint32
}

// exeIcon extracts the executable's main icon at up to 256×256 and crops
// away transparent padding.
func exeIcon(path string) (image.Image, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var hicon uintptr
	var id uint32
	n, _, _ := procPrivateExtractIcon.Call(uintptr(unsafe.Pointer(p)), 0, 256, 256,
		uintptr(unsafe.Pointer(&hicon)), uintptr(unsafe.Pointer(&id)), 1, 0)
	if n == 0 || n == 0xFFFFFFFF || hicon == 0 {
		return nil, errors.New("no icon in " + path)
	}
	defer procDestroyIcon.Call(hicon)

	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil, errors.New("GetIconInfo failed")
	}
	defer procDeleteObject.Call(ii.hbmMask)
	if ii.hbmColor == 0 {
		return nil, errors.New("monochrome icon")
	}
	defer procDeleteObject.Call(ii.hbmColor)

	var bm bitmap
	procGetObject.Call(ii.hbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
	w, h := int(bm.width), int(bm.height)
	if w <= 0 || h <= 0 || w > 512 || h > 512 {
		return nil, errors.New("unexpected icon size")
	}

	hdr := bitmapInfoHeader{size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), width: int32(w), height: -int32(h), planes: 1, bitCount: 32}
	// BITMAPINFO is the header followed by a color table we do not use.
	info := struct {
		h      bitmapInfoHeader
		colors [4]uint32
	}{h: hdr}
	px := make([]byte, w*h*4)
	dc, _, _ := procGetDC.Call(0)
	r, _, _ := procGetDIBits.Call(dc, ii.hbmColor, 0, uintptr(h), uintptr(unsafe.Pointer(&px[0])), uintptr(unsafe.Pointer(&info)), 0)
	procReleaseDC.Call(0, dc)
	if r == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasAlpha := false
	for i := 0; i < len(px); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = px[i+2], px[i+1], px[i], px[i+3]
		if px[i+3] != 0 {
			hasAlpha = true
		}
	}
	if !hasAlpha { // old icons without an alpha channel
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
	}
	return crop(img), nil
}

// crop trims fully transparent rows and columns.
func crop(img *image.NRGBA) image.Image {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 8 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x+1), max(maxY, y+1)
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return img
	}
	return img.SubImage(image.Rect(minX, minY, maxX, maxY))
}

// steamPath reads Steam's install folder from the registry.
func steamPath() string {
	const hkeyCurrentUser = 0x80000001
	const rrfRtRegSz = 0x00000002
	key, _ := syscall.UTF16PtrFromString(`Software\Valve\Steam`)
	val, _ := syscall.UTF16PtrFromString("SteamPath")
	buf := make([]uint16, 520)
	size := uint32(len(buf) * 2)
	r, _, _ := procRegGetValue.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(val)),
		rrfRtRegSz, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
