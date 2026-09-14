// Conky Manager Qt - v1.0 (Go edition)
// Universal Conky desktop manager for Linux environments.

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

	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

const (
	appName    = "Conky Manager Qt"
	appVersion = "v1.0"
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
	if qt.QCoreApplication_Instance() != nil {
		screens := qt.QGuiApplication_Screens()
		primary := qt.QGuiApplication_PrimaryScreen()
		for i, s := range screens {
			if s == nil {
				continue
			}
			g := s.Geometry()
			man := strings.TrimSpace(s.Manufacturer())
			model := strings.TrimSpace(s.Model())
			name := strings.TrimSpace(s.Name())
			id := strings.TrimSpace(strings.Join([]string{man, model}, " "))
			if id == "" {
				id = name
			}
			if id == "" {
				id = fmt.Sprintf("Monitor %d", i+1)
			}
			scale := int(s.DevicePixelRatio() + 0.5)
			if scale < 1 {
				scale = 1
			}
			isPrimary := primary != nil && s.Name() == primary.Name()
			if i == 0 && len(screens) == 1 {
				isPrimary = true
			}
			mons = append(mons, MonitorInfo{
				ID: id, Name: name, Manufacturer: man, Model: model,
				X: g.X(), Y: g.Y(), Width: g.Width(), Height: g.Height(),
				Scale: scale, Primary: isPrimary, Head: i + 1,
			})
		}
	}
	if len(mons) == 0 {
		out, err := exec.Command("xrandr", "--query").Output()
		if err == nil {
			re := regexp.MustCompile(`(?m)^(\S+)\s+connected(?:\s+primary)?\s+(\d+)x(\d+)\+(\d+)\+(\d+)`)
			for i, m := range re.FindAllStringSubmatch(string(out), -1) {
				w, _ := strconv.Atoi(m[2])
				h, _ := strconv.Atoi(m[3])
				x, _ := strconv.Atoi(m[4])
				y, _ := strconv.Atoi(m[5])
				mons = append(mons, MonitorInfo{
					ID: m[1], Name: m[1], Width: w, Height: h, X: x, Y: y,
					Scale: 1, Primary: i == 0, Head: i + 1,
				})
			}
		}
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

var themeFingerprintCache struct {
	sync.Mutex
	at    time.Time
	value string
}

const themeFingerprintCacheTTL = 3 * time.Second

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

func shortList(items []string, n int) []string {
	if len(items) == 0 {
		return []string{"(none)"}
	}
	if len(items) > n {
		return append(append([]string{}, items[:n]...), fmt.Sprintf("… %d more", len(items)-n))
	}
	return items
}
func idle(fn func()) { mainthread.Start(fn) }

func xdgOpen(path string) {
	if strings.Contains(path, "://") {
		qt.QDesktopServices_OpenUrl(qt.NewQUrl3(path))
		return
	}
	qt.QDesktopServices_OpenUrl(qt.QUrl_FromLocalFile(path))
}

func themeTreeFingerprint() string {
	themeFingerprintCache.Lock()
	defer themeFingerprintCache.Unlock()
	if time.Since(themeFingerprintCache.at) < themeFingerprintCacheTTL {
		return themeFingerprintCache.value
	}
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
	themeFingerprintCache.value = hex.EncodeToString(h.Sum(nil))
	themeFingerprintCache.at = time.Now()
	return themeFingerprintCache.value
}

func comboAdd(cb *qt.QComboBox, id, label string) {
	cb.AddItem3(label, qt.NewQVariant11(id))
}

func comboID(cb *qt.QComboBox, def string) string {
	if cb == nil || cb.CurrentIndex() < 0 {
		return def
	}
	d := cb.CurrentData()
	if d == nil {
		return def
	}
	s := d.ToString()
	if s == "" {
		return def
	}
	return s
}

func comboSetID(cb *qt.QComboBox, id string) {
	if cb == nil {
		return
	}
	for i := 0; i < cb.Count(); i++ {
		if d := cb.ItemData(i); d != nil && d.ToString() == id {
			cb.SetCurrentIndex(i)
			return
		}
	}
}

func confirmDialog(parent *qt.QWidget, title, message string) bool {
	r := qt.QMessageBox_Question6(parent, title, message, qt.QMessageBox__Yes|qt.QMessageBox__No, qt.QMessageBox__No)
	return r == qt.QMessageBox__Yes
}

func promptText(parent *qt.QWidget, title, label, initial string) (string, bool) {
	ok := false
	s := qt.QInputDialog_GetText4(parent, title, label, qt.QLineEdit__Normal, initial, &ok)
	return strings.TrimSpace(s), ok
}

func colorToHex(c *qt.QColor) string {
	if c == nil || !c.IsValid() {
		return "#FFFFFF"
	}
	return fmt.Sprintf("#%02X%02X%02X", c.Red(), c.Green(), c.Blue())
}

func hexToColor(s string) *qt.QColor {
	n := normalizeColor(s)
	if n == "" {
		return qt.NewQColor3(255, 255, 255)
	}
	return qt.NewQColor6(n)
}

func wrapScroll(inner *qt.QWidget) *qt.QScrollArea {
	s := qt.NewQScrollArea2()
	s.SetWidgetResizable(true)
	s.SetFrameShape(qt.QFrame__NoFrame)
	s.SetWidget(inner)
	return s
}

func clearLayout(lay *qt.QLayout) {
	if lay == nil {
		return
	}
	for lay.Count() > 0 {
		it := lay.TakeAt(0)
		if it == nil {
			continue
		}
		if w := it.Widget(); w != nil {
			w.Hide()
			w.SetParent(nil)
			w.DeleteLater()
		}
	}
}

func appStyleSheet(dark bool) string {
	if dark {
		return `
* { font-family: "Inter","IBM Plex Sans","Noto Sans","Segoe UI",sans-serif; font-size: 12px; }
QMainWindow, QDialog, QWidget { background: #0b1220; color: #e7eef8; }
QMainWindow { border: 1px solid #26364a; }
QMenuBar { background: #101a2a; color: #cbd8e8; border-bottom: 1px solid #2a3b52; padding: 2px 6px; }
QMenuBar::item { padding: 5px 9px; border-radius: 5px; }
QMenuBar::item:selected { background: #243957; color: #ffffff; }
QMenu { background: #172235; color: #e7eef8; border: 1px solid #38506d; border-radius: 8px; padding: 4px; }
QMenu::item { padding: 6px 14px; border-radius: 5px; }
QMenu::item:selected { background: #2f81f7; color: #ffffff; }
QSplitter::handle { background: #2a3b52; width: 1px; }
QLineEdit, QComboBox, QSpinBox, QDoubleSpinBox, QPlainTextEdit, QTextEdit {
  background: #101a2a; color: #e7eef8; border: 1px solid #30445e; border-radius: 6px; padding: 5px 8px; selection-background-color: #2f81f7; min-height: 16px;
}
QLineEdit:focus, QComboBox:focus, QSpinBox:focus, QDoubleSpinBox:focus, QPlainTextEdit:focus, QTextEdit:focus { border: 1px solid #58a6ff; }
QComboBox::drop-down { border: none; width: 18px; }
QComboBox QAbstractItemView { background: #172235; color: #e7eef8; selection-background-color: #347eea; border: 1px solid #38506d; }
QPushButton {
  background: #18283b; color: #dbe7f5; border: 1px solid #38506d; border-radius: 6px; padding: 5px 9px; min-height: 16px;
}
QPushButton:hover { background: #243957; border-color: #69b1ff; color: #ffffff; }
QPushButton:pressed { background: #14202c; }
QPushButton:disabled { color: #667585; background: #121820; border-color: #202a35; }
QPushButton#accent { background: #347eea; border: 1px solid #69b1ff; color: #ffffff; font-weight: 600; }
QPushButton#accent:hover { background: #4b94f4; }
QPushButton#danger { background: #2d1720; border: 1px solid #8e3b50; color: #ffb8c6; }
QPushButton#danger:hover { background: #421d2a; border-color: #f47087; }
QPushButton#ghost { background: transparent; border: 1px solid #304052; }
QPushButton#chip { min-width: 30px; min-height: 26px; padding: 3px 5px; border-radius: 6px; }
QPushButton#chip:checked { background: #2f81f7; border: 1px solid #58a6ff; color: #ffffff; font-weight: 600; }
QListWidget { background: transparent; border: none; outline: none; padding: 2px; }
QListWidget::item { background: #101a2a; border: 1px solid #2a3b52; border-radius: 8px; margin: 0 0 5px 0; padding: 2px; }
QListWidget::item:hover { background: #182b43; border-color: #4c7198; }
QListWidget::item:selected { background: #1b3554; border: 1px solid #4b9bff; }
QTabWidget::pane { border: 1px solid #2a3b52; background: #101a2a; border-radius: 0 7px 7px 7px; }
QTabBar::tab { background: #142033; color: #9aacc2; border: 1px solid #2a3b52; padding: 6px 11px; margin-right: 3px; border-top-left-radius: 6px; border-top-right-radius: 6px; }
QTabBar::tab:hover { color: #d8e2ed; background: #1b2734; }
QTabBar::tab:selected { background: #101a2a; color: #ffffff; border-color: #4b9bff; border-bottom-color: #111821; }
QGroupBox { background: #101a2a; border: 1px solid #2a3b52; border-radius: 9px; margin-top: 12px; padding: 11px 9px 8px 9px; font-weight: 600; }
QGroupBox::title { subcontrol-origin: margin; left: 11px; padding: 0 6px; color: #9ecbff; }
QCheckBox { spacing: 6px; }
QCheckBox::indicator { width: 28px; height: 16px; border-radius: 8px; background: #263241; border: 1px solid #405164; }
QCheckBox::indicator:checked { background: #2f81f7; border-color: #58a6ff; }
QProgressBar { background: #101a2a; border: 1px solid #2a3b52; border-radius: 7px; text-align: center; height: 12px; color: #e6edf3; }
QProgressBar::chunk { background: #2f81f7; border-radius: 6px; }
QStatusBar { background: #101a2a; color: #9aacc2; border-top: 1px solid #2a3b52; }
QScrollBar:vertical { background: transparent; width: 8px; margin: 2px; }
QScrollBar::handle:vertical { background: #304052; border-radius: 4px; min-height: 24px; }
QScrollBar::handle:vertical:hover { background: #4b6278; }
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }
QToolTip { background: #1f2a36; color: #f0f6fc; border: 1px solid #4b6278; padding: 4px 6px; }
QLabel#muted, QLabel#heroMeta { color: #91a1b3; }
QLabel#title { font-size: 17px; font-weight: 700; color: #f0f6fc; }
QLabel#kpi { font-size: 20px; font-weight: 700; color: #58a6ff; }
QLabel#live { color: #3fb950; font-weight: 600; font-size: 12px; }
QLabel#eyebrow { color: #79c0ff; letter-spacing: 2px; font-size: 10px; font-weight: 700; }
QLabel#heroTitle { color: #f0f6fc; font-size: 20px; font-weight: 700; }
QWidget#hero { background: qlineargradient(x1:0,y1:0,x2:1,y2:1, stop:0 #172b47, stop:0.55 #14243a, stop:1 #202044); border-bottom: 1px solid #4b9bff; }
QWidget#dashboardCard { background: qlineargradient(x1:0,y1:0,x2:1,y2:1, stop:0 #172b47, stop:1 #132238); border: 1px solid #355776; border-radius: 8px; }
QLabel#themeName { color: #edf4fc; font-size: 13px; font-weight: 700; }
QLabel#themeBadge { color: #79c0ff; font-size: 11px; font-weight: 600; }
QLabel { background: transparent; }
QFrame { background: transparent; }
QLabel#runtimeInfo { color: #edf4fc; }
QWidget#sidebar { background: #0d1726; border-right: 1px solid #263d57; }
QWidget#mainPanel { background: #0b1220; }

`
	}
	return `
* { font-family: "Inter","IBM Plex Sans","Noto Sans","Segoe UI",sans-serif; font-size: 12px; }
QMainWindow, QDialog, QWidget { background: #f6f8fb; color: #243044; }
QMainWindow { border: 1px solid #d7e0eb; }
QMenuBar { background: #ffffff; color: #34445a; border-bottom: 1px solid #dce4ee; padding: 2px 6px; }
QMenuBar::item { padding: 5px 9px; border-radius: 5px; }
QMenuBar::item:selected { background: #e8f1ff; color: #1558a6; }
QMenu { background: #ffffff; color: #243044; border: 1px solid #ccd8e6; border-radius: 8px; padding: 4px; }
QMenu::item { padding: 6px 14px; border-radius: 5px; }
QMenu::item:selected { background: #2563c7; color: #ffffff; }
QSplitter::handle { background: #d7e0eb; width: 1px; }
QLineEdit, QComboBox, QSpinBox, QDoubleSpinBox, QPlainTextEdit, QTextEdit {
  background: #ffffff; color: #243044; border: 1px solid #c8d4e2; border-radius: 6px; padding: 5px 8px; selection-background-color: #2f81f7; min-height: 16px;
}
QLineEdit:focus, QComboBox:focus, QSpinBox:focus, QDoubleSpinBox:focus, QPlainTextEdit:focus, QTextEdit:focus { border: 1px solid #2f81f7; }
QComboBox::drop-down { border: none; width: 18px; }
QComboBox QAbstractItemView { background: #ffffff; color: #243044; selection-background-color: #e8f1ff; border: 1px solid #c8d4e2; }
QPushButton { background: #ffffff; color: #34445a; border: 1px solid #c8d4e2; border-radius: 6px; padding: 5px 9px; min-height: 16px; }
QPushButton:hover { background: #f0f6ff; border-color: #2f81f7; color: #1558a6; }
QPushButton:pressed { background: #e3efff; }
QPushButton:disabled { color: #9aa8b8; background: #eef2f6; border-color: #d9e1ea; }
QPushButton#accent { background: #2563c7; border: 1px solid #2563c7; color: #ffffff; font-weight: 600; }
QPushButton#accent:hover { background: #2f75e0; }
QPushButton#danger { background: #fff2f4; border: 1px solid #e5a7b3; color: #a12643; }
QPushButton#danger:hover { background: #ffe4e9; border-color: #c83e5a; }
QPushButton#ghost { background: transparent; border: 1px solid #c8d4e2; }
QPushButton#chip { min-width: 30px; min-height: 26px; padding: 3px 5px; border-radius: 6px; }
QPushButton#chip:checked { background: #2563c7; border: 1px solid #2563c7; color: #ffffff; font-weight: 600; }
QListWidget { background: transparent; border: none; outline: none; padding: 2px; }
QListWidget::item { background: #ffffff; border: 1px solid #dbe3ec; border-radius: 8px; margin: 0 0 5px 0; padding: 2px; }
QListWidget::item:hover { background: #f5f9ff; border-color: #a9c8ef; }
QListWidget::item:selected { background: #eaf3ff; border: 1px solid #2f81f7; }
QTabWidget::pane { border: 1px solid #dbe3ec; background: #ffffff; border-radius: 0 7px 7px 7px; }
QTabBar::tab { background: #edf2f7; color: #617187; border: 1px solid #dbe3ec; padding: 6px 11px; margin-right: 3px; border-top-left-radius: 6px; border-top-right-radius: 6px; }
QTabBar::tab:hover { color: #243044; background: #e5edf6; }
QTabBar::tab:selected { background: #ffffff; color: #1558a6; border-color: #2f81f7; border-bottom-color: #ffffff; }
QGroupBox { background: #ffffff; border: 1px solid #dbe3ec; border-radius: 9px; margin-top: 12px; padding: 11px 9px 8px 9px; font-weight: 600; }
QGroupBox::title { subcontrol-origin: margin; left: 11px; padding: 0 6px; color: #2563c7; }
QCheckBox { spacing: 6px; }
QCheckBox::indicator { width: 28px; height: 16px; border-radius: 8px; background: #d5dee9; border: 1px solid #b8c6d5; }
QCheckBox::indicator:checked { background: #2563c7; border-color: #2563c7; }
QProgressBar { background: #eef2f6; border: 1px solid #dbe3ec; border-radius: 7px; text-align: center; height: 12px; color: #34445a; }
QProgressBar::chunk { background: #2563c7; border-radius: 6px; }
QStatusBar { background: #ffffff; color: #617187; border-top: 1px solid #dbe3ec; }
QScrollBar:vertical { background: transparent; width: 8px; margin: 2px; }
QScrollBar::handle:vertical { background: #c4d0de; border-radius: 4px; min-height: 24px; }
QScrollBar::handle:vertical:hover { background: #9fb2c7; }
QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }
QToolTip { background: #243044; color: #ffffff; border: 1px solid #617187; padding: 4px 6px; }
QLabel#muted, QLabel#heroMeta { color: #617187; }
QLabel#title { font-size: 17px; font-weight: 700; color: #1d2a3a; }
QLabel#kpi { font-size: 20px; font-weight: 700; color: #2563c7; }
QLabel#live { color: #16834d; font-weight: 600; font-size: 12px; }
QLabel#eyebrow { color: #2563c7; letter-spacing: 2px; font-size: 10px; font-weight: 700; }
QLabel#heroTitle { color: #1d2a3a; font-size: 20px; font-weight: 700; }
QWidget#hero { background: qlineargradient(x1:0,y1:0,x2:1,y2:1, stop:0 #eef5ff, stop:0.55 #ffffff, stop:1 #f3f0ff); border-bottom: 1px solid #2f81f7; }
QWidget#dashboardCard { background: qlineargradient(x1:0,y1:0,x2:1,y2:1, stop:0 #ffffff, stop:1 #f5f9ff); border: 1px solid #cbdced; border-radius: 8px; }
QLabel#themeName { color: #243044; font-size: 13px; font-weight: 700; }
QLabel#themeBadge { color: #2563c7; font-size: 11px; font-weight: 600; }
QLabel { background: transparent; }
QFrame { background: transparent; }
QWidget#sidebar { background: #f0f4f9; border-right: 1px solid #d5e0ec; }
QWidget#mainPanel { background: #f6f8fb; }

`
}

type App struct {
	qapp                         *qt.QApplication
	session, desktop, desktopEnv string
	settings                     Settings
	profiles                     ProfilesFile
	themes                       []ThemeItem
	themeStatus                  map[string]string
	themeHealth                  map[string]ThemeHealth
	library                      ThemeLibrary
	viewFilter                   string
	runtimeCache                 map[string]int
	watchStamp                   string

	win                          *qt.QMainWindow
	themeList                    *qt.QListWidget
	selectedThemeSet             map[string]bool
	searchEntry                  *qt.QLineEdit
	filterCombo                  *qt.QComboBox
	statusbar                    *qt.QStatusBar
	statusIndicator, runtimeInfo *qt.QLabel
	envLabel                     *qt.QLabel
	dashLabels                   map[string]*qt.QLabel
	logsView                     *qt.QPlainTextEdit

	posEnabled                           *qt.QCheckBox
	posGapX, posGapY, posNudge           *qt.QSpinBox
	posStatus                            *qt.QLabel
	posMonitor                           *qt.QComboBox
	posButtons                           map[string]*qt.QPushButton
	btnSavePos, btnResetPos, btnApplyPos *qt.QPushButton

	colorEnabled, colorSmartLua          *qt.QCheckBox
	colorPreset                          *qt.QComboBox
	colorSlotsHost                       *qt.QWidget
	colorSlotsLay                        *qt.QVBoxLayout
	colorStatus                          *qt.QLabel
	colorPickers                         map[string]*qt.QPushButton
	themeColorSlots                      map[string]string
	btnSaveCol, btnResetCol, btnApplyCol *qt.QPushButton

	profileName         *qt.QLineEdit
	profileList         *qt.QListWidget
	profileDetails      *qt.QListWidget
	profileDetailsTitle *qt.QLabel

	previewSeconds                   int
	previewPaused                    bool
	busyButtons                      []*qt.QWidget
	workerBusy                       bool
	previewTimer                     *qt.QTimer
	watchTimer                       *qt.QTimer
	posLoading                       bool
	colorLoading                     bool
	activePosTheme, activeColorTheme string
}

func (a *App) parent() *qt.QWidget {
	if a.win == nil {
		return nil
	}
	return a.win.QWidget
}

func (a *App) pushStatus(msg string) {
	if a.statusbar != nil {
		a.statusbar.ShowMessage2(msg, 8000)
	}
}

func (a *App) showMessage(title, message string, kind string) {
	p := a.parent()
	switch kind {
	case "error":
		qt.QMessageBox_Critical(p, title, message)
	case "warn":
		qt.QMessageBox_Warning(p, title, message)
	default:
		qt.QMessageBox_Information(p, title, message)
	}
}

func (a *App) keepBusy(w *qt.QWidget) {
	if w != nil {
		a.busyButtons = append(a.busyButtons, w)
	}
}

func (a *App) setBusy(busy bool) {
	a.workerBusy = busy
	for _, b := range a.busyButtons {
		if b != nil {
			b.SetEnabled(!busy)
		}
	}
}

func (a *App) applyUITheme() {
	if a.qapp == nil {
		return
	}
	dark := true
	switch a.settings.UITheme {
	case "light":
		dark = false
	case "dark":
		dark = true
	default:
		pal := qt.QGuiApplication_Palette()
		if pal != nil {
			c := pal.Color(qt.QPalette__Active, qt.QPalette__Window)
			if c != nil && (c.Red()+c.Green()+c.Blue())/3 > 140 {
				dark = false
			}
		}
	}
	a.qapp.SetStyleSheet(appStyleSheet(dark))
}

func muted(text string) *qt.QLabel {
	l := qt.NewQLabel3(text)
	l.SetObjectName(*qt.NewQAnyStringView3("muted"))
	l.SetWordWrap(true)
	return l
}

func titleLab(text string) *qt.QLabel {
	l := qt.NewQLabel3(text)
	l.SetObjectName(*qt.NewQAnyStringView3("title"))
	return l
}

func (a *App) btn(text, kind string, fn func()) *qt.QPushButton {
	b := qt.NewQPushButton3(text)
	if kind != "" {
		b.SetObjectName(*qt.NewQAnyStringView3(kind))
	}
	b.SetCursor(qt.NewQCursor2(qt.PointingHandCursor))
	if fn != nil {
		b.OnClicked(fn)
	}
	return b
}

func chip(text, tip string, fn func()) *qt.QPushButton {
	b := qt.NewQPushButton3(text)
	b.SetObjectName(*qt.NewQAnyStringView3("chip"))
	b.SetToolTip(tip)
	b.SetCursor(qt.NewQCursor2(qt.PointingHandCursor))
	b.OnClicked(fn)
	return b
}

func cardBox(title string) (*qt.QGroupBox, *qt.QVBoxLayout) {
	g := qt.NewQGroupBox3(title)
	lay := qt.NewQVBoxLayout2()
	lay.SetContentsMargins(8, 10, 8, 8)
	lay.SetSpacing(8)
	g.SetLayout(lay.QLayout)
	return g, lay
}

func hrow(widgets ...*qt.QWidget) *qt.QWidget {
	w := qt.NewQWidget2()
	lay := qt.NewQHBoxLayout2()
	lay.SetContentsMargins(0, 0, 0, 0)
	lay.SetSpacing(8)
	for _, x := range widgets {
		lay.AddWidget(x)
	}
	lay.AddStretch()
	w.SetLayout(lay.QLayout)
	return w
}

func (a *App) setupUI() {
	a.win = qt.NewQMainWindow2()
	a.win.SetWindowTitle(appName + "  " + appVersion)
	ww, wh := 1180, 760
	if scr := qt.QGuiApplication_PrimaryScreen(); scr != nil {
		g := scr.AvailableGeometry()
		if g.Width()-80 < ww {
			ww = g.Width() - 80
		}
		if g.Height()-80 < wh {
			wh = g.Height() - 80
		}
	}
	if ww < 900 {
		ww = 900
	}
	if wh < 620 {
		wh = 620
	}
	a.win.Resize(ww, wh)
	a.win.SetMinimumSize2(860, 560)

	mb := a.win.MenuBar()
	fileM := mb.AddMenuWithTitle("File")
	addAct(fileM, "Settings", a.showSettings)
	addAct(fileM, "Refresh themes", func() { a.loadThemes() })
	fileM.AddSeparator()
	addAct(fileM, "Quit", func() { a.win.Close() })
	tools := mb.AddMenuWithTitle("Tools")
	addAct(tools, "Theme marketplace", a.showThemeStore)
	addAct(tools, "Color palettes", a.showPaletteManager)
	addAct(tools, "Backup history", a.showBackupHistory)
	addAct(tools, "Clear download cache", a.clearDownloadCache)
	help := mb.AddMenuWithTitle("Help")
	addAct(help, "About", a.showAbout)

	root := qt.NewQWidget2()
	rootLay := qt.NewQVBoxLayout(root)
	rootLay.SetContentsMargins(0, 0, 0, 0)
	rootLay.SetSpacing(0)

	hero := qt.NewQWidget2()
	hero.SetObjectName(*qt.NewQAnyStringView3("hero"))
	hLay := qt.NewQHBoxLayout(hero)
	hLay.SetContentsMargins(22, 16, 22, 16)
	leftH := qt.NewQVBoxLayout2()
	brand := qt.NewQLabel3("CONKY MANAGER")
	brand.SetObjectName(*qt.NewQAnyStringView3("eyebrow"))
	name := qt.NewQLabel3("Desktop Atmosphere Studio")
	name.SetObjectName(*qt.NewQAnyStringView3("heroTitle"))
	a.envLabel = qt.NewQLabel3(compositorLabel(a.desktopEnv, a.session) + "  ·  " + appVersion)
	a.envLabel.SetObjectName(*qt.NewQAnyStringView3("heroMeta"))
	leftH.AddWidget(brand.QWidget)
	leftH.AddWidget(name.QWidget)
	leftH.AddWidget(a.envLabel.QWidget)
	hLay.AddLayout(leftH.QLayout)
	hLay.AddStretch()
	a.statusIndicator = qt.NewQLabel3("●  Idle")
	a.statusIndicator.SetObjectName(*qt.NewQAnyStringView3("live"))
	a.statusIndicator.SetStyleSheet("font-size:15px;")
	hLay.AddWidget(a.statusIndicator.QWidget)
	rootLay.AddWidget(hero)

	split := qt.NewQSplitter3(qt.Horizontal)
	split.AddWidget(a.buildSidebar())
	split.AddWidget(a.buildMain())
	split.SetStretchFactor(0, 0)
	split.SetStretchFactor(1, 1)
	split.SetSizes([]int{320, 860})
	rootLay.AddWidget2(split.QWidget, 1)

	a.win.SetCentralWidget(root)
	a.statusbar = qt.NewQStatusBar2()
	a.win.SetStatusBar(a.statusbar)
	a.pushStatus("Ready")
}

func addAct(menu *qt.QMenu, title string, fn func()) {
	act := menu.AddActionWithText(title)
	act.OnTriggered(fn)
}

func (a *App) buildSidebar() *qt.QWidget {
	side := qt.NewQWidget2()
	side.SetObjectName(*qt.NewQAnyStringView3("sidebar"))
	side.SetMinimumWidth(300)
	side.SetMaximumWidth(380)
	lay := qt.NewQVBoxLayout(side)
	lay.SetContentsMargins(16, 16, 12, 16)
	lay.SetSpacing(10)
	lay.AddWidget(titleLab("Library").QWidget)

	a.searchEntry = qt.NewQLineEdit2()
	a.searchEntry.SetPlaceholderText("Search themes…")
	a.searchEntry.OnTextChanged(func(_ string) { a.applyFilter("") })
	lay.AddWidget(a.searchEntry.QWidget)

	a.filterCombo = qt.NewQComboBox2()
	for _, p := range [][2]string{
		{"all", "All themes"}, {"favorites", "Favorites"}, {"running", "Running"},
		{"recent", "Recently used"}, {"needs_fix", "Needs fix"}, {"broken", "Broken"},
	} {
		comboAdd(a.filterCombo, p[0], p[1])
	}
	a.viewFilter = "all"
	a.filterCombo.OnCurrentIndexChanged(func(_ int) {
		a.viewFilter = comboID(a.filterCombo, "all")
		a.applyFilter("")
	})
	lay.AddWidget(a.filterCombo.QWidget)

	a.themeList = qt.NewQListWidget2()
	a.themeList.SetSelectionMode(qt.QAbstractItemView__NoSelection)
	a.themeList.SetSpacing(2)
	a.themeList.SetContextMenuPolicy(qt.CustomContextMenu)
	a.themeList.OnCustomContextMenuRequested(func(pos *qt.QPoint) {
		a.showThemeContextMenu(a.themeList.MapToGlobalWithQPoint(pos))
	})
	lay.AddWidget2(a.themeList.QWidget, 1)

	tools := hrow(
		chip("↻", "Refresh theme list", func() { a.loadThemes() }).QWidget,
		chip("⇩", "Import folder", a.importThemeFolder).QWidget,
		chip("▤", "Import archive", a.importThemeArchive).QWidget,
		chip("◎", "Open marketplace", a.showThemeStore).QWidget,
		chip("⌕", "Health scan all themes", a.healthScan).QWidget,
	)
	lay.AddWidget(tools)

	actions := qt.NewQWidget2()
	g := qt.NewQGridLayout2()
	g.SetSpacing(6)
	specs := []struct {
		lab, tip, kind string
		fn             func()
	}{
		{"▶", "Run selected", "accent", a.runSelected},
		{"■", "Stop selected", "", a.stopSelected},
		{"↻", "Restart selected", "", a.restartSelected},
		{"◉", "Preview selected", "", a.previewSelected},
		{"✎", "Edit selected", "", a.editSelected},
		{"⌂", "Open theme folder", "", a.openSelectedFolder},
		{"⚒", "Smart repair", "", a.smartRepair},
		{"ⓘ", "Theme details", "", a.showThemeDetails},
	}
	for i, s := range specs {
		b := chip(s.lab, s.tip, s.fn)
		if s.kind != "" {
			b.SetObjectName(*qt.NewQAnyStringView3(s.kind))
		}
		a.keepBusy(b.QWidget)
		g.AddWidget2(b.QWidget, i/4, i%4)
	}
	actions.SetLayout(g.QLayout)
	lay.AddWidget(actions)

	mgmt := hrow(
		chip("📄", "Clone theme", a.cloneSelectedTheme).QWidget,
		chip("✏", "Rename theme", a.renameSelectedTheme).QWidget,
		chip("🗑", "Move theme to trash", a.deleteSelectedTheme).QWidget,
		chip("📦", "Export theme archive", a.exportSelectedTheme).QWidget,
		chip("⭐", "Toggle favorite", a.toggleFavoriteSelected).QWidget,
	)
	lay.AddWidget(mgmt)
	return side
}

func (a *App) buildMain() *qt.QWidget {
	panel := qt.NewQWidget2()
	panel.SetObjectName(*qt.NewQAnyStringView3("mainPanel"))
	lay := qt.NewQVBoxLayout(panel)
	lay.SetContentsMargins(8, 16, 16, 12)
	tabs := qt.NewQTabWidget2()
	tabs.AddTab(wrapScroll(a.buildTabControl()).QWidget, "Control")
	tabs.AddTab(wrapScroll(a.buildTabPosition()).QWidget, "Position")
	tabs.AddTab(wrapScroll(a.buildTabColors()).QWidget, "Colors")
	tabs.AddTab(wrapScroll(a.buildTabProfiles()).QWidget, "Profiles")
	tabs.AddTab(wrapScroll(a.buildTabDiagnostics()).QWidget, "Diagnostics")
	lay.AddWidget(tabs.QWidget)
	return panel
}

func (a *App) buildTabControl() *qt.QWidget {
	tab := qt.NewQWidget2()
	lay := qt.NewQVBoxLayout(tab)
	lay.SetContentsMargins(8, 8, 8, 8)
	lay.SetSpacing(12)

	st, stL := cardBox("Runtime")
	a.runtimeInfo = qt.NewQLabel3("No managed Conky instance running")
	a.runtimeInfo.SetWordWrap(true)
	stL.AddWidget(a.runtimeInfo.QWidget)
	stL.AddWidget(muted(compositorLabel(a.desktopEnv, a.session)).QWidget)
	lay.AddWidget(st.QWidget)

	df, db := cardBox("Dashboard")
	a.dashLabels = map[string]*qt.QLabel{}
	grid := qt.NewQGridLayout2()
	grid.SetSpacing(10)
	keys := []string{"themes", "running", "favorites", "ready", "needs_fix", "broken", "recent"}
	labs := []string{"Themes", "Running", "Favorites", "Ready", "Needs fix", "Broken", "Recent"}
	for i, key := range keys {
		cell := qt.NewQWidget2()
		cl := qt.NewQVBoxLayout(cell)
		cl.SetContentsMargins(10, 10, 10, 10)
		cell.SetObjectName(*qt.NewQAnyStringView3("dashboardCard"))
		cl.AddWidget(muted(labs[i]).QWidget)
		val := qt.NewQLabel3("0")
		val.SetObjectName(*qt.NewQAnyStringView3("kpi"))
		a.dashLabels[key] = val
		cl.AddWidget(val.QWidget)
		grid.AddWidget2(cell, i/4, i%4)
	}
	db.AddLayout(grid.QLayout)
	q1 := hrow(
		a.btn("▶  Run", "accent", a.runSelected).QWidget,
		a.btn("⏹  Stop", "", a.stopSelected).QWidget,
		a.btn("🔍  Scan", "", a.healthScan).QWidget,
	)
	q2 := hrow(
		a.btn("📥  Import", "", a.importThemeFolder).QWidget,
		a.btn("🌐  Marketplace", "accent", a.showThemeStore).QWidget,
	)
	db.AddWidget(q1)
	db.AddWidget(q2)
	lay.AddWidget(df.QWidget)

	cf, cb := cardBox("Process Manager")
	row1 := hrow(
		a.btn("▶  Run All", "accent", a.runAll).QWidget,
		a.btn("⏹  Stop All Managed", "danger", a.stopAll).QWidget,
		a.btn("🧹  Kill Orphaned", "", a.killOrphans).QWidget,
	)
	row2 := hrow(
		a.btn("✓  Validate All", "", a.validateAllThemes).QWidget,
		a.btn("🔧  Fix All…", "", a.fixAllThemes).QWidget,
		a.btn("📄  Open Log", "ghost", func() { xdgOpen(logFilePath) }).QWidget,
	)
	cb.AddWidget(row1)
	cb.AddWidget(row2)
	lay.AddWidget(cf.QWidget)

	pf, pb := cardBox("Preview")
	dur := qt.NewQComboBox2()
	for _, p := range [][2]string{{"5", "5 seconds"}, {"15", "15 seconds"}, {"30", "30 seconds"}, {"0", "Unlimited"}} {
		comboAdd(dur, p[0], p[1])
	}
	comboSetID(dur, strconv.Itoa(a.settings.PreviewSeconds))
	a.previewSeconds = a.settings.PreviewSeconds
	dur.OnCurrentIndexChanged(func(_ int) {
		if n, err := strconv.Atoi(comboID(dur, "5")); err == nil {
			a.previewSeconds = n
		}
	})
	pb.AddWidget(hrow(qt.NewQLabel3("Duration").QWidget, dur.QWidget))
	pb.AddWidget(hrow(
		a.btn("▶  Start", "accent", a.previewSelected).QWidget,
		a.btn("⏹  Stop", "", a.stopPreview).QWidget,
		a.btn("🔁  Restart", "", a.restartPreview).QWidget,
		a.btn("⏸  Pause / Resume", "", a.togglePreviewPause).QWidget,
	))
	pb.AddWidget(muted("Preview uses a temporary launch bundle and does not overwrite the original theme.").QWidget)
	lay.AddWidget(pf.QWidget)

	hf, hb := cardBox("Tips")
	for _, t := range []string{
		"Each theme keeps its own monitor, position, and colors when running together.",
		"Stop All Managed never touches Conky processes started outside this app.",
		"Kill Orphaned only removes leftover manager launch-bundle processes.",
		"Favorites, health status, and recently used views live in the sidebar filter.",
		"Clone / Rename / Delete / Export update profiles and saved overrides.",
		"Detected environment: " + compositorLabel(a.desktopEnv, a.session) + ".",
	} {
		hb.AddWidget(muted("•  " + t).QWidget)
	}
	lay.AddWidget(hf.QWidget)
	lay.AddStretch()
	return tab
}

func (a *App) buildTabPosition() *qt.QWidget {
	tab := qt.NewQWidget2()
	lay := qt.NewQVBoxLayout(tab)
	lay.SetContentsMargins(8, 8, 8, 8)
	lay.AddWidget(muted("Each theme keeps its own monitor, alignment, and offset. Running multiple themes no longer shares one position.").QWidget)
	a.buildPositionPanel(lay)
	hf, hb := cardBox("Position Tips")
	for _, t := range []string{
		"Click a grid cell to set alignment (top-left, center, bottom-right, etc.).",
		"gap_x / gap_y are pixel offsets from the chosen screen edge.",
		"Use arrow buttons for fine-tuning, then Apply & Restart to preview live.",
		"Save for Theme keeps a unique position for each Conky theme.",
	} {
		hb.AddWidget(muted("•  " + t).QWidget)
	}
	lay.AddWidget(hf.QWidget)
	lay.AddStretch()
	return tab
}

func (a *App) buildPositionPanel(parent *qt.QVBoxLayout) {
	fr, box := cardBox("Alignment & Offset")
	a.posEnabled = qt.NewQCheckBox3("Control position")
	a.posEnabled.SetChecked(true)
	a.posEnabled.OnClicked(func() { a.onPosEnabled() })
	a.posMonitor = qt.NewQComboBox2()
	a.refreshMonitorCombo("")
	a.posMonitor.OnCurrentIndexChanged(func(_ int) { a.onGapChanged() })
	hdr := hrow(a.posEnabled.QWidget, qt.NewQLabel3("Monitor").QWidget, a.posMonitor.QWidget)
	box.AddWidget(hdr)

	gridW := qt.NewQWidget2()
	grid := qt.NewQGridLayout(gridW)
	grid.SetSpacing(6)
	a.posButtons = map[string]*qt.QPushButton{}
	for i, pair := range positionAlignments {
		id, symbol := pair[0], pair[1]
		b := qt.NewQPushButton3(symbol)
		b.SetObjectName(*qt.NewQAnyStringView3("chip"))
		b.SetCheckable(true)
		b.SetToolTip(strings.ReplaceAll(id, "_", " "))
		alignID := id
		b.OnClicked(func() { a.onAlignToggled(b, alignID) })
		grid.AddWidget2(b.QWidget, i/3, i%3)
		a.posButtons[id] = b
	}

	off := qt.NewQWidget2()
	ol := qt.NewQFormLayout(off)
	a.posGapX = qt.NewQSpinBox2()
	a.posGapX.SetRange(-4000, 4000)
	a.posGapX.SetValue(30)
	a.posGapX.OnValueChanged(func(_ int) { a.onGapChanged() })
	a.posGapY = qt.NewQSpinBox2()
	a.posGapY.SetRange(-4000, 4000)
	a.posGapY.SetValue(50)
	a.posGapY.OnValueChanged(func(_ int) { a.onGapChanged() })
	a.posNudge = qt.NewQSpinBox2()
	a.posNudge.SetRange(1, 100)
	a.posNudge.SetValue(10)
	ol.AddRow3("Horizontal (gap_x)", a.posGapX.QWidget)
	ol.AddRow3("Vertical (gap_y)", a.posGapY.QWidget)
	nudge := hrow(
		chip("←", "Nudge left", func() { a.nudgePosition(-1, 0) }).QWidget,
		chip("→", "Nudge right", func() { a.nudgePosition(1, 0) }).QWidget,
		chip("↑", "Nudge up", func() { a.nudgePosition(0, -1) }).QWidget,
		chip("↓", "Nudge down", func() { a.nudgePosition(0, 1) }).QWidget,
		qt.NewQLabel3("Step").QWidget,
		a.posNudge.QWidget,
	)
	ol.AddRow3("Fine tune", nudge)

	row := qt.NewQWidget2()
	rl := qt.NewQHBoxLayout(row)
	rl.AddWidget(gridW)
	rl.AddWidget2(off, 1)
	box.AddWidget(row)

	a.posStatus = muted("Select a theme to adjust its position.")
	box.AddWidget(a.posStatus.QWidget)
	a.btnSavePos = a.btn("💾  Save for Theme", "", func() { a.savePositionForTheme(true) })
	a.btnResetPos = a.btn("↺  Reset", "ghost", a.resetPositionForTheme)
	a.btnApplyPos = a.btn("▶  Apply & Restart", "accent", a.applyPositionAndRestart)
	box.AddWidget(hrow(a.btnSavePos.QWidget, a.btnResetPos.QWidget, a.btnApplyPos.QWidget))
	a.keepBusy(a.btnSavePos.QWidget)
	a.keepBusy(a.btnResetPos.QWidget)
	a.keepBusy(a.btnApplyPos.QWidget)
	a.setPositionSensitive(false)
	parent.AddWidget(fr.QWidget)
}

func (a *App) refreshMonitorCombo(selected string) {
	if a.posMonitor == nil {
		return
	}
	a.posLoading = true
	prev := selected
	if prev == "" {
		prev = comboID(a.posMonitor, "primary")
	}
	a.posMonitor.Clear()
	comboAdd(a.posMonitor, "primary", "Primary display")
	for _, m := range listMonitors() {
		scale := ""
		if m.Scale > 1 {
			scale = fmt.Sprintf(" · %d%%", m.Scale*100)
		}
		role := ""
		if m.Primary {
			role = " (primary)"
		}
		comboAdd(a.posMonitor, m.ID, fmt.Sprintf("%s%s · %dx%d%s", m.Name, role, m.Width, m.Height, scale))
	}
	comboSetID(a.posMonitor, prev)
	a.posLoading = false
}

func (a *App) setPositionSensitive(enabled bool) {
	for _, w := range []*qt.QWidget{a.posGapX.QWidget, a.posGapY.QWidget, a.posNudge.QWidget, a.btnSavePos.QWidget, a.btnResetPos.QWidget, a.btnApplyPos.QWidget, a.posMonitor.QWidget} {
		if w != nil {
			w.SetEnabled(enabled)
		}
	}
	for _, b := range a.posButtons {
		b.SetEnabled(enabled)
	}
}

func (a *App) setPositionControls(pos Position, enabled bool) {
	a.posLoading = true
	a.posEnabled.SetChecked(enabled)
	for id, btn := range a.posButtons {
		btn.SetChecked(id == pos.Alignment)
	}
	a.posGapX.SetValue(pos.GapX)
	a.posGapY.SetValue(pos.GapY)
	monID := pos.Monitor
	if monID == "" {
		monID = "primary"
	}
	a.refreshMonitorCombo(monID)
	a.posLoading = false
}

func (a *App) getPositionFromControls() *Position {
	if a.posEnabled == nil || !a.posEnabled.IsChecked() {
		return nil
	}
	al := "top_left"
	for id, btn := range a.posButtons {
		if btn.IsChecked() {
			al = id
			break
		}
	}
	monID := comboID(a.posMonitor, "primary")
	head := 0
	if mon := findMonitor(monID, 0); mon != nil {
		head = mon.Head
		monID = mon.ID
	}
	return &Position{Enabled: true, Alignment: al, GapX: a.posGapX.Value(), GapY: a.posGapY.Value(), Monitor: monID, XineramaHead: head}
}

func (a *App) onPosEnabled() {
	if a.posLoading {
		return
	}
	en := a.posEnabled.IsChecked() && a.activePosTheme != ""
	a.setPositionSensitive(en)
	if a.activePosTheme != "" {
		st := "disabled"
		if a.posEnabled.IsChecked() {
			st = "enabled"
		}
		a.posStatus.SetText(filepath.Base(filepath.Dir(a.activePosTheme)) + ": position control " + st + ".")
	}
}

func (a *App) onAlignToggled(button *qt.QPushButton, alignID string) {
	if a.posLoading {
		return
	}
	a.posLoading = true
	for other, btn := range a.posButtons {
		btn.SetChecked(other == alignID)
	}
	a.posLoading = false
	_ = button
	a.posStatus.SetText(fmt.Sprintf("Alignment: %s | X=%d, Y=%d", strings.ReplaceAll(alignID, "_", " "), a.posGapX.Value(), a.posGapY.Value()))
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
	if a.activePosTheme == "" || !a.posEnabled.IsChecked() {
		return
	}
	step := a.posNudge.Value()
	a.posLoading = true
	a.posGapX.SetValue(a.posGapX.Value() + dx*step)
	a.posGapY.SetValue(a.posGapY.Value() + dy*step)
	a.posLoading = false
	a.onGapChanged()
}

func (a *App) buildTabColors() *qt.QWidget {
	tab := qt.NewQWidget2()
	lay := qt.NewQVBoxLayout(tab)
	lay.SetContentsMargins(8, 8, 8, 8)
	lay.AddWidget(muted("Change theme colors without editing original files. Lua ring colors are synced automatically when Smart Lua Match is enabled.").QWidget)

	a.colorEnabled = qt.NewQCheckBox3("Recolor theme")
	a.colorEnabled.SetChecked(true)
	a.colorEnabled.OnClicked(func() { a.onColorEnabled() })
	a.colorPreset = qt.NewQComboBox2()
	comboAdd(a.colorPreset, "custom", "Custom")
	for _, p := range colorPresets {
		comboAdd(a.colorPreset, p.ID, p.Preset.Label)
	}
	for _, p := range loadCustomPalettes() {
		comboAdd(a.colorPreset, "custom:"+p.ID, p.Label)
	}
	a.colorPreset.OnCurrentIndexChanged(func(_ int) { a.onColorPreset() })
	lay.AddWidget(hrow(a.colorEnabled.QWidget, qt.NewQLabel3("Palette").QWidget, a.colorPreset.QWidget))

	a.colorSmartLua = qt.NewQCheckBox3("Smart Lua match")
	a.colorSmartLua.SetChecked(true)
	a.colorSmartLua.SetToolTip("Automatically recolor matching hex values inside Lua ring scripts.")
	lay.AddWidget(a.colorSmartLua.QWidget)

	fr, fl := cardBox("Theme Colors")
	scroll := qt.NewQScrollArea2()
	scroll.SetWidgetResizable(true)
	scroll.SetMinimumHeight(140)
	a.colorSlotsHost = qt.NewQWidget2()
	a.colorSlotsLay = qt.NewQVBoxLayout(a.colorSlotsHost)
	scroll.SetWidget(a.colorSlotsHost)
	fl.AddWidget(scroll.QWidget)
	lay.AddWidget2(fr.QWidget, 1)

	a.colorStatus = muted("Select a theme to detect and customize its colors.")
	lay.AddWidget(a.colorStatus.QWidget)
	a.btnSaveCol = a.btn("💾  Save for Theme", "", func() { a.saveColorsForTheme(true) })
	a.btnResetCol = a.btn("↺  Reset", "ghost", a.resetColorsForTheme)
	a.btnApplyCol = a.btn("▶  Apply & Restart", "accent", a.applyColorsAndRestart)
	palBtn := a.btn("🎨  Palettes", "", a.showPaletteManager)
	lay.AddWidget(hrow(a.btnSaveCol.QWidget, a.btnResetCol.QWidget, a.btnApplyCol.QWidget, palBtn.QWidget))
	hf, hb := cardBox("Color Tips")
	for _, t := range []string{
		"Original theme files are never modified — colors apply in launch bundles only.",
		"Palettes remap only the color slots detected in the selected theme.",
		"Smart Lua Match updates ring meters when they share the same hex values.",
		"Save for Theme keeps custom colors for each Conky theme separately.",
	} {
		hb.AddWidget(muted("•  " + t).QWidget)
	}
	lay.AddWidget(hf.QWidget)
	a.colorPickers = map[string]*qt.QPushButton{}
	a.themeColorSlots = map[string]string{}
	a.setColorSensitive(false)
	a.keepBusy(a.btnSaveCol.QWidget)
	a.keepBusy(a.btnResetCol.QWidget)
	a.keepBusy(a.btnApplyCol.QWidget)
	return tab
}

func (a *App) setColorSensitive(enabled bool) {
	on := a.colorEnabled != nil && a.colorEnabled.IsChecked()
	for _, w := range []*qt.QWidget{a.colorPreset.QWidget, a.colorSmartLua.QWidget, a.btnSaveCol.QWidget, a.btnResetCol.QWidget, a.btnApplyCol.QWidget} {
		if w != nil {
			w.SetEnabled(enabled && on)
		}
	}
	for _, p := range a.colorPickers {
		p.SetEnabled(enabled && on)
	}
}

func (a *App) clearColorPickers() {
	clearLayout(a.colorSlotsLay.QLayout)
	a.colorPickers = map[string]*qt.QPushButton{}
}

func (a *App) populateColorPickers(colors map[string]string) {
	a.clearColorPickers()
	a.themeColorSlots = cloneMap(colors)
	if len(colors) == 0 {
		a.colorSlotsLay.AddWidget(muted("No color slots detected in this theme.").QWidget)
		return
	}
	for _, slot := range orderedThemeColorSlots(colors) {
		row := qt.NewQWidget2()
		hl := qt.NewQHBoxLayout(row)
		lab := colorSlotLabels[slot]
		if lab == "" {
			lab = slot
		}
		hl.AddWidget(qt.NewQLabel3(fmt.Sprintf("%s (%s)", lab, slot)).QWidget)
		hl.AddStretch()
		hex := colors[slot]
		picker := qt.NewQPushButton3(hex)
		picker.SetCursor(qt.NewQCursor2(qt.PointingHandCursor))
		picker.SetStyleSheet(fmt.Sprintf("background:%s; color:#111; min-width:88px; border-radius:8px; font-weight:700;", hex))
		s := slot
		picker.OnClicked(func() { a.onColorPicker(picker, s) })
		hl.AddWidget(picker.QWidget)
		a.colorSlotsLay.AddWidget(row)
		a.colorPickers[slot] = picker
	}
	a.colorSlotsLay.AddStretch()
}

func (a *App) getColorsFromControls() *ColorOverride {
	if a.colorEnabled == nil || !a.colorEnabled.IsChecked() || len(a.colorPickers) == 0 {
		return nil
	}
	colors := map[string]string{}
	for slot, p := range a.colorPickers {
		colors[slot] = strings.TrimSpace(p.Text())
	}
	return &ColorOverride{Colors: colors, SmartLua: a.colorSmartLua.IsChecked(), Enabled: true}
}

func (a *App) onColorEnabled() {
	if a.colorLoading {
		return
	}
	a.setColorSensitive(a.activeColorTheme != "")
	if a.activeColorTheme != "" {
		st := "disabled"
		if a.colorEnabled.IsChecked() {
			st = "enabled"
		}
		a.colorStatus.SetText(filepath.Base(filepath.Dir(a.activeColorTheme)) + ": color customization " + st)
	}
}

func (a *App) onColorPreset() {
	if a.colorLoading || len(a.themeColorSlots) == 0 {
		return
	}
	id := comboID(a.colorPreset, "custom")
	if id == "" || id == "custom" {
		return
	}
	merged := mergePresetColors(a.themeColorSlots, id)
	a.colorLoading = true
	for slot, p := range a.colorPickers {
		if v, ok := merged[slot]; ok {
			p.SetText(v)
			p.SetStyleSheet(fmt.Sprintf("background:%s; color:#111; min-width:88px; border-radius:8px; font-weight:700;", v))
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

func (a *App) onColorPicker(picker *qt.QPushButton, slot string) {
	if a.colorLoading {
		return
	}
	c := qt.QColorDialog_GetColor3(hexToColor(picker.Text()), a.parent(), "Pick color")
	if c == nil || !c.IsValid() {
		return
	}
	hex := colorToHex(c)
	picker.SetText(hex)
	picker.SetStyleSheet(fmt.Sprintf("background:%s; color:#111; min-width:88px; border-radius:8px; font-weight:700;", hex))
	a.colorLoading = true
	comboSetID(a.colorPreset, "custom")
	a.colorLoading = false
	lab := colorSlotLabels[slot]
	if lab == "" {
		lab = slot
	}
	a.colorStatus.SetText(fmt.Sprintf("Updated %s → %s", lab, hex))
}

func (a *App) buildTabProfiles() *qt.QWidget {
	tab := qt.NewQWidget2()
	lay := qt.NewQVBoxLayout(tab)
	lay.SetContentsMargins(8, 8, 8, 8)
	nf, nb := cardBox("Profile Name")
	a.profileName = qt.NewQLineEdit2()
	a.profileName.SetPlaceholderText("My profile name...")
	a.profileName.SetText(a.profiles.LastProfile)
	nb.AddWidget(a.profileName.QWidget)
	lay.AddWidget(nf.QWidget)

	content := qt.NewQWidget2()
	hl := qt.NewQHBoxLayout(content)
	left := qt.NewQWidget2()
	ll := qt.NewQVBoxLayout(left)
	ll.AddWidget(qt.NewQLabel3("Saved Profiles").QWidget)
	a.profileList = qt.NewQListWidget2()
	a.profileList.SetSelectionMode(qt.QAbstractItemView__SingleSelection)
	a.profileList.OnCurrentItemChanged(func(cur *qt.QListWidgetItem, _ *qt.QListWidgetItem) {
		if cur != nil {
			a.profileName.SetText(cur.Data(int(qt.UserRole)).ToString())
		}
		a.showProfileDetails()
	})
	ll.AddWidget2(a.profileList.QWidget, 1)
	hl.AddWidget2(left, 1)

	right := qt.NewQWidget2()
	rl := qt.NewQVBoxLayout(right)
	btns := []*qt.QPushButton{
		a.btn("💾  Save Profile", "accent", a.saveProfile),
		a.btn("➕  Add Themes", "", a.addToProfile),
		a.btn("▶  Run Profile", "", a.runProfile),
		a.btn("📄  Duplicate", "", a.duplicateProfile),
		a.btn("📦  Export .conky-profile", "", a.exportProfile),
		a.btn("📥  Import Profile", "", a.importProfile),
		a.btn("🚀  Enable Autostart", "", a.enableAutostart),
		a.btn("⏹  Disable Autostart", "", a.disableAutostart),
		a.btn("🗑  Delete Profile", "danger", a.deleteProfile),
		a.btn("🔄  Reload", "ghost", a.refreshProfiles),
	}
	for _, b := range btns {
		rl.AddWidget(b.QWidget)
		a.keepBusy(b.QWidget)
	}
	rl.AddStretch()
	hl.AddWidget(right)
	lay.AddWidget2(content, 1)

	df, db := cardBox("Profile Contents")
	a.profileDetailsTitle = muted("No profile selected")
	db.AddWidget(a.profileDetailsTitle.QWidget)
	a.profileDetails = qt.NewQListWidget2()
	a.profileDetails.SetMinimumHeight(120)
	db.AddWidget(a.profileDetails.QWidget)
	lay.AddWidget(df.QWidget)
	return tab
}

func (a *App) buildTabDiagnostics() *qt.QWidget {
	tab := qt.NewQWidget2()
	lay := qt.NewQVBoxLayout(tab)
	lay.AddWidget(hrow(a.btn("🔄  Refresh Log", "", func() { a.refreshLogView(250) }).QWidget))
	fr, box := cardBox("Application Log")
	a.logsView = qt.NewQPlainTextEdit2()
	a.logsView.SetReadOnly(true)
	font := qt.NewQFont6("JetBrains Mono", 11)
	a.logsView.SetFont(font)
	box.AddWidget(a.logsView.QWidget)
	lay.AddWidget2(fr.QWidget, 1)
	return tab
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
			l.SetText(strconv.Itoa(n))
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
		query = a.searchEntry.Text()
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if a.themeList == nil {
		return
	}
	a.themeList.Clear()
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
			star = "★ "
		}
		haystack := strings.ToLower(star + root + " " + filepath.Base(item.Path) + " " + item.Path + " " + health + " " + runtime)
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}
		card := qt.NewQWidget2()
		vl := qt.NewQVBoxLayout(card)
		vl.SetContentsMargins(10, 8, 10, 8)
		vl.SetSpacing(3)
		top := qt.NewQWidget2()
		hl := qt.NewQHBoxLayout(top)
		hl.SetContentsMargins(0, 0, 0, 0)
		check := qt.NewQCheckBox2()
		check.SetChecked(a.selectedThemeSet[resolvePath(item.Path)])
		check.SetToolTip("Select this theme for multi-theme actions")
		path := item.Path
		check.OnClicked(func() {
			a.selectedThemeSet[resolvePath(path)] = check.IsChecked()
			if check.IsChecked() {
				a.loadPositionForSelected()
				a.loadColorsForSelected()
			}
		})
		name := qt.NewQLabel3(star + root)
		name.SetObjectName(*qt.NewQAnyStringView3("themeName"))
		badge := qt.NewQLabel3(themeListStatus(health, runtime))
		badge.SetObjectName(*qt.NewQAnyStringView3("themeBadge"))
		hl.AddWidget(check.QWidget)
		hl.AddWidget2(name.QWidget, 1)
		hl.AddWidget(badge.QWidget)
		vl.AddWidget(top)
		cfg := muted(filepath.Base(item.Path))
		vl.AddWidget(cfg.QWidget)
		pidTxt := runtime
		if pid > 0 {
			pidTxt = fmt.Sprintf("running · pid %d", pid)
		}
		vl.AddWidget(muted(fmt.Sprintf("%s  ·  %s", pidTxt, filepath.Dir(item.Path))).QWidget)
		itemW := qt.NewQListWidgetItem()
		a.themeList.AddItemWithItem(itemW)
		a.themeList.SetItemWidget(itemW, card)
		card.AdjustSize()
		sz := card.SizeHint()
		h := 78
		if sz != nil && sz.Height() > h {
			h = sz.Height() + 8
		}
		itemW.SetSizeHint(qt.NewQSize2(260, h))
		itemW.SetData(int(qt.UserRole), qt.NewQVariant11(item.Path))
	}
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

func (a *App) showThemeContextMenu(global *qt.QPoint) {
	th := a.selectedThemeItem()
	if th == nil {
		return
	}
	menu := qt.NewQMenu2()
	addAct(menu, "Open theme file", func() { xdgOpen(th.Path) })
	addAct(menu, "Edit theme file", func() { xdgOpen(th.Path) })
	addAct(menu, "Open containing folder", func() { xdgOpen(filepath.Dir(th.Path)) })
	menu.AddSeparator()
	addAct(menu, "Open optimized file", func() {
		p, err := optimizeForDesktop(th.Path)
		if err == nil {
			xdgOpen(p)
		}
	})
	menu.AddSeparator()
	addAct(menu, "Validate (original + optimized)", a.validateSelected)
	addAct(menu, "Preview (temporary)", a.previewSelected)
	menu.ExecWithPos(global)
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
		a.colorEnabled.SetChecked(true)
		a.colorSmartLua.SetChecked(saved.SmartLua)
		comboSetID(a.colorPreset, "custom")
		a.populateColorPickers(display)
		a.colorLoading = false
		a.setColorSensitive(true)
		a.colorStatus.SetText(fmt.Sprintf("%s: saved custom colors (%d slots)", filepath.Base(filepath.Dir(th.Path)), len(saved.Colors)))
		return
	}
	a.colorLoading = true
	a.colorEnabled.SetChecked(true)
	a.colorSmartLua.SetChecked(true)
	comboSetID(a.colorPreset, "custom")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return nil
	}
	pos := a.getPositionFromControls()
	if pos == nil {
		a.showMessage("Info", "Enable position control first.", "info")
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
		a.showMessage("Original file was not updated", err.Error(), "error")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	data := loadPositions()
	delete(data.Themes, resolvePath(th.Path))
	savePositions(data)
	a.loadPositionForSelected()
	a.pushStatus("Position reset for " + filepath.Base(filepath.Dir(th.Path)) + ".")
}

func (a *App) applyPositionAndRestart() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	pos := a.savePositionForTheme(false)
	if pos == nil {
		return
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
		return
	}
	a.setBusy(true)
	a.pushStatus("Applying position for " + filepath.Base(filepath.Dir(th.Path)) + "...")
	var colors *ColorOverride
	if a.colorEnabled.IsChecked() {
		colors = a.getColorsFromControls()
	}
	path := th.Path
	go func() {
		defer idle(func() { a.setBusy(false) })
		stopManagedTheme(path)
		res, err := runConky(path, pos, colors)
		if err != nil {
			idle(func() { a.showMessage("Position Error", err.Error(), "error") })
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
		a.showMessage("Info", "Select a theme first.", "info")
		return nil
	}
	payload := a.getColorsFromControls()
	if payload == nil {
		a.showMessage("Info", "Enable recoloring and select at least one color.", "info")
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
		a.showMessage("Original file was not updated", err.Error(), "error")
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
		a.showMessage("Info", "Select a theme first.", "info")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	colors := a.saveColorsForTheme(false)
	if colors == nil {
		return
	}
	var pos *Position
	if a.posEnabled.IsChecked() {
		pos = a.getPositionFromControls()
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
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
			idle(func() { a.showMessage("Color Error", err.Error(), "error") })
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
		a.statusIndicator.SetText("●  Running")
		a.statusIndicator.SetObjectName(*qt.NewQAnyStringView3("live"))
		if detail == "" {
			detail = "Managed Conky is active"
		}
		a.runtimeInfo.SetText(detail)
	} else {
		a.statusIndicator.SetText("●  Idle")
		a.statusIndicator.SetObjectName(*qt.NewQAnyStringView3("muted"))
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
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
		return
	}
	overlay := map[string]themeRunOverride{}
	if a.activePosTheme != "" {
		ov := themeRunOverride{}
		if a.posEnabled != nil && a.posEnabled.IsChecked() {
			ov.pos = a.getPositionFromControls()
			if a.settings.RememberPosition {
				a.savePositionForTheme(false)
			}
		}
		if a.colorEnabled != nil && a.colorEnabled.IsChecked() {
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
		a.showMessage("Info", "Select at least one theme.", "info")
		return
	}
	a.runThemes(th)
}

func (a *App) runAll() {
	if len(a.themes) == 0 {
		a.showMessage("Info", "No valid themes found.", "info")
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
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
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
			a.showMessage("Repair", fmt.Sprintf("Prepared full launch bundles for %d theme(s), failed %d.\nAsset paths, fonts, and visuals are relinked automatically.", o, f), "info")
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
		a.showMessage("Info", "Select at least one theme.", "info")
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
		a.showMessage("Info", "Select at least one theme.", "info")
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
	if a.previewTimer != nil {
		a.previewTimer.Stop()
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
		a.showMessage("Info", "No preview process is running.", "info")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
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
		a.showMessage("Preview Error", err.Error(), "error")
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
	if seconds > 0 {
		pid := res.PID
		path := th.Path
		if a.previewTimer == nil {
			a.previewTimer = qt.NewQTimer2(a.win.QObject)
			a.previewTimer.SetSingleShot(true)
			a.previewTimer.OnTimeout(func() {
				stopPID(pid, true)
				stopManagedTheme(path)
				a.applyFilter("")
				a.pushStatus("Preview stopped.")
			})
		}
		a.previewTimer.Start(seconds * 1000)
	}
}

func (a *App) editSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	box := qt.NewQMessageBox6(qt.QMessageBox__Question, "Edit theme",
		fmt.Sprintf("Open %s in the built-in editor or an external application.", filepath.Base(th.Path)),
		qt.QMessageBox__NoButton, a.parent())
	builtin := box.AddButton2("Built-in editor", qt.QMessageBox__YesRole)
	ext := box.AddButton2("External editor", qt.QMessageBox__NoRole)
	box.AddButton2("Cancel", qt.QMessageBox__RejectRole)
	box.Exec()
	clicked := box.ClickedButton()
	if clicked != nil && clicked.Text() == builtin.Text() {
		a.showEmbeddedEditor(th.Path)
		return
	}
	if clicked != nil && clicked.Text() == ext.Text() {
		xdgOpen(th.Path)
	}
}

func (a *App) openSelectedFolder() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	xdgOpen(filepath.Dir(th.Path))
}

func (a *App) importThemeFolder() {
	folder := qt.QFileDialog_GetExistingDirectory3(a.parent(), "Import Theme Folder", homeDir)
	if folder == "" {
		return
	}
	a.performImport(func() ([]string, error) { return importFolderToConky(folder, "") })
}

func (a *App) importThemeArchive() {
	filt := "Archives (*" + strings.Join(archiveExts, " *") + ");;All files (*)"
	archive := qt.QFileDialog_GetOpenFileName4(a.parent(), "Import Theme Archive", homeDir, filt)
	if archive == "" {
		return
	}
	a.performImport(func() ([]string, error) { return importArchiveToConky(archive, "") })
}

func (a *App) performImport(fn func() ([]string, error)) {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
		return
	}
	a.setBusy(true)
	a.pushStatus("Importing into " + defaultImportDir + "…")
	go func() {
		installed, err := fn()
		idle(func() {
			a.setBusy(false)
			if err != nil {
				a.showMessage("Import Error", err.Error(), "error")
				return
			}
			a.loadThemes()
			a.pushStatus(fmt.Sprintf("Imported %d item(s) into ~/.conky", len(installed)))
			a.showMessage("Import Complete", fmt.Sprintf("Installed into:\n%s\n\nFolders: %s", defaultImportDir, strings.Join(installed, ", ")), "info")
		})
	}()
}

func (a *App) refreshProfiles() {
	a.profiles = loadProfiles()
	a.profileList.Clear()
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
		it := qt.NewQListWidgetItem2(fmt.Sprintf("%s\n%d theme(s)", name, len(a.profiles.Profiles[name])))
		it.SetData(int(qt.UserRole), qt.NewQVariant11(name))
		a.profileList.AddItemWithItem(it)
	}
	a.showProfileDetails()
}

func (a *App) selectedProfileName() string {
	it := a.profileList.CurrentItem()
	if it != nil {
		if n := it.Data(int(qt.UserRole)).ToString(); n != "" {
			return n
		}
	}
	return strings.TrimSpace(a.profileName.Text())
}

func (a *App) saveProfile() {
	name := strings.TrimSpace(a.profileName.Text())
	if name == "" {
		a.showMessage("Error", "Enter a profile name.", "error")
		return
	}
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Error", "Select at least one theme in the sidebar.", "error")
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
		a.showMessage("Error", "Select or enter a profile name.", "error")
		return
	}
	if _, ok := a.profiles.Profiles[name]; !ok {
		a.showMessage("Error", "Profile not found: "+name, "error")
		return
	}
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Error", "Select themes in the sidebar first.", "error")
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
	a.profileDetails.Clear()
	name := a.selectedProfileName()
	if name == "" {
		a.profileDetailsTitle.SetText("No profile selected")
		return
	}
	if dp, ok := a.profiles.Desktop[name]; ok && len(dp.Themes) > 0 {
		for _, t := range dp.Themes {
			mon := t.Monitor
			if mon == "" {
				mon = t.Position.Monitor
			}
			if mon == "" {
				mon = "primary"
			}
			a.profileDetails.AddItem(fmt.Sprintf("%s\n  monitor=%s  %s  gap=%d,%d  enabled=%v", t.Path, mon, t.Position.Alignment, t.Position.GapX, t.Position.GapY, t.Enabled))
		}
		a.profileDetailsTitle.SetText(fmt.Sprintf("%s — %d theme(s) with per-theme desktop settings", name, len(dp.Themes)))
		return
	}
	items := a.profiles.Profiles[name]
	a.profileDetailsTitle.SetText(fmt.Sprintf("%s — %d theme(s)", name, len(items)))
	for _, p := range items {
		a.profileDetails.AddItem(p)
	}
}

func (a *App) deleteProfile() {
	name := a.selectedProfileName()
	if name == "" {
		return
	}
	if !confirmDialog(a.parent(), "Delete Profile", "Delete profile '"+name+"'?") {
		return
	}
	delete(a.profiles.Profiles, name)
	delete(a.profiles.Desktop, name)
	saveProfiles(a.profiles)
	a.refreshProfiles()
}

func (a *App) runProfile() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select a profile.", "error")
		return
	}
	themePaths := a.profiles.Profiles[name]
	if themePaths == nil {
		a.showMessage("Error", "Profile not found: "+name, "error")
		return
	}
	var themes []string
	for _, p := range themePaths {
		if isFile(p) {
			themes = append(themes, p)
		}
	}
	if len(themes) == 0 {
		a.showMessage("Error", "No existing themes in this profile.", "error")
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
		a.showMessage("Error", "Select a profile first.", "error")
		return
	}
	if _, ok := a.profiles.Profiles[name]; !ok {
		if _, ok2 := a.profiles.Desktop[name]; !ok2 {
			a.showMessage("Error", "Profile not found: "+name, "error")
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
		a.showMessage("Error", "Failed to enable autostart: "+err.Error(), "error")
		return
	}
	if fileExists(oldAutostartFile) && oldAutostartFile != autostartFile {
		_ = os.Remove(oldAutostartFile)
	}
	a.showMessage("Autostart", fmt.Sprintf("Enabled XDG autostart for profile: %s\n\nDesktop: %s\nThe .desktop file has no OnlyShowIn filter so GNOME, KDE, XFCE, Cinnamon, MATE, LXQt, Hyprland, Niri, Sway, and COSMIC can all load it when they honor ~/.config/autostart.", name, compositorLabel(a.desktopEnv, a.session)), "info")
}

func (a *App) disableAutostart() {
	for _, p := range []string{autostartFile, oldAutostartFile} {
		_ = os.Remove(p)
	}
	a.showMessage("Autostart", "Autostart disabled.", "info")
}

func (a *App) validateSelected() {
	themes := a.selectedThemePaths()
	if len(themes) == 0 {
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	cfg := themes[0]
	bundle, err := createThemeLaunchBundle(cfg, true, nil, nil)
	if err != nil {
		a.showMessage("Validation Error", err.Error(), "error")
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
	a.showMessage("Validation", fmt.Sprintf("Theme: %s\nLaunch bundle: %s\nPath fixes: %d\nAssets found: %d\nMissing assets:\n%s", filepath.Base(cfg), okTxt, bundle.PathFixes, len(found), miss), "info")
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
	a.logsView.SetPlainText(text)
}

func (a *App) appendLog(line string) {
	a.logsView.AppendPlainText(line)
}

func (a *App) healthScan() {
	if a.workerBusy {
		a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
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

func (a *App) validateAllThemes() { a.healthScan() }

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
		a.showMessage("Fix All", "No repairable issues were found. Run Scan first for detailed results.", "info")
		return
	}
	msg := "The following launch bundles will be rebuilt. Original theme files stay untouched unless a saved color/position override is applied.\n\n" + strings.Join(preview, "\n")
	if len(msg) > 4000 {
		msg = msg[:4000] + "\n…"
	}
	if !confirmDialog(a.parent(), "Fix All Themes", msg) {
		return
	}
	a.smartRepair()
}

func (a *App) toggleFavoriteSelected() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	name, ok := promptText(a.parent(), "Clone Theme", "New folder name:", filepath.Base(resolveThemeRoot(th.Path))+"-copy")
	if !ok || name == "" {
		return
	}
	newCfg, err := cloneTheme(th.Path, name)
	if err != nil {
		a.showMessage("Clone Error", err.Error(), "error")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	old := resolveThemeRoot(th.Path)
	name, ok := promptText(a.parent(), "Rename Theme", "New folder name:", filepath.Base(old))
	if !ok || name == "" {
		return
	}
	newCfg, err := renameTheme(th.Path, name)
	if err != nil {
		a.showMessage("Rename Error", err.Error(), "error")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	root := resolveThemeRoot(th.Path)
	if !confirmDialog(a.parent(), "Move to Trash", "Move this theme folder to the trash?\n\n"+root+"\n\nProfiles and saved overrides for this path will be cleaned up.") {
		return
	}
	stopManagedTheme(th.Path)
	if err := trashPath(root); err != nil {
		a.showMessage("Delete Error", err.Error(), "error")
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
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	dest := qt.QFileDialog_GetSaveFileName4(a.parent(), "Export Theme", filepath.Join(homeDir, filepath.Base(resolveThemeRoot(th.Path))+".conky-theme.tar.gz"), "Archives (*.tar.gz)")
	if dest == "" {
		return
	}
	if err := exportThemeArchive(th.Path, dest); err != nil {
		a.showMessage("Export Error", err.Error(), "error")
		return
	}
	a.pushStatus("Exported " + dest)
}

func (a *App) showThemeDetails() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
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
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Theme Details")
	dlg.Resize(680, 540)
	vl := qt.NewQVBoxLayout(dlg.QWidget)
	view := qt.NewQPlainTextEdit2()
	view.SetReadOnly(true)
	view.SetPlainText(b.String())
	vl.AddWidget(view.QWidget)
	if len(missing) > 0 {
		copyBtn := a.btn("Copy install command", "", func() {
			cmd := suggestedInstallCommand(missing)
			qt.QGuiApplication_Clipboard().SetText(cmd)
			a.pushStatus("Install command copied to clipboard.")
		})
		vl.AddWidget(copyBtn.QWidget)
	}
	bb := qt.NewQDialogButtonBox7(qt.QDialogButtonBox__Close, dlg.QWidget)
	bb.OnRejected(func() { dlg.Reject() })
	vl.AddWidget(bb.QWidget)
	dlg.Exec()
	a.applyFilter("")
}

func (a *App) showPaletteManager() {
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Color Palette Manager")
	dlg.Resize(540, 440)
	box := qt.NewQVBoxLayout(dlg.QWidget)
	box.AddWidget(muted("Built-in palettes stay available. Custom palettes are saved in palettes.json.").QWidget)
	list := qt.NewQListWidget2()
	box.AddWidget(list.QWidget)
	accentHex := "#7AA2F7"
	accentBtn := qt.NewQPushButton3(accentHex)
	accentBtn.SetStyleSheet("background:#7AA2F7; color:#111; font-weight:700; min-width:88px; border-radius:8px;")
	accentBtn.OnClicked(func() {
		c := qt.QColorDialog_GetColor3(hexToColor(accentHex), dlg.QWidget, "Accent")
		if c != nil && c.IsValid() {
			accentHex = colorToHex(c)
			accentBtn.SetText(accentHex)
			accentBtn.SetStyleSheet(fmt.Sprintf("background:%s; color:#111; font-weight:700; min-width:88px; border-radius:8px;", accentHex))
		}
	})
	nameEnt := qt.NewQLineEdit2()
	nameEnt.SetPlaceholderText("Custom palette name")
	refresh := func() {
		list.Clear()
		for _, p := range colorPresets {
			list.AddItem("Preset · " + p.Preset.Label)
		}
		for _, p := range loadCustomPalettes() {
			list.AddItem("Custom · " + p.Label)
		}
	}
	refresh()
	saveBtn := a.btn("Save current colors as palette", "", func() {
		label := strings.TrimSpace(nameEnt.Text())
		if label == "" {
			a.showMessage("Info", "Enter a palette name.", "info")
			return
		}
		cols := map[string]string{}
		if ov := a.getColorsFromControls(); ov != nil {
			cols = ov.Colors
		} else {
			cols = generatePaletteFromAccent(accentHex)
		}
		all := loadCustomPalettes()
		all = append(all, CustomPalette{ID: normalizeFolderName(label), Label: label, Colors: cols})
		saveCustomPalettes(all)
		if a.colorPreset != nil {
			comboAdd(a.colorPreset, "custom:"+normalizeFolderName(label), label)
		}
		refresh()
	})
	genBtn := a.btn("Generate from accent", "", func() {
		pal := generatePaletteFromAccent(accentHex)
		a.colorLoading = true
		for slot, p := range a.colorPickers {
			if v, ok := pal[slot]; ok {
				p.SetText(v)
				p.SetStyleSheet(fmt.Sprintf("background:%s; color:#111; min-width:88px; border-radius:8px; font-weight:700;", v))
			}
		}
		a.colorLoading = false
		a.colorStatus.SetText("Generated palette from accent color.")
	})
	box.AddWidget(hrow(qt.NewQLabel3("Accent").QWidget, accentBtn.QWidget))
	box.AddWidget(nameEnt.QWidget)
	box.AddWidget(saveBtn.QWidget)
	box.AddWidget(genBtn.QWidget)
	bb := qt.NewQDialogButtonBox7(qt.QDialogButtonBox__Close, dlg.QWidget)
	bb.OnRejected(func() { dlg.Reject() })
	box.AddWidget(bb.QWidget)
	dlg.Exec()
}

func (a *App) showBackupHistory() {
	th := a.selectedThemeItem()
	if th == nil {
		a.showMessage("Info", "Select a theme first.", "info")
		return
	}
	backs := listBackups(th.Path)
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Backup History")
	dlg.Resize(740, 500)
	box := qt.NewQVBoxLayout(dlg.QWidget)
	box.AddWidget(muted("Original files stay in place. Restore copies a backup over the current config after creating a new backup.").QWidget)
	combo := qt.NewQComboBox2()
	for _, p := range backs {
		comboAdd(combo, p, filepath.Base(p))
	}
	box.AddWidget(combo.QWidget)
	diffView := qt.NewQPlainTextEdit2()
	diffView.SetReadOnly(true)
	diffView.SetFont(qt.NewQFont6("JetBrains Mono", 10))
	box.AddWidget(diffView.QWidget)
	showDiff := func() {
		p := comboID(combo, "")
		if p == "" {
			diffView.SetPlainText("No backups for this theme yet.")
			return
		}
		diffView.SetPlainText(unifiedDiff(readText(p), readText(th.Path), filepath.Base(p), filepath.Base(th.Path)))
	}
	combo.OnCurrentIndexChanged(func(_ int) { showDiff() })
	showDiff()
	rst := a.btn("Restore selected", "accent", func() {
		p := comboID(combo, "")
		if p == "" {
			return
		}
		if err := writeOriginalThemeConfig(th.Path, readText(p), "restore"); err != nil {
			a.showMessage("Restore Error", err.Error(), "error")
			return
		}
		a.pushStatus("Restored backup " + filepath.Base(p))
		showDiff()
	})
	del := a.btn("Delete selected backup", "danger", func() {
		p := comboID(combo, "")
		if p == "" {
			return
		}
		_ = os.Remove(p)
		combo.Clear()
		backs = listBackups(th.Path)
		for _, x := range backs {
			comboAdd(combo, x, filepath.Base(x))
		}
		showDiff()
	})
	box.AddWidget(hrow(rst.QWidget, del.QWidget))
	bb := qt.NewQDialogButtonBox7(qt.QDialogButtonBox__Close, dlg.QWidget)
	bb.OnRejected(func() { dlg.Reject() })
	box.AddWidget(bb.QWidget)
	dlg.Exec()
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
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Config Editor — " + filepath.Base(path))
	dlg.Resize(860, 580)
	box := qt.NewQVBoxLayout(dlg.QWidget)
	search := qt.NewQLineEdit2()
	search.SetPlaceholderText("Search")
	line := qt.NewQSpinBox2()
	line.SetRange(1, 99999)
	view := qt.NewQPlainTextEdit2()
	view.SetFont(qt.NewQFont6("JetBrains Mono", 11))
	view.SetPlainText(original)
	find := func() {
		q := search.Text()
		if q == "" {
			return
		}
		view.Find(q)
	}
	search.OnReturnPressed(find)
	fb := a.btn("Find", "", find)
	gb := a.btn("Go to line", "", func() {
		view.MoveCursor(qt.QTextCursor__Start)
		for i := 1; i < line.Value(); i++ {
			view.MoveCursor(qt.QTextCursor__Down)
		}
		view.EnsureCursorVisible()
	})
	box.AddWidget(hrow(search.QWidget, fb.QWidget, line.QWidget, gb.QWidget))
	box.AddWidget2(view.QWidget, 1)
	act := hrow(
		a.btn("Save", "accent", func() {
			if err := writeOriginalThemeConfig(path, view.ToPlainText(), "editor"); err != nil {
				a.showMessage("Save Error", err.Error(), "error")
				return
			}
			a.pushStatus("Saved " + filepath.Base(path))
		}).QWidget,
		a.btn("Revert", "ghost", func() { view.SetPlainText(original) }).QWidget,
		a.btn("Validate", "", func() {
			tmp := filepath.Join(os.TempDir(), "conky-manager-validate.conf")
			_ = os.WriteFile(tmp, []byte(view.ToPlainText()), 0o644)
			ok := validateConkyConfig(tmp, filepath.Dir(path), conkyEnv())
			_ = os.Remove(tmp)
			if ok {
				a.showMessage("Validate", "Conky accepted this configuration.", "info")
			} else {
				a.showMessage("Validate", "Conky rejected this configuration. See manager.log.", "error")
			}
		}).QWidget,
		a.btn("Save As…", "", func() {
			dest := qt.QFileDialog_GetSaveFileName4(dlg.QWidget, "Save As", filepath.Join(filepath.Dir(path), filepath.Base(path)), "All files (*)")
			if dest != "" {
				_ = os.WriteFile(dest, []byte(view.ToPlainText()), 0o644)
			}
		}).QWidget,
	)
	box.AddWidget(act)
	bb := qt.NewQDialogButtonBox7(qt.QDialogButtonBox__Close, dlg.QWidget)
	bb.OnRejected(func() { dlg.Reject() })
	box.AddWidget(bb.QWidget)
	dlg.Exec()
}

func (a *App) duplicateProfile() {
	name := a.selectedProfileName()
	if name == "" {
		a.showMessage("Error", "Select a profile first.", "error")
		return
	}
	newName, ok := promptText(a.parent(), "Duplicate Profile", "New profile name:", name+" copy")
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
	dest := qt.QFileDialog_GetSaveFileName4(a.parent(), "Export Profile", filepath.Join(homeDir, normalizeFolderName(name)+".conky-profile"), "Profiles (*.conky-profile)")
	if dest == "" {
		return
	}
	dp := a.profiles.Desktop[name]
	if dp.Name == "" {
		dp = captureDesktopProfile(name, a.profiles.Profiles[name])
	}
	if err := saveJSON(dest, dp); err != nil {
		a.showMessage("Export Error", err.Error(), "error")
		return
	}
	a.pushStatus("Exported profile to " + dest)
}

func (a *App) importProfile() {
	src := qt.QFileDialog_GetOpenFileName4(a.parent(), "Import Profile", homeDir, "Profiles (*.conky-profile);;All files (*)")
	if src == "" {
		return
	}
	var dp DesktopProfile
	if err := loadJSON(src, &dp); err != nil || dp.Name == "" {
		a.showMessage("Import Error", "Not a valid .conky-profile file.", "error")
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

func (a *App) showWaylandNotice() {
	if !strings.Contains(strings.ToLower(a.session), "wayland") {
		return
	}
	kind := displayServerKind()
	msg := fmt.Sprintf("Detected %s on %s (%s).\n\nWindow hints were selected for this compositor automatically.\nIf a theme is invisible, try Layer = above and Window type = dock in Settings.\nXWayland is used when DISPLAY is also available.",
		strings.ToUpper(a.desktopEnv), a.session, kind)
	a.showMessage("Desktop compatibility", msg, "info")
}

func (a *App) showAbout() {
	qt.QMessageBox_About(a.parent(), appName, fmt.Sprintf("<h2>%s %s</h2><p>Universal Conky theme manager for Linux<br/>(KDE, GNOME, XFCE, Cinnamon, MATE, LXQt, and more)</p><p>Go + Qt 6 edition</p>", appName, appVersion))
}

func (a *App) showSettings() {
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Settings")
	dlg.Resize(560, 640)
	outer := qt.NewQVBoxLayout(dlg.QWidget)
	scroll := qt.NewQScrollArea2()
	scroll.SetWidgetResizable(true)
	innerW := qt.NewQWidget2()
	inner := qt.NewQVBoxLayout(innerW)
	scroll.SetWidget(innerW)
	outer.AddWidget(scroll.QWidget)

	deF, deB := cardBox("Desktop Environment")
	preset := qt.NewQComboBox2()
	for _, p := range [][2]string{
		{"auto", "Auto-detect (" + a.desktopEnv + ")"}, {"kde", "KDE / Plasma"}, {"gnome", "GNOME"},
		{"xfce", "XFCE"}, {"cinnamon", "Cinnamon"}, {"mate", "MATE"}, {"lxqt", "LXQt"}, {"lxde", "LXDE"},
		{"hyprland", "Hyprland"}, {"niri", "Niri"}, {"sway", "Sway"}, {"cosmic", "COSMIC"}, {"generic", "Generic / Other"},
	} {
		comboAdd(preset, p[0], p[1])
	}
	comboSetID(preset, a.settings.DesktopPreset)
	optimize := qt.NewQCheckBox3("Auto-optimize themes")
	optimize.SetChecked(a.settings.AutoOptimize)
	visual := qt.NewQCheckBox3("Full visual launch")
	visual.SetChecked(a.settings.FullVisualLaunch)
	compat := qt.NewQCheckBox3("Compat symlinks")
	compat.SetChecked(a.settings.CreateCompatSymlinks)
	remember := qt.NewQCheckBox3("Remember position")
	remember.SetChecked(a.settings.RememberPosition)
	rememberC := qt.NewQCheckBox3("Remember colors")
	rememberC.SetChecked(a.settings.RememberColors)
	scaleAware := qt.NewQCheckBox3("Scale-aware gaps")
	scaleAware.SetChecked(a.settings.ScaleAwarePosition)
	form := qt.NewQFormLayout2()
	form.AddRow3("Environment preset", preset.QWidget)
	deB.AddLayout(form.QLayout)
	for _, sw := range []*qt.QCheckBox{optimize, visual, compat, remember, rememberC, scaleAware} {
		deB.AddWidget(sw.QWidget)
	}
	inner.AddWidget(deF.QWidget)

	posF, posB := cardBox("Default Desktop Position")
	positions := loadPositions()
	def := positions.Default
	defEn := qt.NewQCheckBox3("Enable by default")
	defEn.SetChecked(def.Enabled)
	defAlign := qt.NewQComboBox2()
	for _, p := range positionAlignments {
		comboAdd(defAlign, p[0], p[1]+" "+strings.ReplaceAll(p[0], "_", " "))
	}
	comboSetID(defAlign, def.Alignment)
	defGX := qt.NewQSpinBox2()
	defGX.SetRange(-4000, 4000)
	defGX.SetValue(def.GapX)
	defGY := qt.NewQSpinBox2()
	defGY.SetRange(-4000, 4000)
	defGY.SetValue(def.GapY)
	pf := qt.NewQFormLayout2()
	pf.AddRow3("Default alignment", defAlign.QWidget)
	pf.AddRow3("Default gap_x", defGX.QWidget)
	pf.AddRow3("Default gap_y", defGY.QWidget)
	posB.AddWidget(defEn.QWidget)
	posB.AddLayout(pf.QLayout)
	inner.AddWidget(posF.QWidget)

	winF, winB := cardBox("Window Behavior")
	layer := qt.NewQComboBox2()
	comboAdd(layer, "above", "Above windows (overlay)")
	comboAdd(layer, "below", "Below windows (background)")
	comboSetID(layer, a.settings.Layer)
	wtype := qt.NewQComboBox2()
	comboAdd(wtype, "dock", "Dock")
	comboAdd(wtype, "desktop", "Desktop (GNOME/XFCE/MATE)")
	comboAdd(wtype, "normal", "Normal window")
	comboSetID(wtype, a.settings.WindowType)
	wf := qt.NewQFormLayout2()
	wf.AddRow3("Layer", layer.QWidget)
	wf.AddRow3("Window type", wtype.QWidget)
	winB.AddLayout(wf.QLayout)
	inner.AddWidget(winF.QWidget)

	perfF, perfB := cardBox("Performance")
	smooth := qt.NewQComboBox2()
	comboAdd(smooth, "ultra", "Ultra smooth (less CPU)")
	comboAdd(smooth, "balanced", "Balanced")
	comboAdd(smooth, "performance", "Performance (faster updates)")
	comboSetID(smooth, a.settings.Smoothness)
	nice := qt.NewQSpinBox2()
	nice.SetRange(-5, 19)
	nice.SetValue(a.settings.NiceLevel)
	maxN := qt.NewQSpinBox2()
	maxN.SetRange(1, 16)
	maxN.SetValue(a.settings.MaxInstances)
	health := qt.NewQDoubleSpinBox2()
	health.SetRange(0.4, 5.0)
	health.SetDecimals(1)
	health.SetSingleStep(0.1)
	health.SetValue(a.settings.HealthcheckSeconds)
	pef := qt.NewQFormLayout2()
	pef.AddRow3("Smoothness", smooth.QWidget)
	pef.AddRow3("Nice level", nice.QWidget)
	pef.AddRow3("Max instances", maxN.QWidget)
	pef.AddRow3("Healthcheck (sec)", health.QWidget)
	perfB.AddLayout(pef.QLayout)
	inner.AddWidget(perfF.QWidget)

	tf, tb := cardBox("Preview & Interface")
	prev := qt.NewQSpinBox2()
	prev.SetRange(2, 60)
	prev.SetValue(a.settings.PreviewSeconds)
	ui := qt.NewQComboBox2()
	comboAdd(ui, "auto", "Auto (system)")
	comboAdd(ui, "light", "Light")
	comboAdd(ui, "dark", "Dark")
	comboSetID(ui, a.settings.UITheme)
	tif := qt.NewQFormLayout2()
	tif.AddRow3("Preview seconds", prev.QWidget)
	tif.AddRow3("UI theme", ui.QWidget)
	tb.AddLayout(tif.QLayout)
	inner.AddWidget(tf.QWidget)

	dirF, dirB := cardBox("Custom Theme Directories")
	dirB.AddWidget(muted("One directory per line. These folders are included in theme discovery.").QWidget)
	dirView := qt.NewQPlainTextEdit2()
	dirView.SetPlainText(strings.Join(append([]string{}, a.settings.CustomThemeDirs...), "\n"))
	dirView.SetMinimumHeight(90)
	dirB.AddWidget(dirView.QWidget)
	inner.AddWidget(dirF.QWidget)
	inner.AddStretch()

	bb := qt.NewQDialogButtonBox2()
	saveB := bb.AddButton2("Save", qt.QDialogButtonBox__AcceptRole)
	bb.AddButton2("Cancel", qt.QDialogButtonBox__RejectRole)
	resetB := bb.AddButton2("Reset Defaults", qt.QDialogButtonBox__ResetRole)
	resetB.OnClicked(func() {
		a.settings = defaultSettings()
		_ = saveJSON(settingsFile, a.settings)
		savePositions(defaultPositions())
		saveColors(ColorsFile{Themes: map[string]ColorOverride{}})
		a.applyUITheme()
		a.pushStatus("Settings reset to defaults.")
		dlg.Reject()
	})
	saveB.OnClicked(func() {
		var custom []string
		for _, line := range strings.Split(dirView.ToPlainText(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				custom = append(custom, line)
			}
		}
		a.settings = Settings{
			Layer: comboID(layer, "above"), WindowType: comboID(wtype, "dock"),
			Smoothness: comboID(smooth, "balanced"), NiceLevel: nice.Value(),
			MaxInstances: maxN.Value(), HealthcheckSeconds: health.Value(),
			PreviewSeconds: prev.Value(), UITheme: comboID(ui, "auto"),
			DesktopPreset: comboID(preset, "auto"), AutoOptimize: optimize.IsChecked(),
			FullVisualLaunch: visual.IsChecked(), CreateCompatSymlinks: compat.IsChecked(),
			RememberPosition: remember.IsChecked(), RememberColors: rememberC.IsChecked(),
			ScaleAwarePosition: scaleAware.IsChecked(), CustomThemeDirs: custom,
		}
		_ = saveJSON(settingsFile, a.settings)
		positions = loadPositions()
		positions.Default = Position{Enabled: defEn.IsChecked(), Alignment: comboID(defAlign, "top_left"), GapX: defGX.Value(), GapY: defGY.Value()}
		savePositions(positions)
		a.applyUITheme()
		a.pushStatus("Settings saved.")
		a.showMessage("Success", "Settings saved successfully.", "info")
		a.loadPositionForSelected()
		a.loadColorsForSelected()
		dlg.Accept()
	})
	bb.OnRejected(func() { dlg.Reject() })
	outer.AddWidget(bb.QWidget)
	dlg.Exec()
}

func (a *App) showThemeStore() {
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Theme Marketplace")
	dlg.Resize(980, 640)
	content := qt.NewQVBoxLayout(dlg.QWidget)
	head := qt.NewQLabel3("<b>Conky Theme Marketplace</b><br/><span style='color:#8ea0c4'>Search real store catalogs, inspect metadata, and install directly into ~/.conky.</span>")
	head.SetWordWrap(true)
	content.AddWidget(head.QWidget)
	storeCombo := qt.NewQComboBox2()
	for _, store := range onlineStores {
		comboAdd(storeCombo, store.ID, store.Label)
	}
	sortCombo := qt.NewQComboBox2()
	comboAdd(sortCombo, "downloads", "Most downloaded")
	comboAdd(sortCombo, "latest", "Recently updated")
	comboAdd(sortCombo, "rating", "Best rated")
	catCombo := qt.NewQComboBox2()
	for _, p := range [][2]string{
		{"", "All categories"}, {"minimal", "Minimal"}, {"system monitor", "System Monitor"},
		{"weather", "Weather"}, {"music", "Music"}, {"network", "Network"},
		{"cpu gpu", "CPU/GPU"}, {"dark", "Dark"}, {"light", "Light"},
	} {
		comboAdd(catCombo, p[0], p[1])
	}
	search := qt.NewQLineEdit2()
	search.SetPlaceholderText("Search themes, authors, or keywords…")
	content.AddWidget(hrow(storeCombo.QWidget, sortCombo.QWidget, catCombo.QWidget, search.QWidget))
	status := muted("Ready. Select a store or search for a theme.")
	content.AddWidget(status.QWidget)
	paned := qt.NewQSplitter3(qt.Horizontal)
	list := qt.NewQListWidget2()
	list.SetSelectionMode(qt.QAbstractItemView__SingleSelection)
	paned.AddWidget(list.QWidget)
	details := qt.NewQWidget2()
	dl := qt.NewQVBoxLayout(details)
	title := qt.NewQLabel3("<b>Select a theme</b>")
	title.SetWordWrap(true)
	previewImg := qt.NewQLabel3("Preview")
	previewImg.SetMinimumSize2(320, 180)
	previewImg.SetAlignment(qt.AlignCenter)
	previewImg.SetStyleSheet("background:#0e1730; border:1px solid #243152; border-radius:12px;")
	desc := muted("Choose a result to inspect its description, author, version, and store page.")
	meta := muted("")
	web := a.btn("Open store in browser", "ghost", func() {})
	dl.AddWidget(title.QWidget)
	dl.AddWidget(previewImg.QWidget)
	dl.AddWidget(desc.QWidget)
	dl.AddWidget(meta.QWidget)
	dl.AddWidget(web.QWidget)
	dl.AddStretch()
	paned.AddWidget(details)
	paned.SetSizes([]int{480, 440})
	content.AddWidget2(paned.QWidget, 1)
	loadMore := a.btn("Load more results", "", nil)
	content.AddWidget(loadMore.QWidget)

	var selected *OnlineProduct
	var selectedStore = onlineStores[0]
	var products []OnlineProduct
	page, total, requestID := 1, 0, 0
	loading := false
	currentStore := func() OnlineStore { return getOnlineStore(comboID(storeCombo, onlineStores[0].ID)) }
	showProduct := func(product *OnlineProduct) {
		selected = product
		selectedStore = currentStore()
		if product == nil {
			title.SetText("<b>Select a theme</b>")
			desc.SetText("Choose a result to inspect its description, author, version, and store page.")
			meta.SetText("")
			previewImg.SetText("Preview")
			web.OnClicked(func() { xdgOpen(selectedStore.BrowseURL) })
			web.SetText("Open " + selectedStore.Label + " in browser")
			return
		}
		title.SetText("<b>" + markupEscape(product.Name) + "</b>")
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
		purl := product.PageURL
		web.OnClicked(func() { xdgOpen(purl) })
		web.SetText("View details on " + selectedStore.Label)
		previewImg.SetText("Loading preview…")
		if product.PreviewURL != "" {
			u := product.PreviewURL
			go func() {
				pm := loadRemotePixmap(u, 320, 180)
				idle(func() {
					if selected == nil || selected.PreviewURL != u || pm == nil || pm.IsNull() {
						return
					}
					previewImg.SetPixmap(pm)
				})
			}()
		}
	}
	render := func() {
		list.Clear()
		for _, product := range products {
			author := product.Author
			if author == "" {
				author = "Unknown author"
			}
			it := qt.NewQListWidgetItem2(fmt.Sprintf("%s\n%s  ·  %d downloads  ·  %.1f rating", product.Name, author, product.Downloads, product.Score))
			it.SetData(int(qt.UserRole), qt.NewQVariant4(product.ProductID))
			list.AddItemWithItem(it)
		}
		if total > 0 {
			status.SetText(fmt.Sprintf("Showing %d of %d results · %s", len(products), total, selectedStore.Label))
		} else {
			status.SetText(fmt.Sprintf("Showing %d results · %s", len(products), selectedStore.Label))
		}
		loadMore.SetEnabled(!loading && total > len(products))
	}
	var loadPage func(bool)
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
		loadMore.SetEnabled(false)
		store := currentStore()
		selectedStore = store
		query := search.Text()
		if extra := strings.TrimSpace(comboID(catCombo, "")); extra != "" && !strings.Contains(strings.ToLower(query), extra) {
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
					loadMore.SetEnabled(false)
					return
				}
				if reset {
					products = found
				} else {
					products = append(products, found...)
				}
				total = count
				render()
			})
		}()
	}
	storeCombo.OnCurrentIndexChanged(func(_ int) { loadPage(true) })
	sortCombo.OnCurrentIndexChanged(func(_ int) { loadPage(true) })
	catCombo.OnCurrentIndexChanged(func(_ int) { loadPage(true) })
	search.OnReturnPressed(func() { loadPage(true) })
	list.OnCurrentItemChanged(func(cur *qt.QListWidgetItem, _ *qt.QListWidgetItem) {
		if cur == nil {
			showProduct(nil)
			return
		}
		id := cur.Data(int(qt.UserRole)).ToInt()
		for i := range products {
			if products[i].ProductID == id {
				p := products[i]
				showProduct(&p)
				return
			}
		}
	})
	loadMore.OnClicked(func() { page++; loadPage(false) })
	loadPage(true)
	bb := qt.NewQDialogButtonBox2()
	install := bb.AddButton2("Download & Install", qt.QDialogButtonBox__AcceptRole)
	bb.AddButton2("Close", qt.QDialogButtonBox__RejectRole)
	install.OnClicked(func() {
		if selected == nil {
			return
		}
		product := *selected
		store := selectedStore
		dlg.Accept()
		if a.workerBusy {
			a.showMessage("Busy", "Please wait until the current operation finishes.", "info")
			return
		}
		a.downloadWithProgress(product.Name, func(cb progressFn) ([]string, error) {
			return downloadAndInstallOnlineProduct(store, product.ProductID, cb)
		})
	})
	bb.OnRejected(func() { dlg.Reject() })
	content.AddWidget(bb.QWidget)
	dlg.Exec()
}

func loadRemotePixmap(rawURL string, w, h int) *qt.QPixmap {
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
	pm := qt.NewQPixmap()
	if !pm.LoadFromDataWithData(data) {
		return nil
	}
	return pm.Scaled3(w, h, qt.KeepAspectRatio, qt.SmoothTransformation)
}

func (a *App) downloadWithProgress(title string, worker func(progressFn) ([]string, error)) {
	dlg := qt.NewQDialog(a.parent())
	dlg.SetWindowTitle("Downloading Theme")
	dlg.Resize(480, 180)
	box := qt.NewQVBoxLayout(dlg.QWidget)
	box.AddWidget(qt.NewQLabel3("<b>" + markupEscape(title) + "</b>").QWidget)
	bar := qt.NewQProgressBar2()
	bar.SetRange(0, 100)
	bar.SetTextVisible(true)
	box.AddWidget(bar.QWidget)
	st := muted("Connecting…")
	box.AddWidget(st.QWidget)
	cancelBtn := a.btn("Cancel", "ghost", nil)
	box.AddWidget(cancelBtn.QWidget)
	type resT struct {
		installed []string
		err       error
		done      bool
		canceled  bool
	}
	res := &resT{}
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn.OnClicked(func() {
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
			bar.SetValue(int(f * 100))
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
		idle(func() {
			dlg.Accept()
			a.setBusy(false)
			if res.canceled {
				a.pushStatus("Download canceled.")
				return
			}
			if res.err != nil {
				if confirmDialog(a.parent(), "Download Error", res.err.Error()+"\n\nRetry this download?") {
					a.downloadWithProgress(title, worker)
				}
				return
			}
			a.loadThemes()
			a.pushStatus("Installed theme: " + title)
			a.showMessage("Theme Installed", fmt.Sprintf("%s was installed into ~/.conky\n\nFolders: %s", title, strings.Join(res.installed, ", ")), "info")
		})
	}()
	dlg.Exec()
}

func (a *App) startFileWatcher() {
	a.watchStamp = themeTreeFingerprint()
	a.watchTimer = qt.NewQTimer2(a.win.QObject)
	a.watchTimer.OnTimeout(func() {
		stamp := themeTreeFingerprint()
		if stamp != a.watchStamp {
			a.watchStamp = stamp
			a.loadThemes()
			a.pushStatus("Theme folders changed — list refreshed.")
		}
	})
	// This check walks the theme directories; avoid doing that every few seconds
	// while still keeping automatic refresh responsive for normal use.
	a.watchTimer.Start(10000)
}

func newApp(qapp *qt.QApplication) *App {
	sess, desk := detectEnvironment()
	a := &App{
		qapp: qapp, session: sess, desktop: desk, desktopEnv: detectDesktopEnvironment(),
		settings: loadSettings(), profiles: loadProfiles(), library: loadLibrary(),
		themeStatus: map[string]string{}, themeHealth: map[string]ThemeHealth{},
		selectedThemeSet: map[string]bool{}, viewFilter: "all",
		previewSeconds:  loadSettings().PreviewSeconds,
		colorPickers:    map[string]*qt.QPushButton{},
		themeColorSlots: map[string]string{},
	}
	a.setupUI()
	a.applyUITheme()
	a.loadThemes()
	a.refreshProfiles()
	a.refreshLogView(250)
	a.startFileWatcher()
	a.showWaylandNotice()
	a.win.Show()
	return a
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
	qapp := qt.NewQApplication(os.Args)
	_ = qt.QApplication_SetStyleWithStyle("Fusion")
	_ = newApp(qapp)
	signal.Ignore(syscall.SIGPIPE)
	qt.QApplication_Exec()
}
