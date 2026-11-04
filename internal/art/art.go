// Package art finds a picture for each game tile: a custom cover, Steam's
// cover art, or the executable's icon. Pictures are loaded once at startup
// and sent to the phone inside the (authenticated) WebSocket layout message.
package art

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // register decoder
	"image/png"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Tunayblg/DeskLink/internal/config"
)

const maxCoverBytes = 3 << 20

// Art is the picture for one game.
type Art struct {
	Image string `json:"image,omitempty"` // data: URI
	Kind  string `json:"kind,omitempty"`  // "cover" (poster) or "icon"
	Tint  string `json:"tint,omitempty"`  // dominant color as "r,g,b"
	// Icon is a square icon for small tiles, when Image is a cover.
	Icon     string `json:"icon,omitempty"`
	IconTint string `json:"icon_tint,omitempty"`
}

// Load resolves art for every game. Missing art is logged and left empty;
// the phone then shows the game's name on a plain tile.
func Load(games []config.Game) map[string]Art {
	out := make(map[string]Art, len(games))
	for _, g := range games {
		a, err := load(g)
		if err != nil {
			log.Printf("art: %s: %v", g.ID, err)
		}
		out[g.ID] = a
	}
	return out
}

// GameExe is the executable a game runs as (its icon source), or "".
func GameExe(g config.Game) string {
	switch {
	case g.IconFrom != "":
		return g.IconFrom
	case g.SteamAppID > 0:
		return steamGameExe(g.SteamAppID)
	}
	return g.Target
}

// AppIcon returns an executable's icon at most size pixels wide as a PNG data
// URI, or "".
func AppIcon(exe string, size int) string {
	img, err := exeIcon(exe)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if png.Encode(&buf, shrink(img, size)) != nil {
		return ""
	}
	return dataURI("image/png", buf.Bytes())
}

// shrink scales img down by a whole factor (averaging each block of pixels)
// so it is at most size pixels wide.
func shrink(img image.Image, size int) image.Image {
	b := img.Bounds()
	k := (b.Dx() + size - 1) / size
	if k <= 1 {
		return img
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx()/k, b.Dy()/k))
	for y := 0; y < out.Rect.Dy(); y++ {
		for x := 0; x < out.Rect.Dx(); x++ {
			var r, g, bl, a uint32
			for dy := 0; dy < k; dy++ {
				for dx := 0; dx < k; dx++ {
					pr, pg, pb, pa := img.At(b.Min.X+x*k+dx, b.Min.Y+y*k+dy).RGBA()
					r, g, bl, a = r+pr, g+pg, bl+pb, a+pa
				}
			}
			if a == 0 {
				continue
			}
			// RGBA() is premultiplied; NRGBA is not.
			o := out.PixOffset(x, y)
			out.Pix[o] = uint8(r * 255 / a)
			out.Pix[o+1] = uint8(g * 255 / a)
			out.Pix[o+2] = uint8(bl * 255 / a)
			out.Pix[o+3] = uint8(a / uint32(k*k) >> 8)
		}
	}
	return out
}

func load(g config.Game) (Art, error) {
	exe := GameExe(g)

	cover := g.Cover
	if cover == "" && g.SteamAppID > 0 {
		cover = steamCover(g.SteamAppID)
	}
	var coverErr error
	if cover != "" {
		a, err := coverFile(cover)
		if err == nil {
			if exe != "" {
				if icon, img, err := iconPNG(exe); err == nil && icon != "" {
					a.Icon, a.IconTint = icon, tint(img)
				}
			}
			return a, nil
		}
		coverErr = err
	}

	// No cover (Steam only keeps the ones it has shown in its library): use
	// the game's own icon rather than a blank tile.
	icon, img, err := iconPNG(exe)
	if err == nil && icon != "" {
		return Art{Image: icon, Kind: "icon", Tint: tint(img)}, coverErr
	}
	switch {
	case coverErr != nil:
		return Art{}, coverErr
	case err != nil:
		return Art{}, err
	case g.SteamAppID > 0:
		return Art{}, fmt.Errorf("no cover or icon found for Steam app %d", g.SteamAppID)
	}
	return Art{}, nil
}

// iconPNG extracts an executable's icon as a PNG data URI.
func iconPNG(exe string) (string, image.Image, error) {
	if !strings.EqualFold(filepath.Ext(exe), ".exe") {
		return "", nil, nil
	}
	img, err := exeIcon(exe)
	if err != nil {
		return "", nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", nil, err
	}
	return dataURI("image/png", buf.Bytes()), img, nil
}

var (
	vdfPath    = regexp.MustCompile(`"path"\s+"([^"]+)"`)
	vdfInstall = regexp.MustCompile(`"installdir"\s+"([^"]+)"`)
	// Executables in a game folder that are not the game.
	notTheGame = regexp.MustCompile(`(?i)crash|unins|redist|setup|helper|report|cef|battleye|easyanticheat|^beservice|launcher|vc_|dxsetup|ue4prereq|_be\.exe$`)
)

// SteamGame is an installed Steam game.
type SteamGame struct {
	AppID int    `json:"appid"`
	Name  string `json:"name"`
}

var (
	vdfAppID = regexp.MustCompile(`"appid"\s+"(\d+)"`)
	vdfName  = regexp.MustCompile(`"name"\s+"([^"]+)"`)
)

// SteamGames lists the games installed in every Steam library on this PC,
// skipping Steam's own redistributable packages and tools.
func SteamGames() []SteamGame {
	var out []SteamGame
	seen := map[int]bool{}
	for _, lib := range steamLibraries() {
		files, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			id, name := vdfAppID.FindSubmatch(b), vdfName.FindSubmatch(b)
			if id == nil || name == nil {
				continue
			}
			n := 0
			fmt.Sscan(string(id[1]), &n)
			lower := strings.ToLower(string(name[1]))
			if n == 0 || seen[n] || strings.Contains(lower, "redistributable") || strings.Contains(lower, "steamworks") || strings.HasPrefix(lower, "proton") {
				continue
			}
			seen[n] = true
			out = append(out, SteamGame{AppID: n, Name: string(name[1])})
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func steamLibraries() []string {
	root := steamPath()
	if root == "" {
		return nil
	}
	libs := []string{root}
	if b, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf")); err == nil {
		for _, m := range vdfPath.FindAllSubmatch(b, -1) {
			p := strings.ReplaceAll(string(m[1]), `\\`, `\`)
			if !strings.EqualFold(filepath.Clean(p), filepath.Clean(root)) {
				libs = append(libs, p)
			}
		}
	}
	return libs
}

// steamGameExe guesses a Steam game's main executable: the largest .exe in
// its install folder that is not a known helper. Set icon_from to override.
func steamGameExe(appID int) string {
	for _, lib := range steamLibraries() {
		b, err := os.ReadFile(filepath.Join(lib, "steamapps", fmt.Sprintf("appmanifest_%d.acf", appID)))
		if err != nil {
			continue
		}
		m := vdfInstall.FindSubmatch(b)
		if m == nil {
			continue
		}
		dir := filepath.Join(lib, "steamapps", "common", string(m[1]))
		best, size := "", int64(0)
		base := strings.Count(dir, string(filepath.Separator))
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.Count(p, string(filepath.Separator))-base > 4 {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ".exe") || notTheGame.MatchString(d.Name()) {
				return nil
			}
			if info, err := d.Info(); err == nil && info.Size() > size {
				best, size = p, info.Size()
			}
			return nil
		})
		return best
	}
	return ""
}

func coverFile(path string) (Art, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Art{}, err
	}
	if len(data) > maxCoverBytes {
		return Art{}, fmt.Errorf("%s is larger than 3 MB", path)
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" {
		return Art{}, fmt.Errorf("%s is not a JPEG or PNG image", path)
	}
	a := Art{Image: dataURI(mime, data), Kind: "cover"}
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		a.Tint = tint(img)
	}
	return a, nil
}

func dataURI(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// steamCover looks for Steam's portrait library image in its local cache.
func steamCover(appID int) string {
	root := steamPath()
	if root == "" {
		return ""
	}
	return coverIn(filepath.Join(root, "appcache", "librarycache"), appID)
}

// coverNames are Steam's cached portrait posters, best first. Steam only caches
// the ones it has shown in its library, and a localized copy
// (library_600x900_turkish.jpg) can exist instead of the plain one. Without a
// poster the game's own icon is used; other cached images (capsules, headers)
// are not posters and are ignored.
var coverNames = []string{"library_600x900.jpg", "library_600x900_*.jpg"}

// coverIn finds a game's portrait image in a librarycache folder.
func coverIn(cache string, appID int) string {
	app := filepath.Join(cache, fmt.Sprint(appID))
	for _, name := range coverNames {
		// Newer Steam: <appid>/<hash>/<name>; a little older: <appid>/<name>.
		for _, pattern := range []string{filepath.Join(app, "*", name), filepath.Join(app, name)} {
			if m, _ := filepath.Glob(pattern); len(m) > 0 {
				return m[0]
			}
		}
	}
	// Oldest Steam: <appid>_library_600x900.jpg
	if p := filepath.Join(cache, fmt.Sprintf("%d_library_600x900.jpg", appID)); fileExists(p) {
		return p
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// tint returns the average color of the image's visible, reasonably
// saturated pixels, so dark or grey areas do not wash it out.
func tint(img image.Image) string {
	b := img.Bounds()
	step := max(1, b.Dx()/64)
	var r, g, bl, n float64
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			if ca < 0x8000 {
				continue
			}
			hi := max(cr, cg, cb)
			lo := min(cr, cg, cb)
			if hi < 0x2000 || hi-lo < 0x1800 { // too dark or too grey
				continue
			}
			r, g, bl, n = r+float64(cr>>8), g+float64(cg>>8), bl+float64(cb>>8), n+1
		}
	}
	if n == 0 {
		return "120,130,150"
	}
	return fmt.Sprintf("%d,%d,%d", int(r/n), int(g/n), int(bl/n))
}
