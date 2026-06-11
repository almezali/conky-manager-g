<div align="center">

# 🖥️ Conky Manager GTK

### A modern, lightweight Conky theme manager for every Linux desktop

[![Version](https://img.shields.io/badge/version-0.2-blue.svg)](https://github.com/almezali/conky-manager-g/releases)
[![License](https://img.shields.io/badge/license-GPL--3.0-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey.svg)](#)
[![Made with GTK3](https://img.shields.io/badge/GTK-3.0-orange.svg)](#)

**Manage, optimize, preview, and run your Conky themes — without ever touching a config file.**

[Features](#-features) • [Screenshots](#-screenshots) • [Installation](#-installation) • [Usage](#-usage) • [Supported Distros](#-supported-distributions) • [Contributing](#-contributing)

</div>

---

## 📥 Quick Download

<div align="center">

| Format | Download |
|---|---|
| 📦 **Arch / Manjaro / EndeavourOS** (`pkg.tar.zst`) | [![Download](https://img.shields.io/badge/Download-pkg.tar.zst-1793D1?style=for-the-badge&logo=arch-linux&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.2/conky-manager-g-0.2-1-x86_64.pkg.tar.zst) |
| 🎩 **Fedora / openSUSE / RHEL** (`.rpm`) | [![Download](https://img.shields.io/badge/Download-.rpm-EE0000?style=for-the-badge&logo=redhat&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.2/conky-manager-g-0.2-1.x86_64.rpm) |
| 🐧 **Debian / Ubuntu / Mint** (`.deb`) | [![Download](https://img.shields.io/badge/Download-.deb-A81D33?style=for-the-badge&logo=debian&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.2/conky-manager-g_0.2_x86_64.deb) |
| 🌍 **Universal** (`.AppImage`) | [![Download](https://img.shields.io/badge/Download-AppImage-5C5C5C?style=for-the-badge&logo=appimage&logoColor=white)](https://github.com/almezali/conky-manager-g/releases/download/0.2/conky-manager-x86_64.AppImage) |

</div>

---

## 📸 Screenshots

<div align="center">

| Dark Mode | Light Mode |
|:---:|:---:|
| ![Dark Theme](https://github.com/almezali/conky-manager-g/blob/main/Screenshot_dark.png?raw=true) | ![Light Theme](https://github.com/almezali/conky-manager-g/blob/main/Screenshot_light.png?raw=true) |

</div>

---

## ✨ Features

- 🎨 **Theme Browser** — automatically scans `~/.config/conky`, `~/.conky`, `~/.local/share/conky`, and `/usr/share/conky` for valid Conky configs.
- 🛠️ **Smart Repair / Auto-Optimize** — automatically rewrites configs (legacy & new Conky syntax) to match your desktop environment's window layer, type, and hints.
- 🖥️ **Desktop Environment Auto-Detection** — detects KDE Plasma, GNOME, XFCE, Cinnamon, MATE, LXQt, and LXDE, and applies the correct window layering presets automatically.
- ▶️ **Run / Stop Themes** — launch single or multiple themes at once, with automatic health-checking before falling back to the original config.
- 👁️ **Live Preview** — temporarily preview any theme for a configurable duration before committing.
- 💾 **Profiles** — save groups of themes as named profiles, run them together, and manage them from a dedicated tab.
- 🚀 **Autostart Integration** — enable/disable launching a profile automatically at login via `~/.config/autostart`.
- 🩺 **Health Scan** — validates all detected themes and reports **Stable / Needs Fix / Broken** status with color-coded indicators.
- 📥 **Import Themes** — import new `.conf`/`conkyrc` themes directly from the UI.
- ⚙️ **Advanced Settings**
  - Window layer (Above / Below windows)
  - Window type (dock / desktop / normal)
  - Smoothness/performance profile
  - Process niceness level
  - Max running instances
  - Health-check interval
  - Preview duration
  - UI theme (Light / Dark / Auto)
  - Desktop preset override
- 🌗 **Adaptive UI Theme** — automatically follows your system's light/dark preference, with manual override.
- 📂 **Quick Access** — open the optimized configs folder, edit themes in your default editor, or open the theme's folder in your file manager.
- 📜 **Built-in Log Viewer** — view real-time logs for debugging Conky launch issues.
- 🔄 **Legacy Migration** — automatically migrates settings/profiles from the older `conky-kde-manager` data directory.

---

## 🐧 Supported Distributions

Conky Manager GTK is distributed in **four universal formats**, covering virtually every modern Linux distribution:

| Package | Compatible Distributions |
|---|---|
| **`.pkg.tar.zst`** | Arch Linux, Manjaro, EndeavourOS, Garuda Linux, ArcoLinux, and other Arch-based distros |
| **`.rpm`** | Fedora, RHEL, CentOS Stream, AlmaLinux, Rocky Linux, openSUSE (Leap/Tumbleweed) |
| **`.deb`** | Debian, Ubuntu, Linux Mint, Pop!_OS, Zorin OS, elementary OS, MX Linux, KDE neon |
| **`.AppImage`** | Any modern x86_64 Linux distribution (no installation required — run anywhere) |

> **Requirements:** GTK 3.0, Python 3, and [Conky](https://github.com/brndnmtthws/conky) installed on your system.

---

## 🚀 Installation

### Arch Linux / Manjaro (`pkg.tar.zst`)
```bash
sudo pacman -U conky-manager-g-0.2-1-x86_64.pkg.tar.zst
```

### Fedora / openSUSE / RHEL (`.rpm`)
```bash
sudo rpm -i conky-manager-g-0.2-1.x86_64.rpm
# or
sudo dnf install ./conky-manager-g-0.2-1.x86_64.rpm
```

### Debian / Ubuntu / Mint (`.deb`)
```bash
sudo dpkg -i conky-manager-g_0.2_x86_64.deb
sudo apt-get install -f   # resolve any missing dependencies
```

### AppImage (Universal)
```bash
chmod +x conky-manager-x86_64.AppImage
./conky-manager-x86_64.AppImage
```

> 💡 Make sure **Conky** is installed on your system, e.g.:
> ```bash
> sudo pacman -S conky      # Arch
> sudo dnf install conky    # Fedora
> sudo apt install conky    # Debian/Ubuntu
> ```

---

## 📖 Usage

1. **Launch** the application from your applications menu, or run `conky-manager-g` from a terminal.
2. The app **scans your system** for available Conky themes automatically.
3. Select a theme from the sidebar:
   - **▶ Run** to launch it
   - **👁 Preview** to test it temporarily
   - **🩺 Smart Repair** to auto-optimize it for your desktop environment
   - **📥 Import** to add new theme files
4. Go to the **Profiles** tab to:
   - Save a group of selected themes as a named profile
   - Run all themes in a profile at once
   - Enable **Autostart** so the profile launches automatically at login
5. Open **⚙ Settings** (top-right menu) to fine-tune:
   - Window layer & type
   - Performance/smoothness
   - Process priority (nice level)
   - Health-check & preview timing
   - UI theme (Light/Dark/Auto)
6. Use **🔄 Refresh Themes** to rescan after adding new theme files manually.
7. Check **📜 Logs** for troubleshooting if a theme fails to launch.

---

## 🤝 Contributing

Contributions, bug reports, and feature requests are welcome! Feel free to open an [issue](https://github.com/almezali/conky-manager-g/issues) or submit a pull request.

---

## 📄 License

This project is licensed under the **GPL-3.0 License**.

---

<div align="center">

Made with ❤️ for the Linux community — 2026

</div>
