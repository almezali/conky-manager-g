// Conky Manager GTK - v0.8 (Go edition)
// Universal Conky desktop manager for Linux environments.
// GTK 3 UI follows the user's system theme (Adwaita, Breeze, Yaru, etc.).
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

const (
	appName    = "Conky Manager GTK"
	appVersion = "v0.8"
	storeCat   = "124"

	maxArchiveBytes int64 = 512 * 1024 * 1024
	maxArchiveFiles       = 4000
	maxArchiveDepth       = 12
	maxRecentThemes       = 24
)

var (
	homeDir          = os.Getenv("HOME")
	oldBaseDir       = filepath.Join(homeDir, ".local", "share", "conky-kde-manager")
	baseDir          = filepath.Join(homeDir, ".local", "share", "conky-manager")
	logFilePath      = filepath.Join(baseDir, "manager.log")
	optimizedDir     = filepath.Join(baseDir, "optimized")
	launchDir        = filepath.Join(baseDir, "launch")
	settingsFile     = filepath.Join(baseDir, "settings.json")
	positionsFile    = filepath.Join(baseDir, "positions.json")
	colorsFile       = filepath.Join(baseDir, "colors.json")
	profilesFile     = filepath.Join(baseDir, "profiles.json")
	runtimeFile      = filepath.Join(baseDir, "runtime.json")
	libraryFile      = filepath.Join(baseDir, "library.json")
	palettesFile     = filepath.Join(baseDir, "palettes.json")
	autostartDir     = filepath.Join(homeDir, ".config", "autostart")
	autostartFile    = filepath.Join(autostartDir, "conky-manager.desktop")
	oldAutostartFile = filepath.Join(autostartDir, "conky-kde-manager.desktop")
	defaultImportDir = filepath.Join(homeDir, ".conky")
	downloadsDir     = filepath.Join(baseDir, "downloads")
	themeDirs        = []string{
		filepath.Join(homeDir, ".config", "conky"),
		filepath.Join(homeDir, ".conky"),
		filepath.Join(homeDir, ".local", "share", "conky"),
		"/usr/share/conky",
	}
)

var archiveExts = []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tar.xz", ".txz", ".7z"}
var stripSuffixes = []string{"-main", "-master", "-gh-pages", "-devel"}
var allowedHosts = []string{
	"gitlab.com", "codeberg.org",
	"pling.com", "www.pling.com", "gnome-look.org", "www.gnome-look.org",
	"kde-look.org", "www.kde-look.org", "opendesktop.org", "www.opendesktop.org",
}

var assetDirNames = []string{"res", "img", "imgs", "images", "assets", "scripts", "fonts", "lua", "lib", "icons", "include", "config"}
var assetSuffixes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".lua", ".ttf", ".otf", ".woff", ".woff2", ".sh"}

var pathRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$\{image\s+([^}\s]+)`),
	regexp.MustCompile(`(?i)lua_load\s*=\s*['"]([^'"]+)['"]`),
	regexp.MustCompile(`\$\{(?:exec|execi|execpi|execp|texeci|texecpi)\s+(?:\d+\s+)?((?:~|\$HOME|/)[^}\s|]+)`),
	regexp.MustCompile(`(~/.config/conky/[^\s'"{}$|]+)`),
	regexp.MustCompile(`(\$HOME/.config/conky/[^\s'"{}$|]+)`),
	regexp.MustCompile(`(~/.conky/[^\s'"{}$|]+)`),
}

var namedColors = map[string]string{
	"white": "#FFFFFF", "black": "#000000", "red": "#FF0000", "green": "#00FF00",
	"blue": "#0000FF", "yellow": "#FFFF00", "orange": "#FFA500", "gray": "#808080", "grey": "#808080",
}

var colorSlotOrder = []string{
	"default_color", "color1", "color2", "color3", "color4", "color5", "color6", "color7",
	"color8", "color9", "color0", "color", "default_outline_color", "default_shade_color", "own_window_colour",
}

var colorSlotLabels = map[string]string{
	"default_color": "Text", "color": "Text (legacy)", "color0": "Base", "color1": "Primary",
	"color2": "Secondary", "color3": "Accent 3", "color4": "Accent 4", "color5": "Accent 5",
	"default_outline_color": "Outline", "default_shade_color": "Shade", "own_window_colour": "Window",
}

type colorPreset struct {
	Label  string
	Colors map[string]string
}

var colorPresets = []struct {
	ID     string
	Preset colorPreset
}{
	{"tokyo_night", colorPreset{"Tokyo Night", map[string]string{"default_color": "#A9B1D6", "color1": "#F7768E", "color2": "#7AA2F7", "default_outline_color": "#565F89", "default_shade_color": "#1A1B26"}}},
	{"dracula", colorPreset{"Dracula", map[string]string{"default_color": "#F8F8F2", "color1": "#FF79C6", "color2": "#8BE9FD", "default_outline_color": "#6272A4", "default_shade_color": "#282A36"}}},
	{"nord", colorPreset{"Nord", map[string]string{"default_color": "#ECEFF4", "color1": "#88C0D0", "color2": "#81A1C1", "default_outline_color": "#4C566A", "default_shade_color": "#2E3440"}}},
	{"catppuccin_mocha", colorPreset{"Catppuccin Mocha", map[string]string{"default_color": "#CDD6F4", "color1": "#F5C2E7", "color2": "#89B4FA", "default_outline_color": "#6C7086", "default_shade_color": "#1E1E2E"}}},
	{"rose_pine", colorPreset{"Rose Pine", map[string]string{"default_color": "#E0DEF4", "color1": "#EBBCBA", "color2": "#9CCFD8", "default_outline_color": "#6E6A86", "default_shade_color": "#191724"}}},
	{"gruvbox_dark", colorPreset{"Gruvbox Dark", map[string]string{"default_color": "#EBDBB2", "color1": "#FB4934", "color2": "#83A598", "default_outline_color": "#928374", "default_shade_color": "#282828"}}},
	{"solarized_dark", colorPreset{"Solarized Dark", map[string]string{"default_color": "#839496", "color1": "#DC322F", "color2": "#268BD2", "default_outline_color": "#586E75", "default_shade_color": "#002B36"}}},
	{"everforest", colorPreset{"Everforest", map[string]string{"default_color": "#D3C6AA", "color1": "#E67E80", "color2": "#7FBBB3", "default_outline_color": "#859289", "default_shade_color": "#2D353B"}}},
}

var positionAlignments = [][2]string{
	{"top_left", "↖"}, {"top_middle", "↑"}, {"top_right", "↗"},
	{"middle_left", "←"}, {"middle_middle", "●"}, {"middle_right", "→"},
	{"bottom_left", "↙"}, {"bottom_middle", "↓"}, {"bottom_right", "↘"},
}

var dePresets = map[string][2]string{
	"kde": {"above", "dock"}, "gnome": {"below", "desktop"}, "xfce": {"below", "desktop"},
	"cinnamon": {"below", "desktop"}, "mate": {"below", "desktop"}, "lxqt": {"below", "desktop"},
	"lxde": {"below", "desktop"}, "hyprland": {"above", "override"}, "niri": {"above", "override"},
	"sway": {"above", "override"}, "cosmic": {"below", "desktop"}, "generic": {"above", "dock"},
}

var desktopSettingsNew = map[string]string{
	"update_interval": "1.0", "update_interval_on_battery": "1.5", "cpu_avg_samples": "2", "net_avg_samples": "2",
	"double_buffer": "true", "no_buffers": "true", "own_window": "true", "own_window_argb_visual": "true",
	"own_window_argb_value": "0", "own_window_transparent": "true", "draw_shades": "false", "draw_outline": "false",
	"draw_borders": "false", "draw_graph_borders": "false", "use_xft": "true", "xftalpha": "1.0",
}
var desktopSettingsLegacy = map[string]string{
	"update_interval": "1.0", "cpu_avg_samples": "2", "net_avg_samples": "2", "double_buffer": "yes",
	"no_buffers": "yes", "own_window": "yes", "own_window_argb_visual": "yes", "own_window_argb_value": "0",
	"own_window_transparent": "yes", "draw_shades": "no", "draw_outline": "no", "draw_borders": "no",
	"draw_graph_borders": "no", "use_xft": "yes", "xftalpha": "1.0",
}

type Settings struct {
	Layer                string   `json:"layer"`
	WindowType           string   `json:"window_type"`
	Smoothness           string   `json:"smoothness"`
	NiceLevel            int      `json:"nice_level"`
	MaxInstances         int      `json:"max_instances"`
	HealthcheckSeconds   float64  `json:"healthcheck_seconds"`
	PreviewSeconds       int      `json:"preview_seconds"`
	UITheme              string   `json:"ui_theme"`
	DesktopPreset        string   `json:"desktop_preset"`
	AutoOptimize         bool     `json:"auto_optimize"`
	FullVisualLaunch     bool     `json:"full_visual_launch"`
	CreateCompatSymlinks bool     `json:"create_compat_symlinks"`
	RememberPosition     bool     `json:"remember_position"`
	RememberColors       bool     `json:"remember_colors"`
	ScaleAwarePosition   bool     `json:"scale_aware_position"`
	CustomThemeDirs      []string `json:"custom_theme_dirs,omitempty"`
	PositionEnabled      bool     `json:"position_enabled,omitempty"`
	Alignment            string   `json:"alignment,omitempty"`
	GapX                 int      `json:"gap_x,omitempty"`
	GapY                 int      `json:"gap_y,omitempty"`
	Monitor              string   `json:"monitor,omitempty"`
	XineramaHead         int      `json:"xinerama_head,omitempty"`
}

func defaultSettings() Settings {
	return Settings{
		Layer: "above", WindowType: "dock", Smoothness: "balanced", NiceLevel: 10,
		MaxInstances: 8, HealthcheckSeconds: 2.0, PreviewSeconds: 5, UITheme: "auto",
		DesktopPreset: "auto", AutoOptimize: true, FullVisualLaunch: true,
		CreateCompatSymlinks: true, RememberPosition: true, RememberColors: true,
		ScaleAwarePosition: false,
	}
}

type Position struct {
	Enabled      bool   `json:"enabled"`
	Alignment    string `json:"alignment"`
	GapX         int    `json:"gap_x"`
	GapY         int    `json:"gap_y"`
	Monitor      string `json:"monitor,omitempty"`
	XineramaHead int    `json:"xinerama_head,omitempty"`
}

type PositionsFile struct {
	Default Position            `json:"default"`
	Themes  map[string]Position `json:"themes"`
}

type ColorOverride struct {
	Colors   map[string]string `json:"colors"`
	SmartLua bool              `json:"smart_lua"`
	Enabled  bool              `json:"enabled"`
}

type ColorsFile struct {
	Themes map[string]ColorOverride `json:"themes"`
}

type ProfilesFile struct {
	Profiles    map[string][]string       `json:"profiles"`
	Desktop     map[string]DesktopProfile `json:"desktop_profiles,omitempty"`
	LastProfile string                    `json:"last_profile"`
}

type ProfileTheme struct {
	Path         string        `json:"path"`
	Monitor      string        `json:"monitor,omitempty"`
	Position     Position      `json:"position"`
	Colors       ColorOverride `json:"colors"`
	Enabled      bool          `json:"enabled"`
	StartupDelay int           `json:"startup_delay,omitempty"`
}

type DesktopProfile struct {
	Name      string         `json:"name"`
	Themes    []ProfileTheme `json:"themes"`
	Autostart bool           `json:"autostart,omitempty"`
}

type ManagedProcess struct {
	PID          int    `json:"pid"`
	ThemeID      string `json:"theme_id"`
	ConfigPath   string `json:"config_path"`
	LaunchConfig string `json:"launch_config"`
	StartedAt    int64  `json:"started_at"`
	Preview      bool   `json:"preview"`
}

type RuntimeFile struct {
	Processes []ManagedProcess `json:"processes"`
}

type RecentEntry struct {
	Path string `json:"path"`
	At   int64  `json:"at"`
}

type ThemeLibrary struct {
	Favorites  map[string]bool `json:"favorites"`
	Recent     []RecentEntry   `json:"recent"`
	CustomDirs []string        `json:"custom_dirs"`
}

type HealthIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

type ThemeHealth struct {
	Status  string        `json:"status"`
	Issues  []HealthIssue `json:"issues"`
	Scanned int64         `json:"scanned"`
}

type MonitorInfo struct {
	ID           string
	Name         string
	Manufacturer string
	Model        string
	X, Y         int
	Width        int
	Height       int
	Scale        int
	Primary      bool
	Head         int
}

type CustomPalette struct {
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Colors map[string]string `json:"colors"`
}

type ThemeItem struct{ Path string }

func (t ThemeItem) Label() string {
	return filepath.Base(filepath.Dir(t.Path)) + "/" + filepath.Base(t.Path)
}

type OnlineStore struct {
	ID, Label, SiteURL, BrowseURL, APIHost, DetailsAPIHost, PageURLTemplate string
}

type OnlineProduct struct {
	ProductID                      int
	Name, Summary, Author, Version string
	Downloads                      int
	Score                          float64
	PreviewURL, PageURL, StoreID   string
}

type ThemeLaunchBundle struct {
	SourceConfig, ThemeRoot, LaunchDir, LaunchConfig string
	PathFixes                                        int
	MissingAssets                                    []string
	FontsDir                                         string
	CompatLinks                                      [][2]string
}

// OpenDesktop-compatible stores are optional sources. The web URLs remain useful
// even when an instance temporarily disables its OCS API.
var onlineStores = []OnlineStore{
	{"gnome-look", "GNOME-Look.org", "https://www.gnome-look.org", "https://www.gnome-look.org/browse?cat=124&ord=downloads", "api.pling.com", "api.pling.com", "https://www.gnome-look.org/p/%d"},
	{"kde-look", "KDE-Look.org", "https://www.kde-look.org", "https://www.kde-look.org/browse?cat=124&ord=downloads", "api.pling.com", "api.pling.com", "https://www.kde-look.org/p/%d"},
	{"pling", "Pling.com", "https://www.pling.com", "https://www.pling.com/browse?cat=124&ord=downloads", "api.pling.com", "api.pling.com", "https://www.pling.com/p/%d"},
}

var httpClient = &http.Client{Timeout: 120 * time.Second}

func initAppDirs() {
	migrateDataDir()
	for _, d := range []string{baseDir, optimizedDir, launchDir, downloadsDir, themeDirs[0], defaultImportDir} {
		_ = os.MkdirAll(d, 0o755)
	}
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
	}
	log.SetFlags(log.Ldate | log.Ltime)
}

func migrateDataDir() {
	st, err := os.Stat(oldBaseDir)
	if err != nil || !st.IsDir() || oldBaseDir == baseDir {
		return
	}
	_ = os.MkdirAll(baseDir, 0o755)
	_ = os.MkdirAll(optimizedDir, 0o755)
	for _, name := range []string{"settings.json", "profiles.json", "manager.log"} {
		oldP, newP := filepath.Join(oldBaseDir, name), filepath.Join(baseDir, name)
		if fileExists(oldP) && !fileExists(newP) {
			_ = copyFile(oldP, newP)
		}
	}
	oldOpt := filepath.Join(oldBaseDir, "optimized")
	entries, err := os.ReadDir(oldOpt)
	if err != nil {
		return
	}
	for _, e := range entries {
		src, dst := filepath.Join(oldOpt, e.Name()), filepath.Join(optimizedDir, e.Name())
		if !e.IsDir() && !fileExists(dst) {
			_ = copyFile(src, dst)
		}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func isDir(p string) bool      { st, err := os.Stat(p); return err == nil && st.IsDir() }
func isFile(p string) bool     { st, err := os.Stat(p); return err == nil && !st.IsDir() }

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	if st, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, st.Mode())
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func removeAllIfExists(p string) {
	if fileExists(p) || isDir(p) {
		_ = os.RemoveAll(p)
	}
}

func loadJSON(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func saveJSON(path string, obj any) error {
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadSettings() Settings {
	s := defaultSettings()
	_ = loadJSON(settingsFile, &s)
	if s.MaxInstances < 1 {
		s.MaxInstances = 1
	}
	if s.HealthcheckSeconds <= 0 {
		s.HealthcheckSeconds = 2
	}
	if s.PreviewSeconds < 1 {
		s.PreviewSeconds = 5
	}
	return s
}

func detectEnvironment() (session, desktop string) {
	session = os.Getenv("XDG_SESSION_TYPE")
	if session == "" {
		session = "unknown"
	}
	desktop = os.Getenv("XDG_CURRENT_DESKTOP")
	if desktop == "" {
		desktop = "unknown"
	}
	return
}

func detectDesktopEnvironment() string {
	combined := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + " " + os.Getenv("DESKTOP_SESSION") + " " + os.Getenv("XDG_SESSION_DESKTOP"))
	switch {
	case strings.Contains(combined, "hyprland"):
		return "hyprland"
	case strings.Contains(combined, "niri"):
		return "niri"
	case strings.Contains(combined, "sway"):
		return "sway"
	case strings.Contains(combined, "cosmic"):
		return "cosmic"
	case strings.Contains(combined, "kde") || strings.Contains(combined, "plasma"):
		return "kde"
	case strings.Contains(combined, "gnome"):
		return "gnome"
	case strings.Contains(combined, "xfce"):
		return "xfce"
	case strings.Contains(combined, "cinnamon"):
		return "cinnamon"
	case strings.Contains(combined, "mate"):
		return "mate"
	case strings.Contains(combined, "lxqt"):
		return "lxqt"
	case strings.Contains(combined, "lxde"):
		return "lxde"
	default:
		return "generic"
	}
}

func displayServerKind() string {
	sess := strings.ToLower(os.Getenv("XDG_SESSION_TYPE"))
	if sess == "wayland" {
		if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") != "" {
			return "xwayland"
		}
		return "wayland"
	}
	if sess == "x11" || os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return sess
}

func compositorLabel(de, session string) string {
	kind := displayServerKind()
	return fmt.Sprintf("%s · %s · %s", strings.ToUpper(de), session, kind)
}

func resolveRuntimeSettings(user Settings) Settings {
	merged := user
	preset := merged.DesktopPreset
	if preset == "" {
		preset = "auto"
	}
	de := detectDesktopEnvironment()
	if preset != "auto" {
		de = preset
	}
	def, ok := dePresets[de]
	if !ok {
		def = dePresets["generic"]
	}
	if preset == "auto" {
		if merged.Layer == "" {
			merged.Layer = def[0]
		}
		if merged.WindowType == "" {
			merged.WindowType = def[1]
		}
	}
	return merged
}

func which(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isConkyInstalled() bool { return which("conky") }

func conkyVersion() string {
	out, err := exec.Command("conky", "-v").CombinedOutput()
	if err != nil {
		return ""
	}
	first := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return first
}

func defaultLibrary() ThemeLibrary {
	return ThemeLibrary{Favorites: map[string]bool{}, Recent: []RecentEntry{}, CustomDirs: []string{}}
}

func loadLibrary() ThemeLibrary {
	lib := defaultLibrary()
	_ = loadJSON(libraryFile, &lib)
	if lib.Favorites == nil {
		lib.Favorites = map[string]bool{}
	}
	return lib
}

func saveLibrary(lib ThemeLibrary) { _ = saveJSON(libraryFile, lib) }

func loadRuntime() RuntimeFile {
	var r RuntimeFile
	_ = loadJSON(runtimeFile, &r)
	return r
}

func saveRuntime(r RuntimeFile) { _ = saveJSON(runtimeFile, r) }

func themeID(cfgPath string) string { return resolvePath(cfgPath) }

func isFavorite(cfgPath string) bool {
	return loadLibrary().Favorites[themeID(cfgPath)]
}

func setFavorite(cfgPath string, on bool) {
	lib := loadLibrary()
	id := themeID(cfgPath)
	if on {
		lib.Favorites[id] = true
	} else {
		delete(lib.Favorites, id)
	}
	saveLibrary(lib)
}

func recordRecent(cfgPath string) {
	lib := loadLibrary()
	id := themeID(cfgPath)
	var next []RecentEntry
	next = append(next, RecentEntry{Path: id, At: time.Now().Unix()})
	for _, r := range lib.Recent {
		if resolvePath(r.Path) != id {
			next = append(next, r)
		}
	}
	if len(next) > maxRecentThemes {
		next = next[:maxRecentThemes]
	}
	lib.Recent = next
	saveLibrary(lib)
}

func recentSet() map[string]bool {
	out := map[string]bool{}
	for _, r := range loadLibrary().Recent {
		out[resolvePath(r.Path)] = true
	}
	return out
}

func allThemeRoots() []string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || !isDir(p) {
			return
		}
		r := resolvePath(p)
		if seen[r] {
			return
		}
		seen[r] = true
		roots = append(roots, r)
	}
	for _, r := range themeDirs {
		add(r)
	}
	s := loadSettings()
	for _, r := range s.CustomThemeDirs {
		add(r)
	}
	for _, r := range loadLibrary().CustomDirs {
		add(r)
	}
	return roots
}

func listMonitors() []MonitorInfo {
	var mons []MonitorInfo
	disp, err := gdk.DisplayGetDefault()
	if err != nil || disp == nil {
		return []MonitorInfo{{ID: "primary", Name: "Primary", Width: 1920, Height: 1080, Scale: 1, Primary: true, Head: 1}}
	}
	n := disp.GetNMonitors()
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		mon, err := disp.GetMonitor(i)
		if err != nil || mon == nil {
			continue
		}
		g := mon.GetGeometry()
		name := strings.TrimSpace(mon.GetModel())
		man := strings.TrimSpace(mon.GetManufacturer())
		scale := mon.GetScaleFactor()
		if scale < 1 {
			scale = 1
		}
		id := strings.TrimSpace(strings.Join([]string{man, name}, " "))
		if id == "" {
			id = fmt.Sprintf("Monitor %d", i+1)
		}
		primary := false
		if pm, err := disp.GetPrimaryMonitor(); err == nil && pm != nil {
			pg := pm.GetGeometry()
			primary = pg.GetX() == g.GetX() && pg.GetY() == g.GetY() && pg.GetWidth() == g.GetWidth()
		}
		if i == 0 && n == 1 {
			primary = true
		}
		mons = append(mons, MonitorInfo{
			ID: id, Name: id, Manufacturer: man, Model: name,
			X: g.GetX(), Y: g.GetY(), Width: g.GetWidth(), Height: g.GetHeight(),
			Scale: scale, Primary: primary, Head: i + 1,
		})
	}
	if len(mons) == 0 {
		mons = []MonitorInfo{{ID: "primary", Name: "Primary", Width: 1920, Height: 1080, Scale: 1, Primary: true, Head: 1}}
	}
	return mons
}

func findMonitor(id string, head int) *MonitorInfo {
	mons := listMonitors()
	if id != "" {
		for i := range mons {
			if mons[i].ID == id || mons[i].Name == id || mons[i].Model == id {
				return &mons[i]
			}
		}
	}
	if head > 0 {
		for i := range mons {
			if mons[i].Head == head {
				return &mons[i]
			}
		}
	}
	for i := range mons {
		if mons[i].Primary {
			return &mons[i]
		}
	}
	if len(mons) > 0 {
		return &mons[0]
	}
	return nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func readProcCmdline(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(bytes.TrimRight(b, "\x00")), "\x00", " ")
}

func scanConkyProcesses() []struct {
	PID     int
	Cmdline string
} {
	var out []struct {
		PID     int
		Cmdline string
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		cmd := exec.Command("pgrep", "-af", "conky")
		b, err := cmd.Output()
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			cmdl := strings.Join(fields[1:], " ")
			if !strings.Contains(cmdl, "conky") {
				continue
			}
			out = append(out, struct {
				PID     int
				Cmdline string
			}{pid, cmdl})
		}
		return out
	}
	self := os.Getpid()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		cmdl := readProcCmdline(pid)
		if cmdl == "" {
			continue
		}
		base := strings.ToLower(filepath.Base(strings.Fields(cmdl)[0]))
		if base != "conky" && !strings.Contains(cmdl, " conky ") && !strings.HasPrefix(cmdl, "conky ") {
			continue
		}
		if strings.Contains(cmdl, "conky-manager") && !strings.Contains(cmdl, " -c ") {
			continue
		}
		out = append(out, struct {
			PID     int
			Cmdline string
		}{pid, cmdl})
	}
	return out
}

func pruneRuntime() RuntimeFile {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	r := loadRuntime()
	live := scanConkyProcesses()
	alive := map[int]string{}
	for _, p := range live {
		alive[p.PID] = p.Cmdline
	}
	var kept []ManagedProcess
	for _, p := range r.Processes {
		if _, ok := alive[p.PID]; ok && pidAlive(p.PID) {
			kept = append(kept, p)
		}
	}
	r.Processes = kept
	saveRuntime(r)
	return r
}

func registerManagedProcess(p ManagedProcess) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	r := loadRuntime()
	var next []ManagedProcess
	for _, old := range r.Processes {
		if old.ThemeID == p.ThemeID || old.PID == p.PID {
			if old.PID != p.PID && pidAlive(old.PID) {
				stopPID(old.PID, false)
			}
			continue
		}
		next = append(next, old)
	}
	next = append(next, p)
	r.Processes = next
	saveRuntime(r)
}

func markProcessPreview(pid int, preview bool) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	r := loadRuntime()
	for i := range r.Processes {
		if r.Processes[i].PID == pid {
			r.Processes[i].Preview = preview
		}
	}
	saveRuntime(r)
}

func managedForTheme(cfgPath string) *ManagedProcess {
	id := themeID(cfgPath)
	r := pruneRuntime()
	for i := range r.Processes {
		if r.Processes[i].ThemeID == id {
			cp := r.Processes[i]
			return &cp
		}
	}
	return nil
}

func runtimePIDMap() map[string]int {
	out := map[string]int{}
	for _, p := range pruneRuntime().Processes {
		out[p.ThemeID] = p.PID
	}
	return out
}

func stopPID(pid int, force bool) {
	if pid <= 0 || !pidAlive(pid) {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(80 * time.Millisecond)
	}
	if force {
		_ = proc.Signal(syscall.SIGKILL)
	}
}

func stopManagedTheme(cfgPath string) bool {
	id := themeID(cfgPath)
	r := pruneRuntime()
	found := false
	var next []ManagedProcess
	for _, p := range r.Processes {
		if p.ThemeID == id {
			stopPID(p.PID, true)
			found = true
			continue
		}
		next = append(next, p)
	}
	r.Processes = next
	saveRuntime(r)
	return found
}

func stopAllManaged() int {
	r := pruneRuntime()
	n := 0
	for _, p := range r.Processes {
		stopPID(p.PID, true)
		n++
	}
	saveRuntime(RuntimeFile{})
	return n
}

func killOrphanedConky() int {
	r := pruneRuntime()
	managed := map[int]bool{}
	for _, p := range r.Processes {
		managed[p.PID] = true
	}
	n := 0
	for _, proc := range scanConkyProcesses() {
		if managed[proc.PID] {
			continue
		}
		cmdl := proc.Cmdline
		if strings.Contains(cmdl, launchDir) || strings.Contains(cmdl, "CONKY_MANAGER") || strings.Contains(cmdl, optimizedDir) {
			stopPID(proc.PID, true)
			n++
		}
	}
	return n
}

func killUnmanagedConky() int {
	r := pruneRuntime()
	managed := map[int]bool{}
	for _, p := range r.Processes {
		managed[p.PID] = true
	}
	n := 0
	for _, proc := range scanConkyProcesses() {
		if managed[proc.PID] {
			continue
		}
		stopPID(proc.PID, true)
		n++
	}
	return n
}

var runtimeMu sync.Mutex

func pausePID(pid int) error {
	if !pidAlive(pid) {
		return fmt.Errorf("process is not running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGSTOP)
}

func resumePID(pid int) error {
	if !pidAlive(pid) {
		return fmt.Errorf("process is not running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGCONT)
}

func remapStoredPath(oldPath, newPath string) {
	oldID, newID := resolvePath(oldPath), resolvePath(newPath)
	pos := loadPositions()
	if v, ok := pos.Themes[oldID]; ok {
		pos.Themes[newID] = v
		delete(pos.Themes, oldID)
		savePositions(pos)
	}
	cols := loadColors()
	if v, ok := cols.Themes[oldID]; ok {
		cols.Themes[newID] = v
		delete(cols.Themes, oldID)
		saveColors(cols)
	}
	lib := loadLibrary()
	if lib.Favorites[oldID] {
		delete(lib.Favorites, oldID)
		lib.Favorites[newID] = true
	}
	for i := range lib.Recent {
		if resolvePath(lib.Recent[i].Path) == oldID {
			lib.Recent[i].Path = newID
		}
	}
	saveLibrary(lib)
	pf := loadProfiles()
	for name, paths := range pf.Profiles {
		for i, p := range paths {
			if resolvePath(p) == oldID {
				pf.Profiles[name][i] = newID
			}
		}
	}
	for name, dp := range pf.Desktop {
		for i, t := range dp.Themes {
			if resolvePath(t.Path) == oldID {
				pf.Desktop[name].Themes[i].Path = newID
			}
		}
	}
	saveProfiles(pf)
	r := loadRuntime()
	for i := range r.Processes {
		if r.Processes[i].ThemeID == oldID {
			r.Processes[i].ThemeID = newID
			r.Processes[i].ConfigPath = newID
		}
	}
	saveRuntime(r)
}

func resolveThemeColors(cfgPath string) *ColorOverride {
	data := loadColors()
	if c, ok := data.Themes[resolvePath(cfgPath)]; ok && c.Enabled && len(c.Colors) > 0 {
		cp := c
		return &cp
	}
	return nil
}

func dirSize(root string) int64 {
	var n int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n += info.Size()
		}
		return nil
	})
	return n
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
}

func listThemeFiles(root string, suffixes ...string) []string {
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		low := strings.ToLower(path)
		for _, s := range suffixes {
			if strings.HasSuffix(low, s) {
				out = append(out, path)
				break
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

var execCommandPattern = regexp.MustCompile(`(?i)\$\{(?:exec|execi|execpi|execp|texeci|texecpi)\s+(?:\d+\s+)?([^}]+)\}`)
var shellWordPattern = regexp.MustCompile("`([^`]+)`")

var trackedCommands = []string{
	"curl", "wget", "jq", "playerctl", "sensors", "nmcli", "mpstat", "iostat",
	"nvidia-smi", "amixer", "pactl", "wpctl", "awk", "bc", "python3", "python",
	"perl", "ruby", "lua", "ffmpeg", "convert", "notify-send", "dunstify",
	"acpi", "upower", "iw", "iwconfig", "htop", "iotop", "free", "df", "ping",
	"ip", "ss", "lscpu", "lsblk", "smartctl", "amdgpu_top", "rocm-smi",
}

func extractReferencedCommands(text string) []string {
	seen := map[string]bool{}
	addLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return
		}
		line = strings.TrimPrefix(line, "sudo ")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return
		}
		cmd := filepath.Base(fields[0])
		cmd = strings.Trim(cmd, `"'`)
		if cmd == "" || strings.ContainsAny(cmd, "=$") {
			return
		}
		seen[cmd] = true
	}
	for _, m := range execCommandPattern.FindAllStringSubmatch(text, -1) {
		addLine(m[1])
	}
	for _, m := range shellWordPattern.FindAllStringSubmatch(text, -1) {
		addLine(m[1])
	}
	low := strings.ToLower(text)
	for _, c := range trackedCommands {
		if strings.Contains(low, c) {
			seen[c] = true
		}
	}
	var out []string
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func collectThemeDependencies(cfgPath string) (present, missing []string) {
	root := resolveThemeRoot(cfgPath)
	text := readText(cfgPath)
	for _, sh := range listThemeFiles(root, ".sh") {
		text += "\n" + readText(sh)
	}
	for _, lua := range listThemeFiles(root, ".lua") {
		text += "\n" + readText(lua)
	}
	ignore := map[string]bool{
		"conky": true, "echo": true, "cat": true, "grep": true, "sed": true,
		"head": true, "tail": true, "tr": true, "cut": true, "date": true,
		"sleep": true, "true": true, "false": true, "test": true, "printf": true,
		"ls": true, "basename": true, "dirname": true, "uname": true, "whoami": true,
	}
	seen := map[string]bool{}
	for _, cmd := range extractReferencedCommands(text) {
		if ignore[cmd] || seen[cmd] {
			continue
		}
		seen[cmd] = true
		if which(cmd) || (cmd == "sensors" && which("sensors")) {
			present = append(present, cmd)
		} else {
			missing = append(missing, cmd)
		}
	}
	sort.Strings(present)
	sort.Strings(missing)
	return present, missing
}

func suggestedInstallCommand(pkgs []string) string {
	if len(pkgs) == 0 {
		return ""
	}
	mapped := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		switch p {
		case "sensors":
			mapped = append(mapped, "lm_sensors")
		default:
			mapped = append(mapped, p)
		}
	}
	switch {
	case which("dnf"):
		return "sudo dnf install -y " + strings.Join(mapped, " ")
	case which("apt"):
		return "sudo apt install -y " + strings.Join(mapped, " ")
	case which("pacman"):
		return "sudo pacman -S --needed " + strings.Join(mapped, " ")
	case which("zypper"):
		return "sudo zypper install -y " + strings.Join(mapped, " ")
	default:
		return "install: " + strings.Join(mapped, " ")
	}
}

func inspectThemeHealth(cfgPath string) ThemeHealth {
	h := ThemeHealth{Status: "stable", Scanned: time.Now().Unix()}
	add := func(sev, code, msg, fix string) {
		h.Issues = append(h.Issues, HealthIssue{Severity: sev, Code: code, Message: msg, Fix: fix})
	}
	if !isFile(cfgPath) {
		h.Status = "broken"
		add("error", "missing_config", "Configuration file is missing.", "Restore the theme or re-import it.")
		return h
	}
	content := readText(cfgPath)
	if !isValidTheme(cfgPath) {
		h.Status = "broken"
		add("error", "syntax", "File does not look like a Conky configuration.", "Open the config and repair conky.config / TEXT sections.")
	}
	if st, err := os.Stat(cfgPath); err == nil {
		if st.Mode().Perm()&0o400 == 0 {
			h.Status = "broken"
			add("error", "permissions", "Configuration is not readable.", "chmod u+r on the theme config.")
		}
	}
	found, missing := scanThemeAssets(cfgPath)
	_ = found
	for _, m := range missing {
		low := strings.ToLower(m)
		code := "broken_path"
		fix := "Place the missing file in the theme folder or update the path."
		switch {
		case strings.HasSuffix(low, ".lua"):
			code = "missing_lua"
		case strings.HasSuffix(low, ".sh"):
			code = "missing_script"
		case strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") || strings.HasSuffix(low, ".svg"):
			code = "missing_image"
		case strings.HasSuffix(low, ".ttf") || strings.HasSuffix(low, ".otf"):
			code = "missing_font"
		}
		add("warning", code, "Missing asset: "+m, fix)
	}
	_, missCmds := collectThemeDependencies(cfgPath)
	for _, c := range missCmds {
		add("warning", "missing_dependency", "Missing command: "+c, suggestedInstallCommand([]string{c}))
	}
	de := detectDesktopEnvironment()
	kind := displayServerKind()
	if kind == "wayland" && (de == "hyprland" || de == "sway" || de == "niri") {
		add("info", "wayland", "Running on "+de+" (Wayland). Conky may need XWayland or compositor-specific window rules.", "Keep own_window enabled and try Layer = above if the theme is invisible.")
	}
	ver := conkyVersion()
	if ver != "" && strings.Contains(content, "conky.config") && strings.Contains(strings.ToLower(ver), "conky 1.9") {
		add("warning", "conky_version", "Theme uses Lua syntax but Conky reports an older version: "+ver, "Install Conky 1.10+ for this theme.")
	}
	bundle, err := createThemeLaunchBundle(cfgPath, true, resolveThemePosition(cfgPath, nil), resolveThemeColors(cfgPath))
	if err != nil {
		h.Status = "broken"
		add("error", "launch_bundle", err.Error(), "Run Smart Repair, then validate the theme.")
	} else if !validateConkyConfig(bundle.LaunchConfig, bundle.LaunchDir, buildLaunchEnv(bundle)) {
		h.Status = "broken"
		add("error", "runtime", "Conky rejected the launch configuration.", "Check manager.log for the parser error, then repair the config.")
	}
	hasErr, hasWarn := false, false
	for _, iss := range h.Issues {
		if iss.Severity == "error" {
			hasErr = true
		}
		if iss.Severity == "warning" {
			hasWarn = true
		}
	}
	if hasErr {
		h.Status = "broken"
	} else if hasWarn {
		h.Status = "needs_fix"
	} else {
		h.Status = "stable"
	}
	return h
}

func listBackups(cfgPath string) []string {
	dir := filepath.Join(baseDir, "backups", filepath.Base(filepath.Dir(cfgPath)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	base := filepath.Base(cfgPath)
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] > out[j] })
	return out
}

func unifiedDiff(a, b, nameA, nameB string) string {
	la := strings.Split(strings.ReplaceAll(a, "\r\n", "\n"), "\n")
	lb := strings.Split(strings.ReplaceAll(b, "\r\n", "\n"), "\n")
	var buf strings.Builder
	buf.WriteString("--- " + nameA + "\n+++ " + nameB + "\n")
	max := len(la)
	if len(lb) > max {
		max = len(lb)
	}
	for i := 0; i < max; i++ {
		var sa, sb string
		if i < len(la) {
			sa = la[i]
		}
		if i < len(lb) {
			sb = lb[i]
		}
		if sa == sb {
			continue
		}
		if i < len(la) {
			buf.WriteString("-" + sa + "\n")
		}
		if i < len(lb) {
			buf.WriteString("+" + sb + "\n")
		}
	}
	if buf.Len() < 40 {
		return "(no textual differences)"
	}
	return buf.String()
}

func trashPath(path string) error {
	if which("gio") {
		cmd := exec.Command("gio", "trash", path)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	trash := filepath.Join(homeDir, ".local", "share", "Trash", "files")
	_ = os.MkdirAll(trash, 0o755)
	dest := filepath.Join(trash, filepath.Base(path))
	if fileExists(dest) || isDir(dest) {
		dest = dest + "-" + time.Now().Format("20060102-150405")
	}
	return os.Rename(path, dest)
}

func uniqueDir(parent, name string) string {
	dest := filepath.Join(parent, name)
	if !fileExists(dest) && !isDir(dest) {
		return dest
	}
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s-%d", dest, i)
		if !fileExists(cand) && !isDir(cand) {
			return cand
		}
	}
	return dest + "-" + strconv.FormatInt(time.Now().Unix(), 10)
}

func rewriteTextPaths(content, oldRoot, newRoot string) string {
	repls := [][2]string{
		{oldRoot, newRoot},
		{filepath.Base(oldRoot), filepath.Base(newRoot)},
	}
	for _, r := range repls {
		if r[0] != "" && r[0] != r[1] {
			content = strings.ReplaceAll(content, r[0], r[1])
		}
	}
	return content
}

func cloneTheme(cfgPath, newName string) (string, error) {
	root := resolveThemeRoot(cfgPath)
	parent := filepath.Dir(root)
	name := normalizeFolderName(newName)
	if name == "" {
		name = filepath.Base(root) + "-copy"
	}
	dest := uniqueDir(parent, name)
	if err := copyTree(root, dest); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, cfgPath)
	if err != nil {
		return "", err
	}
	newCfg := filepath.Join(dest, rel)
	_ = os.WriteFile(newCfg, []byte(rewriteTextPaths(readText(newCfg), root, dest)), 0o644)
	oldPos := resolveThemePosition(cfgPath, nil)
	if oldPos != nil {
		data := loadPositions()
		data.Themes[resolvePath(newCfg)] = *oldPos
		savePositions(data)
	}
	if c := resolveThemeColors(cfgPath); c != nil {
		cols := loadColors()
		cols.Themes[resolvePath(newCfg)] = *c
		saveColors(cols)
	}
	return newCfg, nil
}

func renameTheme(cfgPath, newName string) (string, error) {
	root := resolveThemeRoot(cfgPath)
	parent := filepath.Dir(root)
	name := normalizeFolderName(newName)
	if name == "" {
		return "", fmt.Errorf("invalid name")
	}
	dest := filepath.Join(parent, name)
	if dest == root {
		return cfgPath, nil
	}
	if fileExists(dest) || isDir(dest) {
		return "", fmt.Errorf("a folder named %s already exists", name)
	}
	if err := os.Rename(root, dest); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, cfgPath)
	if err != nil {
		return "", err
	}
	newCfg := filepath.Join(dest, rel)
	_ = os.WriteFile(newCfg, []byte(rewriteTextPaths(readText(newCfg), root, dest)), 0o644)
	remapStoredPath(cfgPath, newCfg)
	return newCfg, nil
}

func exportThemeArchive(cfgPath, destFile string) error {
	root := resolveThemeRoot(cfgPath)
	if destFile == "" {
		destFile = filepath.Join(homeDir, filepath.Base(root)+".conky-theme.tar.gz")
	}
	cmd := exec.Command("tar", "-C", filepath.Dir(root), "-czf", destFile, filepath.Base(root))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func loadCustomPalettes() []CustomPalette {
	var out []CustomPalette
	_ = loadJSON(palettesFile, &out)
	return out
}

func saveCustomPalettes(p []CustomPalette) { _ = saveJSON(palettesFile, p) }

func generatePaletteFromAccent(accent string) map[string]string {
	n := normalizeColor(accent)
	if n == "" {
		n = "#7AA2F7"
	}
	body := n[1:]
	r, _ := strconv.ParseInt(body[0:2], 16, 64)
	g, _ := strconv.ParseInt(body[2:4], 16, 64)
	b, _ := strconv.ParseInt(body[4:6], 16, 64)
	mix := func(rr, gg, bb, t int64) string {
		return fmt.Sprintf("#%02X%02X%02X", (r*(100-t)+rr*t)/100, (g*(100-t)+gg*t)/100, (b*(100-t)+bb*t)/100)
	}
	return map[string]string{
		"default_color":         mix(255, 255, 255, 70),
		"color1":                n,
		"color2":                mix(80, 180, 255, 45),
		"default_outline_color": mix(40, 40, 50, 55),
		"default_shade_color":   mix(16, 16, 24, 80),
	}
}

func confirmDialog(parent *gtk.Window, title, message string) bool {
	dlg := gtk.MessageDialogNew(parent, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO, "%s", title)
	dlg.FormatSecondaryText("%s", message)
	resp := dlg.Run()
	dlg.Destroy()
	return resp == gtk.RESPONSE_YES
}

func promptText(parent *gtk.Window, title, label, initial string) (string, bool) {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle(title)
	dlg.SetTransientFor(parent)
	dlg.SetDefaultSize(420, 120)
	box, _ := dlg.GetContentArea()
	setMargins(box, 12)
	box.PackStart(newLabel(label, false), false, false, 0)
	entry, _ := gtk.EntryNew()
	entry.SetText(initial)
	box.PackStart(entry, false, false, 8)
	dlg.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dlg.AddButton("OK", gtk.RESPONSE_OK)
	dlg.ShowAll()
	resp := dlg.Run()
	txt, _ := entry.GetText()
	dlg.Destroy()
	return strings.TrimSpace(txt), resp == gtk.RESPONSE_OK
}

func readText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func isValidTheme(cfg string) bool {
	text := readText(cfg)
	return strings.Contains(text, "conky.config") || strings.Contains(text, "own_window") || strings.Contains(text, "TEXT")
}

func findThemes() []string {
	var themes []string
	seen := map[string]bool{}
	for _, root := range allThemeRoots() {
		if !isDir(root) {
			continue
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			name := info.Name()
			if strings.HasSuffix(strings.ToLower(name), ".conf") || name == "conkyrc" {
				if isValidTheme(path) {
					resolved, _ := filepath.Abs(path)
					if resolved, err := filepath.EvalSymlinks(resolved); err == nil {
						if !seen[resolved] {
							seen[resolved] = true
							themes = append(themes, resolved)
						}
					}
				}
			}
			return nil
		})
	}
	sort.Strings(themes)
	return themes
}

func isNewSyntax(content string) bool {
	return strings.Contains(content, "conky.config") && strings.Contains(content, "conky.text")
}

func windowHints(desktopEnv, layer string, newSyntax bool) string {
	above := layer == "above"
	type pair [2]string
	hints := map[string]pair{
		"kde":      {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"gnome":    {"undecorated,above,skip_taskbar,skip_pager", "undecorated,below,skip_taskbar,skip_pager"},
		"xfce":     {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"cinnamon": {"undecorated,above,skip_taskbar,skip_pager", "undecorated,below,skip_taskbar,skip_pager"},
		"mate":     {"undecorated,above,skip_taskbar,skip_pager", "undecorated,below,skip_taskbar,skip_pager"},
		"lxqt":     {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"lxde":     {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"hyprland": {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"niri":     {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"sway":     {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
		"cosmic":   {"undecorated,above,skip_taskbar,skip_pager", "undecorated,below,skip_taskbar,skip_pager"},
		"generic":  {"undecorated,above,sticky,skip_taskbar,skip_pager", "undecorated,below,sticky,skip_taskbar,skip_pager"},
	}
	h, ok := hints[desktopEnv]
	if !ok {
		h = hints["generic"]
	}
	raw := h[1]
	if above {
		raw = h[0]
	}
	if newSyntax {
		return `"` + raw + `"`
	}
	return raw
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func computeDesktopSettings(settings Settings, newSyntax bool) map[string]string {
	runtime := resolveRuntimeSettings(settings)
	upd, cpu, net := "1.0", "2", "2"
	switch runtime.Smoothness {
	case "ultra":
		upd, cpu, net = "1.5", "3", "3"
	case "performance":
		upd, cpu, net = "0.5", "2", "2"
	}
	layer, wtype := runtime.Layer, runtime.WindowType
	if layer == "" {
		layer = "above"
	}
	if wtype == "" {
		wtype = "dock"
	}
	preset := runtime.DesktopPreset
	if preset == "" {
		preset = "auto"
	}
	de := detectDesktopEnvironment()
	if preset != "auto" {
		de = preset
	}
	var pos map[string]string
	if runtime.PositionEnabled {
		pos = map[string]string{
			"alignment":     runtime.Alignment,
			"gap_x":         strconv.Itoa(runtime.GapX),
			"gap_y":         strconv.Itoa(runtime.GapY),
			"xinerama_head": strconv.Itoa(runtime.XineramaHead),
		}
		if runtime.XineramaHead <= 0 {
			delete(pos, "xinerama_head")
		}
	}
	if newSyntax {
		base := cloneMap(desktopSettingsNew)
		base["update_interval"], base["cpu_avg_samples"], base["net_avg_samples"] = upd, cpu, net
		base["own_window_type"] = `"` + wtype + `"`
		base["own_window_hints"] = windowHints(de, layer, true)
		if pos != nil {
			base["alignment"] = "'" + pos["alignment"] + "'"
			base["gap_x"], base["gap_y"] = pos["gap_x"], pos["gap_y"]
			if pos["xinerama_head"] != "" {
				base["xinerama_head"] = pos["xinerama_head"]
			}
		}
		return base
	}
	base := cloneMap(desktopSettingsLegacy)
	base["update_interval"], base["cpu_avg_samples"], base["net_avg_samples"] = upd, cpu, net
	base["own_window_type"] = wtype
	base["own_window_hints"] = windowHints(de, layer, false)
	if pos != nil {
		base["alignment"], base["gap_x"], base["gap_y"] = pos["alignment"], pos["gap_x"], pos["gap_y"]
		if pos["xinerama_head"] != "" {
			base["xinerama_head"] = pos["xinerama_head"]
		}
	}
	return base
}

func defaultPositions() PositionsFile {
	return PositionsFile{
		Default: Position{Enabled: true, Alignment: "top_left", GapX: 30, GapY: 50},
		Themes:  map[string]Position{},
	}
}

func loadPositions() PositionsFile {
	data := defaultPositions()
	_ = loadJSON(positionsFile, &data)
	if data.Themes == nil {
		data.Themes = map[string]Position{}
	}
	if data.Default.Alignment == "" {
		data.Default.Alignment = "top_left"
	}
	return data
}

func savePositions(data PositionsFile) { _ = saveJSON(positionsFile, data) }

func parseThemePosition(content string) Position {
	res := Position{Alignment: "top_left", GapX: 30, GapY: 50}
	var re *regexp.Regexp
	if isNewSyntax(content) {
		re = regexp.MustCompile(`(?im)^\s*(alignment|gap_x|gap_y)\s*=\s*([^,\n]+)`)
	} else {
		re = regexp.MustCompile(`(?im)^\s*(alignment|gap_x|gap_y)\s+(\S+)`)
	}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		key := strings.ToLower(m[1])
		val := strings.Trim(strings.TrimSpace(m[2]), `"'`)
		if key == "alignment" {
			res.Alignment = val
		} else if n, err := strconv.Atoi(strings.Split(val, ".")[0]); err == nil {
			if key == "gap_x" {
				res.GapX = n
			} else {
				res.GapY = n
			}
		}
	}
	return res
}

func resolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	return abs
}

func resolveThemePosition(cfgPath string, positions *PositionsFile) *Position {
	p := loadPositions()
	if positions != nil {
		p = *positions
	}
	saved, ok := p.Themes[resolvePath(cfgPath)]
	if ok && (saved.Enabled || saved.Alignment != "" || saved.GapX != 0 || saved.GapY != 0) {
		if saved.Alignment == "" {
			saved.Alignment = "top_left"
		}
		return &saved
	}
	return nil
}

func mergePositionSettings(base Settings, cfgPath string, position *Position) Settings {
	merged := base
	effective := position
	if effective == nil {
		effective = resolveThemePosition(cfgPath, nil)
	}
	if effective == nil {
		return merged
	}
	merged.PositionEnabled = true
	merged.Alignment = effective.Alignment
	merged.GapX, merged.GapY = effective.GapX, effective.GapY
	merged.Monitor = effective.Monitor
	merged.XineramaHead = effective.XineramaHead
	if loadSettings().ScaleAwarePosition {
		if mon := findMonitor(effective.Monitor, effective.XineramaHead); mon != nil && mon.Scale > 1 {
			merged.GapX = effective.GapX * mon.Scale
			merged.GapY = effective.GapY * mon.Scale
		}
	}
	return merged
}

func patchNewSyntax(content string, settings Settings) string {
	start := strings.Index(content, "conky.config")
	if start < 0 {
		return content
	}
	brace := strings.Index(content[start:], "{")
	if brace < 0 {
		return content
	}
	braceIdx := start + brace
	depth := 0
	endIdx := -1
	for i := braceIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				endIdx = i
			}
		}
		if endIdx != -1 {
			break
		}
	}
	if endIdx == -1 {
		return content
	}
	block := content[braceIdx+1 : endIdx]
	desktop := computeDesktopSettings(settings, true)
	for key, value := range desktop {
		re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*=\s*.*?,\s*$`)
		repl := "    " + key + " = " + value + ","
		if loc := re.FindStringIndex(block); loc != nil {
			block = block[:loc[0]] + repl + block[loc[1]:]
		} else {
			block = repl + "\n" + block
		}
	}
	return content[:braceIdx+1] + block + content[endIdx:]
}

func patchLegacySyntax(content string, settings Settings) string {
	lines := strings.Split(content, "\n")
	textIdx := len(lines)
	for i, line := range lines {
		if strings.ToUpper(strings.TrimSpace(line)) == "TEXT" {
			textIdx = i
			break
		}
	}
	head := append([]string{}, lines[:textIdx]...)
	tail := lines[textIdx:]
	keyToIdx := map[string]int{}
	for i, line := range head {
		st := strings.TrimSpace(line)
		if st == "" || strings.HasPrefix(st, "#") {
			continue
		}
		key := strings.Fields(st)[0]
		keyToIdx[key] = i
	}
	desktop := computeDesktopSettings(settings, false)
	for key, value := range desktop {
		newLine := key + " " + value
		if idx, ok := keyToIdx[key]; ok {
			head[idx] = newLine
		} else {
			head = append([]string{newLine}, head...)
			for k, v := range keyToIdx {
				keyToIdx[k] = v + 1
			}
			keyToIdx[key] = 0
		}
	}
	return strings.Join(append(head, tail...), "\n") + "\n"
}

func applyPositionPatch(content string, position Position) string {
	s := defaultSettings()
	s.PositionEnabled = true
	s.Alignment, s.GapX, s.GapY = position.Alignment, position.GapX, position.GapY
	if isNewSyntax(content) {
		return patchNewSyntax(content, s)
	}
	return patchLegacySyntax(content, s)
}

// writeOriginalThemeConfig deliberately writes to the discovered source config.
// Launch bundles remain runtime copies only; user edits must never disappear when
// a preview or optimization bundle is rebuilt.
func writeOriginalThemeConfig(cfgPath, updated, reason string) error {
	cfgPath = resolvePath(cfgPath)
	if !isFile(cfgPath) {
		return fmt.Errorf("theme configuration not found: %s", cfgPath)
	}
	original, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	backupDir := filepath.Join(baseDir, "backups", filepath.Base(filepath.Dir(cfgPath)))
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().Format("20060102-150405")
	backup := filepath.Join(backupDir, filepath.Base(cfgPath)+"."+stamp+"."+normalizeFolderName(reason)+".bak")
	if err := os.WriteFile(backup, original, 0o644); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(cfgPath), ".conky-manager-edit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.WriteString(updated); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err = os.Rename(tmpName, cfgPath); err != nil {
		return err
	}
	log.Printf("[INFO] Updated original theme config: %s (backup: %s)", cfgPath, backup)
	return nil
}

func loadColors() ColorsFile {
	data := ColorsFile{Themes: map[string]ColorOverride{}}
	_ = loadJSON(colorsFile, &data)
	if data.Themes == nil {
		data.Themes = map[string]ColorOverride{}
	}
	return data
}

func saveColors(data ColorsFile) { _ = saveJSON(colorsFile, data) }

func normalizeColor(value string) string {
	raw := strings.Trim(strings.TrimSpace(value), `"'`)
	if raw == "" {
		return ""
	}
	if named, ok := namedColors[strings.ToLower(raw)]; ok {
		return named
	}
	hex := raw
	if strings.HasPrefix(raw, "#") {
		hex = regexp.MustCompile(`[^0-9A-Fa-f]`).ReplaceAllString(raw[1:], "")
	} else {
		hex = regexp.MustCompile(`[^0-9A-Fa-f]`).ReplaceAllString(raw, "")
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) == 6 {
		return "#" + strings.ToUpper(hex)
	}
	return ""
}

func formatConkyColorValue(color string, newSyntax bool) string {
	n := normalizeColor(color)
	if newSyntax {
		if n != "" {
			return "'" + n + "'"
		}
		return "'" + strings.Trim(strings.TrimSpace(color), `"'`) + "'"
	}
	if n != "" {
		return n[1:]
	}
	return strings.Trim(strings.TrimSpace(color), `"'`)
}

func parseThemeColors(content string) map[string]string {
	slots := map[string]string{}
	var re *regexp.Regexp
	if isNewSyntax(content) {
		re = regexp.MustCompile(`(?im)^\s*(default_color|default_outline_color|default_shade_color|own_window_colour|color\d*)\s*=\s*([^,\n]+)`)
	} else {
		re = regexp.MustCompile(`(?im)^\s*(default_color|default_outline_color|default_shade_color|own_window_colour|color\d*)\s+(\S+)`)
	}
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		slots[m[1]] = strings.Trim(strings.TrimSpace(m[2]), `"'`)
	}
	return slots
}

func orderedThemeColorSlots(themeColors map[string]string) []string {
	var ordered []string
	seen := map[string]bool{}
	for _, slot := range colorSlotOrder {
		if _, ok := themeColors[slot]; ok && !seen[slot] {
			ordered = append(ordered, slot)
			seen[slot] = true
		}
	}
	var extra []string
	for slot := range themeColors {
		if !seen[slot] {
			extra = append(extra, slot)
		}
	}
	sort.Strings(extra)
	return append(ordered, extra...)
}

func mergePresetColors(themeColors map[string]string, presetID string) map[string]string {
	merged := cloneMap(themeColors)
	if strings.HasPrefix(presetID, "custom:") {
		id := strings.TrimPrefix(presetID, "custom:")
		for _, p := range loadCustomPalettes() {
			if p.ID == id {
				for slot, value := range p.Colors {
					if _, ok := merged[slot]; ok {
						merged[slot] = value
					}
				}
				return merged
			}
		}
	}
	for _, p := range colorPresets {
		if p.ID == presetID {
			for slot, value := range p.Preset.Colors {
				if _, ok := merged[slot]; ok {
					merged[slot] = value
				}
			}
			break
		}
	}
	return merged
}

func buildSmartColorRemap(original, newColors map[string]string) map[string]string {
	remap := map[string]string{}
	for slot, newVal := range newColors {
		oldVal, ok := original[slot]
		if !ok {
			continue
		}
		oh, nh := normalizeColor(oldVal), normalizeColor(newVal)
		if oh != "" && nh != "" && oh != nh {
			remap[oh] = nh
		}
	}
	return remap
}

func applyColorPatch(content string, colors map[string]string) (string, int) {
	fixes := 0
	if len(colors) == 0 {
		return content, 0
	}
	newSyn := isNewSyntax(content)
	for slot, newColor := range colors {
		value := formatConkyColorValue(newColor, newSyn)
		var re *regexp.Regexp
		if newSyn {
			re = regexp.MustCompile(`(?im)(^\s*` + regexp.QuoteMeta(slot) + `\s*=\s*)([^,\n]+)(,?\s*$)`)
			if re.FindStringIndex(content) != nil {
				done := false
				content = re.ReplaceAllStringFunc(content, func(s string) string {
					if done {
						return s
					}
					done = true
					m := re.FindStringSubmatch(s)
					return m[1] + value + m[3]
				})
				fixes++
			}
		} else {
			re = regexp.MustCompile(`(?im)(^\s*` + regexp.QuoteMeta(slot) + `\s+)(\S+)(.*$)`)
			if re.FindStringIndex(content) != nil {
				done := false
				content = re.ReplaceAllStringFunc(content, func(s string) string {
					if done {
						return s
					}
					done = true
					m := re.FindStringSubmatch(s)
					return m[1] + value + m[3]
				})
				fixes++
			}
		}
	}
	return content, fixes
}

func applyHexRemap(content string, remap map[string]string) (string, int) {
	fixes := 0
	for oldC, newC := range remap {
		oh, nh := normalizeColor(oldC), normalizeColor(newC)
		if oh == "" || nh == "" || oh == nh {
			continue
		}
		ob, nb := oh[1:], nh[1:]
		pairs := [][2]string{
			{`0x` + ob, `0x` + strings.ToUpper(nb)},
			{`#` + ob, `#` + strings.ToUpper(nb)},
		}
		for _, p := range pairs {
			re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(p[0]))
			n := len(re.FindAllString(content, -1))
			if n > 0 {
				content = re.ReplaceAllString(content, p[1])
				fixes += n
			}
		}
	}
	return content, fixes
}

func looksLikeThemeRoot(path string) bool {
	if !isDir(path) {
		return false
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	names := map[string]bool{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names[strings.ToLower(e.Name())] = true
	}
	for _, n := range assetDirNames {
		if names[strings.ToLower(n)] {
			return true
		}
	}
	if fileExists(filepath.Join(path, "start.sh")) {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(path, "*.conf"))
	return len(matches) > 0
}

func resolveThemeRoot(cfgPath string) string {
	cfgPath = resolvePath(cfgPath)
	dir := filepath.Dir(cfgPath)
	if n := strings.ToLower(filepath.Base(dir)); n == "config" || n == "conky" || n == "scripts" {
		for _, parent := range []string{filepath.Dir(dir), filepath.Dir(filepath.Dir(dir))} {
			if isDir(parent) && looksLikeThemeRoot(parent) {
				return resolvePath(parent)
			}
		}
	}
	if looksLikeThemeRoot(dir) {
		return resolvePath(dir)
	}
	p := dir
	for i := 0; i < 4; i++ {
		p = filepath.Dir(p)
		if p == "/" || p == "." {
			break
		}
		if looksLikeThemeRoot(p) {
			return resolvePath(p)
		}
	}
	return resolvePath(dir)
}

func expandThemePath(pathStr string) string {
	s := strings.Trim(strings.TrimSpace(pathStr), `"'`)
	s = strings.ReplaceAll(s, "$HOME", homeDir)
	if strings.HasPrefix(s, "~/") {
		s = filepath.Join(homeDir, s[2:])
	}
	return s
}

func findAssetInTheme(themeRoot, rawPath string) string {
	themeRoot = resolvePath(themeRoot)
	expanded := expandThemePath(rawPath)
	if expanded == "" || strings.HasPrefix(expanded, "-") {
		return ""
	}
	for _, tok := range []string{"$", "|", "&&", "||", "`"} {
		if strings.Contains(expanded, tok) {
			return ""
		}
	}
	if isFile(expanded) {
		return resolvePath(expanded)
	}
	if strings.HasPrefix(expanded, homeDir) {
		rel, err := filepath.Rel(homeDir, expanded)
		if err == nil {
			parts := strings.Split(rel, string(os.PathSeparator))
			if len(parts) >= 3 && parts[0] == ".config" && parts[1] == "conky" {
				tail := "."
				if len(parts) > 3 {
					tail = filepath.Join(parts[3:]...)
				}
				remapped := filepath.Join(themeRoot, tail)
				if isFile(remapped) {
					return resolvePath(remapped)
				}
			}
			if len(parts) >= 2 && parts[0] == ".conky" {
				tail := "."
				if len(parts) > 2 {
					tail = filepath.Join(parts[2:]...)
				}
				remapped := filepath.Join(themeRoot, tail)
				if isFile(remapped) {
					return resolvePath(remapped)
				}
			}
		}
	}
	for _, base := range []string{themeRoot, filepath.Join(themeRoot, "res"), filepath.Join(themeRoot, "assets"), filepath.Join(themeRoot, "images"), filepath.Join(themeRoot, "img"), filepath.Join(themeRoot, "scripts")} {
		remapped := filepath.Join(base, expanded)
		if isFile(remapped) {
			return resolvePath(remapped)
		}
	}
	filename := filepath.Base(expanded)
	if filename != "" && filename != "." && filename != ".." {
		var matches []string
		_ = filepath.Walk(themeRoot, func(p string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && info.Name() == filename {
				matches = append(matches, p)
			}
			return nil
		})
		if len(matches) == 1 {
			return resolvePath(matches[0])
		}
	}
	return ""
}

func rewriteThemeAssetPaths(content, themeRoot, launchDirPath string) (string, int, []string) {
	themeRoot = resolvePath(themeRoot)
	fixes := 0
	var missing []string
	seenMissing := map[string]bool{}
	updated := content
	for _, pat := range pathRefPatterns {
		updated = pat.ReplaceAllStringFunc(updated, func(m string) string {
			sub := pat.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			original := sub[1]
			resolved := findAssetInTheme(themeRoot, original)
			if resolved != "" {
				fixes++
				return strings.Replace(m, original, resolved, 1)
			}
			lower := strings.ToLower(original)
			if strings.HasPrefix(original, "~/.cache/") || strings.HasPrefix(original, homeDir+"/.cache/") {
				return m
			}
			for _, ext := range assetSuffixes {
				if strings.HasSuffix(lower, ext) {
					if !seenMissing[original] {
						seenMissing[original] = true
						missing = append(missing, original)
					}
					break
				}
			}
			return m
		})
	}
	themeName := filepath.Base(themeRoot)
	replacements := map[string]string{
		"~/.config/conky/" + themeName:          themeRoot,
		"$HOME/.config/conky/" + themeName:      themeRoot,
		homeDir + "/.config/conky/" + themeName: themeRoot,
		"~/.conky/" + themeName:                 themeRoot,
		"$HOME/.conky/" + themeName:             themeRoot,
		homeDir + "/.conky/" + themeName:        themeRoot,
	}
	for old, neu := range replacements {
		if strings.Contains(updated, old) {
			updated = strings.ReplaceAll(updated, old, neu)
			fixes++
		}
	}
	if launchDirPath != "" {
		launchDirPath = resolvePath(launchDirPath)
		scriptRoot := filepath.Join(launchDirPath, "scripts")
		if isDir(scriptRoot) {
			for _, old := range []string{themeRoot + "/scripts", filepath.Join(themeRoot, "scripts")} {
				if strings.Contains(updated, old) {
					updated = strings.ReplaceAll(updated, old, scriptRoot)
					fixes++
				}
			}
		}
	}
	return updated, fixes, missing
}

func ensureCompatInstallSymlink(themeRoot string) [][2]string {
	settings := loadSettings()
	if !settings.CreateCompatSymlinks {
		return nil
	}
	themeRoot = resolvePath(themeRoot)
	compat := filepath.Join(homeDir, ".config", "conky", filepath.Base(themeRoot))
	var links [][2]string
	_ = os.MkdirAll(filepath.Dir(compat), 0o755)
	st, err := os.Lstat(compat)
	if err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			target, _ := filepath.EvalSymlinks(compat)
			if resolvePath(target) == themeRoot {
				return links
			}
			_ = os.Remove(compat)
		} else {
			return links
		}
	}
	if err := os.Symlink(themeRoot, compat); err != nil {
		log.Printf("[WARN] Could not create compatibility symlink %s: %v", compat, err)
		return links
	}
	log.Printf("[INFO] Created compatibility symlink: %s -> %s", compat, themeRoot)
	return [][2]string{{compat, themeRoot}}
}

func materializePatchedScripts(themeRoot, ldir string, hexRemap map[string]string) int {
	scriptsSrc := filepath.Join(themeRoot, "scripts")
	if !isDir(scriptsSrc) {
		return 0
	}
	scriptsDest := filepath.Join(ldir, "scripts")
	removeAllIfExists(scriptsDest)
	if err := copyTree(scriptsSrc, scriptsDest); err != nil {
		return 0
	}
	patched := 0
	_ = filepath.Walk(scriptsDest, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".sh" && ext != ".lua" {
			return nil
		}
		original := readText(path)
		updated, fixes, _ := rewriteThemeAssetPaths(original, themeRoot, ldir)
		if hexRemap != nil && ext == ".lua" {
			var cf int
			updated, cf = applyHexRemap(updated, hexRemap)
			fixes += cf
		}
		if fixes > 0 {
			_ = os.WriteFile(path, []byte(updated), info.Mode())
			if ext == ".sh" {
				_ = os.Chmod(path, info.Mode()|0o111)
			}
			patched++
		}
		return nil
	})
	return patched
}

func inAssetDirs(name string) bool {
	l := strings.ToLower(name)
	for _, n := range assetDirNames {
		if strings.ToLower(n) == l {
			return true
		}
	}
	return false
}

func inAssetSuffix(name string) bool {
	l := strings.ToLower(filepath.Ext(name))
	for _, s := range assetSuffixes {
		if s == l {
			return true
		}
	}
	return false
}

func linkThemeAssets(themeRoot, ldir string, skipDirs map[string]bool) {
	entries, err := os.ReadDir(themeRoot)
	if err != nil {
		return
	}
	for _, item := range entries {
		if strings.HasPrefix(item.Name(), ".") {
			continue
		}
		src := filepath.Join(themeRoot, item.Name())
		if item.IsDir() && inAssetDirs(item.Name()) {
			if skipDirs[strings.ToLower(item.Name())] {
				continue
			}
			dest := filepath.Join(ldir, item.Name())
			_ = os.RemoveAll(dest)
			_ = os.Symlink(resolvePath(src), dest)
		} else if !item.IsDir() && inAssetSuffix(item.Name()) {
			dest := filepath.Join(ldir, item.Name())
			_ = os.RemoveAll(dest)
			_ = os.Symlink(resolvePath(src), dest)
		}
	}
	nested := filepath.Join(themeRoot, ".config", "conky")
	if isDir(nested) {
		destCfg := filepath.Join(ldir, ".config")
		_ = os.MkdirAll(destCfg, 0o755)
		target := filepath.Join(destCfg, "conky")
		_ = os.RemoveAll(target)
		_ = os.Symlink(resolvePath(nested), target)
	}
}

func conkyEnv() []string {
	env := os.Environ()
	hasDisp, hasWay := false, false
	for _, e := range env {
		if strings.HasPrefix(e, "DISPLAY=") {
			hasDisp = true
		}
		if strings.HasPrefix(e, "WAYLAND_DISPLAY=") {
			hasWay = true
		}
	}
	if !hasDisp && !hasWay {
		env = append(env, "DISPLAY=:0")
	}
	return env
}

func envMapFromSlice(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			m[e[:i]] = e[i+1:]
		}
	}
	return m
}

func envSliceFromMap(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func buildLaunchEnv(bundle ThemeLaunchBundle) []string {
	m := envMapFromSlice(conkyEnv())
	m["CONKY_THEME_ROOT"] = bundle.ThemeRoot
	m["CONKY_MANAGER_LAUNCH_DIR"] = bundle.LaunchDir
	m["CONKY_MANAGER_THEME_ID"] = bundle.SourceConfig
	if bundle.FontsDir != "" && isDir(bundle.FontsDir) {
		fcDir := filepath.Join(bundle.LaunchDir, ".fontconfig")
		_ = os.MkdirAll(fcDir, 0o755)
		fontsConf := filepath.Join(fcDir, "fonts.conf")
		txt := "<?xml version=\"1.0\"?>\n<!DOCTYPE fontconfig SYSTEM \"fonts.dtd\">\n<fontconfig>\n  <dir>" + resolvePath(bundle.FontsDir) + "</dir>\n</fontconfig>\n"
		_ = os.WriteFile(fontsConf, []byte(txt), 0o644)
		m["FONTCONFIG_FILE"] = fontsConf
	}
	return envSliceFromMap(m)
}

func normalizeFolderName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '+' || r == '.' || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	cleaned := strings.TrimSpace(b.String())
	if cleaned == "" {
		cleaned = "conky-theme"
	}
	for _, suf := range stripSuffixes {
		if strings.HasSuffix(cleaned, suf) {
			cleaned = cleaned[:len(cleaned)-len(suf)]
		}
	}
	cleaned = strings.Trim(cleaned, "-_")
	if cleaned == "" {
		return "conky-theme"
	}
	return cleaned
}

func createThemeLaunchBundle(cfgPath string, applyDesktopOptimize bool, positionOverride *Position, colorOverride *ColorOverride) (ThemeLaunchBundle, error) {
	cfgPath = resolvePath(cfgPath)
	themeRoot := resolveThemeRoot(cfgPath)
	original := readText(cfgPath)
	content := original
	originalColors := parseThemeColors(original)
	var hexRemap map[string]string
	colorFixes := 0

	if applyDesktopOptimize {
		settings := loadSettings()
		if settings.AutoOptimize || positionOverride != nil {
			if positionOverride != nil {
				settings = mergePositionSettings(settings, cfgPath, positionOverride)
			}
			if isNewSyntax(content) {
				content = patchNewSyntax(content, settings)
			} else {
				content = patchLegacySyntax(content, settings)
			}
		}
	} else if positionOverride != nil {
		content = applyPositionPatch(content, *positionOverride)
	}

	if colorOverride != nil && len(colorOverride.Colors) > 0 {
		if colorOverride.SmartLua {
			hexRemap = buildSmartColorRemap(originalColors, colorOverride.Colors)
		}
		content, colorFixes = applyColorPatch(content, colorOverride.Colors)
	}

	content, pathFixes, missing := rewriteThemeAssetPaths(content, themeRoot, "")
	compatLinks := ensureCompatInstallSymlink(themeRoot)

	bundleName := normalizeFolderName(filepath.Base(themeRoot) + "_" + strings.TrimSuffix(filepath.Base(cfgPath), filepath.Ext(cfgPath)))
	ldir := filepath.Join(launchDir, bundleName)
	removeAllIfExists(ldir)
	if err := os.MkdirAll(ldir, 0o755); err != nil {
		return ThemeLaunchBundle{}, err
	}
	patchedScripts := materializePatchedScripts(themeRoot, ldir, hexRemap)
	extraFixes := 0
	content, extraFixes, missing = rewriteThemeAssetPaths(content, themeRoot, ldir)
	pathFixes += extraFixes + patchedScripts + colorFixes

	launchConfig := filepath.Join(ldir, filepath.Base(cfgPath))
	if err := os.WriteFile(launchConfig, []byte(content), 0o644); err != nil {
		return ThemeLaunchBundle{}, err
	}
	linkThemeAssets(themeRoot, ldir, map[string]bool{"scripts": true})

	fonts := filepath.Join(themeRoot, "fonts")
	if !isDir(fonts) {
		fonts = ""
	}
	log.Printf("[INFO] Prepared launch bundle for %s (fixes=%d, missing=%d, root=%s)", cfgPath, pathFixes, len(missing), themeRoot)
	return ThemeLaunchBundle{
		SourceConfig: cfgPath, ThemeRoot: themeRoot, LaunchDir: ldir, LaunchConfig: launchConfig,
		PathFixes: pathFixes, MissingAssets: missing, FontsDir: fonts, CompatLinks: compatLinks,
	}, nil
}

func scanThemeAssets(cfgPath string) (found, missing []string) {
	cfgPath = resolvePath(cfgPath)
	themeRoot := resolveThemeRoot(cfgPath)
	content := readText(cfgPath)
	_, _, missing = rewriteThemeAssetPaths(content, themeRoot, "")
	seen := map[string]bool{}
	for _, pat := range pathRefPatterns {
		for _, m := range pat.FindAllStringSubmatch(content, -1) {
			if len(m) < 2 {
				continue
			}
			if r := findAssetInTheme(themeRoot, m[1]); r != "" && !seen[r] {
				seen[r] = true
				found = append(found, r)
			}
		}
	}
	sort.Strings(found)
	sort.Strings(missing)
	return found, missing
}

func optimizeForDesktop(cfgPath string) (string, error) {
	b, err := createThemeLaunchBundle(cfgPath, true, nil, nil)
	if err != nil {
		return "", err
	}
	return b.LaunchConfig, nil
}

func validateConkyConfig(configPath, cwd string, env []string) bool {
	if !isFile(configPath) {
		return false
	}
	if cwd == "" {
		cwd = filepath.Dir(configPath)
	}
	if env == nil {
		env = conkyEnv()
	}
	cmd := exec.Command("conky", "-c", configPath, "-i", "1", "-q")
	cmd.Dir, cmd.Env = cwd, env
	cmd.Stdout = io.Discard
	err := cmd.Run()
	return err == nil
}

func killRunningConky() bool {
	stopAllManaged()
	return true
}

func startConkyWithHealthcheck(configPath, cwd string, env []string) (*exec.Cmd, bool) {
	settings := loadSettings()
	health := settings.HealthcheckSeconds
	niceLevel := settings.NiceLevel
	if cwd == "" {
		cwd = filepath.Dir(configPath)
	}
	if env == nil {
		env = conkyEnv()
	}
	logF, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logF = os.Stderr
	}
	args := []string{"conky", "-q", "-c", configPath}
	if which("nice") {
		args = append([]string{"nice", "-n", strconv.Itoa(niceLevel)}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir, cmd.Env = cwd, env
	cmd.Stdout, cmd.Stderr = logF, logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return cmd, false
	}
	deadline := time.Now().Add(time.Duration(health * float64(time.Second)))
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return cmd, false
		}
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			_ = cmd.Wait()
			return cmd, false
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		_ = cmd.Wait()
		return cmd, false
	}
	go func() { _ = cmd.Wait() }()
	return cmd, true
}

type runResult struct {
	PID  int
	Path string
	Mode string
}

func runConky(cfgPath string, positionOverride *Position, colorOverride *ColorOverride) (runResult, error) {
	if !isFile(cfgPath) {
		return runResult{}, fmt.Errorf("config not found: %s", cfgPath)
	}
	settings := loadSettings()
	type cand struct {
		name   string
		path   string
		bundle *ThemeLaunchBundle
	}
	var candidates []cand
	if settings.FullVisualLaunch {
		b, err := createThemeLaunchBundle(cfgPath, true, positionOverride, colorOverride)
		if err != nil {
			log.Printf("[WARN] Full visual launch bundle failed for %s: %v", cfgPath, err)
		} else {
			candidates = append(candidates, cand{"full-visual", b.LaunchConfig, &b})
		}
	}
	if settings.AutoOptimize && !settings.FullVisualLaunch {
		opt, err := optimizeForDesktop(cfgPath)
		if err != nil {
			log.Printf("[WARN] Optimization failed for %s: %v", cfgPath, err)
		} else {
			fb, _ := createThemeLaunchBundle(cfgPath, false, positionOverride, colorOverride)
			candidates = append(candidates, cand{"optimized", opt, &fb})
		}
	}
	themeRoot := resolveThemeRoot(cfgPath)
	ensureCompatInstallSymlink(themeRoot)
	fonts := filepath.Join(themeRoot, "fonts")
	if !isDir(fonts) {
		fonts = ""
	}
	fb := ThemeLaunchBundle{SourceConfig: cfgPath, ThemeRoot: themeRoot, LaunchDir: themeRoot, LaunchConfig: cfgPath, FontsDir: fonts}
	candidates = append(candidates, cand{"original-root", cfgPath, &fb})

	lastErr := "unknown error"
	for _, c := range candidates {
		if c.name == "optimized" && !validateConkyConfig(c.path, "", nil) {
			log.Printf("[WARN] Skipping invalid optimized config: %s", c.path)
			continue
		}
		ldir := themeRoot
		if c.bundle != nil {
			ldir = c.bundle.LaunchDir
		}
		env := buildLaunchEnv(*c.bundle)
		if c.bundle != nil && len(c.bundle.MissingAssets) > 0 {
			n := len(c.bundle.MissingAssets)
			if n > 5 {
				n = 5
			}
			log.Printf("[WARN] Theme %s has missing assets: %s", cfgPath, strings.Join(c.bundle.MissingAssets[:n], ", "))
		}
		proc, healthy := startConkyWithHealthcheck(c.path, ldir, env)
		if healthy && proc.Process != nil {
			log.Printf("[INFO] Started Conky (%s): %s (pid=%d, root=%s, fixes=%d)", c.name, c.path, proc.Process.Pid, ldir, c.bundle.PathFixes)
			registerManagedProcess(ManagedProcess{
				PID: proc.Process.Pid, ThemeID: themeID(cfgPath), ConfigPath: cfgPath,
				LaunchConfig: c.path, StartedAt: time.Now().Unix(),
			})
			recordRecent(cfgPath)
			return runResult{PID: proc.Process.Pid, Path: c.path, Mode: c.name}, nil
		}
		lastErr = c.name + " process exited during startup"
	}
	return runResult{}, fmt.Errorf("theme failed to start: %s (%s)", cfgPath, lastErr)
}

func archiveKind(path string) string {
	lower := strings.ToLower(filepath.Base(path))
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tgz", ".tbz2", ".txz"} {
		if strings.HasSuffix(lower, ext) {
			return strings.TrimPrefix(ext, ".")
		}
	}
	for _, ext := range []string{".zip", ".tar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return strings.TrimPrefix(ext, ".")
		}
	}
	return ""
}

func unsafeArchivePath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := filepath.ToSlash(filepath.Clean(name))
	if name == "" || strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(name, "..\\") || strings.Contains(name, "/../") {
		return true
	}
	return false
}

func pathWithin(root, candidate string) bool {
	r, err1 := filepath.Abs(root)
	c, err2 := filepath.Abs(candidate)
	if err1 != nil || err2 != nil {
		return false
	}
	return r == c || strings.HasPrefix(c, r+string(os.PathSeparator))
}

func extractZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	lim := extractLimits{maxBytes: maxArchiveBytes, maxFiles: maxArchiveFiles}
	for _, m := range r.File {
		if unsafeArchivePath(m.Name) {
			return fmt.Errorf("unsafe path in archive: %s", m.Name)
		}
		if strings.Count(filepath.ToSlash(m.Name), "/") > maxArchiveDepth {
			return fmt.Errorf("archive path is nested too deeply: %s", m.Name)
		}
		if m.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in theme archives: %s", m.Name)
		}
		if m.UncompressedSize64 > uint64(maxArchiveBytes) {
			return fmt.Errorf("archive file exceeds maximum size: %s", m.Name)
		}
	}
	for _, m := range r.File {
		p := filepath.Join(dest, m.Name)
		if !pathWithin(dest, p) {
			return fmt.Errorf("archive entry escapes destination: %s", m.Name)
		}
		if m.FileInfo().IsDir() {
			_ = os.MkdirAll(p, 0o755)
			continue
		}
		if err := lim.add(int64(m.UncompressedSize64)); err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		rc, err := m.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(p)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, int64(m.UncompressedSize64)+1))
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type extractLimits struct {
	maxBytes int64
	maxFiles int
	files    int
	bytes    int64
}

func (l *extractLimits) add(n int64) error {
	if n < 0 {
		n = 0
	}
	l.files++
	l.bytes += n
	if l.files > l.maxFiles {
		return fmt.Errorf("archive contains too many files (limit %d)", l.maxFiles)
	}
	if l.bytes > l.maxBytes {
		return fmt.Errorf("archive extracted size exceeds the security limit")
	}
	return nil
}

func extractTarStream(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	lim := extractLimits{maxBytes: maxArchiveBytes, maxFiles: maxArchiveFiles}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if unsafeArchivePath(h.Name) {
			return fmt.Errorf("unsafe path in archive: %s", h.Name)
		}
		if strings.Count(filepath.ToSlash(h.Name), "/") > maxArchiveDepth {
			return fmt.Errorf("archive path is nested too deeply: %s", h.Name)
		}
		p := filepath.Join(dest, h.Name)
		if !pathWithin(dest, p) {
			return fmt.Errorf("archive entry escapes destination: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(p, 0o755)
		case tar.TypeReg:
			if err := lim.add(h.Size); err != nil {
				return err
			}
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, io.LimitReader(tr, h.Size+1))
			f.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("symlinks are not allowed in theme archives: %s", h.Name)
		}
	}
}

func extractArchive(archive, dest string) error {
	kind := archiveKind(archive)
	if kind == "" {
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(archive))
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if kind == "zip" {
		return extractZip(archive, dest)
	}
	if kind == "7z" {
		bin := "7z"
		if !which("7z") {
			if which("7za") {
				bin = "7za"
			} else {
				return fmt.Errorf("7z archives require p7zip (7z command) to be installed")
			}
		}
		cmd := exec.Command(bin, "x", archive, "-o"+dest, "-y")
		return cmd.Run()
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader = f
	switch kind {
	case "tar.gz", "tgz":
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	case "tar.bz2", "tbz2":
		r = bzip2.NewReader(f)
	case "tar.xz", "txz":
		f.Close()
		cmd := exec.Command("tar", "-xJf", archive, "-C", dest)
		return cmd.Run()
	}
	return extractTarStream(r, dest)
}

func themeFolderForConfig(cfg, extractRoot string) string {
	folder := filepath.Dir(cfg)
	base := strings.ToLower(filepath.Base(folder))
	if (base == "config" || base == "conky" || base == "scripts") && filepath.Dir(folder) != extractRoot {
		folder = filepath.Dir(folder)
	}
	rel, err := filepath.Rel(extractRoot, folder)
	if err == nil {
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) > 1 && parts[0] != "." {
			return filepath.Join(extractRoot, parts[0])
		}
	}
	return folder
}

func discoverInstallUnits(extractRoot string) []string {
	units := map[string]string{}
	_ = filepath.Walk(extractRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(strings.ToLower(name), ".conf") || name == "conkyrc" {
			if isValidTheme(path) {
				u := themeFolderForConfig(path, extractRoot)
				units[resolvePath(u)] = u
			}
		}
		return nil
	})
	if len(units) > 0 {
		var out []string
		for _, u := range units {
			out = append(out, u)
		}
		sort.Slice(out, func(i, j int) bool {
			return strings.ToLower(filepath.Base(out[i])) < strings.ToLower(filepath.Base(out[j]))
		})
		return out
	}
	entries, _ := os.ReadDir(extractRoot)
	var children []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			children = append(children, filepath.Join(extractRoot, e.Name()))
		}
	}
	if len(children) == 1 {
		return children
	}
	if len(children) > 0 {
		sort.Slice(children, func(i, j int) bool {
			return strings.ToLower(filepath.Base(children[i])) < strings.ToLower(filepath.Base(children[j]))
		})
		return children
	}
	return []string{extractRoot}
}

func uniqueDestination(base, name string) string {
	dest := filepath.Join(base, name)
	if !fileExists(dest) && !isDir(dest) {
		return dest
	}
	for i := 2; ; i++ {
		c := filepath.Join(base, fmt.Sprintf("%s-%d", name, i))
		if !fileExists(c) && !isDir(c) {
			return c
		}
	}
}

func installTree(source, targetBase, preferredName string) (string, error) {
	_ = os.MkdirAll(targetBase, 0o755)
	folder := preferredName
	if folder == "" {
		folder = filepath.Base(source)
	}
	folder = normalizeFolderName(folder)
	dest := uniqueDestination(targetBase, folder)
	if err := copyTree(source, dest); err != nil {
		return "", err
	}
	ensureCompatInstallSymlink(dest)
	log.Printf("[INFO] Installed theme folder: %s -> %s", source, dest)
	return filepath.Base(dest), nil
}

func importFolderToConky(source, targetBase string) ([]string, error) {
	if !isDir(source) {
		return nil, fmt.Errorf("not a folder: %s", source)
	}
	if targetBase == "" {
		targetBase = defaultImportDir
	}
	name, err := installTree(source, targetBase, filepath.Base(source))
	if err != nil {
		return nil, err
	}
	return []string{name}, nil
}

func importArchiveToConky(archivePath, targetBase string) ([]string, error) {
	if !isFile(archivePath) {
		return nil, fmt.Errorf("archive not found: %s", archivePath)
	}
	if archiveKind(archivePath) == "" {
		return nil, fmt.Errorf("unsupported archive: %s\nsupported: %s", filepath.Base(archivePath), strings.Join(archiveExts, ", "))
	}
	if targetBase == "" {
		targetBase = defaultImportDir
	}
	tmp, err := os.MkdirTemp(baseDir, "conky-import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := extractArchive(archivePath, tmp); err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(tmp)
	workRoot := tmp
	var vis []os.DirEntry
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			vis = append(vis, e)
		}
	}
	if len(vis) == 1 && vis[0].IsDir() {
		workRoot = filepath.Join(tmp, vis[0].Name())
	}
	defaultName := normalizeFolderName(filepath.Base(workRoot))
	units := discoverInstallUnits(workRoot)
	var installed []string
	if len(units) == 1 {
		n, err := installTree(units[0], targetBase, defaultName)
		if err != nil {
			return nil, err
		}
		installed = append(installed, n)
	} else {
		for _, u := range units {
			n, err := installTree(u, targetBase, filepath.Base(u))
			if err != nil {
				return nil, err
			}
			installed = append(installed, n)
		}
	}
	return installed, nil
}

func hostAllowed(host string) bool {
	host = strings.ToLower(host)
	if matched, _ := regexp.MatchString(`^files\d+\.pling\.com$`, host); matched {
		return true
	}
	for _, a := range allowedHosts {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

func validateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if strings.ToLower(u.Scheme) != "https" || u.Hostname() == "" {
		return fmt.Errorf("only HTTPS downloads are allowed")
	}
	if !hostAllowed(strings.ToLower(u.Hostname())) {
		return fmt.Errorf("download host not allowed: %s", u.Hostname())
	}
	return nil
}

type progressFn func(float64, string)

func downloadFile(rawURL, dest string, cb progressFn) error {
	if err := validateDownloadURL(rawURL); err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", appName+"/"+appVersion)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 512*1024*1024 {
		return fmt.Errorf("download is too large (over 512 MiB)")
	}
	total := resp.ContentLength
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	var downloaded int64
	maxBytes := int64(512 * 1024 * 1024)
	buf := make([]byte, 64*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if downloaded > maxBytes {
				return fmt.Errorf("download is too large (over 512 MiB)")
			}
			if cb != nil && total > 0 {
				cb(float64(downloaded)/float64(total), fmt.Sprintf("Downloading… %d KB", downloaded/1024))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if cb != nil {
		cb(1, "Download complete")
	}
	return nil
}

func loadRemotePixbuf(rawURL string, w, h int) *gdk.Pixbuf {
	if rawURL == "" {
		return nil
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", appName+"/"+appVersion)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil || len(data) == 0 {
		return nil
	}
	loader, err := gdk.PixbufLoaderNew()
	if err != nil {
		return nil
	}
	if _, err := loader.Write(data); err != nil {
		_ = loader.Close()
		return nil
	}
	if err := loader.Close(); err != nil {
		return nil
	}
	pb, err := loader.GetPixbuf()
	if err != nil || pb == nil {
		return nil
	}
	if w > 0 && h > 0 {
		if scaled, err := pb.ScaleSimple(w, h, gdk.INTERP_BILINEAR); err == nil && scaled != nil {
			return scaled
		}
	}
	return pb
}

func captureDesktopProfile(name string, paths []string) DesktopProfile {
	var themes []ProfileTheme
	for _, p := range paths {
		pt := ProfileTheme{Path: p, Enabled: true}
		if pos := resolveThemePosition(p, nil); pos != nil {
			pt.Position = *pos
			pt.Monitor = pos.Monitor
		}
		if c := resolveThemeColors(p); c != nil {
			pt.Colors = *c
		}
		themes = append(themes, pt)
	}
	return DesktopProfile{Name: name, Themes: themes}
}

func stripHTML(text string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	cleaned := re.ReplaceAllString(text, " ")
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

func ocsRequest(apiHost, endpoint string, params url.Values) (map[string]any, error) {
	if params == nil {
		params = url.Values{}
	}
	if params.Get("format") == "" {
		params.Set("format", "json")
	}
	raw := fmt.Sprintf("https://%s/ocs/v1/content/%s?%s", apiHost, endpoint, params.Encode())
	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appName+"/"+appVersion)
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("store API returned HTTP %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if st, _ := payload["status"].(string); st != "" && st != "ok" {
		msg, _ := payload["message"].(string)
		if msg == "" {
			msg = "Store API request failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return payload, nil
}

func getOnlineStore(id string) OnlineStore {
	for _, s := range onlineStores {
		if s.ID == id {
			return s
		}
	}
	return onlineStores[0]
}

func anyToInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	}
	return 0
}

func anyToFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		n, _ := strconv.ParseFloat(t, 64)
		return n
	}
	return 0
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asMapSlice(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		if m, ok := v.(map[string]any); ok {
			return []map[string]any{m}
		}
		return nil
	}
	var out []map[string]any
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func browseOnlineStore(store OnlineStore, query string, page, perPage int, sort string) ([]OnlineProduct, int, error) {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("itemsperpage", strconv.Itoa(perPage))
	params.Set("ord", sort)
	params.Set("categories", storeCat)
	search := strings.TrimSpace(query)
	if search != "" {
		params.Set("search", search)
	} else if store.ID == "kde-look" {
		params.Set("search", "conky")
	}
	payload, err := ocsRequest(store.APIHost, "data", params)
	if err != nil {
		return nil, 0, err
	}
	total := anyToInt(payload["totalitems"])
	var products []OnlineProduct
	for _, item := range asMapSlice(payload["data"]) {
		tn := strings.ToLower(strings.TrimSpace(anyToString(item["typename"])))
		xt := strings.ToLower(strings.TrimSpace(anyToString(item["xdg_type"])))
		// Some store instances omit typename/xdg_type or use a localized value.
		// Category 124 is already the Conky category, so reject only an explicit
		// non-Conky classification instead of losing valid catalog entries.
		if (tn != "" && tn != "conky") && (xt != "" && xt != "conky") {
			continue
		}
		id := anyToInt(item["id"])
		preview := anyToString(item["previewpic1"])
		if preview == "" {
			preview = anyToString(item["smallpreviewpic1"])
		}
		if preview == "" {
			preview = anyToString(item["previewpic2"])
		}
		name := anyToString(item["name"])
		if name == "" {
			name = "Unnamed theme"
		}
		products = append(products, OnlineProduct{
			ProductID: id, Name: name, Summary: stripHTML(anyToString(item["summary"])),
			Author: anyToString(item["personid"]), Downloads: anyToInt(item["downloads"]),
			Score: anyToFloat(item["score"]), Version: anyToString(item["version"]),
			PreviewURL: preview, PageURL: fmt.Sprintf(store.PageURLTemplate, id), StoreID: store.ID,
		})
	}
	return products, total, nil
}

func fetchOnlineProductDetails(store OnlineStore, productID int) (map[string]any, error) {
	payload, err := ocsRequest(store.DetailsAPIHost, fmt.Sprintf("data/%d", productID), nil)
	if err != nil {
		return nil, err
	}
	items := asMapSlice(payload["data"])
	if len(items) == 0 {
		return nil, fmt.Errorf("theme not found on %s: %d", store.Label, productID)
	}
	return items[0], nil
}

func extractProductDownloads(details map[string]any) [][2]string {
	var links [][2]string
	for i := 1; i <= 5; i++ {
		link := anyToString(details[fmt.Sprintf("downloadlink%d", i)])
		name := anyToString(details[fmt.Sprintf("downloadname%d", i)])
		if name == "" {
			name = fmt.Sprintf("download-%d", i)
		}
		if link != "" {
			links = append(links, [2]string{name, link})
		}
	}
	return links
}

func pickThemeDownload(details map[string]any) (string, string, error) {
	links := extractProductDownloads(details)
	exts := []string{".zip", ".tar.gz", ".tar.xz", ".tar.bz2", ".tar", ".tgz", ".txz", ".7z"}
	for _, l := range links {
		lower := strings.ToLower(l[0])
		for _, e := range exts {
			if strings.HasSuffix(lower, e) {
				return l[0], l[1], nil
			}
		}
	}
	if len(links) > 0 {
		return links[0][0], links[0][1], nil
	}
	return "", "", fmt.Errorf("no downloadable archive found for this theme")
}

func downloadAndInstallOnlineProduct(store OnlineStore, productID int, cb progressFn) ([]string, error) {
	if cb != nil {
		cb(0.05, "Reading theme info from "+store.Label+"…")
	}
	details, err := fetchOnlineProductDetails(store, productID)
	if err != nil {
		return nil, err
	}
	downloadName, downloadURL, err := pickThemeDownload(details)
	if err != nil {
		return nil, err
	}
	if cb != nil {
		cb(0.12, "Downloading "+downloadName+"…")
	}
	suffix := filepath.Ext(downloadName)
	if suffix == "" {
		suffix = ".zip"
	}
	archivePath := filepath.Join(downloadsDir, fmt.Sprintf("store-%d-%d%s", productID, time.Now().Unix(), suffix))
	if err := downloadFile(downloadURL, archivePath, cb); err != nil {
		return nil, err
	}
	want := strings.TrimSpace(anyToString(details["downloadsha256"]))
	if want == "" {
		want = strings.TrimSpace(anyToString(details["sha256"]))
	}
	if want != "" {
		got, err := sha256File(archivePath)
		if err != nil {
			return nil, fmt.Errorf("could not hash downloaded archive: %w", err)
		}
		if !strings.EqualFold(got, want) {
			return nil, fmt.Errorf("SHA256 mismatch for %s\nexpected %s\ngot %s", downloadName, want, got)
		}
	}
	if cb != nil {
		cb(0.9, "Extracting and installing into ~/.conky…")
	}
	var installed []string
	if archiveKind(archivePath) != "" {
		installed, err = importArchiveToConky(archivePath, defaultImportDir)
		if err != nil {
			return nil, err
		}
	} else {
		folder := normalizeFolderName(anyToString(details["name"]))
		if folder == "conky-theme" {
			folder = normalizeFolderName(downloadName)
		}
		dest := uniqueDestination(defaultImportDir, folder)
		_ = os.MkdirAll(dest, 0o755)
		if err := copyFile(archivePath, filepath.Join(dest, filepath.Base(archivePath))); err != nil {
			return nil, err
		}
		installed = []string{filepath.Base(dest)}
	}
	if cb != nil {
		cb(1, fmt.Sprintf("Installed %d item(s)", len(installed)))
	}
	return installed, nil
}

func loadProfiles() ProfilesFile {
	p := ProfilesFile{Profiles: map[string][]string{}, Desktop: map[string]DesktopProfile{}}
	_ = loadJSON(profilesFile, &p)
	if p.Profiles == nil {
		p.Profiles = map[string][]string{}
	}
	if p.Desktop == nil {
		p.Desktop = map[string]DesktopProfile{}
	}
	for name, paths := range p.Profiles {
		if _, ok := p.Desktop[name]; ok {
			continue
		}
		var themes []ProfileTheme
		for _, path := range paths {
			pt := ProfileTheme{Path: path, Enabled: true}
			if pos := resolveThemePosition(path, nil); pos != nil {
				pt.Position = *pos
				pt.Monitor = pos.Monitor
			}
			if c := resolveThemeColors(path); c != nil {
				pt.Colors = *c
			}
			themes = append(themes, pt)
		}
		p.Desktop[name] = DesktopProfile{Name: name, Themes: themes}
	}
	return p
}

func saveProfiles(p ProfilesFile) { _ = saveJSON(profilesFile, p) }

func runProfileCLI(name string) int {
	if !isConkyInstalled() {
		return 1
	}
	profiles := loadProfiles()
	var items []ProfileTheme
	if dp, ok := profiles.Desktop[name]; ok {
		items = dp.Themes
	} else {
		for _, p := range profiles.Profiles[name] {
			items = append(items, ProfileTheme{Path: p, Enabled: true})
		}
	}
	var themes []ProfileTheme
	for _, t := range items {
		if t.Enabled && isFile(t.Path) {
			themes = append(themes, t)
		}
	}
	if len(themes) == 0 {
		return 1
	}
	stopAllManaged()
	settings := loadSettings()
	maxN := settings.MaxInstances
	if maxN < 1 {
		maxN = 1
	}
	if len(themes) > maxN {
		themes = themes[:maxN]
	}
	for _, t := range themes {
		if t.StartupDelay > 0 {
			time.Sleep(time.Duration(t.StartupDelay) * time.Second)
		}
		pos := t.Position
		var posPtr *Position
		if pos.Enabled || pos.Alignment != "" || t.Monitor != "" {
			if t.Monitor != "" {
				pos.Monitor = t.Monitor
				if mon := findMonitor(t.Monitor, pos.XineramaHead); mon != nil {
					pos.XineramaHead = mon.Head
				}
			}
			posPtr = &pos
		}
		var colPtr *ColorOverride
		if t.Colors.Enabled && len(t.Colors.Colors) > 0 {
			c := t.Colors
			colPtr = &c
		} else {
			colPtr = resolveThemeColors(t.Path)
		}
		if _, err := runConky(t.Path, posPtr, colPtr); err != nil {
			log.Printf("[WARN] %v", err)
		}
	}
	return 0
}

func markupEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func addClass(w gtk.IWidget, class string) {
	if class == "" || w == nil {
		return
	}
	sc, err := w.ToWidget().GetStyleContext()
	if err == nil {
		sc.AddClass(class)
	}
}

func setMargins(w gtk.IWidget, n uint) {
	if ww, ok := w.(interface {
		SetMarginTop(int)
		SetMarginBottom(int)
		SetMarginStart(int)
		SetMarginEnd(int)
	}); ok {
		i := int(n)
		ww.SetMarginTop(i)
		ww.SetMarginBottom(i)
		ww.SetMarginStart(i)
		ww.SetMarginEnd(i)
	}
}

func newLabel(text string, dim bool) *gtk.Label {
	l, _ := gtk.LabelNew(text)
	l.SetXAlign(0)
	l.SetLineWrap(true)
	l.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	l.SetMaxWidthChars(42)
	if dim {
		addClass(l, "dim-label")
	}
	return l
}

func newMarkupLabel(markup string) *gtk.Label {
	l, _ := gtk.LabelNew("")
	l.SetMarkup(markup)
	l.SetXAlign(0)
	l.SetLineWrap(true)
	l.SetLineWrapMode(pango.WRAP_WORD_CHAR)
	l.SetMaxWidthChars(42)
	return l
}

func settingsRow(text string, widget gtk.IWidget) *gtk.Box {
	row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	l, _ := gtk.LabelNew(text)
	l.SetXAlign(0)
	l.SetSizeRequest(160, -1)
	row.PackStart(l, false, false, 0)
	row.PackStart(widget, true, true, 0)
	return row
}

func removeChildren(c *gtk.Container) {
	list := c.GetChildren()
	if list == nil {
		return
	}
	var widgets []*gtk.Widget
	for l := list; l != nil; l = l.Next() {
		w := widgetFromListNode(l)
		if w != nil {
			widgets = append(widgets, w)
		}
	}
	for _, w := range widgets {
		c.Remove(w)
	}
}

func widgetFromListNode(l *glib.List) *gtk.Widget {
	if l == nil {
		return nil
	}
	switch v := l.Data().(type) {
	case *gtk.Widget:
		return v
	case gtk.IWidget:
		return v.ToWidget()
	default:
		return nil
	}
}

func scrollPage(child gtk.IWidget) *gtk.ScrolledWindow {
	s, _ := gtk.ScrolledWindowNew(nil, nil)
	s.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	s.Add(child)
	return s
}

func framedBox(title string, etched bool) (*gtk.Frame, *gtk.Box) {
	fr, _ := gtk.FrameNew(title)
	if etched {
		fr.SetShadowType(gtk.SHADOW_ETCHED_IN)
	} else {
		fr.SetShadowType(gtk.SHADOW_IN)
	}
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	setMargins(box, 12)
	fr.Add(box)
	return fr, box
}

func xdgOpen(path string) {
	_ = exec.Command("xdg-open", path).Start()
}

// ---------------------------------------------------------------------------
// GTK application
// ---------------------------------------------------------------------------

type App struct {
	session, desktop, desktopEnv string
	settings                     Settings
	profiles                     ProfilesFile
	themes, filtered             []ThemeItem
	themeStatus                  map[string]string
	themeHealth                  map[string]ThemeHealth
	library                      ThemeLibrary
	viewFilter                   string
	runtimeCache                 map[string]int
	watchStamp                   string

	win                          *gtk.Window
	themeStore                   *gtk.ListStore
	themeTree                    *gtk.TreeView
	themeCards                   *gtk.ListBox
	selectedThemeSet             map[string]bool
	themeCardChecks              map[string]*gtk.CheckButton
	searchEntry                  *gtk.SearchEntry
	filterCombo                  *gtk.ComboBoxText
	notebook                     *gtk.Notebook
	statusbar                    *gtk.Statusbar
	statusCtx                    uint
	statusIndicator, runtimeInfo *gtk.Label
	dashLabels                   map[string]*gtk.Label
	logsView                     *gtk.TextView
	logsBuffer                   *gtk.TextBuffer

	posEnabled                           *gtk.Switch
	posGapX, posGapY, posNudge           *gtk.SpinButton
	posStatus                            *gtk.Label
	posMonitor                           *gtk.ComboBoxText
	posButtons                           map[string]*gtk.ToggleButton
	btnSavePos, btnResetPos, btnApplyPos *gtk.Button

	colorEnabled, colorSmartLua          *gtk.Switch
	colorPreset                          *gtk.ComboBoxText
	colorSlotsBox                        *gtk.Box
	colorStatus                          *gtk.Label
	colorPickers                         map[string]*gtk.ColorButton
	themeColorSlots                      map[string]string
	btnSaveCol, btnResetCol, btnApplyCol *gtk.Button

	profileName                 *gtk.Entry
	profileList, profileDetails *gtk.ListBox
	profileDetailsTitle         *gtk.Label
	profileRowNames             map[uintptr]string

	previewSeconds                   int
	previewPaused                    bool
	busyButtons                      []*gtk.Widget
	workerBusy                       bool
	mu                               sync.Mutex
	previewID                        glib.SourceHandle
	posLoading                       bool
	colorLoading                     bool
	activePosTheme, activeColorTheme string

	btnRefresh, btnImport, btnDownload, btnHealth            *gtk.Widget
	btnRunSel, btnPreview, btnEdit, btnOpenFolder, btnRepair *gtk.Widget
	btnRunAll, btnStop, btnValidate                          *gtk.Widget
	btnStopSel, btnRestartSel, btnKillOrphans                *gtk.Widget
}

func idle(fn func()) {
	glib.IdleAdd(func() bool { fn(); return false })
}

func (a *App) pushStatus(msg string) {
	a.statusbar.Pop(a.statusCtx)
	a.statusbar.Push(a.statusCtx, msg)
}

func (a *App) showMessage(title, message string, msgType gtk.MessageType) {
	dlg := gtk.MessageDialogNew(a.win, gtk.DIALOG_MODAL, msgType, gtk.BUTTONS_OK, "%s", title)
	dlg.FormatSecondaryText("%s", message)
	dlg.Run()
	dlg.Destroy()
}

func (a *App) setBusy(busy bool) {
	a.workerBusy = busy
	for _, b := range a.busyButtons {
		if b != nil {
			b.SetSensitive(!busy)
		}
	}
}

func (a *App) applyUITheme() {
	s, err := gtk.SettingsGetDefault()
	if err != nil {
		return
	}
	switch a.settings.UITheme {
	case "dark":
		_ = s.SetProperty("gtk-application-prefer-dark-theme", true)
	case "light":
		_ = s.SetProperty("gtk-application-prefer-dark-theme", false)
	}
}

func (a *App) actionBtn(label, class string, handler func()) *gtk.Button {
	btn, _ := gtk.ButtonNewWithLabel(label)
	if class != "" {
		addClass(btn, class)
	}
	fitButton(btn)
	btn.Connect("clicked", handler)
	return btn
}

func fitButton(w gtk.IWidget) {
	if w == nil {
		return
	}
	tw := w.ToWidget()
	tw.SetHExpand(false)
	tw.SetVExpand(false)
	tw.SetHAlign(gtk.ALIGN_START)
	tw.SetVAlign(gtk.ALIGN_CENTER)
}

func compactIconButton(icon, tooltip string, handler func()) *gtk.Button {
	b, _ := gtk.ButtonNewWithLabel(icon)
	b.SetTooltipText(tooltip)
	b.SetSizeRequest(34, 30)
	b.SetHAlign(gtk.ALIGN_CENTER)
	b.SetVAlign(gtk.ALIGN_CENTER)
	b.Connect("clicked", handler)
	return b
}

func preferredWindowSize() (int, int) {
	sw, sh := 1024, 768
	disp, err := gdk.DisplayGetDefault()
	if err == nil && disp != nil {
		if mon, err := disp.GetPrimaryMonitor(); err == nil && mon != nil {
			g := mon.GetGeometry()
			sw, sh = g.GetWidth(), g.GetHeight()
		}
	}
	w := sw - 64
	h := sh - 88
	if w > 960 {
		w = 960
	}
	if h > 700 {
		h = 700
	}
	if w < 640 {
		w = 640
	}
	if h < 480 {
		h = 480
	}
	return w, h
}

func (a *App) keepBusy(w *gtk.Widget) {
	a.busyButtons = append(a.busyButtons, w)
}

func widgetOf(b *gtk.Button) *gtk.Widget { return &b.Widget }

func (a *App) setupUI() error {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	a.win = win
	win.SetTitle(appName + " " + appVersion)
	ww, wh := preferredWindowSize()
	win.SetDefaultSize(ww, wh)
	win.SetResizable(true)
	win.SetPosition(gtk.WIN_POS_CENTER)
	geom := gdk.Geometry{}
	geom.SetMinWidth(640)
	geom.SetMinHeight(480)
	win.SetGeometryHints(nil, geom, gdk.HINT_MIN_SIZE)
	win.Connect("destroy", gtk.MainQuit)

	mainBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	win.Add(mainBox)

	header, _ := gtk.HeaderBarNew()
	header.SetShowCloseButton(true)
	header.SetTitle(appName)
	header.SetSubtitle(fmt.Sprintf("%s | %s", appVersion, compositorLabel(a.desktopEnv, a.session)))
	win.SetTitlebar(header)

	menuBtn, _ := gtk.MenuButtonNew()
	icon, _ := gtk.ImageNewFromIconName("open-menu-symbolic", gtk.ICON_SIZE_BUTTON)
	menuBtn.Add(icon)
	fitButton(menuBtn)
	menu, _ := gtk.MenuNew()
	addMenuItem(menu, "⚙ Settings", a.showSettings)
	addMenuItem(menu, "🔄 Refresh Themes", func() { a.loadThemes() })
	addMenuItem(menu, "🌐 Get Themes Online", a.showThemeStore)
	addMenuItem(menu, "🎨 Color Palettes", a.showPaletteManager)
	addMenuItem(menu, "🗂 Backup History", a.showBackupHistory)
	addMenuItem(menu, "🧹 Clear Download Cache", a.clearDownloadCache)
	sep, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep)
	addMenuItem(menu, "About", a.showAbout)
	menu.ShowAll()
	menuBtn.SetPopup(menu)
	header.PackEnd(menuBtn)

	content, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	mainBox.PackStart(content, true, true, 0)
	a.setupSidebar(content)
	a.setupMainPanel(content)
	a.setupStatusbar(mainBox)
	win.ShowAll()
	win.Resize(ww, wh)
	win.SetPosition(gtk.WIN_POS_CENTER)
	return nil
}

func addMenuItem(menu *gtk.Menu, label string, fn func()) {
	item, _ := gtk.MenuItemNewWithLabel(label)
	item.Connect("activate", fn)
	menu.Append(item)
}

func (a *App) setupSidebar(parent *gtk.Box) {
	sideScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	sideScroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	sideScroll.SetMinContentWidth(200)
	sideScroll.SetShadowType(gtk.SHADOW_IN)

	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	setMargins(box, 8)
	sideScroll.Add(box)
	parent.PackStart(sideScroll, false, true, 0)

	title := newMarkupLabel("<b>Themes</b>")
	box.PackStart(title, false, false, 0)

	search, _ := gtk.SearchEntryNew()
	search.SetPlaceholderText("Search themes...")
	search.Connect("search-changed", func() {
		txt, _ := search.GetText()
		a.applyFilter(txt)
	})
	a.searchEntry = search
	box.PackStart(search, false, false, 0)

	a.filterCombo, _ = gtk.ComboBoxTextNew()
	for _, p := range [][2]string{
		{"all", "All themes"}, {"favorites", "⭐ Favorites"}, {"running", "● Running"},
		{"recent", "Recently used"}, {"needs_fix", "Needs fix"}, {"broken", "Broken"},
	} {
		a.filterCombo.Append(p[0], p[1])
	}
	a.filterCombo.SetActiveID("all")
	a.viewFilter = "all"
	a.filterCombo.Connect("changed", func() {
		a.viewFilter = a.filterCombo.GetActiveID()
		a.applyFilter("")
	})
	box.PackStart(a.filterCombo, false, false, 0)

	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scroll.SetMinContentHeight(80)
	scroll.SetVExpand(true)

	a.themeCards, _ = gtk.ListBoxNew()
	a.themeCards.SetSelectionMode(gtk.SELECTION_NONE)
	a.selectedThemeSet = map[string]bool{}
	a.themeCardChecks = map[string]*gtk.CheckButton{}
	scroll.Add(a.themeCards)
	box.PackStart(scroll, true, true, 0)

	toolbar, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 3)
	btnRefresh := compactIconButton("↻", "Refresh theme list", func() { a.loadThemes() })
	a.btnRefresh = widgetOf(btnRefresh)

	importBtn, _ := gtk.MenuButtonNew()
	il, _ := gtk.LabelNew("⇩")
	importBtn.Add(il)
	importBtn.SetTooltipText("Import folder or archive into ~/.conky")
	imenu, _ := gtk.MenuNew()
	addMenuItem(imenu, "Import folder…", a.importThemeFolder)
	addMenuItem(imenu, "Import archive (zip, tar…)…", a.importThemeArchive)
	imenu.ShowAll()
	importBtn.SetPopup(imenu)
	fitButton(importBtn)
	importBtn.SetSizeRequest(34, 30)
	a.btnImport = &importBtn.Widget

	btnDL := compactIconButton("◎", "Open Theme Marketplace", a.showThemeStore)
	btnDL.SetTooltipText("Search and install themes from GNOME-Look, KDE-Look, and Pling")
	addClass(btnDL, "suggested-action")
	a.btnDownload = widgetOf(btnDL)

	btnHealth := compactIconButton("⌕", "Scan all themes", a.healthScan)
	btnHealth.SetTooltipText("Health scan all themes")
	a.btnHealth = widgetOf(btnHealth)
	for _, b := range []*gtk.Button{btnRefresh, nil} {
		_ = b
	}
	toolbar.PackStart(btnRefresh, false, false, 0)
	toolbar.PackStart(importBtn, false, false, 0)
	toolbar.PackStart(btnDL, false, false, 0)
	toolbar.PackStart(btnHealth, false, false, 0)
	box.PackStart(toolbar, false, false, 0)

	actions, _ := gtk.GridNew()
	actions.SetColumnSpacing(3)
	actions.SetRowSpacing(3)
	actions.SetColumnHomogeneous(false)
	actions.SetHAlign(gtk.ALIGN_START)
	btnRun := compactIconButton("▶", "Run selected theme", a.runSelected)
	addClass(btnRun, "suggested-action")
	btnStopSel := compactIconButton("■", "Stop selected theme", a.stopSelected)
	btnRestart := compactIconButton("↻", "Restart selected theme", a.restartSelected)
	btnPrev := compactIconButton("◉", "Preview selected theme", a.previewSelected)
	btnEdit := compactIconButton("✎", "Edit selected theme", a.editSelected)
	btnOpen := compactIconButton("⌂", "Open theme folder", a.openSelectedFolder)
	btnRepair := compactIconButton("⚒", "Smart repair selected theme", a.smartRepair)
	btnInfo := compactIconButton("ⓘ", "Show theme details", a.showThemeDetails)
	a.btnRunSel, a.btnPreview, a.btnEdit, a.btnOpenFolder, a.btnRepair = widgetOf(btnRun), widgetOf(btnPrev), widgetOf(btnEdit), widgetOf(btnOpen), widgetOf(btnRepair)
	a.btnStopSel, a.btnRestartSel = widgetOf(btnStopSel), widgetOf(btnRestart)
	for i, b := range []*gtk.Button{btnRun, btnStopSel, btnRestart, btnPrev, btnEdit, btnOpen, btnRepair, btnInfo} {
		fitButton(b)
		actions.Attach(b, i%4, i/4, 1, 1)
	}
	box.PackStart(actions, false, false, 0)

	mgmt, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 2)
	for _, spec := range []struct {
		lab, tip string
		fn       func()
	}{
		{"📄", "Clone theme", a.cloneSelectedTheme},
		{"✏", "Rename theme", a.renameSelectedTheme},
		{"🗑", "Move theme to trash", a.deleteSelectedTheme},
		{"📦", "Export theme archive", a.exportSelectedTheme},
		{"⭐", "Toggle favorite", a.toggleFavoriteSelected},
	} {
		b, _ := gtk.ButtonNewWithLabel(spec.lab)
		b.SetTooltipText(spec.tip)
		b.SetSizeRequest(34, 30)
		fn := spec.fn
		b.Connect("clicked", fn)
		fitButton(b)
		mgmt.PackStart(b, false, false, 0)
	}
	box.PackStart(mgmt, false, false, 0)

	a.keepBusy(a.btnRefresh)
	a.keepBusy(a.btnImport)
	a.keepBusy(a.btnDownload)
	a.keepBusy(a.btnHealth)
	a.keepBusy(a.btnRunSel)
	a.keepBusy(a.btnStopSel)
	a.keepBusy(a.btnRestartSel)
	a.keepBusy(a.btnPreview)
	a.keepBusy(a.btnEdit)
	a.keepBusy(a.btnOpenFolder)
	a.keepBusy(a.btnRepair)
}

func (a *App) setupMainPanel(parent *gtk.Box) {
	panelScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	panelScroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	panel, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(panel, 9)
	panelScroll.Add(panel)
	parent.PackStart(panelScroll, true, true, 0)
	nb, _ := gtk.NotebookNew()
	nb.SetScrollable(true)
	a.notebook = nb
	panel.PackStart(nb, true, true, 0)
	a.buildTabControl()
	a.buildTabPosition()
	a.buildTabColors()
	a.buildTabProfiles()
	a.buildTabDiagnostics()
}

func (a *App) buildTabControl() {
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(tab, 8)

	stF, stB := framedBox("Runtime Status", true)
	header, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	a.statusIndicator, _ = gtk.LabelNew("")
	a.statusIndicator.SetMarkup("<span size='large' foreground='#64748b'>●</span> Idle")
	header.PackStart(a.statusIndicator, false, false, 0)
	a.runtimeInfo, _ = gtk.LabelNew("No managed Conky instance running")
	a.runtimeInfo.SetXAlign(0)
	a.runtimeInfo.SetLineWrap(true)
	header.PackStart(a.runtimeInfo, true, true, 0)
	stB.PackStart(header, false, false, 0)
	env := newMarkupLabel(fmt.Sprintf("<span foreground='#888888'>%s</span>", compositorLabel(a.desktopEnv, a.session)))
	stB.PackStart(env, false, false, 0)
	tab.PackStart(stF, false, false, 0)

	df, db := framedBox("Dashboard", true)
	a.dashLabels = map[string]*gtk.Label{}
	grid, _ := gtk.GridNew()
	grid.SetColumnSpacing(10)
	grid.SetRowSpacing(8)
	grid.SetColumnHomogeneous(true)
	for i, key := range []string{"themes", "running", "favorites", "ready", "needs_fix", "broken", "recent"} {
		lab := []string{"Themes", "Running", "Favorites", "Ready", "Needs fix", "Broken", "Recent"}[i]
		cell, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
		cell.PackStart(newLabel(lab, true), false, false, 0)
		val := newMarkupLabel("<b>0</b>")
		a.dashLabels[key] = val
		cell.PackStart(val, false, false, 0)
		grid.Attach(cell, i%4, i/4, 1, 1)
	}
	db.PackStart(grid, false, false, 0)
	quick, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	q1, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	q2, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	qRun := a.actionBtn("▶ Run", "suggested-action", a.runSelected)
	qStop := a.actionBtn("⏹ Stop", "", a.stopSelected)
	qScan := a.actionBtn("🔍 Scan", "", a.healthScan)
	qImp := a.actionBtn("📥 Import", "", a.importThemeFolder)
	qMkt := a.actionBtn("🌐 Marketplace", "", a.showThemeStore)
	for _, b := range []*gtk.Button{qRun, qStop, qScan} {
		q1.PackStart(b, false, false, 0)
	}
	for _, b := range []*gtk.Button{qImp, qMkt} {
		q2.PackStart(b, false, false, 0)
	}
	quick.PackStart(q1, false, false, 0)
	quick.PackStart(q2, false, false, 0)
	db.PackStart(quick, false, false, 0)
	tab.PackStart(df, false, false, 0)

	cf, cb := framedBox("Process Manager", true)
	row1, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	row2, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	btnRunAll := a.actionBtn("▶ Run All", "suggested-action", a.runAll)
	btnStop := a.actionBtn("⏹ Stop All Managed", "destructive-action", a.stopAll)
	btnOrphans := a.actionBtn("🧹 Kill Orphaned", "", a.killOrphans)
	btnVal, _ := gtk.ButtonNewWithLabel("✓ Validate All")
	btnVal.Connect("clicked", a.validateAllThemes)
	fitButton(btnVal)
	btnFix, _ := gtk.ButtonNewWithLabel("🔧 Fix All…")
	btnFix.Connect("clicked", a.fixAllThemes)
	fitButton(btnFix)
	btnLog, _ := gtk.ButtonNewWithLabel("📄 Open Log")
	btnLog.Connect("clicked", func() { xdgOpen(logFilePath) })
	fitButton(btnLog)
	a.btnRunAll, a.btnStop, a.btnValidate, a.btnKillOrphans = widgetOf(btnRunAll), widgetOf(btnStop), widgetOf(btnVal), widgetOf(btnOrphans)
	for _, b := range []gtk.IWidget{btnRunAll, btnStop, btnOrphans} {
		row1.PackStart(b, false, false, 0)
	}
	for _, b := range []gtk.IWidget{btnVal, btnFix, btnLog} {
		fitButton(b)
		row2.PackStart(b, false, false, 0)
	}
	cb.PackStart(row1, false, false, 0)
	cb.PackStart(row2, false, false, 0)
	tab.PackStart(cf, false, false, 0)

	pf, pb := framedBox("Preview", true)
	prow, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	durRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	btnRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	dur, _ := gtk.ComboBoxTextNew()
	dur.Append("5", "5 seconds")
	dur.Append("15", "15 seconds")
	dur.Append("30", "30 seconds")
	dur.Append("0", "Unlimited")
	dur.SetActiveID(strconv.Itoa(a.settings.PreviewSeconds))
	if a.settings.PreviewSeconds != 5 && a.settings.PreviewSeconds != 15 && a.settings.PreviewSeconds != 30 && a.settings.PreviewSeconds != 0 {
		dur.SetActiveID("5")
	}
	a.previewSeconds = a.settings.PreviewSeconds
	dur.Connect("changed", func() {
		if n, err := strconv.Atoi(dur.GetActiveID()); err == nil {
			a.previewSeconds = n
		}
	})
	pStart := a.actionBtn("▶ Start", "suggested-action", a.previewSelected)
	pStop := a.actionBtn("⏹ Stop", "", a.stopPreview)
	pRestart := a.actionBtn("🔁 Restart", "", a.restartPreview)
	pPause := a.actionBtn("⏸ Pause / Resume", "", a.togglePreviewPause)
	durRow.PackStart(newLabel("Duration:", false), false, false, 0)
	durRow.PackStart(dur, true, true, 0)
	for _, b := range []*gtk.Button{pStart, pStop, pRestart, pPause} {
		btnRow.PackStart(b, false, false, 0)
	}
	prow.PackStart(durRow, false, false, 0)
	prow.PackStart(btnRow, false, false, 0)
	pb.PackStart(prow, false, false, 0)
	pb.PackStart(newLabel("Preview uses a temporary launch bundle and does not overwrite the original theme.", true), false, false, 0)
	tab.PackStart(pf, false, false, 0)

	hf, hb := framedBox("Tips", true)
	for _, t := range []string{
		"• Each theme keeps its own monitor, position, and colors when running together.",
		"• Stop All Managed never touches Conky processes started outside this app.",
		"• Kill Orphaned only removes leftover manager launch-bundle processes.",
		"• Favorites, health status, and recently used views live in the sidebar filter.",
		"• Clone / Rename / Delete / Export update profiles and saved overrides.",
		fmt.Sprintf("• Detected environment: %s.", compositorLabel(a.desktopEnv, a.session)),
	} {
		hb.PackStart(newLabel(t, true), false, false, 0)
	}
	tab.PackStart(hf, true, true, 0)
	a.keepBusy(a.btnRunAll)
	a.keepBusy(a.btnValidate)
	lbl, _ := gtk.LabelNew("Control")
	a.notebook.AppendPage(scrollPage(tab), lbl)
}

func (a *App) screenSize() (int, int) {
	disp, err := gdk.DisplayGetDefault()
	if err != nil || disp == nil {
		return 1920, 1080
	}
	mon, err := disp.GetPrimaryMonitor()
	if err != nil || mon == nil {
		return 1920, 1080
	}
	g := mon.GetGeometry()
	return g.GetWidth(), g.GetHeight()
}

func (a *App) buildTabPosition() {
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(tab, 8)
	intro, _ := gtk.LabelNew("Each theme keeps its own monitor, alignment, and offset. Running multiple themes no longer shares one position.")
	intro.SetXAlign(0)
	intro.SetLineWrap(true)
	addClass(intro, "dim-label")
	tab.PackStart(intro, false, false, 0)
	a.buildPositionPanel(tab)
	hf, hb := framedBox("Position Tips", true)
	for _, t := range []string{
		"• Click a grid cell to set alignment (top-left, center, bottom-right, etc.).",
		"• gap_x / gap_y are pixel offsets from the chosen screen edge.",
		"• Use arrow buttons for fine-tuning, then Apply & Restart to preview live.",
		"• Save for Theme keeps a unique position for each Conky theme.",
	} {
		hb.PackStart(newLabel(t, true), false, false, 0)
	}
	tab.PackStart(hf, true, true, 0)
	a.keepBusy(widgetOf(a.btnSavePos))
	a.keepBusy(widgetOf(a.btnResetPos))
	a.keepBusy(widgetOf(a.btnApplyPos))
	lbl, _ := gtk.LabelNew("Position")
	a.notebook.AppendPage(scrollPage(tab), lbl)
}

func (a *App) buildPositionPanel(parent *gtk.Box) {
	fr, box := framedBox("Alignment & Offset", true)
	header, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	a.posEnabled, _ = gtk.SwitchNew()
	a.posEnabled.SetActive(true)
	a.posEnabled.SetTooltipText("When enabled, the manager controls where the theme appears on your desktop.")
	a.posEnabled.Connect("notify::active", a.onPosEnabled)
	header.PackStart(newLabel("Control position:", false), false, false, 0)
	header.PackStart(a.posEnabled, false, false, 0)
	a.posMonitor, _ = gtk.ComboBoxTextNew()
	a.refreshMonitorCombo("")
	a.posMonitor.SetTooltipText("Assign this theme to a connected display (HDMI, DP, eDP, …)")
	a.posMonitor.Connect("changed", a.onGapChanged)
	header.PackStart(newLabel("Monitor:", false), false, false, 0)
	header.PackStart(a.posMonitor, true, true, 0)
	mons := listMonitors()
	scr := newLabel(fmt.Sprintf("%d display(s)", len(mons)), true)
	header.PackEnd(scr, false, false, 0)
	box.PackStart(header, false, false, 0)

	gridRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	grid, _ := gtk.GridNew()
	grid.SetColumnSpacing(4)
	grid.SetRowSpacing(4)
	a.posButtons = map[string]*gtk.ToggleButton{}
	for i, pair := range positionAlignments {
		alignID, symbol := pair[0], pair[1]
		btn, _ := gtk.ToggleButtonNewWithLabel(symbol)
		btn.SetTooltipText(strings.ReplaceAll(alignID, "_", " "))
		fitButton(btn)
		id := alignID
		btn.Connect("toggled", func() { a.onAlignToggled(btn, id) })
		grid.Attach(btn, i%3, i/3, 1, 1)
		a.posButtons[alignID] = btn
	}
	gridRow.PackStart(grid, false, false, 0)

	off, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	a.posGapX, _ = gtk.SpinButtonNewWithRange(-4000, 4000, 1)
	a.posGapX.SetValue(30)
	a.posGapX.SetTooltipText("Horizontal offset from screen edge (gap_x)")
	a.posGapX.Connect("value-changed", a.onGapChanged)
	off.PackStart(settingsRow("Horizontal (gap_x):", a.posGapX), false, false, 0)
	a.posGapY, _ = gtk.SpinButtonNewWithRange(-4000, 4000, 1)
	a.posGapY.SetValue(50)
	a.posGapY.SetTooltipText("Vertical offset from screen edge (gap_y)")
	a.posGapY.Connect("value-changed", a.onGapChanged)
	off.PackStart(settingsRow("Vertical (gap_y):", a.posGapY), false, false, 0)
	a.posNudge, _ = gtk.SpinButtonNewWithRange(1, 100, 1)
	a.posNudge.SetValue(10)
	nudgeBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	addNudge := func(lab string, dx, dy int) {
		b, _ := gtk.ButtonNewWithLabel(lab)
		fitButton(b)
		b.Connect("clicked", func() { a.nudgePosition(dx, dy) })
		nudgeBox.PackStart(b, false, false, 0)
	}
	addNudge("←", -1, 0)
	addNudge("→", 1, 0)
	addNudge("↑", 0, -1)
	addNudge("↓", 0, 1)
	nudgeRow, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	nudgeRow.PackStart(newLabel("Fine tune:", false), false, false, 0)
	nudgeRow.PackStart(nudgeBox, false, false, 0)
	nudgeRow.PackStart(newLabel("Step:", false), false, false, 0)
	nudgeRow.PackStart(a.posNudge, false, false, 0)
	off.PackStart(nudgeRow, false, false, 0)
	gridRow.PackStart(off, true, true, 0)
	box.PackStart(gridRow, false, false, 0)

	a.posStatus = newLabel("Select a theme to adjust its position.", true)
	box.PackStart(a.posStatus, false, false, 0)
	act, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	a.btnSavePos, _ = gtk.ButtonNewWithLabel("💾 Save for Theme")
	a.btnSavePos.Connect("clicked", func() { a.savePositionForTheme(true) })
	a.btnResetPos, _ = gtk.ButtonNewWithLabel("↺ Reset")
	a.btnResetPos.Connect("clicked", a.resetPositionForTheme)
	a.btnApplyPos, _ = gtk.ButtonNewWithLabel("▶ Apply & Restart")
	addClass(a.btnApplyPos, "suggested-action")
	a.btnApplyPos.Connect("clicked", a.applyPositionAndRestart)
	for _, b := range []*gtk.Button{a.btnSavePos, a.btnResetPos, a.btnApplyPos} {
		fitButton(b)
		act.PackStart(b, false, false, 0)
	}
	box.PackStart(act, false, false, 0)
	a.setPositionSensitive(false)
	parent.PackStart(fr, false, false, 0)
}

func (a *App) refreshMonitorCombo(selected string) {
	if a.posMonitor == nil {
		return
	}
	a.posLoading = true
	prev := selected
	if prev == "" {
		prev = a.posMonitor.GetActiveID()
	}
	// ComboBoxText has no RemoveAll in older gotk3; rebuild by setting IDs if present.
	for _, id := range []string{"primary"} {
		_ = id
	}
	model := a.posMonitor.ToWidget()
	_ = model
	a.posMonitor.RemoveAll()
	a.posMonitor.Append("primary", "Primary display")
	for _, m := range listMonitors() {
		scale := ""
		if m.Scale > 1 {
			scale = fmt.Sprintf(" · %d%%", m.Scale*100)
		}
		role := ""
		if m.Primary {
			role = " (primary)"
		}
		a.posMonitor.Append(m.ID, fmt.Sprintf("%s%s · %dx%d%s", m.Name, role, m.Width, m.Height, scale))
	}
	if prev != "" {
		a.posMonitor.SetActiveID(prev)
	}
	if a.posMonitor.GetActiveID() == "" {
		a.posMonitor.SetActiveID("primary")
	}
	a.posLoading = false
}

func (a *App) setPositionSensitive(enabled bool) {
	for _, w := range []gtk.IWidget{a.posGapX, a.posGapY, a.posNudge, a.btnSavePos, a.btnResetPos, a.btnApplyPos} {
		w.ToWidget().SetSensitive(enabled)
	}
	if a.posMonitor != nil {
		a.posMonitor.SetSensitive(enabled)
	}
	for _, b := range a.posButtons {
		b.SetSensitive(enabled)
	}
}

func (a *App) setPositionControls(pos Position, enabled bool) {
	a.posLoading = true
	a.posEnabled.SetActive(enabled)
	for id, btn := range a.posButtons {
		btn.SetActive(id == pos.Alignment)
	}
	a.posGapX.SetValue(float64(pos.GapX))
	a.posGapY.SetValue(float64(pos.GapY))
	monID := pos.Monitor
	if monID == "" {
		monID = "primary"
	}
	a.refreshMonitorCombo(monID)
	a.posLoading = false
}

func (a *App) getPositionFromControls() *Position {
	if !a.posEnabled.GetActive() {
		return nil
	}
	al := "top_left"
	for id, btn := range a.posButtons {
		if btn.GetActive() {
			al = id
			break
		}
	}
	monID := "primary"
	if a.posMonitor != nil {
		if id := a.posMonitor.GetActiveID(); id != "" {
			monID = id
		}
	}
	head := 0
	if mon := findMonitor(monID, 0); mon != nil {
		head = mon.Head
		monID = mon.ID
	}
	return &Position{Enabled: true, Alignment: al, GapX: a.posGapX.GetValueAsInt(), GapY: a.posGapY.GetValueAsInt(), Monitor: monID, XineramaHead: head}
}

func (a *App) onPosEnabled() {
	if a.posLoading {
		return
	}
	en := a.posEnabled.GetActive() && a.activePosTheme != ""
	a.setPositionSensitive(en)
	if a.activePosTheme != "" {
		st := "disabled"
		if a.posEnabled.GetActive() {
			st = "enabled"
		}
		a.posStatus.SetText(filepath.Base(filepath.Dir(a.activePosTheme)) + ": position control " + st + ".")
	}
}

func (a *App) onAlignToggled(button *gtk.ToggleButton, alignID string) {
	if a.posLoading {
		return
	}
	if !button.GetActive() {
		a.posLoading = true
		button.SetActive(true)
		a.posLoading = false
		return
	}
	for other, btn := range a.posButtons {
		if other != alignID && btn.GetActive() {
			a.posLoading = true
			btn.SetActive(false)
			a.posLoading = false
		}
	}
	a.posStatus.SetText(fmt.Sprintf("Alignment: %s | X=%d, Y=%d", strings.ReplaceAll(alignID, "_", " "), a.posGapX.GetValueAsInt(), a.posGapY.GetValueAsInt()))
}

func (a *App) onGapChanged() {
	if a.posLoading {
		return
	}
	p := a.getPositionFromControls()
	if p == nil {
		return
	}
	a.posStatus.SetText(fmt.Sprintf("Offset: X=%d, Y=%d (%s)", p.GapX, p.GapY, strings.ReplaceAll(p.Alignment, "_", " ")))
}

func (a *App) nudgePosition(dx, dy int) {
	if a.activePosTheme == "" || !a.posEnabled.GetActive() {
		return
	}
	step := a.posNudge.GetValueAsInt()
	a.posLoading = true
	a.posGapX.SetValue(a.posGapX.GetValue() + float64(dx*step))
	a.posGapY.SetValue(a.posGapY.GetValue() + float64(dy*step))
	a.posLoading = false
	a.onGapChanged()
}

func (a *App) buildTabColors() {
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(tab, 8)
	intro, _ := gtk.LabelNew("Change theme colors without editing original files. Lua ring colors are synced automatically when Smart Lua Match is enabled.")
	intro.SetXAlign(0)
	intro.SetLineWrap(true)
	addClass(intro, "dim-label")
	tab.PackStart(intro, false, false, 0)

	header, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	a.colorEnabled, _ = gtk.SwitchNew()
	a.colorEnabled.SetActive(true)
	a.colorEnabled.Connect("notify::active", a.onColorEnabled)
	header.PackStart(newLabel("Recolor theme:", false), false, false, 0)
	header.PackStart(a.colorEnabled, false, false, 0)
	a.colorPreset, _ = gtk.ComboBoxTextNew()
	a.colorPreset.Append("custom", "Custom")
	for _, p := range colorPresets {
		a.colorPreset.Append(p.ID, p.Preset.Label)
	}
	for _, p := range loadCustomPalettes() {
		a.colorPreset.Append("custom:"+p.ID, p.Label)
	}
	a.colorPreset.SetActiveID("custom")
	a.colorPreset.Connect("changed", a.onColorPreset)
	header.PackEnd(a.colorPreset, false, false, 0)
	header.PackEnd(newLabel("Palette:", false), false, false, 0)
	tab.PackStart(header, false, false, 0)

	smart, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	a.colorSmartLua, _ = gtk.SwitchNew()
	a.colorSmartLua.SetActive(true)
	a.colorSmartLua.SetTooltipText("Automatically recolor matching hex values inside Lua ring scripts.")
	smart.PackStart(newLabel("Smart Lua match:", false), false, false, 0)
	smart.PackStart(a.colorSmartLua, false, false, 0)
	tab.PackStart(smart, false, false, 0)

	fr, _ := gtk.FrameNew("Theme Colors")
	fr.SetShadowType(gtk.SHADOW_ETCHED_IN)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scroll.SetMinContentHeight(80)
	a.colorSlotsBox, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	setMargins(a.colorSlotsBox, 12)
	scroll.Add(a.colorSlotsBox)
	fr.Add(scroll)
	tab.PackStart(fr, true, true, 0)

	a.colorStatus = newLabel("Select a theme to detect and customize its colors.", true)
	tab.PackStart(a.colorStatus, false, false, 0)
	act, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	a.btnSaveCol, _ = gtk.ButtonNewWithLabel("💾 Save for Theme")
	a.btnSaveCol.Connect("clicked", func() { a.saveColorsForTheme(true) })
	a.btnResetCol, _ = gtk.ButtonNewWithLabel("↺ Reset")
	a.btnResetCol.Connect("clicked", a.resetColorsForTheme)
	a.btnApplyCol, _ = gtk.ButtonNewWithLabel("▶ Apply & Restart")
	addClass(a.btnApplyCol, "suggested-action")
	a.btnApplyCol.Connect("clicked", a.applyColorsAndRestart)
	palBtn, _ := gtk.ButtonNewWithLabel("🎨 Palettes")
	palBtn.Connect("clicked", a.showPaletteManager)
	for _, b := range []*gtk.Button{a.btnSaveCol, a.btnResetCol, a.btnApplyCol, palBtn} {
		fitButton(b)
		act.PackStart(b, false, false, 0)
	}
	tab.PackStart(act, false, false, 0)
	hf, hb := framedBox("Color Tips", true)
	for _, t := range []string{
		"• Original theme files are never modified — colors apply in launch bundles only.",
		"• Palettes remap only the color slots detected in the selected theme.",
		"• Smart Lua Match updates ring meters when they share the same hex values.",
		"• Save for Theme keeps custom colors for each Conky theme separately.",
	} {
		hb.PackStart(newLabel(t, true), false, false, 0)
	}
	tab.PackStart(hf, false, false, 0)
	a.colorPickers = map[string]*gtk.ColorButton{}
	a.themeColorSlots = map[string]string{}
	a.setColorSensitive(false)
	a.keepBusy(widgetOf(a.btnSaveCol))
	a.keepBusy(widgetOf(a.btnResetCol))
	a.keepBusy(widgetOf(a.btnApplyCol))
	lbl, _ := gtk.LabelNew("Colors")
	a.notebook.AppendPage(scrollPage(tab), lbl)
}

func (a *App) hexToRGBA(color string) *gdk.RGBA {
	n := normalizeColor(color)
	if n == "" {
		return gdk.NewRGBA(1, 1, 1, 1)
	}
	body := n[1:]
	r64, _ := strconv.ParseInt(body[0:2], 16, 64)
	g64, _ := strconv.ParseInt(body[2:4], 16, 64)
	b64, _ := strconv.ParseInt(body[4:6], 16, 64)
	return gdk.NewRGBA(float64(r64)/255, float64(g64)/255, float64(b64)/255, 1)
}

func rgbaToHex(r *gdk.RGBA) string {
	f := r.Floats()
	return fmt.Sprintf("#%02X%02X%02X", int(f[0]*255), int(f[1]*255), int(f[2]*255))
}

func (a *App) setColorSensitive(enabled bool) {
	on := a.colorEnabled.GetActive()
	for _, w := range []gtk.IWidget{a.colorPreset, a.colorSmartLua, a.btnSaveCol, a.btnResetCol, a.btnApplyCol} {
		w.ToWidget().SetSensitive(enabled && on)
	}
	for _, p := range a.colorPickers {
		p.SetSensitive(enabled && on)
	}
}

func (a *App) clearColorPickers() {
	removeChildren(&a.colorSlotsBox.Container)
	a.colorPickers = map[string]*gtk.ColorButton{}
}

func (a *App) populateColorPickers(colors map[string]string) {
	a.clearColorPickers()
	a.themeColorSlots = cloneMap(colors)
	if len(colors) == 0 {
		a.colorSlotsBox.PackStart(newLabel("No color slots detected in this theme.", true), false, false, 0)
		a.colorSlotsBox.ShowAll()
		return
	}
	for _, slot := range orderedThemeColorSlots(colors) {
		row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
		lab := colorSlotLabels[slot]
		if lab == "" {
			lab = slot
		}
		row.PackStart(newLabel(fmt.Sprintf("%s (%s):", lab, slot), false), false, false, 0)
		picker, _ := gtk.ColorButtonNew()
		picker.SetRGBA(a.hexToRGBA(colors[slot]))
		picker.SetTooltipText("Original: " + colors[slot])
		s := slot
		picker.Connect("color-set", func() { a.onColorPicker(picker, s) })
		row.PackEnd(picker, false, false, 0)
		a.colorSlotsBox.PackStart(row, false, false, 0)
		a.colorPickers[slot] = picker
	}
	a.colorSlotsBox.ShowAll()
}

func (a *App) getColorsFromControls() *ColorOverride {
	if !a.colorEnabled.GetActive() || len(a.colorPickers) == 0 {
		return nil
	}
	colors := map[string]string{}
	for slot, p := range a.colorPickers {
		colors[slot] = rgbaToHex(p.GetRGBA())
	}
	return &ColorOverride{Colors: colors, SmartLua: a.colorSmartLua.GetActive(), Enabled: true}
}

func (a *App) onColorEnabled() {
	if a.colorLoading {
		return
	}
	a.setColorSensitive(a.activeColorTheme != "")
	if a.activeColorTheme != "" {
		st := "disabled"
		if a.colorEnabled.GetActive() {
			st = "enabled"
		}
		a.colorStatus.SetText(filepath.Base(filepath.Dir(a.activeColorTheme)) + ": color customization " + st)
	}
}

func (a *App) onColorPreset() {
	if a.colorLoading || len(a.themeColorSlots) == 0 {
		return
	}
	id := a.colorPreset.GetActiveID()
	if id == "" || id == "custom" {
		return
	}
	merged := mergePresetColors(a.themeColorSlots, id)
	a.colorLoading = true
	for slot, p := range a.colorPickers {
		if v, ok := merged[slot]; ok {
			p.SetRGBA(a.hexToRGBA(v))
		}
	}
	a.colorLoading = false
	label := id
	for _, p := range colorPresets {
		if p.ID == id {
			label = p.Preset.Label
		}
	}
	a.colorStatus.SetText("Applied palette: " + label)
}

func (a *App) onColorPicker(picker *gtk.ColorButton, slot string) {
	if a.colorLoading {
		return
	}
	a.colorLoading = true
	a.colorPreset.SetActiveID("custom")
	a.colorLoading = false
	lab := colorSlotLabels[slot]
	if lab == "" {
		lab = slot
	}
	a.colorStatus.SetText(fmt.Sprintf("Updated %s → %s", lab, rgbaToHex(picker.GetRGBA())))
}

func (a *App) buildTabProfiles() {
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(tab, 8)
	nf, nb := framedBox("Profile Name", true)
	a.profileName, _ = gtk.EntryNew()
	a.profileName.SetPlaceholderText("My profile name...")
	a.profileName.SetText(a.profiles.LastProfile)
	nb.PackStart(a.profileName, false, false, 0)
	tab.PackStart(nf, false, false, 0)

	content, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	left, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	left.PackStart(newLabel("Saved Profiles", false), false, false, 0)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scroll.SetMinContentHeight(80)
	a.profileList, _ = gtk.ListBoxNew()
	a.profileList.SetSelectionMode(gtk.SELECTION_SINGLE)
	a.profileList.Connect("row-selected", func() { a.onProfileSelected() })
	scroll.Add(a.profileList)
	left.PackStart(scroll, true, true, 0)
	content.PackStart(left, true, true, 0)

	right, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	btns := []*gtk.Button{
		a.actionBtn("💾 Save Profile", "suggested-action", a.saveProfile),
		a.actionBtn("➕ Add Themes", "", a.addToProfile),
		a.actionBtn("▶ Run Profile", "", a.runProfile),
		a.actionBtn("📄 Duplicate", "", a.duplicateProfile),
		a.actionBtn("📦 Export .conky-profile", "", a.exportProfile),
		a.actionBtn("📥 Import Profile", "", a.importProfile),
		a.actionBtn("🚀 Enable Autostart", "", a.enableAutostart),
		a.actionBtn("⏹ Disable Autostart", "", a.disableAutostart),
		a.actionBtn("🗑 Delete Profile", "", a.deleteProfile),
		a.actionBtn("🔄 Reload", "", a.refreshProfiles),
	}
	for _, b := range btns {
		right.PackStart(b, false, false, 0)
		a.keepBusy(widgetOf(b))
	}
	content.PackStart(right, false, false, 0)
	tab.PackStart(content, true, true, 0)

	df, db := framedBox("Profile Contents", true)
	a.profileDetailsTitle = newLabel("No profile selected", true)
	db.PackStart(a.profileDetailsTitle, false, false, 0)
	dscroll, _ := gtk.ScrolledWindowNew(nil, nil)
	dscroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	dscroll.SetMinContentHeight(80)
	a.profileDetails, _ = gtk.ListBoxNew()
	dscroll.Add(a.profileDetails)
	db.PackStart(dscroll, true, true, 0)
	tab.PackStart(df, false, false, 0)
	a.profileRowNames = map[uintptr]string{}
	lbl, _ := gtk.LabelNew("Profiles")
	a.notebook.AppendPage(scrollPage(tab), lbl)
}

func (a *App) buildTabDiagnostics() {
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	setMargins(tab, 8)
	top, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	btn, _ := gtk.ButtonNewWithLabel("🔄 Refresh Log")
	btn.Connect("clicked", func() { a.refreshLogView(250) })
	fitButton(btn)
	top.PackStart(btn, false, false, 0)
	tab.PackStart(top, false, false, 0)
	fr, box := framedBox("Application Log", true)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	a.logsView, _ = gtk.TextViewNew()
	a.logsView.SetEditable(false)
	a.logsView.SetMonospace(true)
	a.logsBuffer, _ = a.logsView.GetBuffer()
	scroll.Add(a.logsView)
	box.PackStart(scroll, true, true, 0)
	tab.PackStart(fr, true, true, 0)
	lbl, _ := gtk.LabelNew("Diagnostics")
	a.notebook.AppendPage(scrollPage(tab), lbl)
}

func (a *App) setupStatusbar(parent *gtk.Box) {
	sb, _ := gtk.StatusbarNew()
	a.statusbar = sb
	a.statusCtx = sb.GetContextId("main")
	parent.PackStart(sb, false, false, 0)
	a.pushStatus("Ready")
}

func (a *App) showWaylandNotice() {
	if !strings.Contains(strings.ToLower(a.session), "wayland") {
		return
	}
	kind := displayServerKind()
	msg := fmt.Sprintf("Detected %s on %s (%s).\n\nWindow hints were selected for this compositor automatically.\nIf a theme is invisible, try Layer = above and Window type = dock in Settings.\nXWayland is used when DISPLAY is also available.",
		strings.ToUpper(a.desktopEnv), a.session, kind)
	a.showMessage("Desktop compatibility", msg, gtk.MESSAGE_INFO)
}

func (a *App) showAbout() {
	about, _ := gtk.AboutDialogNew()
	about.SetTransientFor(a.win)
	about.SetProgramName(appName)
	about.SetVersion(appVersion)
	about.SetComments("Universal Conky theme manager for Linux\n(KDE, GNOME, XFCE, Cinnamon, MATE, LXQt, and more)\nGo + GTK 3 edition")
	about.SetWebsite("https://github.com/almezali/conky-manager-g")
	about.Run()
	about.Destroy()
}

func comboID(c *gtk.ComboBoxText, def string) string {
	id := c.GetActiveID()
	if id == "" {
		return def
	}
	return id
}

func (a *App) showSettings() {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Settings")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(520, 520)
	dlg.SetResizable(true)
	content, _ := dlg.GetContentArea()
	content.SetSpacing(10)
	setMargins(content, 12)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scroll.SetMinContentHeight(200)
	inner, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	scroll.Add(inner)
	content.PackStart(scroll, true, true, 0)

	deF, deB := framedBox("Desktop Environment", true)
	preset, _ := gtk.ComboBoxTextNew()
	for _, p := range [][2]string{
		{"auto", "Auto-detect (" + a.desktopEnv + ")"}, {"kde", "KDE / Plasma"}, {"gnome", "GNOME"},
		{"xfce", "XFCE"}, {"cinnamon", "Cinnamon"}, {"mate", "MATE"}, {"lxqt", "LXQt"}, {"lxde", "LXDE"},
		{"hyprland", "Hyprland"}, {"niri", "Niri"}, {"sway", "Sway"}, {"cosmic", "COSMIC"}, {"generic", "Generic / Other"},
	} {
		preset.Append(p[0], p[1])
	}
	preset.SetActiveID(a.settings.DesktopPreset)
	optimize, _ := gtk.SwitchNew()
	optimize.SetActive(a.settings.AutoOptimize)
	visual, _ := gtk.SwitchNew()
	visual.SetActive(a.settings.FullVisualLaunch)
	compat, _ := gtk.SwitchNew()
	compat.SetActive(a.settings.CreateCompatSymlinks)
	remember, _ := gtk.SwitchNew()
	remember.SetActive(a.settings.RememberPosition)
	rememberC, _ := gtk.SwitchNew()
	rememberC.SetActive(a.settings.RememberColors)
	scaleAware, _ := gtk.SwitchNew()
	scaleAware.SetActive(a.settings.ScaleAwarePosition)
	deB.PackStart(settingsRow("Environment preset:", preset), false, false, 0)
	packSwitch := func(parent *gtk.Box, lab string, sw *gtk.Switch) {
		row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
		l, _ := gtk.LabelNew(lab)
		l.SetXAlign(0)
		l.SetSizeRequest(160, -1)
		row.PackStart(l, false, false, 0)
		row.PackStart(sw, false, false, 0)
		parent.PackStart(row, false, false, 0)
	}
	packSwitch(deB, "Auto-optimize themes:", optimize)
	packSwitch(deB, "Full visual launch:", visual)
	packSwitch(deB, "Compat symlinks:", compat)
	packSwitch(deB, "Remember position:", remember)
	packSwitch(deB, "Remember colors:", rememberC)
	packSwitch(deB, "Scale-aware gaps:", scaleAware)
	inner.PackStart(deF, false, false, 0)

	posF, posB := framedBox("Default Desktop Position", true)
	positions := loadPositions()
	def := positions.Default
	defEn, _ := gtk.SwitchNew()
	defEn.SetActive(def.Enabled)
	defAlign, _ := gtk.ComboBoxTextNew()
	for _, p := range positionAlignments {
		defAlign.Append(p[0], p[1]+" "+strings.ReplaceAll(p[0], "_", " "))
	}
	defAlign.SetActiveID(def.Alignment)
	defGX, _ := gtk.SpinButtonNewWithRange(-4000, 4000, 1)
	defGX.SetValue(float64(def.GapX))
	defGY, _ := gtk.SpinButtonNewWithRange(-4000, 4000, 1)
	defGY.SetValue(float64(def.GapY))
	posB.PackStart(settingsRow("Enable by default:", defEn), false, false, 0)
	posB.PackStart(settingsRow("Default alignment:", defAlign), false, false, 0)
	posB.PackStart(settingsRow("Default gap_x:", defGX), false, false, 0)
	posB.PackStart(settingsRow("Default gap_y:", defGY), false, false, 0)
	inner.PackStart(posF, false, false, 0)

	winF, winB := framedBox("Window Behavior", true)
	layer, _ := gtk.ComboBoxTextNew()
	layer.Append("above", "Above windows (overlay)")
	layer.Append("below", "Below windows (background)")
	layer.SetActiveID(a.settings.Layer)
	wtype, _ := gtk.ComboBoxTextNew()
	wtype.Append("dock", "Dock")
	wtype.Append("desktop", "Desktop (GNOME/XFCE/MATE)")
	wtype.Append("normal", "Normal window")
	wtype.SetActiveID(a.settings.WindowType)
	winB.PackStart(settingsRow("Layer:", layer), false, false, 0)
	winB.PackStart(settingsRow("Window type:", wtype), false, false, 0)
	inner.PackStart(winF, false, false, 0)

	pf, pb := framedBox("Performance", true)
	smooth, _ := gtk.ComboBoxTextNew()
	smooth.Append("ultra", "Ultra smooth (less CPU)")
	smooth.Append("balanced", "Balanced")
	smooth.Append("performance", "Performance (faster updates)")
	smooth.SetActiveID(a.settings.Smoothness)
	nice, _ := gtk.SpinButtonNewWithRange(-5, 19, 1)
	nice.SetValue(float64(a.settings.NiceLevel))
	maxN, _ := gtk.SpinButtonNewWithRange(1, 16, 1)
	maxN.SetValue(float64(a.settings.MaxInstances))
	health, _ := gtk.SpinButtonNewWithRange(0.4, 5.0, 0.1)
	health.SetDigits(1)
	health.SetValue(a.settings.HealthcheckSeconds)
	pb.PackStart(settingsRow("Smoothness:", smooth), false, false, 0)
	pb.PackStart(settingsRow("Nice level:", nice), false, false, 0)
	pb.PackStart(settingsRow("Max instances:", maxN), false, false, 0)
	pb.PackStart(settingsRow("Healthcheck (sec):", health), false, false, 0)
	inner.PackStart(pf, false, false, 0)

	tf, tb := framedBox("Preview & Interface", true)
	prev, _ := gtk.SpinButtonNewWithRange(2, 60, 1)
	prev.SetValue(float64(a.settings.PreviewSeconds))
	ui, _ := gtk.ComboBoxTextNew()
	ui.Append("auto", "Auto (system)")
	ui.Append("light", "Light")
	ui.Append("dark", "Dark")
	ui.SetActiveID(a.settings.UITheme)
	tb.PackStart(settingsRow("Preview seconds:", prev), false, false, 0)
	tb.PackStart(settingsRow("UI theme:", ui), false, false, 0)
	inner.PackStart(tf, false, false, 0)

	dirF, dirB := framedBox("Custom Theme Directories", true)
	dirHelp := newLabel("One directory per line. These folders are included in theme discovery.", true)
	dirB.PackStart(dirHelp, false, false, 0)
	dirView, _ := gtk.TextViewNew()
	dirView.SetWrapMode(gtk.WRAP_WORD_CHAR)
	dirBuf, _ := dirView.GetBuffer()
	dirBuf.SetText(strings.Join(append([]string{}, a.settings.CustomThemeDirs...), "\n"))
	dirScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	dirScroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	dirScroll.SetMinContentHeight(80)
	dirScroll.Add(dirView)
	dirB.PackStart(dirScroll, true, true, 0)
	inner.PackStart(dirF, false, false, 0)

	dlg.AddButton("Reset Defaults", gtk.RESPONSE_REJECT)
	dlg.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dlg.AddButton("Save", gtk.RESPONSE_OK)
	dlg.ShowAll()
	resp := dlg.Run()
	if resp == gtk.RESPONSE_REJECT {
		a.settings = defaultSettings()
		_ = saveJSON(settingsFile, a.settings)
		savePositions(defaultPositions())
		saveColors(ColorsFile{Themes: map[string]ColorOverride{}})
		a.applyUITheme()
		a.pushStatus("Settings reset to defaults.")
	} else if resp == gtk.RESPONSE_OK {
		start := dirBuf.GetStartIter()
		end := dirBuf.GetEndIter()
		dirText, _ := dirBuf.GetText(start, end, false)
		var custom []string
		for _, line := range strings.Split(dirText, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				custom = append(custom, line)
			}
		}
		a.settings = Settings{
			Layer: comboID(layer, "above"), WindowType: comboID(wtype, "dock"),
			Smoothness: comboID(smooth, "balanced"), NiceLevel: nice.GetValueAsInt(),
			MaxInstances: maxN.GetValueAsInt(), HealthcheckSeconds: health.GetValue(),
			PreviewSeconds: prev.GetValueAsInt(), UITheme: comboID(ui, "auto"),
			DesktopPreset: comboID(preset, "auto"), AutoOptimize: optimize.GetActive(),
			FullVisualLaunch: visual.GetActive(), CreateCompatSymlinks: compat.GetActive(),
			RememberPosition: remember.GetActive(), RememberColors: rememberC.GetActive(),
			ScaleAwarePosition: scaleAware.GetActive(), CustomThemeDirs: custom,
		}
		_ = saveJSON(settingsFile, a.settings)
		positions = loadPositions()
		positions.Default = Position{Enabled: defEn.GetActive(), Alignment: comboID(defAlign, "top_left"), GapX: defGX.GetValueAsInt(), GapY: defGY.GetValueAsInt()}
		savePositions(positions)
		a.applyUITheme()
		a.pushStatus("Settings saved.")
		a.showMessage("Success", "Settings saved successfully.", gtk.MESSAGE_INFO)
		a.loadPositionForSelected()
		a.loadColorsForSelected()
	}
	dlg.Destroy()
}

func (a *App) loadThemes() {
	var items []ThemeItem
	for _, p := range findThemes() {
		items = append(items, ThemeItem{Path: p})
		ensureCompatInstallSymlink(resolveThemeRoot(p))
	}
	a.themes = items
	a.applyFilter("")
	a.pushStatus(fmt.Sprintf("Loaded %d valid theme(s).", len(a.themes)))
}

func themeRuntimeState(item ThemeItem, pids map[string]int) (string, int) {
	if pid, ok := pids[themeID(item.Path)]; ok && pid > 0 {
		return "running", pid
	}
	return "stopped", 0
}

func (a *App) rebuildPIDMap() map[string]int {
	out := runtimePIDMap()
	live := scanConkyProcesses()
	for _, item := range a.themes {
		id := themeID(item.Path)
		if out[id] > 0 {
			continue
		}
		root := resolveThemeRoot(item.Path)
		base := filepath.Base(item.Path)
		for _, proc := range live {
			if strings.Contains(proc.Cmdline, id) || strings.Contains(proc.Cmdline, root) || strings.Contains(proc.Cmdline, base) {
				out[id] = proc.PID
				break
			}
		}
	}
	a.runtimeCache = out
	return out
}

func themeListStatus(health, runtime string) string {
	if runtime == "running" {
		return "● Running"
	}
	switch health {
	case "stable":
		return "✓ Ready"
	case "needs_fix":
		return "⚠ Needs fix"
	case "broken":
		return "✗ Broken"
	default:
		return "✦ New"
	}
}

func (a *App) refreshDashboard() {
	if a.dashLabels == nil {
		return
	}
	pids := a.rebuildPIDMap()
	fav := loadLibrary().Favorites
	recent := recentSet()
	running, favorites, ready, needs, broken, rec := 0, 0, 0, 0, 0, 0
	for _, item := range a.themes {
		st, _ := themeRuntimeState(item, pids)
		if st == "running" {
			running++
		}
		if fav[themeID(item.Path)] {
			favorites++
		}
		if recent[themeID(item.Path)] {
			rec++
		}
		switch a.themeStatus[item.Label()] {
		case "stable":
			ready++
		case "needs_fix":
			needs++
		case "broken":
			broken++
		}
	}
	set := func(key string, n int) {
		if l := a.dashLabels[key]; l != nil {
			l.SetMarkup(fmt.Sprintf("<b>%d</b>", n))
		}
	}
	set("themes", len(a.themes))
	set("running", running)
	set("favorites", favorites)
	set("ready", ready)
	set("needs_fix", needs)
	set("broken", broken)
	set("recent", rec)
	if running > 0 {
		a.updateRuntime(true, fmt.Sprintf("%d managed instance(s) running", running))
	} else {
		a.updateRuntime(false, "")
	}
}

func (a *App) applyFilter(query string) {
	if query == "" && a.searchEntry != nil {
		query, _ = a.searchEntry.GetText()
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if a.themeCards == nil {
		return
	}
	clearListBox(a.themeCards)
	a.themeCardChecks = map[string]*gtk.CheckButton{}
	pids := a.rebuildPIDMap()
	a.runtimeCache = pids
	fav := loadLibrary().Favorites
	recent := recentSet()
	view := a.viewFilter
	if view == "" {
		view = "all"
	}
	for _, item := range a.themes {
		root := filepath.Base(resolveThemeRoot(item.Path))
		health := a.themeStatus[item.Label()]
		runtime, pid := themeRuntimeState(item, pids)
		id := themeID(item.Path)
		switch view {
		case "favorites":
			if !fav[id] {
				continue
			}
		case "running":
			if runtime != "running" {
				continue
			}
		case "recent":
			if !recent[id] {
				continue
			}
		case "needs_fix":
			if health != "needs_fix" {
				continue
			}
		case "broken":
			if health != "broken" {
				continue
			}
		}
		star := ""
		if fav[id] {
			star = "⭐ "
		}
		haystack := strings.ToLower(star + root + " " + filepath.Base(item.Path) + " " + item.Path + " " + health + " " + runtime)
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}
		row, _ := gtk.ListBoxRowNew()
		card, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
		setMargins(card, 9)
		top, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
		check, _ := gtk.CheckButtonNew()
		check.SetActive(a.selectedThemeSet[resolvePath(item.Path)])
		check.SetTooltipText("Select this theme for multi-theme actions")
		path := item.Path
		check.Connect("toggled", func() {
			a.selectedThemeSet[resolvePath(path)] = check.GetActive()
			if check.GetActive() {
				a.loadPositionForSelected()
				a.loadColorsForSelected()
			}
		})
		a.themeCardChecks[resolvePath(item.Path)] = check
		top.PackStart(check, false, false, 0)
		name := newMarkupLabel("<b>" + star + markupEscape(root) + "</b>")
		top.PackStart(name, true, true, 0)
		badge := newLabel(themeListStatus(health, runtime), false)
		top.PackEnd(badge, false, false, 0)
		card.PackStart(top, false, false, 0)
		config := newLabel(filepath.Base(item.Path), true)
		config.SetEllipsize(pango.ELLIPSIZE_END)
		card.PackStart(config, false, false, 0)
		pidTxt := runtime
		if pid > 0 {
			pidTxt = fmt.Sprintf("running · pid %d", pid)
		}
		info := newLabel(fmt.Sprintf("%s  ·  %s", pidTxt, filepath.Dir(item.Path)), true)
		info.SetEllipsize(pango.ELLIPSIZE_END)
		card.PackStart(info, false, false, 0)
		row.Add(card)
		row.Connect("activate", func() { check.SetActive(!check.GetActive()) })
		row.Connect("button-press-event", func(_ *gtk.ListBoxRow, ev *gdk.Event) bool {
			be := gdk.EventButtonNewFromEvent(ev)
			if be.Button() == 1 && be.X() > 36 {
				check.SetActive(!check.GetActive())
				return true
			}
			return false
		})
		a.themeCards.Add(row)
	}
	a.themeCards.ShowAll()
	a.refreshDashboard()
}

func (a *App) selectedThemePaths() []string {
	var paths []string
	for _, item := range a.themes {
		if a.selectedThemeSet[resolvePath(item.Path)] {
			paths = append(paths, item.Path)
		}
	}
	return paths
}

func (a *App) selectedThemeItem() *ThemeItem {
	ps := a.selectedThemePaths()
	if len(ps) == 0 {
		return nil
	}
	return &ThemeItem{Path: ps[0]}
}

func (a *App) showThemeContextMenu(ev *gdk.EventButton) {
	th := a.selectedThemeItem()
	if th == nil {
		return
	}
	menu, _ := gtk.MenuNew()
	addMenuItem(menu, "Open theme file", func() { xdgOpen(th.Path) })
	addMenuItem(menu, "Edit theme file", func() { xdgOpen(th.Path) })
	addMenuItem(menu, "Open containing folder", func() { xdgOpen(filepath.Dir(th.Path)) })
	sep, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep)
	addMenuItem(menu, "Open optimized file", func() {
		p, err := optimizeForDesktop(th.Path)
		if err == nil {
			xdgOpen(p)
		}
	})
	sep2, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep2)
	addMenuItem(menu, "Validate (original + optimized)", a.validateSelected)
	addMenuItem(menu, "Preview (temporary)", a.previewSelected)
	menu.ShowAll()
	menu.PopupAtPointer(ev.Event)
}

func (a *App) loadPositionForSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.activePosTheme = ""
		a.setPositionSensitive(false)
		a.posStatus.SetText("Select a theme to adjust its position.")
		return
	}
	a.activePosTheme = th.Path
	a.setPositionSensitive(true)
	content := readText(th.Path)
	if saved := resolveThemePosition(th.Path, nil); saved != nil {
		a.setPositionControls(*saved, true)
		a.posStatus.SetText(fmt.Sprintf("%s: saved position (%s, X=%d, Y=%d)", filepath.Base(filepath.Dir(th.Path)), saved.Alignment, saved.GapX, saved.GapY))
		return
	}
	parsed := parseThemePosition(content)
	defaults := loadPositions().Default
	a.setPositionControls(parsed, defaults.Enabled)
	a.posStatus.SetText(fmt.Sprintf("%s: from theme file (%s, X=%d, Y=%d)", filepath.Base(filepath.Dir(th.Path)), parsed.Alignment, parsed.GapX, parsed.GapY))
}

func (a *App) loadColorsForSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.activeColorTheme = ""
		a.clearColorPickers()
		a.setColorSensitive(false)
		a.colorStatus.SetText("Select a theme to detect and customize its colors.")
		return
	}
	a.activeColorTheme = th.Path
	content := readText(th.Path)
	base := parseThemeColors(content)
	if saved := resolveThemeColors(th.Path); saved != nil {
		display := cloneMap(base)
		for k, v := range saved.Colors {
			display[k] = v
		}
		a.colorLoading = true
		a.colorEnabled.SetActive(true)
		a.colorSmartLua.SetActive(saved.SmartLua)
		a.colorPreset.SetActiveID("custom")
		a.populateColorPickers(display)
		a.colorLoading = false
		a.setColorSensitive(true)
		a.colorStatus.SetText(fmt.Sprintf("%s: saved custom colors (%d slots)", filepath.Base(filepath.Dir(th.Path)), len(saved.Colors)))
		return
	}
	a.colorLoading = true
	a.colorEnabled.SetActive(true)
	a.colorSmartLua.SetActive(true)
	a.colorPreset.SetActiveID("custom")
	a.populateColorPickers(base)
	a.colorLoading = false
	a.setColorSensitive(len(base) > 0)
	if len(base) > 0 {
		a.colorStatus.SetText(fmt.Sprintf("%s: detected %d color slot(s) from theme file", filepath.Base(filepath.Dir(th.Path)), len(base)))
	} else {
		a.colorStatus.SetText(fmt.Sprintf("%s: no standard color slots found in config", filepath.Base(filepath.Dir(th.Path))))
	}
}

func (a *App) savePositionForTheme(show bool) *Position {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return nil
	}
	pos := a.getPositionFromControls()
	if pos == nil {
		a.showMessage("Info", "Enable position control first.", gtk.MESSAGE_INFO)
		return nil
	}
	data := loadPositions()
	if data.Themes == nil {
		data.Themes = map[string]Position{}
	}
	pos.Enabled = true
	data.Themes[resolvePath(th.Path)] = *pos
	savePositions(data)
	updated := applyPositionPatch(readText(th.Path), *pos)
	if err := writeOriginalThemeConfig(th.Path, updated, "position"); err != nil {
		a.showMessage("Original file was not updated", err.Error(), gtk.MESSAGE_ERROR)
		return nil
	}
	a.posStatus.SetText(fmt.Sprintf("Saved for %s: %s, X=%d, Y=%d", filepath.Base(filepath.Dir(th.Path)), pos.Alignment, pos.GapX, pos.GapY))
	if show {
		a.pushStatus("Position saved for " + filepath.Base(filepath.Dir(th.Path)) + ".")
	}
	return pos
}

func (a *App) resetPositionForTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	data := loadPositions()
	delete(data.Themes, resolvePath(th.Path))
	savePositions(data)
	a.loadPositionForSelected()
	a.pushStatus("Position reset for " + filepath.Base(filepath.Dir(th.Path)) + ".")
}

func (a *App) captureRunPosition() *Position {
	pos := a.getPositionFromControls()
	if pos != nil && a.settings.RememberPosition {
		a.savePositionForTheme(false)
	}
	return pos
}

func (a *App) captureRunColors() *ColorOverride {
	c := a.getColorsFromControls()
	if c != nil && a.settings.RememberColors {
		a.saveColorsForTheme(false)
	}
	return c
}

func (a *App) applyPositionAndRestart() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	pos := a.savePositionForTheme(false)
	if pos == nil {
		return
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	a.setBusy(true)
	a.pushStatus("Applying position for " + filepath.Base(filepath.Dir(th.Path)) + "...")
	var colors *ColorOverride
	if a.colorEnabled.GetActive() {
		colors = a.getColorsFromControls()
	}
	path := th.Path
	go func() {
		defer idle(func() { a.setBusy(false) })
		stopManagedTheme(path)
		res, err := runConky(path, pos, colors)
		if err != nil {
			idle(func() { a.showMessage("Position Error", err.Error(), gtk.MESSAGE_ERROR) })
			return
		}
		idle(func() {
			a.updateRuntime(true, fmt.Sprintf("%s [%s] @ %s X=%d Y=%d", filepath.Base(path), res.Mode, pos.Alignment, pos.GapX, pos.GapY))
			a.pushStatus(fmt.Sprintf("Position applied: %s (%s, X=%d, Y=%d)", filepath.Base(filepath.Dir(path)), pos.Alignment, pos.GapX, pos.GapY))
			a.applyFilter("")
		})
	}()
}

func (a *App) saveColorsForTheme(show bool) *ColorOverride {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return nil
	}
	payload := a.getColorsFromControls()
	if payload == nil {
		a.showMessage("Info", "Enable recoloring and select at least one color.", gtk.MESSAGE_INFO)
		return nil
	}
	data := loadColors()
	if data.Themes == nil {
		data.Themes = map[string]ColorOverride{}
	}
	data.Themes[resolvePath(th.Path)] = *payload
	saveColors(data)
	updated, _ := applyColorPatch(readText(th.Path), payload.Colors)
	if err := writeOriginalThemeConfig(th.Path, updated, "colors"); err != nil {
		a.showMessage("Original file was not updated", err.Error(), gtk.MESSAGE_ERROR)
		return nil
	}
	a.colorStatus.SetText(fmt.Sprintf("Saved colors for %s (%d slots)", filepath.Base(filepath.Dir(th.Path)), len(payload.Colors)))
	if show {
		a.pushStatus("Colors saved for " + filepath.Base(filepath.Dir(th.Path)) + ".")
	}
	return payload
}

func (a *App) resetColorsForTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	data := loadColors()
	delete(data.Themes, resolvePath(th.Path))
	saveColors(data)
	a.loadColorsForSelected()
	a.pushStatus("Colors reset for " + filepath.Base(filepath.Dir(th.Path)) + ".")
}

func (a *App) applyColorsAndRestart() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	colors := a.saveColorsForTheme(false)
	if colors == nil {
		return
	}
	var pos *Position
	if a.posEnabled.GetActive() {
		pos = a.getPositionFromControls()
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	a.setBusy(true)
	a.pushStatus("Applying colors for " + filepath.Base(filepath.Dir(th.Path)) + "...")
	path := th.Path
	go func() {
		defer idle(func() { a.setBusy(false) })
		stopManagedTheme(path)
		res, err := runConky(path, pos, colors)
		if err != nil {
			idle(func() { a.showMessage("Color Error", err.Error(), gtk.MESSAGE_ERROR) })
			return
		}
		idle(func() {
			a.updateRuntime(true, filepath.Base(path)+" ["+res.Mode+"] with custom colors")
			a.pushStatus("Colors applied for " + filepath.Base(filepath.Dir(path)) + ".")
		})
	}()
}

func (a *App) updateRuntime(running bool, detail string) {
	if a.statusIndicator == nil {
		return
	}
	if running {
		a.statusIndicator.SetMarkup("<span size='large' foreground='#10b981'>●</span> Running")
		if detail == "" {
			detail = "Managed Conky is active"
		}
		a.runtimeInfo.SetText(detail)
	} else {
		a.statusIndicator.SetMarkup("<span size='large' foreground='#64748b'>●</span> Idle")
		a.runtimeInfo.SetText("No managed Conky instance running")
	}
}

type themeRunOverride struct {
	pos *Position
	col *ColorOverride
}

func (a *App) runThemesWorker(themes []string, overlay map[string]themeRunOverride) {
	settings := loadSettings()
	maxN := settings.MaxInstances
	if maxN < 1 {
		maxN = 1
	}
	runList := themes
	if len(runList) > maxN {
		runList = runList[:maxN]
	}
	started, fallback := 0, 0
	for _, theme := range runList {
		stopManagedTheme(theme)
		pos := resolveThemePosition(theme, nil)
		col := resolveThemeColors(theme)
		if ov, ok := overlay[resolvePath(theme)]; ok {
			if ov.pos != nil {
				pos = ov.pos
			}
			if ov.col != nil {
				col = ov.col
			}
		}
		res, err := runConky(theme, pos, col)
		if err != nil {
			t, e := theme, err
			idle(func() { a.appendLog(fmt.Sprintf("Failed: %s (%v)", t, e)) })
			continue
		}
		started++
		if res.Mode != "full-visual" {
			fallback++
		}
		name := filepath.Base(theme)
		mode := res.Mode
		pid := res.PID
		idle(func() { a.appendLog(fmt.Sprintf("Started: %s (pid=%d, mode=%s)", name, pid, mode)) })
	}
	s, f, lim := started, fallback, len(runList)
	idle(func() {
		a.setBusy(false)
		a.pushStatus(fmt.Sprintf("Started %d theme(s). Fallback: %d. Limited to %d.", s, f, lim))
		a.applyFilter("")
		a.refreshLogView(250)
	})
}

func (a *App) runThemes(themes []string) {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	overlay := map[string]themeRunOverride{}
	if a.activePosTheme != "" {
		ov := themeRunOverride{}
		if a.posEnabled != nil && a.posEnabled.GetActive() {
			ov.pos = a.getPositionFromControls()
			if a.settings.RememberPosition {
				a.savePositionForTheme(false)
			}
		}
		if a.colorEnabled != nil && a.colorEnabled.GetActive() {
			ov.col = a.getColorsFromControls()
			if ov.col != nil && a.settings.RememberColors {
				a.saveColorsForTheme(false)
			}
		}
		overlay[resolvePath(a.activePosTheme)] = ov
	}
	a.setBusy(true)
	a.pushStatus("Starting themes...")
	go a.runThemesWorker(themes, overlay)
}

func (a *App) runSelected() {
	th := a.selectedThemePaths()
	if len(th) == 0 {
		a.showMessage("Info", "Select at least one theme.", gtk.MESSAGE_INFO)
		return
	}
	a.runThemes(th)
}

func (a *App) runAll() {
	if len(a.themes) == 0 {
		a.showMessage("Info", "No valid themes found.", gtk.MESSAGE_INFO)
		return
	}
	var paths []string
	for _, t := range a.themes {
		paths = append(paths, t.Path)
	}
	a.runThemes(paths)
}

func (a *App) smartRepair() {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	a.setBusy(true)
	a.pushStatus("Running smart repair...")
	themes := a.themes
	go func() {
		ok, fail := 0, 0
		for _, t := range themes {
			if _, err := optimizeForDesktop(t.Path); err != nil {
				fail++
				p, e := t.Path, err
				idle(func() { a.appendLog(fmt.Sprintf("Repair failed: %s (%v)", p, e)) })
			} else {
				ok++
			}
		}
		o, f := ok, fail
		idle(func() {
			a.setBusy(false)
			a.pushStatus(fmt.Sprintf("Smart repair complete. Success: %d, Failed: %d.", o, f))
			a.showMessage("Repair", fmt.Sprintf("Prepared full launch bundles for %d theme(s), failed %d.\nAsset paths, fonts, and visuals are relinked automatically.", o, f), gtk.MESSAGE_INFO)
		})
	}()
}

func (a *App) stopAll() {
	n := stopAllManaged()
	a.applyFilter("")
	a.pushStatus(fmt.Sprintf("Stopped %d managed Conky process(es).", n))
}

func (a *App) stopSelected() {
	paths := a.selectedThemePaths()
	if len(paths) == 0 {
		a.showMessage("Info", "Select at least one theme.", gtk.MESSAGE_INFO)
		return
	}
	n := 0
	for _, p := range paths {
		if stopManagedTheme(p) {
			n++
		}
	}
	a.applyFilter("")
	a.pushStatus(fmt.Sprintf("Stopped %d selected theme(s).", n))
}

func (a *App) restartSelected() {
	paths := a.selectedThemePaths()
	if len(paths) == 0 {
		a.showMessage("Info", "Select at least one theme.", gtk.MESSAGE_INFO)
		return
	}
	a.runThemes(paths)
}

func (a *App) killOrphans() {
	n := killOrphanedConky()
	a.applyFilter("")
	a.pushStatus(fmt.Sprintf("Removed %d orphaned manager process(es).", n))
}

func (a *App) stopPreview() {
	if a.previewID != 0 {
		glib.SourceRemove(a.previewID)
		a.previewID = 0
	}
	r := pruneRuntime()
	for _, p := range r.Processes {
		if p.Preview {
			stopPID(p.PID, true)
			stopManagedTheme(p.ConfigPath)
		}
	}
	a.previewPaused = false
	a.applyFilter("")
	a.pushStatus("Preview stopped.")
}

func (a *App) restartPreview() {
	a.stopPreview()
	a.previewSelected()
}

func (a *App) togglePreviewPause() {
	r := pruneRuntime()
	found := false
	for _, p := range r.Processes {
		if !p.Preview {
			continue
		}
		found = true
		if a.previewPaused {
			_ = resumePID(p.PID)
		} else {
			_ = pausePID(p.PID)
		}
	}
	if !found {
		a.showMessage("Info", "No preview process is running.", gtk.MESSAGE_INFO)
		return
	}
	a.previewPaused = !a.previewPaused
	if a.previewPaused {
		a.pushStatus("Preview paused (SIGSTOP).")
	} else {
		a.pushStatus("Preview resumed.")
	}
}

func (a *App) previewSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	seconds := a.previewSeconds
	if seconds < 0 {
		seconds = loadSettings().PreviewSeconds
	}
	pos := a.getPositionFromControls()
	colors := a.getColorsFromControls()
	a.stopPreview()
	stopManagedTheme(th.Path)
	res, err := runConky(th.Path, pos, colors)
	if err != nil {
		a.showMessage("Preview Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	markProcessPreview(res.PID, true)
	posText := ""
	if pos != nil {
		posText = fmt.Sprintf(" @ %s X=%d Y=%d", pos.Alignment, pos.GapX, pos.GapY)
	}
	dur := "unlimited"
	if seconds > 0 {
		dur = fmt.Sprintf("%ds", seconds)
	}
	a.updateRuntime(true, fmt.Sprintf("Preview: %s [%s]%s", filepath.Base(th.Path), res.Mode, posText))
	a.pushStatus(fmt.Sprintf("Preview started (%s): %s [%s]", dur, filepath.Base(th.Path), res.Mode))
	a.applyFilter("")
	if a.previewID != 0 {
		glib.SourceRemove(a.previewID)
		a.previewID = 0
	}
	if seconds > 0 {
		pid := res.PID
		a.previewID = glib.TimeoutSecondsAdd(uint(seconds), func() bool {
			stopPID(pid, true)
			stopManagedTheme(th.Path)
			a.applyFilter("")
			a.pushStatus("Preview stopped.")
			a.previewID = 0
			return false
		})
	}
}

func (a *App) editSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	dlg := gtk.MessageDialogNew(a.win, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE, "Edit theme")
	dlg.FormatSecondaryText("Open %s in the built-in editor or an external application.", filepath.Base(th.Path))
	dlg.AddButton("External editor", gtk.RESPONSE_NO)
	dlg.AddButton("Built-in editor", gtk.RESPONSE_YES)
	resp := dlg.Run()
	dlg.Destroy()
	if resp == gtk.RESPONSE_YES {
		a.showEmbeddedEditor(th.Path)
		return
	}
	if resp == gtk.RESPONSE_NO {
		xdgOpen(th.Path)
	}
}

func (a *App) openSelectedFolder() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	xdgOpen(filepath.Dir(th.Path))
}

func (a *App) importThemeFolder() {
	dlg, _ := gtk.FileChooserDialogNewWith2Buttons("Import Theme Folder", a.win, gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER, "Cancel", gtk.RESPONSE_CANCEL, "Import", gtk.RESPONSE_OK)
	dlg.SetCurrentFolder(homeDir)
	resp := dlg.Run()
	folder := dlg.GetFilename()
	dlg.Destroy()
	if resp != gtk.RESPONSE_OK || folder == "" {
		return
	}
	a.performImport(func() ([]string, error) { return importFolderToConky(folder, "") })
}

func (a *App) importThemeArchive() {
	dlg, _ := gtk.FileChooserDialogNewWith2Buttons("Import Theme Archive", a.win, gtk.FILE_CHOOSER_ACTION_OPEN, "Cancel", gtk.RESPONSE_CANCEL, "Import", gtk.RESPONSE_OK)
	dlg.SetCurrentFolder(homeDir)
	filt, _ := gtk.FileFilterNew()
	filt.SetName("Archives")
	for _, ext := range archiveExts {
		filt.AddPattern("*" + ext)
	}
	dlg.AddFilter(filt)
	all, _ := gtk.FileFilterNew()
	all.SetName("All files")
	all.AddPattern("*")
	dlg.AddFilter(all)
	resp := dlg.Run()
	archive := dlg.GetFilename()
	dlg.Destroy()
	if resp != gtk.RESPONSE_OK || archive == "" {
		return
	}
	a.performImport(func() ([]string, error) { return importArchiveToConky(archive, "") })
}

func (a *App) performImport(fn func() ([]string, error)) {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	a.setBusy(true)
	a.pushStatus("Importing into " + defaultImportDir + "…")
	go func() {
		installed, err := fn()
		idle(func() {
			a.setBusy(false)
			if err != nil {
				a.showMessage("Import Error", err.Error(), gtk.MESSAGE_ERROR)
				return
			}
			a.loadThemes()
			a.pushStatus(fmt.Sprintf("Imported %d item(s) into ~/.conky", len(installed)))
			a.showMessage("Import Complete", fmt.Sprintf("Installed into:\n%s\n\nFolders: %s", defaultImportDir, strings.Join(installed, ", ")), gtk.MESSAGE_INFO)
		})
	}()
}

func (a *App) showThemeStore() {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Theme Marketplace")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(920, 620)
	dlg.SetResizable(true)
	content, _ := dlg.GetContentArea()
	content.SetSpacing(10)
	setMargins(content, 10)
	header := newMarkupLabel("<b>Conky Theme Marketplace</b>\n<span foreground='#888888'>Search real store catalogs, inspect metadata, and install directly into ~/.conky.</span>")
	header.SetLineWrap(true)
	content.PackStart(header, false, false, 0)

	controls, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	storeCombo, _ := gtk.ComboBoxTextNew()
	for _, store := range onlineStores {
		storeCombo.Append(store.ID, store.Label)
	}
	storeCombo.SetActiveID(onlineStores[0].ID)
	sortCombo, _ := gtk.ComboBoxTextNew()
	sortCombo.Append("downloads", "Most downloaded")
	sortCombo.Append("latest", "Recently updated")
	sortCombo.Append("rating", "Best rated")
	sortCombo.SetActiveID("downloads")
	catCombo, _ := gtk.ComboBoxTextNew()
	for _, p := range [][2]string{
		{"", "All categories"}, {"minimal", "Minimal"}, {"system monitor", "System Monitor"},
		{"weather", "Weather"}, {"music", "Music"}, {"network", "Network"},
		{"cpu gpu", "CPU/GPU"}, {"dark", "Dark"}, {"light", "Light"},
	} {
		catCombo.Append(p[0], p[1])
	}
	catCombo.SetActiveID("")
	search, _ := gtk.SearchEntryNew()
	search.SetPlaceholderText("Search themes, authors, or keywords…")
	search.SetTooltipText("Search is performed in the selected store catalog")
	controls.PackStart(storeCombo, false, false, 0)
	controls.PackStart(sortCombo, false, false, 0)
	controls.PackStart(catCombo, false, false, 0)
	controls.PackStart(search, true, true, 0)
	content.PackStart(controls, false, false, 0)

	status := newLabel("Ready. Select a store or search for a theme.", true)
	content.PackStart(status, false, false, 0)
	paned, _ := gtk.PanedNew(gtk.ORIENTATION_HORIZONTAL)
	paned.SetWideHandle(true)
	listScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	listScroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	list, _ := gtk.ListBoxNew()
	list.SetSelectionMode(gtk.SELECTION_SINGLE)
	listScroll.Add(list)
	paned.Pack1(listScroll, true, true)

	details, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	details.SetSizeRequest(0, -1)
	title := newMarkupLabel("<b>Select a theme</b>")
	title.SetLineWrap(true)
	details.PackStart(title, false, false, 0)
	previewImg, _ := gtk.ImageNewFromIconName("image-x-generic", gtk.ICON_SIZE_DIALOG)
	previewImg.SetSizeRequest(320, 180)
	details.PackStart(previewImg, false, false, 0)
	desc := newLabel("Choose a result to inspect its description, author, version, and store page.", true)
	desc.SetLineWrap(true)
	details.PackStart(desc, false, false, 0)
	meta := newLabel("", true)
	meta.SetLineWrap(true)
	details.PackStart(meta, false, false, 0)
	note := newLabel("Installation is performed only after selecting Download & Install. Files are placed in ~/.conky and can be scanned immediately.", true)
	note.SetLineWrap(true)
	details.PackStart(note, false, false, 0)
	web, _ := gtk.LinkButtonNewWithLabel(onlineStores[0].BrowseURL, "Open store in browser")
	web.SetHAlign(gtk.ALIGN_START)
	details.PackStart(web, false, false, 0)
	paned.Pack2(details, true, true)
	paned.SetPosition(450)
	content.PackStart(paned, true, true, 0)

	loadMore, _ := gtk.ButtonNewWithLabel("Load more results")
	fitButton(loadMore)
	content.PackStart(loadMore, false, false, 0)

	var selected *OnlineProduct
	var selectedStore = onlineStores[0]
	var products []OnlineProduct
	rows := map[uintptr]OnlineProduct{}
	page, total, requestID := 1, 0, 0
	loading := false

	currentStore := func() OnlineStore { return getOnlineStore(comboID(storeCombo, onlineStores[0].ID)) }
	showProduct := func(product *OnlineProduct) {
		selected = product
		selectedStore = currentStore()
		if product == nil {
			title.SetMarkup("<b>Select a theme</b>")
			desc.SetText("Choose a result to inspect its description, author, version, and store page.")
			meta.SetText("")
			previewImg.SetFromIconName("image-x-generic", gtk.ICON_SIZE_DIALOG)
			web.SetUri(selectedStore.BrowseURL)
			web.SetLabel("Open " + selectedStore.Label + " in browser")
			return
		}
		title.SetMarkup("<b>" + markupEscape(product.Name) + "</b>")
		summary := product.Summary
		if summary == "" {
			summary = "No description was supplied by the store."
		}
		desc.SetText(summary)
		version := product.Version
		if version == "" {
			version = "not specified"
		}
		meta.SetText(fmt.Sprintf("Author: %s\nDownloads: %d\nRating: %.1f\nVersion: %s\nStore: %s", product.Author, product.Downloads, product.Score, version, selectedStore.Label))
		web.SetUri(product.PageURL)
		web.SetLabel("View details on " + selectedStore.Label)
		previewImg.SetFromIconName("image-x-generic", gtk.ICON_SIZE_DIALOG)
		if product.PreviewURL != "" {
			purl := product.PreviewURL
			go func() {
				pb := loadRemotePixbuf(purl, 320, 180)
				idle(func() {
					if selected == nil || selected.PreviewURL != purl || pb == nil {
						return
					}
					previewImg.SetFromPixbuf(pb)
				})
			}()
		}
	}

	render := func(appendRows bool) {
		if !appendRows {
			clearListBox(list)
			rows = map[uintptr]OnlineProduct{}
		}
		start := 0
		if appendRows {
			start = len(rows)
		}
		var first *gtk.ListBoxRow
		for i, product := range products {
			if i < start {
				continue
			}
			row, _ := gtk.ListBoxRowNew()
			box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 3)
			setMargins(box, 9)
			box.PackStart(newMarkupLabel("<b>"+markupEscape(product.Name)+"</b>"), false, false, 0)
			author := product.Author
			if author == "" {
				author = "Unknown author"
			}
			box.PackStart(newLabel(fmt.Sprintf("%s  ·  %d downloads  ·  %.1f rating", author, product.Downloads, product.Score), true), false, false, 0)
			row.Add(box)
			rows[row.Native()] = product
			list.Add(row)
			if first == nil && !appendRows {
				first = row
			}
		}
		list.ShowAll()
		if first != nil {
			list.SelectRow(first)
		}
		if total > 0 {
			status.SetText(fmt.Sprintf("Showing %d of %d results · %s", len(products), total, selectedStore.Label))
		} else {
			status.SetText(fmt.Sprintf("Showing %d results · %s", len(products), selectedStore.Label))
		}
		loadMore.SetSensitive(!loading && total > len(products))
	}

	loadPage := func(reset bool) {}
	loadPage = func(reset bool) {
		if loading {
			return
		}
		if reset {
			page = 1
			products = nil
			selected = nil
			showProduct(nil)
		}
		loading = true
		requestID++
		myRequest := requestID
		if reset {
			status.SetText("Searching " + currentStore().Label + "…")
		} else {
			status.SetText("Loading more results…")
		}
		loadMore.SetSensitive(false)
		store := currentStore()
		selectedStore = store
		query, _ := search.GetText()
		if extra := strings.TrimSpace(catCombo.GetActiveID()); extra != "" && !strings.Contains(strings.ToLower(query), extra) {
			query = strings.TrimSpace(query + " " + extra)
		}
		sortID := comboID(sortCombo, "downloads")
		pg := page
		go func() {
			found, count, err := browseOnlineStore(store, query, pg, 24, sortID)
			idle(func() {
				if myRequest != requestID {
					return
				}
				loading = false
				if err != nil {
					status.SetText("Store unavailable: " + err.Error())
					loadMore.SetSensitive(false)
					return
				}
				if reset {
					products = found
				} else {
					products = append(products, found...)
				}
				total = count
				render(!reset)
			})
		}()
	}

	storeCombo.Connect("changed", func() { loadPage(true) })
	sortCombo.Connect("changed", func() { loadPage(true) })
	catCombo.Connect("changed", func() { loadPage(true) })
	search.Connect("activate", func() { loadPage(true) })
	search.Connect("search-changed", func() {
		// Do not issue a request for every keystroke; pressing Enter searches immediately.
		status.SetText("Press Enter to search " + currentStore().Label + ".")
	})
	list.Connect("row-selected", func() {
		row := list.GetSelectedRow()
		if row == nil {
			showProduct(nil)
			return
		}
		product := rows[row.Native()]
		showProduct(&product)
	})
	loadMore.Connect("clicked", func() { page++; loadPage(false) })
	loadPage(true)

	dlg.AddButton("Close", gtk.RESPONSE_CLOSE)
	dlg.AddButton("Download & Install", gtk.RESPONSE_OK)
	dlg.ShowAll()
	response := dlg.Run()
	dlg.Destroy()
	if response != gtk.RESPONSE_OK || selected == nil {
		return
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	product := *selected
	store := selectedStore
	a.downloadWithProgress(product.Name, func(cb progressFn) ([]string, error) {
		return downloadAndInstallOnlineProduct(store, product.ProductID, cb)
	})
}

func (a *App) downloadWithProgress(title string, worker func(progressFn) ([]string, error)) {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Downloading Theme")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(460, 150)
	dlg.SetDeletable(false)
	box, _ := dlg.GetContentArea()
	box.SetSpacing(10)
	setMargins(box, 15)
	tl := newMarkupLabel("<b>" + markupEscape(title) + "</b>")
	box.PackStart(tl, false, false, 0)
	bar, _ := gtk.ProgressBarNew()
	bar.SetShowText(true)
	box.PackStart(bar, false, false, 0)
	st := newLabel("Connecting…", true)
	box.PackStart(st, false, false, 0)
	cancelBtn, _ := gtk.ButtonNewWithLabel("Cancel")
	fitButton(cancelBtn)
	box.PackStart(cancelBtn, false, false, 0)
	dlg.ShowAll()

	type resT struct {
		installed []string
		err       error
		done      bool
		canceled  bool
	}
	res := &resT{}
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn.Connect("clicked", func() {
		res.canceled = true
		cancel()
		st.SetText("Canceling…")
	})
	cb := func(f float64, msg string) {
		if ctx.Err() != nil {
			return
		}
		idle(func() {
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			bar.SetFraction(f)
			bar.SetText(fmt.Sprintf("%d%%", int(f*100)))
			st.SetText(msg)
		})
	}
	a.setBusy(true)
	go func() {
		inst, err := worker(cb)
		if ctx.Err() != nil {
			res.err = fmt.Errorf("download canceled")
			res.canceled = true
		} else {
			res.installed, res.err = inst, err
		}
		res.done = true
	}()
	glib.TimeoutAdd(150, func() bool {
		if !res.done {
			return true
		}
		dlg.Destroy()
		a.setBusy(false)
		if res.canceled {
			a.pushStatus("Download canceled.")
			return false
		}
		if res.err != nil {
			retry := gtk.MessageDialogNew(a.win, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_YES_NO, "Download Error")
			retry.FormatSecondaryText("%s\n\nRetry this download?", res.err.Error())
			r := retry.Run()
			retry.Destroy()
			if r == gtk.RESPONSE_YES {
				a.downloadWithProgress(title, worker)
			}
			return false
		}
		a.loadThemes()
		a.pushStatus("Installed theme: " + title)
		a.showMessage("Theme Installed", fmt.Sprintf("%s was installed into ~/.conky\n\nFolders: %s", title, strings.Join(res.installed, ", ")), gtk.MESSAGE_INFO)
		return false
	})
	dlg.Run()
}

func clearListBox(lb *gtk.ListBox) {
	removeChildren(&lb.Container)
}

func (a *App) refreshProfiles() {
	a.profiles = loadProfiles()
	clearListBox(a.profileList)
	a.profileRowNames = map[uintptr]string{}
	var names []string
	seen := map[string]bool{}
	for n := range a.profiles.Profiles {
		seen[n] = true
		names = append(names, n)
	}
	for n := range a.profiles.Desktop {
		if !seen[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		row, _ := gtk.ListBoxRowNew()
		bx, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
		bx.SetMarginTop(5)
		bx.SetMarginBottom(5)
		bx.SetMarginStart(10)
		bx.SetMarginEnd(10)
		bx.PackStart(newMarkupLabel("<b>"+markupEscape(name)+"</b>"), false, false, 0)
		bx.PackStart(newLabel(fmt.Sprintf("%d theme(s)", len(a.profiles.Profiles[name])), true), false, false, 0)
		row.Add(bx)
		a.profileRowNames[row.Native()] = name
		a.profileList.Add(row)
	}
	a.profileList.ShowAll()
	a.showProfileDetails()
}

func (a *App) onProfileSelected() {
	row := a.profileList.GetSelectedRow()
	if row != nil {
		if n, ok := a.profileRowNames[row.Native()]; ok {
			a.profileName.SetText(n)
		}
		a.showProfileDetails()
	}
}

func (a *App) selectedProfileName() string {
	row := a.profileList.GetSelectedRow()
	if row != nil {
		if n, ok := a.profileRowNames[row.Native()]; ok {
			return n
		}
	}
	t, _ := a.profileName.GetText()
	return strings.TrimSpace(t)
}

func (a *App) saveProfile() {
	name, _ := a.profileName.GetText()
	name = strings.TrimSpace(name)
	if name == "" {
		a.showMessage("Error", "Enter a profile name.", gtk.MESSAGE_ERROR)
		return
	}
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Error", "Select at least one theme in the sidebar.", gtk.MESSAGE_ERROR)
		return
	}
	a.profiles.Profiles[name] = themes
	if a.profiles.Desktop == nil {
		a.profiles.Desktop = map[string]DesktopProfile{}
	}
	a.profiles.Desktop[name] = captureDesktopProfile(name, themes)
	a.profiles.LastProfile = name
	saveProfiles(a.profiles)
	a.refreshProfiles()
	a.pushStatus("Saved profile: " + name)
}

func (a *App) addToProfile() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select or enter a profile name.", gtk.MESSAGE_ERROR)
		return
	}
	if _, ok := a.profiles.Profiles[name]; !ok {
		a.showMessage("Error", "Profile not found: "+name, gtk.MESSAGE_ERROR)
		return
	}
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Error", "Select themes in the sidebar first.", gtk.MESSAGE_ERROR)
		return
	}
	cur := map[string]bool{}
	for _, p := range a.profiles.Profiles[name] {
		cur[p] = true
	}
	for _, t := range themes {
		cur[t] = true
	}
	var out []string
	for p := range cur {
		out = append(out, p)
	}
	sort.Strings(out)
	a.profiles.Profiles[name] = out
	if a.profiles.Desktop == nil {
		a.profiles.Desktop = map[string]DesktopProfile{}
	}
	a.profiles.Desktop[name] = captureDesktopProfile(name, out)
	saveProfiles(a.profiles)
	a.refreshProfiles()
	a.pushStatus(fmt.Sprintf("Updated profile: %s (+%d)", name, len(themes)))
}

func (a *App) showProfileDetails() {
	clearListBox(a.profileDetails)
	name := a.selectedProfileName()
	if name == "" {
		a.profileDetailsTitle.SetText("No profile selected")
		return
	}
	items := a.profiles.Profiles[name]
	extra := ""
	if dp, ok := a.profiles.Desktop[name]; ok && len(dp.Themes) > 0 {
		items = nil
		for _, t := range dp.Themes {
			items = append(items, t.Path)
			mon := t.Monitor
			if mon == "" {
				mon = t.Position.Monitor
			}
			if mon == "" {
				mon = "primary"
			}
			extraRow := fmt.Sprintf("%s\n  monitor=%s  %s  gap=%d,%d  enabled=%v", t.Path, mon, t.Position.Alignment, t.Position.GapX, t.Position.GapY, t.Enabled)
			row, _ := gtk.ListBoxRowNew()
			l, _ := gtk.LabelNew(extraRow)
			l.SetXAlign(0)
			l.SetMarginTop(4)
			l.SetMarginBottom(4)
			l.SetMarginStart(10)
			l.SetLineWrap(true)
			row.Add(l)
			a.profileDetails.Add(row)
		}
		a.profileDetailsTitle.SetText(fmt.Sprintf("%s — %d theme(s) with per-theme desktop settings", name, len(dp.Themes)))
		a.profileDetails.ShowAll()
		return
	}
	a.profileDetailsTitle.SetText(fmt.Sprintf("%s — %d theme(s)%s", name, len(items), extra))
	for _, p := range items {
		row, _ := gtk.ListBoxRowNew()
		l, _ := gtk.LabelNew(p)
		l.SetXAlign(0)
		l.SetMarginTop(4)
		l.SetMarginBottom(4)
		l.SetMarginStart(10)
		row.Add(l)
		a.profileDetails.Add(row)
	}
	a.profileDetails.ShowAll()
}

func (a *App) deleteProfile() {
	name := a.selectedProfileName()
	if name == "" {
		return
	}
	confirm := gtk.MessageDialogNew(a.win, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO, "Delete Profile")
	confirm.FormatSecondaryText("Delete profile '%s'?", name)
	if confirm.Run() != gtk.RESPONSE_YES {
		confirm.Destroy()
		return
	}
	confirm.Destroy()
	delete(a.profiles.Profiles, name)
	delete(a.profiles.Desktop, name)
	saveProfiles(a.profiles)
	a.refreshProfiles()
}

func (a *App) runProfile() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select a profile.", gtk.MESSAGE_ERROR)
		return
	}
	themePaths := a.profiles.Profiles[name]
	if themePaths == nil {
		a.showMessage("Error", "Profile not found: "+name, gtk.MESSAGE_ERROR)
		return
	}
	var themes []string
	for _, p := range themePaths {
		if isFile(p) {
			themes = append(themes, p)
		}
	}
	if len(themes) == 0 {
		a.showMessage("Error", "No existing themes in this profile.", gtk.MESSAGE_ERROR)
		return
	}
	a.profiles.LastProfile = name
	saveProfiles(a.profiles)
	if dp, ok := a.profiles.Desktop[name]; ok && len(dp.Themes) > 0 {
		go func() {
			_ = runProfileCLI(name)
			idle(func() { a.applyFilter("") })
		}()
		return
	}
	a.runThemes(themes)
}

func (a *App) enableAutostart() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select a profile first.", gtk.MESSAGE_ERROR)
		return
	}
	if _, ok := a.profiles.Profiles[name]; !ok {
		if _, ok2 := a.profiles.Desktop[name]; !ok2 {
			a.showMessage("Error", "Profile not found: "+name, gtk.MESSAGE_ERROR)
			return
		}
	}
	_ = os.MkdirAll(autostartDir, 0o755)
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}
	desktop := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + appName,
		"Comment=Start Conky profile on login",
		fmt.Sprintf("Exec=%q --run-profile %q", execPath, name),
		"Terminal=false",
		"Hidden=false",
		"X-GNOME-Autostart-enabled=true",
		"X-GNOME-Autostart-Delay=3",
		"X-KDE-autostart-after=panel",
		"X-KDE-StartupNotify=false",
		"StartupNotify=false",
		"",
	}, "\n")
	if err := os.WriteFile(autostartFile, []byte(desktop), 0o644); err != nil {
		a.showMessage("Error", "Failed to enable autostart: "+err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	if fileExists(oldAutostartFile) && oldAutostartFile != autostartFile {
		_ = os.Remove(oldAutostartFile)
	}
	a.showMessage("Autostart", fmt.Sprintf("Enabled XDG autostart for profile: %s\n\nDesktop: %s\nThe .desktop file has no OnlyShowIn filter so GNOME, KDE, XFCE, Cinnamon, MATE, LXQt, Hyprland, Niri, Sway, and COSMIC can all load it when they honor ~/.config/autostart.", name, compositorLabel(a.desktopEnv, a.session)), gtk.MESSAGE_INFO)
}

func (a *App) disableAutostart() {
	for _, p := range []string{autostartFile, oldAutostartFile} {
		_ = os.Remove(p)
	}
	a.showMessage("Autostart", "Autostart disabled.", gtk.MESSAGE_INFO)
}

func (a *App) validateSelected() {
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	cfg := themes[0]
	bundle, err := createThemeLaunchBundle(cfg, true, nil, nil)
	if err != nil {
		a.showMessage("Validation Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	okLaunch := validateConkyConfig(bundle.LaunchConfig, bundle.LaunchDir, buildLaunchEnv(bundle))
	found, missing := scanThemeAssets(cfg)
	miss := "None"
	if len(missing) > 0 {
		n := len(missing)
		if n > 8 {
			n = 8
		}
		miss = strings.Join(missing[:n], "\n")
		if len(missing) > 8 {
			miss += fmt.Sprintf("\n… and %d more", len(missing)-8)
		}
	}
	okTxt := "FAIL"
	if okLaunch {
		okTxt = "OK"
	}
	a.showMessage("Validation", fmt.Sprintf("Theme: %s\nLaunch bundle: %s\nPath fixes: %d\nAssets found: %d\nMissing assets:\n%s", filepath.Base(cfg), okTxt, bundle.PathFixes, len(found), miss), gtk.MESSAGE_INFO)
}

func (a *App) refreshLogView(lines int) {
	b, err := os.ReadFile(logFilePath)
	text := "(Log file not available yet.)"
	if err == nil {
		all := strings.Split(string(b), "\n")
		if len(all) > lines {
			all = all[len(all)-lines:]
		}
		text = strings.Join(all, "\n")
	}
	a.logsBuffer.SetText(text)
}

func (a *App) appendLog(line string) {
	end := a.logsBuffer.GetEndIter()
	a.logsBuffer.Insert(end, line+"\n")
}

func (a *App) healthScan() {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", gtk.MESSAGE_INFO)
		return
	}
	a.setBusy(true)
	a.pushStatus("Health scan running...")
	themes := a.themes
	go func() {
		ok, warn, bad := 0, 0, 0
		status := map[string]string{}
		health := map[string]ThemeHealth{}
		for _, item := range themes {
			label := item.Label()
			idle(func() { a.pushStatus("Checking: " + label) })
			h := inspectThemeHealth(item.Path)
			health[label] = h
			status[label] = h.Status
			switch h.Status {
			case "broken":
				bad++
			case "needs_fix":
				warn++
			default:
				ok++
			}
		}
		o, w, b := ok, warn, bad
		idle(func() {
			a.themeStatus = status
			a.themeHealth = health
			a.setBusy(false)
			a.applyFilter("")
			a.pushStatus(fmt.Sprintf("Health scan complete. Stable: %d, Needs fix: %d, Broken: %d.", o, w, b))
		})
	}()
}

func (a *App) validateAllThemes() {
	a.healthScan()
}

func (a *App) fixAllThemes() {
	if len(a.themes) == 0 {
		return
	}
	var preview []string
	for _, t := range a.themes {
		h := a.themeHealth[t.Label()]
		if h.Status == "" {
			h = inspectThemeHealth(t.Path)
		}
		if h.Status == "stable" {
			continue
		}
		preview = append(preview, "• "+t.Label()+" ["+h.Status+"]")
		for _, iss := range h.Issues {
			if iss.Fix != "" {
				preview = append(preview, "    fix: "+iss.Fix)
			}
		}
	}
	if len(preview) == 0 {
		a.showMessage("Fix All", "No repairable issues were found. Run Scan first for detailed results.", gtk.MESSAGE_INFO)
		return
	}
	msg := "The following launch bundles will be rebuilt. Original theme files stay untouched unless a saved color/position override is applied.\n\n" + strings.Join(preview, "\n")
	if len(msg) > 4000 {
		msg = msg[:4000] + "\n…"
	}
	if !confirmDialog(a.win, "Fix All Themes", msg) {
		return
	}
	a.smartRepair()
}

func (a *App) toggleFavoriteSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	on := !isFavorite(th.Path)
	setFavorite(th.Path, on)
	a.applyFilter("")
	if on {
		a.pushStatus("Added to favorites.")
	} else {
		a.pushStatus("Removed from favorites.")
	}
}

func (a *App) cloneSelectedTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	name, ok := promptText(a.win, "Clone Theme", "New folder name:", filepath.Base(resolveThemeRoot(th.Path))+"-copy")
	if !ok || name == "" {
		return
	}
	newCfg, err := cloneTheme(th.Path, name)
	if err != nil {
		a.showMessage("Clone Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	a.loadThemes()
	a.selectedThemeSet[resolvePath(newCfg)] = true
	a.applyFilter("")
	a.pushStatus("Cloned theme to " + filepath.Dir(newCfg))
}

func (a *App) renameSelectedTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	old := resolveThemeRoot(th.Path)
	name, ok := promptText(a.win, "Rename Theme", "New folder name:", filepath.Base(old))
	if !ok || name == "" {
		return
	}
	newCfg, err := renameTheme(th.Path, name)
	if err != nil {
		a.showMessage("Rename Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	delete(a.selectedThemeSet, resolvePath(th.Path))
	a.selectedThemeSet[resolvePath(newCfg)] = true
	a.profiles = loadProfiles()
	a.loadThemes()
	a.refreshProfiles()
	a.pushStatus("Renamed theme to " + filepath.Base(resolveThemeRoot(newCfg)))
}

func (a *App) deleteSelectedTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	root := resolveThemeRoot(th.Path)
	if !confirmDialog(a.win, "Move to Trash", "Move this theme folder to the trash?\n\n"+root+"\n\nProfiles and saved overrides for this path will be cleaned up.") {
		return
	}
	stopManagedTheme(th.Path)
	if err := trashPath(root); err != nil {
		a.showMessage("Delete Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	id := resolvePath(th.Path)
	pos := loadPositions()
	delete(pos.Themes, id)
	savePositions(pos)
	cols := loadColors()
	delete(cols.Themes, id)
	saveColors(cols)
	lib := loadLibrary()
	delete(lib.Favorites, id)
	saveLibrary(lib)
	delete(a.selectedThemeSet, id)
	a.loadThemes()
	a.pushStatus("Moved theme to trash.")
}

func (a *App) exportSelectedTheme() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	dlg, _ := gtk.FileChooserDialogNewWith2Buttons("Export Theme", a.win, gtk.FILE_CHOOSER_ACTION_SAVE, "Cancel", gtk.RESPONSE_CANCEL, "Export", gtk.RESPONSE_OK)
	dlg.SetCurrentName(filepath.Base(resolveThemeRoot(th.Path)) + ".conky-theme.tar.gz")
	dlg.SetCurrentFolder(homeDir)
	resp := dlg.Run()
	dest := dlg.GetFilename()
	dlg.Destroy()
	if resp != gtk.RESPONSE_OK || dest == "" {
		return
	}
	if err := exportThemeArchive(th.Path, dest); err != nil {
		a.showMessage("Export Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	a.pushStatus("Exported " + dest)
}

func (a *App) showThemeDetails() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	root := resolveThemeRoot(th.Path)
	st, _ := os.Stat(th.Path)
	mod := "unknown"
	if st != nil {
		mod = st.ModTime().Format("2006-01-02 15:04")
	}
	images := listThemeFiles(root, ".png", ".jpg", ".jpeg", ".svg", ".webp")
	luas := listThemeFiles(root, ".lua")
	scripts := listThemeFiles(root, ".sh")
	fonts := listThemeFiles(root, ".ttf", ".otf", ".woff", ".woff2")
	present, missing := collectThemeDependencies(th.Path)
	h := inspectThemeHealth(th.Path)
	a.themeHealth[th.Label()] = h
	a.themeStatus[th.Label()] = h.Status
	pid := 0
	if mp := managedForTheme(th.Path); mp != nil {
		pid = mp.PID
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\nConfig: %s\nLocation: %s\nSize: %s\nModified: %s\nHealth: %s\nPID: %d\n\n",
		filepath.Base(root), th.Path, root, formatBytes(dirSize(root)), mod, h.Status, pid)
	fmt.Fprintf(&b, "Images (%d):\n%s\n\nLua (%d):\n%s\n\nScripts (%d):\n%s\n\nFonts (%d):\n%s\n\n",
		len(images), strings.Join(shortList(images, 12), "\n"), len(luas), strings.Join(shortList(luas, 8), "\n"),
		len(scripts), strings.Join(shortList(scripts, 8), "\n"), len(fonts), strings.Join(shortList(fonts, 8), "\n"))
	fmt.Fprintf(&b, "Dependencies present: %s\nMissing: %s\n\nIssues:\n", strings.Join(present, ", "), strings.Join(missing, ", "))
	if len(h.Issues) == 0 {
		b.WriteString("None\n")
	}
	for _, iss := range h.Issues {
		fmt.Fprintf(&b, "• [%s/%s] %s\n  %s\n", iss.Severity, iss.Code, iss.Message, iss.Fix)
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\nInstall hint:\n%s\n", suggestedInstallCommand(missing))
	}
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Theme Details")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(640, 520)
	box, _ := dlg.GetContentArea()
	setMargins(box, 10)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	view, _ := gtk.TextViewNew()
	view.SetEditable(false)
	view.SetWrapMode(gtk.WRAP_WORD_CHAR)
	buf, _ := view.GetBuffer()
	buf.SetText(b.String())
	scroll.Add(view)
	box.PackStart(scroll, true, true, 0)
	if len(missing) > 0 {
		copyBtn := a.actionBtn("Copy install command", "", func() {
			cmd := suggestedInstallCommand(missing)
			a.showMessage("Install command", cmd, gtk.MESSAGE_INFO)
			a.pushStatus("Install command ready to copy from the dialog.")
		})
		box.PackStart(copyBtn, false, false, 6)
	}
	dlg.AddButton("Close", gtk.RESPONSE_CLOSE)
	dlg.ShowAll()
	dlg.Run()
	dlg.Destroy()
	a.applyFilter("")
}

func shortList(items []string, n int) []string {
	if len(items) == 0 {
		return []string{"(none)"}
	}
	if len(items) > n {
		return append(append([]string{}, items[:n]...), fmt.Sprintf("… %d more", len(items)-n))
	}
	return items
}

func (a *App) showPaletteManager() {
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Color Palette Manager")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(520, 420)
	box, _ := dlg.GetContentArea()
	setMargins(box, 12)
	box.SetSpacing(8)
	box.PackStart(newLabel("Built-in palettes stay available. Custom palettes are saved in palettes.json.", true), false, false, 0)
	list, _ := gtk.ListBoxNew()
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetMinContentHeight(180)
	scroll.Add(list)
	box.PackStart(scroll, true, true, 0)
	accent, _ := gtk.ColorButtonNew()
	accent.SetRGBA(a.hexToRGBA("#7AA2F7"))
	nameEnt, _ := gtk.EntryNew()
	nameEnt.SetPlaceholderText("Custom palette name")
	refresh := func() {
		clearListBox(list)
		for _, p := range colorPresets {
			row, _ := gtk.ListBoxRowNew()
			row.Add(newLabel("Preset · "+p.Preset.Label, false))
			list.Add(row)
		}
		for _, p := range loadCustomPalettes() {
			row, _ := gtk.ListBoxRowNew()
			row.Add(newLabel("Custom · "+p.Label, false))
			list.Add(row)
		}
		list.ShowAll()
	}
	refresh()
	saveBtn := a.actionBtn("Save current colors as palette", "", func() {
		label, _ := nameEnt.GetText()
		label = strings.TrimSpace(label)
		if label == "" {
			a.showMessage("Info", "Enter a palette name.", gtk.MESSAGE_INFO)
			return
		}
		cols := map[string]string{}
		if ov := a.getColorsFromControls(); ov != nil {
			cols = ov.Colors
		} else {
			cols = generatePaletteFromAccent(rgbaToHex(accent.GetRGBA()))
		}
		all := loadCustomPalettes()
		all = append(all, CustomPalette{ID: normalizeFolderName(label), Label: label, Colors: cols})
		saveCustomPalettes(all)
		if a.colorPreset != nil {
			a.colorPreset.Append("custom:"+normalizeFolderName(label), label)
		}
		refresh()
	})
	genBtn := a.actionBtn("Generate from accent", "", func() {
		pal := generatePaletteFromAccent(rgbaToHex(accent.GetRGBA()))
		a.colorLoading = true
		for slot, p := range a.colorPickers {
			if v, ok := pal[slot]; ok {
				p.SetRGBA(a.hexToRGBA(v))
			}
		}
		a.colorLoading = false
		a.colorStatus.SetText("Generated palette from accent color.")
	})
	box.PackStart(settingsRow("Accent:", accent), false, false, 0)
	box.PackStart(nameEnt, false, false, 0)
	box.PackStart(saveBtn, false, false, 0)
	box.PackStart(genBtn, false, false, 0)
	dlg.AddButton("Close", gtk.RESPONSE_CLOSE)
	dlg.ShowAll()
	dlg.Run()
	dlg.Destroy()
}

func (a *App) showBackupHistory() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", gtk.MESSAGE_INFO)
		return
	}
	backs := listBackups(th.Path)
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Backup History")
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(720, 480)
	box, _ := dlg.GetContentArea()
	setMargins(box, 10)
	box.SetSpacing(8)
	box.PackStart(newLabel("Original files stay in place. Restore copies a backup over the current config after creating a new backup.", true), false, false, 0)
	combo, _ := gtk.ComboBoxTextNew()
	for _, p := range backs {
		combo.Append(p, filepath.Base(p))
	}
	if len(backs) > 0 {
		combo.SetActive(0)
	}
	box.PackStart(combo, false, false, 0)
	diffView, _ := gtk.TextViewNew()
	diffView.SetEditable(false)
	diffView.SetMonospace(true)
	dbuf, _ := diffView.GetBuffer()
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.Add(diffView)
	box.PackStart(scroll, true, true, 0)
	showDiff := func() {
		p := combo.GetActiveID()
		if p == "" {
			dbuf.SetText("No backups for this theme yet.")
			return
		}
		dbuf.SetText(unifiedDiff(readText(p), readText(th.Path), filepath.Base(p), filepath.Base(th.Path)))
	}
	combo.Connect("changed", showDiff)
	showDiff()
	act, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	rst := a.actionBtn("Restore selected", "suggested-action", func() {
		p := combo.GetActiveID()
		if p == "" {
			return
		}
		if err := writeOriginalThemeConfig(th.Path, readText(p), "restore"); err != nil {
			a.showMessage("Restore Error", err.Error(), gtk.MESSAGE_ERROR)
			return
		}
		a.pushStatus("Restored backup " + filepath.Base(p))
		showDiff()
	})
	del := a.actionBtn("Delete selected backup", "destructive-action", func() {
		p := combo.GetActiveID()
		if p == "" {
			return
		}
		_ = os.Remove(p)
		combo.RemoveAll()
		backs = listBackups(th.Path)
		for _, x := range backs {
			combo.Append(x, filepath.Base(x))
		}
		if len(backs) > 0 {
			combo.SetActive(0)
		}
		showDiff()
	})
	act.PackStart(rst, false, false, 0)
	act.PackStart(del, false, false, 0)
	box.PackStart(act, false, false, 0)
	dlg.AddButton("Close", gtk.RESPONSE_CLOSE)
	dlg.ShowAll()
	dlg.Run()
	dlg.Destroy()
}

func (a *App) clearDownloadCache() {
	entries, _ := os.ReadDir(downloadsDir)
	n := 0
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(downloadsDir, e.Name()))
		n++
	}
	a.pushStatus(fmt.Sprintf("Cleared %d cached download(s).", n))
}

func (a *App) showEmbeddedEditor(path string) {
	original := readText(path)
	dlg, _ := gtk.DialogNew()
	dlg.SetTitle("Config Editor — " + filepath.Base(path))
	dlg.SetTransientFor(a.win)
	dlg.SetDefaultSize(820, 560)
	box, _ := dlg.GetContentArea()
	setMargins(box, 8)
	tools, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	search, _ := gtk.EntryNew()
	search.SetPlaceholderText("Search")
	line, _ := gtk.SpinButtonNewWithRange(1, 99999, 1)
	view, _ := gtk.TextViewNew()
	view.SetMonospace(true)
	buf, _ := view.GetBuffer()
	buf.SetText(original)
	find := func() {
		q, _ := search.GetText()
		if q == "" {
			return
		}
		text, _ := buf.GetText(buf.GetStartIter(), buf.GetEndIter(), false)
		idx := strings.Index(text, q)
		if idx >= 0 {
			it := buf.GetIterAtOffset(idx)
			buf.PlaceCursor(it)
			view.ScrollToIter(it, 0.2, true, 0, 0.3)
		}
	}
	goLine := func() {
		it := buf.GetIterAtLine(line.GetValueAsInt() - 1)
		buf.PlaceCursor(it)
		view.ScrollToIter(it, 0.2, true, 0, 0.3)
	}
	search.Connect("activate", find)
	fb, _ := gtk.ButtonNewWithLabel("Find")
	fb.Connect("clicked", find)
	fitButton(fb)
	gb, _ := gtk.ButtonNewWithLabel("Go to line")
	gb.Connect("clicked", goLine)
	fitButton(gb)
	tools.PackStart(search, true, true, 0)
	tools.PackStart(fb, false, false, 0)
	tools.PackStart(line, false, false, 0)
	tools.PackStart(gb, false, false, 0)
	box.PackStart(tools, false, false, 0)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.Add(view)
	box.PackStart(scroll, true, true, 0)
	save := func() string {
		t, _ := buf.GetText(buf.GetStartIter(), buf.GetEndIter(), false)
		return t
	}
	act, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	act.PackStart(a.actionBtn("Save", "suggested-action", func() {
		if err := writeOriginalThemeConfig(path, save(), "editor"); err != nil {
			a.showMessage("Save Error", err.Error(), gtk.MESSAGE_ERROR)
			return
		}
		a.pushStatus("Saved " + filepath.Base(path))
	}), false, false, 0)
	act.PackStart(a.actionBtn("Revert", "", func() { buf.SetText(original) }), false, false, 0)
	act.PackStart(a.actionBtn("Validate", "", func() {
		tmp := filepath.Join(os.TempDir(), "conky-manager-validate.conf")
		_ = os.WriteFile(tmp, []byte(save()), 0o644)
		ok := validateConkyConfig(tmp, filepath.Dir(path), conkyEnv())
		_ = os.Remove(tmp)
		if ok {
			a.showMessage("Validate", "Conky accepted this configuration.", gtk.MESSAGE_INFO)
		} else {
			a.showMessage("Validate", "Conky rejected this configuration. See manager.log.", gtk.MESSAGE_ERROR)
		}
	}), false, false, 0)
	act.PackStart(a.actionBtn("Save As…", "", func() {
		fc, _ := gtk.FileChooserDialogNewWith2Buttons("Save As", a.win, gtk.FILE_CHOOSER_ACTION_SAVE, "Cancel", gtk.RESPONSE_CANCEL, "Save", gtk.RESPONSE_OK)
		fc.SetCurrentName(filepath.Base(path))
		if fc.Run() == gtk.RESPONSE_OK {
			_ = os.WriteFile(fc.GetFilename(), []byte(save()), 0o644)
		}
		fc.Destroy()
	}), false, false, 0)
	box.PackStart(act, false, false, 0)
	dlg.AddButton("Close", gtk.RESPONSE_CLOSE)
	dlg.ShowAll()
	dlg.Run()
	dlg.Destroy()
}

func (a *App) duplicateProfile() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select a profile first.", gtk.MESSAGE_ERROR)
		return
	}
	newName, ok := promptText(a.win, "Duplicate Profile", "New profile name:", name+" copy")
	if !ok || newName == "" {
		return
	}
	paths := append([]string{}, a.profiles.Profiles[name]...)
	a.profiles.Profiles[newName] = paths
	if dp, ok := a.profiles.Desktop[name]; ok {
		dp.Name = newName
		a.profiles.Desktop[newName] = dp
	} else {
		a.profiles.Desktop[newName] = captureDesktopProfile(newName, paths)
	}
	saveProfiles(a.profiles)
	a.refreshProfiles()
}

func (a *App) exportProfile() {
	name := a.selectedProfileName()
	if name == "" {
		return
	}
	dlg, _ := gtk.FileChooserDialogNewWith2Buttons("Export Profile", a.win, gtk.FILE_CHOOSER_ACTION_SAVE, "Cancel", gtk.RESPONSE_CANCEL, "Export", gtk.RESPONSE_OK)
	dlg.SetCurrentName(normalizeFolderName(name) + ".conky-profile")
	if dlg.Run() != gtk.RESPONSE_OK {
		dlg.Destroy()
		return
	}
	dest := dlg.GetFilename()
	dlg.Destroy()
	dp := a.profiles.Desktop[name]
	if dp.Name == "" {
		dp = captureDesktopProfile(name, a.profiles.Profiles[name])
	}
	if err := saveJSON(dest, dp); err != nil {
		a.showMessage("Export Error", err.Error(), gtk.MESSAGE_ERROR)
		return
	}
	a.pushStatus("Exported profile to " + dest)
}

func (a *App) importProfile() {
	dlg, _ := gtk.FileChooserDialogNewWith2Buttons("Import Profile", a.win, gtk.FILE_CHOOSER_ACTION_OPEN, "Cancel", gtk.RESPONSE_CANCEL, "Import", gtk.RESPONSE_OK)
	if dlg.Run() != gtk.RESPONSE_OK {
		dlg.Destroy()
		return
	}
	src := dlg.GetFilename()
	dlg.Destroy()
	var dp DesktopProfile
	if err := loadJSON(src, &dp); err != nil || dp.Name == "" {
		a.showMessage("Import Error", "Not a valid .conky-profile file.", gtk.MESSAGE_ERROR)
		return
	}
	var paths []string
	for _, t := range dp.Themes {
		paths = append(paths, t.Path)
	}
	a.profiles.Profiles[dp.Name] = paths
	a.profiles.Desktop[dp.Name] = dp
	saveProfiles(a.profiles)
	a.refreshProfiles()
	a.pushStatus("Imported profile " + dp.Name)
}

func themeTreeFingerprint() string {
	h := sha256.New()
	w := bufio.NewWriter(h)
	for _, root := range allThemeRoots() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			fmt.Fprintf(w, "%s:%d:%d\n", path, info.Size(), info.ModTime().UnixNano())
			return nil
		})
	}
	_ = w.Flush()
	return hex.EncodeToString(h.Sum(nil))
}

func (a *App) startFileWatcher() {
	a.watchStamp = themeTreeFingerprint()
	glib.TimeoutSecondsAdd(4, func() bool {
		stamp := themeTreeFingerprint()
		if stamp != a.watchStamp {
			a.watchStamp = stamp
			a.loadThemes()
			a.pushStatus("Theme folders changed — list refreshed.")
		}
		return true
	})
}

func newApp() (*App, error) {
	sess, desk := detectEnvironment()
	a := &App{
		session: sess, desktop: desk, desktopEnv: detectDesktopEnvironment(),
		settings: loadSettings(), profiles: loadProfiles(), library: loadLibrary(),
		themeStatus: map[string]string{}, themeHealth: map[string]ThemeHealth{},
		selectedThemeSet: map[string]bool{}, viewFilter: "all",
		previewSeconds: loadSettings().PreviewSeconds,
	}
	if err := a.setupUI(); err != nil {
		return nil, err
	}
	a.applyUITheme()
	a.loadThemes()
	a.refreshProfiles()
	a.refreshLogView(250)
	a.startFileWatcher()
	a.showWaylandNotice()
	return a, nil
}

func main() {
	initAppDirs()
	args := os.Args[1:]
	for i, a := range args {
		if a == "--run-profile" && i+1 < len(args) {
			os.Exit(runProfileCLI(args[i+1]))
		}
	}
	if !isConkyInstalled() {
		fmt.Println("Conky is not installed. Install it first, then run this app again.")
		os.Exit(1)
	}
	gtk.Init(nil)
	if _, err := newApp(); err != nil {
		fmt.Println("Error: GTK3 UI failed:", err)
		fmt.Println("Install: gtk3-devel (or libgtk-3-dev) and build with CGO enabled.")
		os.Exit(1)
	}
	signal.Ignore(syscall.SIGPIPE)
	gtk.Main()
}
