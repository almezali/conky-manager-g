<div align="center">

<img src="https://raw.githubusercontent.com/almezali/conky-manager-g/main/Screenshot_dark.png" width="80" alt="Conky Manager GTK Logo" style="border-radius:16px"/>

# Conky Manager GTK

**A modern, lightweight Conky theme manager — beautiful, powerful, and built for every Linux desktop.**

[![Version](https://img.shields.io/badge/version-0.7-5C6BC0?style=flat-square)](https://github.com/almezali/conky-manager-g/releases)
[![GTK](https://img.shields.io/badge/GTK-3.0-4CAF50?style=flat-square&logo=gtk)](https://gtk.org)
[![Platform](https://img.shields.io/badge/Linux-x86__64-FCC624?style=flat-square&logo=linux&logoColor=black)](https://kernel.org)
[![GitHub](https://img.shields.io/badge/GitHub-almezali-181717?style=flat-square&logo=github)](https://github.com/almezali/conky-manager-g)

---

### Manage · Customize · Download · Autostart your Conky themes — no config files needed.

</div>

---

## ⚡ Quick Download — v0.7

<div align="center">

| Distro Family | Format | Download |
|:---:|:---:|:---:|
| 🔵 **Arch · Manjaro · EndeavourOS · Garuda** | `pkg.tar.zst` | [![Arch](https://img.shields.io/badge/Download-pkg.tar.zst-1793D1?style=for-the-badge&logo=arch-linux&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.7/conky-manager-g-0.7-1-x86_64.pkg.tar.zst) |
| 🔴 **Fedora · openSUSE · RHEL · AlmaLinux** | `.rpm` | [![RPM](https://img.shields.io/badge/Download-.rpm-EE0000?style=for-the-badge&logo=red-hat&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.7/conky-manager-g-0.7-1.x86_64.rpm) |
| 🟠 **Debian · Ubuntu · Mint · Pop!_OS · Zorin** | `.deb` | [![DEB](https://img.shields.io/badge/Download-.deb-A81D33?style=for-the-badge&logo=debian&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.7/conky-manager-g_0.7_x86_64.deb) |
| 🌍 **Any Linux (no install needed)** | `.AppImage` | [![AppImage](https://img.shields.io/badge/Download-AppImage-5C5C5C?style=for-the-badge&logo=linux&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.7/conky-manager-x86_64.AppImage) |

</div>

---

## 📸 Screenshots

<div align="center">

| 🌑 Dark Mode | ☀️ Light Mode |
|:---:|:---:|
| ![Dark](https://github.com/almezali/conky-manager-g/blob/main/Screenshot_dark.png?raw=true) | ![Light](https://github.com/almezali/conky-manager-g/blob/main/Screenshot_light.png?raw=true) |

</div>

---

## ✨ Features

### 🎨 Theme Management
- **Automatic Theme Discovery** — scans `~/.config/conky`, `~/.conky`, `~/.local/share/conky`, and `/usr/share/conky` for all valid Conky configs.
- **Run / Stop** a single theme or all themes simultaneously with one click.
- **Live Preview Mode** — temporarily launch any theme for a configurable duration before committing.
- **Smart Search & Filter** — instantly find themes in your library by name.
- **Right-Click Context Menu** — quick access to run, preview, edit, open folder, repair, and validate actions.
- **Edit Theme** — open the selected config in your default text editor directly from the app.
- **Open Theme Folder** — reveal theme files in your file manager instantly.

### 🛠️ Smart Repair & Optimization
- **Auto-Optimize** — automatically rewrites theme configs (both legacy and new Conky syntax) to work perfectly on your current desktop environment.
- **Asset Path Rewriter** — detects and corrects broken image, Lua script, font, and icon paths in theme configs so assets always load correctly.
- **Compatibility Symlinks** — optionally creates symlinks for cross-directory compatibility, making themes portable.
- **Launch Bundles** — generates isolated launch environments per theme with fully resolved asset references.

### 🖥️ Desktop Environment Auto-Detection
Automatically detects your DE and applies the correct window layering preset:

| Desktop | Layer | Window Type |
|:---:|:---:|:---:|
| KDE Plasma | Above | Dock |
| GNOME | Below | Desktop |
| XFCE | Below | Desktop |
| Cinnamon | Below | Desktop |
| MATE | Below | Desktop |
| LXQt | Below | Desktop |
| LXDE | Below | Desktop |

### 📍 Position Control (New in v0.7)
- **Per-Theme Position Memory** — saves each theme's screen position independently.
- **9-Point Alignment Grid** — intuitive ↖ ↑ ↗ ← ● → ↙ ↓ ↘ button grid for quick placement.
- **Gap X / Y Fine-Tuning** — pixel-level offset controls for precise positioning.
- **Nudge Controls** — small arrow buttons to micro-adjust position while the theme is live.
- **Apply & Restart** — reapply position settings and restart the theme in one click.
- **Reset to Default** — instantly revert position to your global defaults.

### 🎨 Color Customization (New in v0.7)
- **Per-Theme Color Editor** — modify up to 11 color slots per theme: text, primary, secondary, accents, outline, shade, and window background.
- **Built-in Color Presets** — one-click application of popular community palettes:
  - 🌃 Tokyo Night · 🧛 Dracula · 🏔️ Nord · 🐱 Catppuccin Mocha
  - 🌹 Rosé Pine · 🌿 Gruvbox Dark · ☀️ Solarized Dark · 🌲 Everforest
- **Smart Color Remap** — intelligently remaps hex colors used in the raw config body, not just declared variables.
- **Color Memory** — saves your custom colors per theme and reapplies them automatically on every launch.
- **GTK Color Picker** — native color picker dialog for free color selection.
- **Apply & Restart** — instantly apply new colors and restart the running theme.

### 🌐 Theme Store & Download (New in v0.7)
- **Built-in Theme Gallery** with two tabs:
  - **GitHub Curated Collection** — 15+ hand-picked, popular themes with search, descriptions, author info, and tags:
    - Harmony, Deneb, Vega, Constellation, Ormix, Helium, Hydra, Fuchsia, Mimosa Light, Gotham, OS Monitoring, Conky Lua, MX Conky Collection, Quatro Fusion, Nebula, and more.
  - **OpenDesktop Stores** — browse, search, and install directly from gnome-look.org, KDE Look, Pling.com, and OpenDesktop.org with ratings and download counts.
- **One-Click Download & Install** — downloads archives, extracts them, and installs themes to `~/.conky` automatically.
- **Progress Dialog** — real-time download and installation progress with status messages.
- **Secure HTTPS Downloads** — multi-distro SSL/CA bundle detection for reliable downloads from GitHub, GitLab, Codeberg, Pling, and OpenDesktop.

### 💾 Profiles & Autostart
- **Named Profiles** — save any combination of themes as a named profile.
- **Run Profile** — launch all themes in a profile at once with a single click.
- **Autostart Integration** — enable/disable a profile as a login startup item via `~/.config/autostart`.
  - Works on GNOME, KDE, XFCE, Cinnamon, MATE, LXDE, LXQt.
  - Supports startup delay (3 s) and KDE panel wait.
- **Profile Details View** — see which themes are saved in each profile.
- **Add / Remove Themes** from profiles dynamically.

### 📥 Import Themes
- **Import Folder** — import a local theme folder directly into `~/.conky`.
- **Import Archive** — import `.zip`, `.tar.gz`, `.tar.xz`, `.tar.bz2`, `.tgz`, `.txz`, `.tbz2`, and `.7z` archives — auto-extracted and installed.
- **Smart Folder Detection** — automatically identifies theme roots within extracted archives.
- **Duplicate Avoidance** — unique destination names prevent overwriting existing themes.

### 🩺 Health & Diagnostics
- **Health Scan** — validates all discovered themes and reports:
  - 🟢 **Stable** — config launches cleanly with all assets present.
  - 🟡 **Needs Fix** — config launches but has missing assets.
  - 🔴 **Broken** — config fails validation.
- **Per-Theme Validation** — detailed report of launch bundle status, path fixes applied, assets found, and missing assets.
- **Diagnostics Tab** — built-in log viewer showing the last 250 lines of the application log in a monospace view.
- **Asset Scanner** — lists all referenced assets and identifies which are missing.

### ⚙️ Settings
| Setting | Options |
|:---|:---|
| Desktop Preset | Auto / KDE / GNOME / XFCE / Cinnamon / MATE / LXQt / LXDE / Generic |
| Window Layer | Above windows (overlay) / Below windows (background) |
| Window Type | Dock / Desktop / Normal |
| Smoothness | Smooth / Balanced / Performance |
| Process Nice Level | 0–19 (lower = higher priority) |
| Max Running Instances | 1–10 |
| Health Check Interval | Configurable (seconds) |
| Preview Duration | Configurable (seconds) |
| UI Theme | Auto (system) / Light / Dark |
| Auto-Optimize | Enable/Disable |
| Full Visual Launch | Enable/Disable |
| Compatibility Symlinks | Enable/Disable |
| Remember Position | Enable/Disable |
| Remember Colors | Enable/Disable |

### 🌗 Adaptive UI
- Follows your system's light/dark preference automatically, with manual override.
- Wayland-aware — detects Wayland sessions and warns about compositor-specific behavior.

### 🔁 Legacy Migration
- Automatically migrates settings, profiles, and optimized configs from the old `conky-kde-manager` directory to the new unified `conky-manager` directory.

### 🖥️ CLI Support
- `--run-profile "Profile Name"` — launch a saved profile headlessly from a terminal or script (used by autostart).

---

## 🐧 Supported Distributions

| Package | Distributions |
|:---|:---|
| **`pkg.tar.zst`** | Arch Linux, Manjaro, EndeavourOS, Garuda Linux, ArcoLinux, CachyOS, and all Arch-based distros |
| **`.rpm`** | Fedora, RHEL 9/10, CentOS Stream, AlmaLinux, Rocky Linux, openSUSE Leap, openSUSE Tumbleweed |
| **`.deb`** | Debian 11/12, Ubuntu 22.04/24.04, Linux Mint, Pop!_OS, Zorin OS, elementary OS, MX Linux, KDE neon, Kubuntu |
| **`.AppImage`** | Any modern x86_64 Linux distribution — no installation, no root required |

---

## 🚀 Installation

### Prerequisites

Install **Conky** and **GTK 3 Python bindings** for your distro:

```bash
# Arch / Manjaro
sudo pacman -S conky python-gobject

# Fedora
sudo dnf install conky python3-gobject gtk3

# Debian / Ubuntu / Mint
sudo apt install conky python3-gi python3-gi-cairo gir1.2-gtk-3.0
```

---

### 🔵 Arch Linux / Manjaro / EndeavourOS

```bash
sudo pacman -U conky-manager-g-0.7-1-x86_64.pkg.tar.zst
```

### 🔴 Fedora / openSUSE / RHEL / AlmaLinux

```bash
# Fedora / RHEL
sudo dnf install ./conky-manager-g-0.7-1.x86_64.rpm

# openSUSE
sudo zypper install ./conky-manager-g-0.7-1.x86_64.rpm
```

### 🟠 Debian / Ubuntu / Linux Mint / Pop!_OS

```bash
sudo dpkg -i conky-manager-g_0.7_x86_64.deb
sudo apt-get install -f     # auto-resolve any missing dependencies
```

### 🌍 AppImage (Universal — No Install Required)

```bash
chmod +x conky-manager-x86_64.AppImage
./conky-manager-x86_64.AppImage
```

> 💡 **Tip:** Move the AppImage to `~/.local/bin/` and create a `.desktop` shortcut to integrate it into your app launcher.

---

## 📖 How to Use

1. **Launch** — open from your applications menu or run `conky-manager-g` in a terminal.
2. **Browse Themes** — the sidebar auto-discovers all Conky themes on your system.
3. **Run a Theme** — select it and click **▶ Run**, or right-click for the context menu.
4. **Preview** — use **👁 Preview** to test a theme temporarily before keeping it running.
5. **Smart Repair** — click **🔧 Smart Repair** to auto-optimize a theme for your desktop.
6. **Position** — go to the **Position** tab to set per-theme screen alignment and offsets.
7. **Colors** — go to the **Colors** tab to customize the color palette, choose a preset, or pick individual colors.
8. **Download Themes** — click **🌐 Get Themes** to browse the GitHub gallery or OpenDesktop stores and install with one click.
9. **Import** — use **📥 Import Folder** or **Import Archive** to add themes from local files.
10. **Profiles** — use the **Profiles** tab to group themes, run sets together, and enable autostart at login.
11. **Health Scan** — click **🩺 Health Scan** to validate all themes and see their status.
12. **Settings** — open **⚙ Settings** from the top-right menu to fine-tune all behavior.
13. **Diagnostics** — check the **Diagnostics** tab for app logs when troubleshooting.

---

## 📁 Data Locations

| Path | Purpose |
|:---|:---|
| `~/.local/share/conky-manager/` | App data root |
| `~/.local/share/conky-manager/settings.json` | User settings |
| `~/.local/share/conky-manager/profiles.json` | Saved profiles |
| `~/.local/share/conky-manager/positions.json` | Per-theme position overrides |
| `~/.local/share/conky-manager/colors.json` | Per-theme color overrides |
| `~/.local/share/conky-manager/optimized/` | Auto-optimized configs |
| `~/.local/share/conky-manager/launch/` | Theme launch bundles |
| `~/.local/share/conky-manager/downloads/` | Downloaded theme archives |
| `~/.local/share/conky-manager/manager.log` | Application log |
| `~/.config/autostart/conky-manager.desktop` | Autostart entry |

---

## 🤝 Contributing

Contributions, bug reports, and feature requests are welcome!

- 🐛 **Bug reports:** [Open an issue](https://github.com/almezali/conky-manager-g/issues)
- 💡 **Feature requests:** [Start a discussion](https://github.com/almezali/conky-manager-g/discussions)
- 🔧 **Pull requests:** Fork the repo and submit a PR

---

## 📄 License

This project is released under the **GNU General Public License v3.0**.

---

<div align="center">

Made with ❤️ for the Linux community

[⬆ Back to top](#conky-manager-gtk)

</div>
