#!/usr/bin/env python3
"""
Conky Manager GTK - v0.7
Universal Conky theme manager for Linux desktop environments.
Supports KDE, GNOME, XFCE, Cinnamon, MATE, LXQt, and others.
"""

from __future__ import annotations

import gi

gi.require_version("Gtk", "3.0")
from gi.repository import Gtk, GLib, Gdk

import json
import logging
import os
import re
import shutil
import ssl
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable
from urllib.parse import urlencode, urlparse
from urllib.request import Request, urlopen

APP_NAME = "Conky Manager GTK"
APP_VERSION = "v0.7"

HOME = Path.home()
THEME_DIRS = [
    HOME / ".config" / "conky",
    HOME / ".conky",
    HOME / ".local" / "share" / "conky",
    Path("/usr/share/conky"),
]
OLD_BASE_DIR = HOME / ".local" / "share" / "conky-kde-manager"
BASE_DIR = HOME / ".local" / "share" / "conky-manager"
LOG_FILE = BASE_DIR / "manager.log"
OPTIMIZED_DIR = BASE_DIR / "optimized"
LAUNCH_DIR = BASE_DIR / "launch"
SETTINGS_FILE = BASE_DIR / "settings.json"
POSITIONS_FILE = BASE_DIR / "positions.json"
COLORS_FILE = BASE_DIR / "colors.json"
PROFILES_FILE = BASE_DIR / "profiles.json"
AUTOSTART_DIR = HOME / ".config" / "autostart"
AUTOSTART_FILE = AUTOSTART_DIR / "conky-manager.desktop"
OLD_AUTOSTART_FILE = AUTOSTART_DIR / "conky-kde-manager.desktop"
DEFAULT_IMPORT_DIR = HOME / ".conky"
DOWNLOADS_DIR = BASE_DIR / "downloads"
ARCHIVE_EXTENSIONS = (
    ".zip",
    ".tar",
    ".tar.gz",
    ".tgz",
    ".tar.bz2",
    ".tbz2",
    ".tar.xz",
    ".txz",
    ".7z",
)
STRIP_FOLDER_SUFFIXES = ("-main", "-master", "-gh-pages", "-devel")
ALLOWED_DOWNLOAD_HOSTS = (
    "github.com",
    "www.github.com",
    "raw.githubusercontent.com",
    "gitlab.com",
    "codeberg.org",
    "pling.com",
    "www.pling.com",
    "gnome-look.org",
    "www.gnome-look.org",
    "kde-look.org",
    "www.kde-look.org",
    "opendesktop.org",
    "www.opendesktop.org",
)
CONKY_STORE_CATEGORY = "124"

SYSTEM_CA_BUNDLES = (
    "/etc/ssl/certs/ca-certificates.crt",
    "/etc/pki/tls/cert.pem",
    "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
    "/etc/ssl/ca-bundle.pem",
)

_ssl_context: ssl.SSLContext | None = None


def _ca_bundle_candidates() -> list[Path]:
    candidates: list[Path] = []
    try:
        import certifi

        cert_path = Path(certifi.where())
        if cert_path.is_file():
            candidates.append(cert_path)
    except ImportError:
        pass

    env_cert = os.environ.get("SSL_CERT_FILE")
    if env_cert:
        candidates.append(Path(env_cert))

    for path_str in SYSTEM_CA_BUNDLES:
        candidates.append(Path(path_str))

    seen: set[Path] = set()
    unique: list[Path] = []
    for candidate in candidates:
        resolved = candidate.expanduser()
        if resolved in seen:
            continue
        seen.add(resolved)
        unique.append(resolved)
    return unique


def get_ssl_context() -> ssl.SSLContext:
    global _ssl_context
    if _ssl_context is not None:
        return _ssl_context

    for cafile in _ca_bundle_candidates():
        if cafile.is_file():
            try:
                _ssl_context = ssl.create_default_context(cafile=str(cafile))
                logging.info("Using CA bundle: %s", cafile)
                return _ssl_context
            except ssl.SSLError as exc:
                logging.warning("Could not load CA bundle %s: %s", cafile, exc)

    _ssl_context = ssl.create_default_context()
    logging.warning("Using system default SSL context without explicit CA bundle")
    return _ssl_context


def urlopen_secure(request: Request, timeout: int = 60):
    """HTTPS helper that works in Nuitka onefile builds across distributions."""
    return urlopen(request, timeout=timeout, context=get_ssl_context())


def _migrate_data_dir():
    """Import settings from the legacy KDE-only data directory."""
    if not OLD_BASE_DIR.is_dir() or OLD_BASE_DIR == BASE_DIR:
        return
    BASE_DIR.mkdir(parents=True, exist_ok=True)
    OPTIMIZED_DIR.mkdir(parents=True, exist_ok=True)
    for name in ("settings.json", "profiles.json", "manager.log"):
        old_file = OLD_BASE_DIR / name
        new_file = BASE_DIR / name
        if old_file.exists() and not new_file.exists():
            shutil.copy2(old_file, new_file)
    old_opt = OLD_BASE_DIR / "optimized"
    if old_opt.is_dir():
        for item in old_opt.iterdir():
            dest = OPTIMIZED_DIR / item.name
            if item.is_file() and not dest.exists():
                shutil.copy2(item, dest)


_migrate_data_dir()
BASE_DIR.mkdir(parents=True, exist_ok=True)
OPTIMIZED_DIR.mkdir(parents=True, exist_ok=True)
LAUNCH_DIR.mkdir(parents=True, exist_ok=True)
DOWNLOADS_DIR.mkdir(parents=True, exist_ok=True)
THEME_DIRS[0].mkdir(parents=True, exist_ok=True)
DEFAULT_IMPORT_DIR.mkdir(parents=True, exist_ok=True)

logging.basicConfig(
    filename=str(LOG_FILE),
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)

STATUS_COLORS = {
    "stable": "#10b981",
    "needs_fix": "#f59e0b",
    "broken": "#ef4444",
    "": "#888888",
}

DEFAULT_SETTINGS: dict[str, Any] = {
    "layer": "above",
    "window_type": "dock",
    "smoothness": "balanced",
    "nice_level": 10,
    "max_instances": 1,
    "healthcheck_seconds": 2.0,
    "preview_seconds": 5,
    "ui_theme": "auto",
    "desktop_preset": "auto",
    "auto_optimize": True,
    "full_visual_launch": True,
    "create_compat_symlinks": True,
    "remember_position": True,
    "remember_colors": True,
}

POSITION_ALIGNMENTS = (
    ("top_left", "↖"),
    ("top_middle", "↑"),
    ("top_right", "↗"),
    ("middle_left", "←"),
    ("middle_middle", "●"),
    ("middle_right", "→"),
    ("bottom_left", "↙"),
    ("bottom_middle", "↓"),
    ("bottom_right", "↘"),
)

DEFAULT_POSITIONS: dict[str, Any] = {
    "default": {
        "enabled": True,
        "alignment": "top_left",
        "gap_x": 30,
        "gap_y": 50,
    },
    "themes": {},
}

COLOR_SLOT_ORDER = (
    "default_color",
    "color1",
    "color2",
    "color3",
    "color4",
    "color5",
    "color6",
    "color7",
    "color8",
    "color9",
    "color0",
    "color",
    "default_outline_color",
    "default_shade_color",
    "own_window_colour",
)

COLOR_SLOT_LABELS = {
    "default_color": "Text",
    "color": "Text (legacy)",
    "color0": "Base",
    "color1": "Primary",
    "color2": "Secondary",
    "color3": "Accent 3",
    "color4": "Accent 4",
    "color5": "Accent 5",
    "default_outline_color": "Outline",
    "default_shade_color": "Shade",
    "own_window_colour": "Window",
}

NAMED_COLORS = {
    "white": "#FFFFFF",
    "black": "#000000",
    "red": "#FF0000",
    "green": "#00FF00",
    "blue": "#0000FF",
    "yellow": "#FFFF00",
    "orange": "#FFA500",
    "gray": "#808080",
    "grey": "#808080",
}

DEFAULT_COLORS: dict[str, Any] = {
    "themes": {},
}

COLOR_PRESETS: dict[str, dict[str, Any]] = {
    "tokyo_night": {
        "label": "Tokyo Night",
        "colors": {
            "default_color": "#A9B1D6",
            "color1": "#F7768E",
            "color2": "#7AA2F7",
            "default_outline_color": "#565F89",
            "default_shade_color": "#1A1B26",
        },
    },
    "dracula": {
        "label": "Dracula",
        "colors": {
            "default_color": "#F8F8F2",
            "color1": "#FF79C6",
            "color2": "#8BE9FD",
            "default_outline_color": "#6272A4",
            "default_shade_color": "#282A36",
        },
    },
    "nord": {
        "label": "Nord",
        "colors": {
            "default_color": "#ECEFF4",
            "color1": "#88C0D0",
            "color2": "#81A1C1",
            "default_outline_color": "#4C566A",
            "default_shade_color": "#2E3440",
        },
    },
    "catppuccin_mocha": {
        "label": "Catppuccin Mocha",
        "colors": {
            "default_color": "#CDD6F4",
            "color1": "#F5C2E7",
            "color2": "#89B4FA",
            "default_outline_color": "#6C7086",
            "default_shade_color": "#1E1E2E",
        },
    },
    "rose_pine": {
        "label": "Rose Pine",
        "colors": {
            "default_color": "#E0DEF4",
            "color1": "#EBBCBA",
            "color2": "#9CCFD8",
            "default_outline_color": "#6E6A86",
            "default_shade_color": "#191724",
        },
    },
    "gruvbox_dark": {
        "label": "Gruvbox Dark",
        "colors": {
            "default_color": "#EBDBB2",
            "color1": "#FB4934",
            "color2": "#83A598",
            "default_outline_color": "#928374",
            "default_shade_color": "#282828",
        },
    },
    "solarized_dark": {
        "label": "Solarized Dark",
        "colors": {
            "default_color": "#839496",
            "color1": "#DC322F",
            "color2": "#268BD2",
            "default_outline_color": "#586E75",
            "default_shade_color": "#002B36",
        },
    },
    "everforest": {
        "label": "Everforest",
        "colors": {
            "default_color": "#D3C6AA",
            "color1": "#E67E80",
            "color2": "#7FBBB3",
            "default_outline_color": "#859289",
            "default_shade_color": "#2D353B",
        },
    },
}

ASSET_DIR_NAMES = (
    "res",
    "img",
    "imgs",
    "images",
    "assets",
    "scripts",
    "fonts",
    "lua",
    "lib",
    "icons",
    "include",
    "config",
)
ASSET_FILE_SUFFIXES = (
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".svg",
    ".lua",
    ".ttf",
    ".otf",
    ".woff",
    ".woff2",
    ".sh",
)
PATH_REFERENCE_PATTERNS = (
    re.compile(r"\$\{image\s+([^}\s]+)"),
    re.compile(r"lua_load\s*=\s*['\"]([^'\"]+)['\"]", re.IGNORECASE),
    re.compile(
        r"\$\{(?:exec|execi|execpi|execp|texeci|texecpi)\s+"
        r"(?:\d+\s+)?((?:~|\$HOME|/)[^}\s|]+)"
    ),
    re.compile(r"(~/.config/conky/[^\s'\"${}|]+)"),
    re.compile(r"(\$HOME/.config/conky/[^\s'\"${}|]+)"),
    re.compile(r"(~/.conky/[^\s'\"${}|]+)"),
)

DE_PRESETS: dict[str, dict[str, str]] = {
    "kde": {"layer": "above", "window_type": "dock"},
    "gnome": {"layer": "below", "window_type": "desktop"},
    "xfce": {"layer": "below", "window_type": "desktop"},
    "cinnamon": {"layer": "below", "window_type": "desktop"},
    "mate": {"layer": "below", "window_type": "desktop"},
    "lxqt": {"layer": "below", "window_type": "desktop"},
    "lxde": {"layer": "below", "window_type": "desktop"},
    "generic": {"layer": "above", "window_type": "dock"},
}


# ---------------------------------------------------------------------------
# Core logic (shared with Qt edition)
# ---------------------------------------------------------------------------

def detect_environment():
    session = os.environ.get("XDG_SESSION_TYPE", "unknown")
    desktop = os.environ.get("XDG_CURRENT_DESKTOP", "unknown")
    return session, desktop


def detect_desktop_environment() -> str:
    desktop = os.environ.get("XDG_CURRENT_DESKTOP", "").lower()
    session_desktop = os.environ.get("DESKTOP_SESSION", "").lower()
    combined = f"{desktop} {session_desktop}"
    if any(token in combined for token in ("kde", "plasma")):
        return "kde"
    if "gnome" in combined:
        return "gnome"
    if "xfce" in combined:
        return "xfce"
    if "cinnamon" in combined:
        return "cinnamon"
    if "mate" in combined:
        return "mate"
    if "lxqt" in combined:
        return "lxqt"
    if "lxde" in combined:
        return "lxde"
    return "generic"


def resolve_runtime_settings(user_settings: dict | None = None) -> dict:
    merged = dict(DEFAULT_SETTINGS)
    if user_settings:
        merged.update(user_settings)
    preset = merged.get("desktop_preset", "auto")
    de = detect_desktop_environment() if preset == "auto" else preset
    de_defaults = DE_PRESETS.get(de, DE_PRESETS["generic"])
    if preset == "auto":
        merged.setdefault("layer", de_defaults["layer"])
        merged.setdefault("window_type", de_defaults["window_type"])
    return merged


def load_json(path: Path, default):
    try:
        if path.exists():
            return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        pass
    return default


def save_json(path: Path, obj):
    path.write_text(json.dumps(obj, indent=2, ensure_ascii=False), encoding="utf-8")


def is_conky_installed():
    return shutil.which("conky") is not None


def is_valid_theme(cfg_path: Path):
    try:
        text = cfg_path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return False
    return "conky.config" in text or "own_window" in text or "TEXT" in text


def find_themes():
    themes: list[Path] = []
    seen: set[Path] = set()
    for root_dir in THEME_DIRS:
        if not root_dir.is_dir():
            continue
        candidates: list[Path] = list(root_dir.rglob("*.conf"))
        for name in ("conky.conf", "conkyrc"):
            candidate = root_dir / name
            if candidate.is_file():
                candidates.append(candidate)
        for cfg_file in candidates:
            if cfg_file.is_file() and is_valid_theme(cfg_file):
                resolved = cfg_file.resolve()
                if resolved not in seen:
                    seen.add(resolved)
                    themes.append(resolved)
    return sorted(themes)


DESKTOP_SETTINGS_NEW_BASE = {
    "update_interval": "1.0",
    "update_interval_on_battery": "1.5",
    "cpu_avg_samples": "2",
    "net_avg_samples": "2",
    "double_buffer": "true",
    "no_buffers": "true",
    "own_window": "true",
    "own_window_argb_visual": "true",
    "own_window_argb_value": "0",
    "own_window_transparent": "true",
    "draw_shades": "false",
    "draw_outline": "false",
    "draw_borders": "false",
    "draw_graph_borders": "false",
    "use_xft": "true",
    "xftalpha": "1.0",
}

DESKTOP_SETTINGS_LEGACY_BASE = {
    "update_interval": "1.0",
    "cpu_avg_samples": "2",
    "net_avg_samples": "2",
    "double_buffer": "yes",
    "no_buffers": "yes",
    "own_window": "yes",
    "own_window_argb_visual": "yes",
    "own_window_argb_value": "0",
    "own_window_transparent": "yes",
    "draw_shades": "no",
    "draw_outline": "no",
    "draw_borders": "no",
    "draw_graph_borders": "no",
    "use_xft": "yes",
    "xftalpha": "1.0",
}


def _window_hints(desktop_env: str, layer: str, new_syntax: bool) -> str:
    above = layer == "above"
    hints_by_de = {
        "kde": (
            "undecorated,above,sticky,skip_taskbar,skip_pager",
            "undecorated,below,sticky,skip_taskbar,skip_pager",
        ),
        "gnome": (
            "undecorated,above,skip_taskbar,skip_pager",
            "undecorated,below,skip_taskbar,skip_pager",
        ),
        "xfce": (
            "undecorated,above,sticky,skip_taskbar,skip_pager",
            "undecorated,below,sticky,skip_taskbar,skip_pager",
        ),
        "cinnamon": (
            "undecorated,above,skip_taskbar,skip_pager",
            "undecorated,below,skip_taskbar,skip_pager",
        ),
        "mate": (
            "undecorated,above,skip_taskbar,skip_pager",
            "undecorated,below,skip_taskbar,skip_pager",
        ),
        "lxqt": (
            "undecorated,above,sticky,skip_taskbar,skip_pager",
            "undecorated,below,sticky,skip_taskbar,skip_pager",
        ),
        "lxde": (
            "undecorated,above,sticky,skip_taskbar,skip_pager",
            "undecorated,below,sticky,skip_taskbar,skip_pager",
        ),
        "generic": (
            "undecorated,above,sticky,skip_taskbar,skip_pager",
            "undecorated,below,sticky,skip_taskbar,skip_pager",
        ),
    }
    hints = hints_by_de.get(desktop_env, hints_by_de["generic"])
    raw = hints[0] if above else hints[1]
    return f'"{raw}"' if new_syntax else raw


def compute_desktop_settings(settings: dict, new_syntax: bool):
    runtime = resolve_runtime_settings(settings)
    smooth = runtime.get("smoothness", "balanced")
    if smooth == "ultra":
        upd, cpu, net = "1.5", "3", "3"
    elif smooth == "performance":
        upd, cpu, net = "0.5", "2", "2"
    else:
        upd, cpu, net = "1.0", "2", "2"

    layer = runtime.get("layer", "above")
    window_type = runtime.get("window_type", "dock")
    preset = runtime.get("desktop_preset", "auto")
    desktop_env = detect_desktop_environment() if preset == "auto" else preset

    position_values: dict[str, str] = {}
    if runtime.get("position_enabled"):
        alignment = runtime.get("alignment", "top_left")
        gap_x = str(int(runtime.get("gap_x", 30)))
        gap_y = str(int(runtime.get("gap_y", 50)))
        position_values = {
            "alignment": alignment,
            "gap_x": gap_x,
            "gap_y": gap_y,
        }

    if new_syntax:
        base = dict(DESKTOP_SETTINGS_NEW_BASE)
        base["update_interval"] = upd
        base["cpu_avg_samples"] = cpu
        base["net_avg_samples"] = net
        base["own_window_type"] = f'"{window_type}"'
        base["own_window_hints"] = _window_hints(desktop_env, layer, new_syntax=True)
        if position_values:
            base["alignment"] = f"'{position_values['alignment']}'"
            base["gap_x"] = position_values["gap_x"]
            base["gap_y"] = position_values["gap_y"]
        return base

    base = dict(DESKTOP_SETTINGS_LEGACY_BASE)
    base["update_interval"] = upd
    base["cpu_avg_samples"] = cpu
    base["net_avg_samples"] = net
    base["own_window_type"] = window_type
    base["own_window_hints"] = _window_hints(desktop_env, layer, new_syntax=False)
    if position_values:
        base["alignment"] = position_values["alignment"]
        base["gap_x"] = position_values["gap_x"]
        base["gap_y"] = position_values["gap_y"]
    return base


def load_positions() -> dict[str, Any]:
    data = load_json(POSITIONS_FILE, DEFAULT_POSITIONS)
    if "default" not in data:
        data["default"] = dict(DEFAULT_POSITIONS["default"])
    if "themes" not in data:
        data["themes"] = {}
    return data


def save_positions(data: dict[str, Any]) -> None:
    save_json(POSITIONS_FILE, data)


def parse_theme_position(content: str) -> dict[str, Any]:
    """Read alignment and gap offsets from a Conky config."""
    result = dict(DEFAULT_POSITIONS["default"])
    result.pop("enabled", None)

    if _is_new_syntax(content):
        for key in ("alignment", "gap_x", "gap_y"):
            match = re.search(rf"^\s*{key}\s*=\s*([^,\n]+)", content, re.MULTILINE | re.IGNORECASE)
            if not match:
                continue
            value = match.group(1).strip().strip("'\"")
            if key == "alignment":
                result[key] = value
            else:
                try:
                    result[key] = int(float(value))
                except ValueError:
                    pass
        return result

    for key in ("alignment", "gap_x", "gap_y"):
        match = re.search(rf"^\s*{key}\s+(\S+)", content, re.MULTILINE | re.IGNORECASE)
        if not match:
            continue
        value = match.group(1).strip().strip("'\"")
        if key == "alignment":
            result[key] = value
        else:
            try:
                result[key] = int(float(value))
            except ValueError:
                pass
    return result


def resolve_theme_position(cfg_path: Path, positions: dict[str, Any] | None = None) -> dict[str, Any] | None:
    """Return saved position override for a theme, if any."""
    positions = positions or load_positions()
    saved = positions.get("themes", {}).get(str(cfg_path.resolve()))
    if saved and saved.get("enabled", True):
        return {
            "alignment": saved.get("alignment", "top_left"),
            "gap_x": int(saved.get("gap_x", 30)),
            "gap_y": int(saved.get("gap_y", 50)),
        }
    return None


def merge_position_settings(base_settings: dict, cfg_path: Path, position: dict[str, Any] | None = None) -> dict:
    merged = dict(base_settings)
    effective = position
    if effective is None:
        effective = resolve_theme_position(cfg_path)
    if not effective:
        return merged
    merged["position_enabled"] = True
    merged["alignment"] = effective.get("alignment", "top_left")
    merged["gap_x"] = int(effective.get("gap_x", 30))
    merged["gap_y"] = int(effective.get("gap_y", 50))
    return merged


def apply_position_patch(content: str, position: dict[str, Any]) -> str:
    """Patch alignment/gap_x/gap_y into config content."""
    settings = {
        "position_enabled": True,
        "alignment": position.get("alignment", "top_left"),
        "gap_x": int(position.get("gap_x", 30)),
        "gap_y": int(position.get("gap_y", 50)),
    }
    if _is_new_syntax(content):
        return _patch_new_syntax(content, settings)
    return _patch_legacy_syntax(content, settings)


def load_colors() -> dict[str, Any]:
    data = load_json(COLORS_FILE, DEFAULT_COLORS)
    if "themes" not in data:
        data["themes"] = {}
    return data


def save_colors(data: dict[str, Any]) -> None:
    save_json(COLORS_FILE, data)


def normalize_color(value: str) -> str | None:
    if not value:
        return None
    raw = value.strip().strip("'\"")
    if not raw:
        return None
    lowered = raw.lower()
    if lowered in NAMED_COLORS:
        return NAMED_COLORS[lowered]
    if raw.startswith("#"):
        hex_part = re.sub(r"[^0-9A-Fa-f]", "", raw[1:])
        if len(hex_part) == 3:
            hex_part = "".join(ch * 2 for ch in hex_part)
        if len(hex_part) == 6:
            return f"#{hex_part.upper()}"
        return None
    hex_part = re.sub(r"[^0-9A-Fa-f]", "", raw)
    if len(hex_part) == 3:
        hex_part = "".join(ch * 2 for ch in hex_part)
    if len(hex_part) == 6:
        return f"#{hex_part.upper()}"
    return None


def format_conky_color_value(color: str, new_syntax: bool) -> str:
    normalized = normalize_color(color)
    if new_syntax:
        if normalized:
            return f"'{normalized}'"
        return f"'{color.strip().strip(chr(39) + chr(34))}'"
    if normalized:
        return normalized[1:]
    return color.strip().strip("'\"")


def parse_theme_colors(content: str) -> dict[str, str]:
    slots: dict[str, str] = {}
    if _is_new_syntax(content):
        pattern = re.compile(
            r"^\s*(default_color|default_outline_color|default_shade_color|own_window_colour|color\d*)\s*=\s*([^,\n]+)",
            re.MULTILINE | re.IGNORECASE,
        )
        for match in pattern.finditer(content):
            slot = match.group(1)
            value = match.group(2).strip().strip("'\"")
            slots[slot] = value
        return slots

    pattern = re.compile(
        r"^\s*(default_color|default_outline_color|default_shade_color|own_window_colour|color\d*)\s+(\S+)",
        re.MULTILINE | re.IGNORECASE,
    )
    for match in pattern.finditer(content):
        slot = match.group(1)
        value = match.group(2).strip().strip("'\"")
        slots[slot] = value
    return slots


def ordered_theme_color_slots(theme_colors: dict[str, str]) -> list[str]:
    ordered: list[str] = []
    seen: set[str] = set()
    for slot in COLOR_SLOT_ORDER:
        if slot in theme_colors and slot not in seen:
            ordered.append(slot)
            seen.add(slot)
    for slot in sorted(theme_colors):
        if slot not in seen:
            ordered.append(slot)
            seen.add(slot)
    return ordered


def resolve_theme_colors(cfg_path: Path, colors_data: dict[str, Any] | None = None) -> dict[str, Any] | None:
    colors_data = colors_data or load_colors()
    saved = colors_data.get("themes", {}).get(str(cfg_path.resolve()))
    if saved and saved.get("enabled", True) and saved.get("colors"):
        return {
            "colors": dict(saved["colors"]),
            "smart_lua": bool(saved.get("smart_lua", True)),
        }
    return None


def merge_preset_colors(theme_colors: dict[str, str], preset_id: str) -> dict[str, str]:
    preset = COLOR_PRESETS.get(preset_id)
    if not preset:
        return dict(theme_colors)
    merged = dict(theme_colors)
    for slot, value in preset.get("colors", {}).items():
        if slot in merged:
            merged[slot] = value
    return merged


def build_smart_color_remap(original_colors: dict[str, str], new_colors: dict[str, str]) -> dict[str, str]:
    remap: dict[str, str] = {}
    for slot, new_value in new_colors.items():
        old_value = original_colors.get(slot)
        if not old_value:
            continue
        old_hex = normalize_color(old_value)
        new_hex = normalize_color(new_value)
        if old_hex and new_hex and old_hex != new_hex:
            remap[old_hex] = new_hex
    return remap


def apply_color_patch(content: str, colors: dict[str, str]) -> tuple[str, int]:
    fixes = 0
    if not colors:
        return content, fixes
    new_syntax = _is_new_syntax(content)
    for slot, new_color in colors.items():
        if new_syntax:
            value = format_conky_color_value(new_color, new_syntax=True)
            pattern = re.compile(
                rf"(^\s*{re.escape(slot)}\s*=\s*)([^,\n]+)(,?\s*$)",
                re.MULTILINE | re.IGNORECASE,
            )
            if pattern.search(content):
                content = pattern.sub(rf"\g<1>{value}\3", content, count=1)
                fixes += 1
        else:
            value = format_conky_color_value(new_color, new_syntax=False)
            pattern = re.compile(
                rf"(^\s*{re.escape(slot)}\s+)(\S+)(.*$)",
                re.MULTILINE | re.IGNORECASE,
            )
            if pattern.search(content):
                content = pattern.sub(rf"\g<1>{value}\3", content, count=1)
                fixes += 1
    return content, fixes


def apply_hex_remap(content: str, remap: dict[str, str]) -> tuple[str, int]:
    fixes = 0
    for old_color, new_color in remap.items():
        old_hex = normalize_color(old_color)
        new_hex = normalize_color(new_color)
        if not old_hex or not new_hex or old_hex == new_hex:
            continue
        old_body = old_hex[1:]
        new_body = new_hex[1:]
        replacements = (
            (rf"0x{old_body}", f"0x{new_body.upper()}"),
            (rf"0x{old_body.lower()}", f"0x{new_body.upper()}"),
            (rf"#{old_body}", f"#{new_body.upper()}"),
            (rf"#{old_body.lower()}", f"#{new_body.upper()}"),
        )
        for pattern, replacement in replacements:
            new_content, count = re.subn(pattern, replacement, content, flags=re.IGNORECASE)
            if count:
                content = new_content
                fixes += count
    return content, fixes


def _is_new_syntax(content: str):
    return "conky.config" in content and "conky.text" in content


def _patch_new_syntax(content: str, settings: dict):
    start_idx = content.find("conky.config")
    brace_idx = content.find("{", start_idx)
    if start_idx < 0 or brace_idx < 0:
        return content

    depth = 0
    end_idx = -1
    for idx in range(brace_idx, len(content)):
        char = content[idx]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                end_idx = idx
                break
    if end_idx == -1:
        return content

    block = content[brace_idx + 1 : end_idx]
    desktop = compute_desktop_settings(settings, new_syntax=True)
    for key, value in desktop.items():
        pattern = re.compile(rf"(^\s*{re.escape(key)}\s*=\s*.*?,\s*$)", re.MULTILINE)
        replacement = f"    {key} = {value},"
        if pattern.search(block):
            block = pattern.sub(replacement, block, count=1)
        else:
            block = f"{replacement}\n{block}"

    return content[: brace_idx + 1] + block + content[end_idx:]


def _patch_legacy_syntax(content: str, settings: dict):
    lines = content.splitlines()
    text_idx = len(lines)
    for idx, line in enumerate(lines):
        if line.strip().upper() == "TEXT":
            text_idx = idx
            break

    head = lines[:text_idx]
    tail = lines[text_idx:]
    key_to_idx: dict[str, int] = {}
    for idx, line in enumerate(head):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        key = stripped.split()[0]
        key_to_idx[key] = idx

    desktop = compute_desktop_settings(settings, new_syntax=False)
    for key, value in desktop.items():
        new_line = f"{key} {value}"
        if key in key_to_idx:
            head[key_to_idx[key]] = new_line
        else:
            head.insert(0, new_line)
    return "\n".join(head + tail) + "\n"


@dataclass
class ThemeLaunchBundle:
    source_config: Path
    theme_root: Path
    launch_dir: Path
    launch_config: Path
    path_fixes: int = 0
    missing_assets: list[str] | None = None
    fonts_dir: Path | None = None
    compat_links: list[tuple[str, str]] | None = None

    def __post_init__(self):
        if self.missing_assets is None:
            self.missing_assets = []
        if self.compat_links is None:
            self.compat_links = []


def _looks_like_theme_root(path: Path) -> bool:
    if not path.is_dir():
        return False
    names = {item.name.lower() for item in path.iterdir() if not item.name.startswith(".")}
    if names & {name.lower() for name in ASSET_DIR_NAMES}:
        return True
    if any(path.glob("*.conf")) or (path / "start.sh").exists():
        return True
    return False


def resolve_theme_root(cfg_path: Path) -> Path:
    cfg_path = cfg_path.resolve()
    directory = cfg_path.parent

    if directory.name.lower() in ("config", "conky", "scripts"):
        for parent in (directory.parent, directory.parent.parent):
            if parent.is_dir() and _looks_like_theme_root(parent):
                return parent.resolve()

    if _looks_like_theme_root(directory):
        return directory.resolve()

    for parent in list(directory.parents)[:4]:
        if _looks_like_theme_root(parent):
            return parent.resolve()
    return directory.resolve()


def _expand_theme_path(path_str: str) -> str:
    return os.path.expanduser(path_str.replace("$HOME", str(HOME)).strip("'\""))


def _find_asset_in_theme(theme_root: Path, raw_path: str) -> Path | None:
    theme_root = theme_root.resolve()
    expanded = _expand_theme_path(raw_path)
    if not expanded or expanded.startswith("-"):
        return None
    if any(token in expanded for token in ("$", "|", "&&", "||", "`")):
        return None

    candidate = Path(expanded)
    if candidate.is_file():
        return candidate.resolve()

    home = str(HOME)
    if expanded.startswith(home):
        rel = Path(expanded).relative_to(home)
        parts = rel.parts
        if len(parts) >= 3 and parts[0] == ".config" and parts[1] == "conky":
            tail = Path(*parts[3:]) if len(parts) > 3 else Path(".")
            remapped = (theme_root / tail).resolve()
            if remapped.is_file():
                return remapped
        if len(parts) >= 2 and parts[0] == ".conky":
            tail = Path(*parts[2:]) if len(parts) > 2 else Path(".")
            remapped = (theme_root / tail).resolve()
            if remapped.is_file():
                return remapped

    for base in (
        theme_root,
        theme_root / "res",
        theme_root / "assets",
        theme_root / "images",
        theme_root / "img",
        theme_root / "scripts",
    ):
        remapped = (base / expanded).resolve()
        if remapped.is_file():
            return remapped

    filename = Path(expanded).name
    if filename and filename not in (".", ".."):
        matches = [p for p in theme_root.rglob(filename) if p.is_file()]
        if len(matches) == 1:
            return matches[0].resolve()
    return None


def rewrite_theme_asset_paths(
    content: str,
    theme_root: Path,
    launch_dir: Path | None = None,
) -> tuple[str, int, list[str]]:
    theme_root = theme_root.resolve()
    fixes = 0
    missing: list[str] = []
    seen_missing: set[str] = set()

    def replace_path(match: re.Match[str]) -> str:
        nonlocal fixes
        original = match.group(1)
        resolved = _find_asset_in_theme(theme_root, original)
        if resolved:
            fixes += 1
            return match.group(0).replace(original, str(resolved))
        lowered = original.lower()
        if original.startswith("~/.cache/") or original.startswith(f"{HOME}/.cache/"):
            return match.group(0)
        if any(lowered.endswith(ext) for ext in ASSET_FILE_SUFFIXES):
            if original not in seen_missing:
                seen_missing.add(original)
                missing.append(original)
        return match.group(0)

    updated = content
    for pattern in PATH_REFERENCE_PATTERNS:
        updated = pattern.sub(replace_path, updated)

    theme_name = theme_root.name
    replacements = {
        f"~/.config/conky/{theme_name}": str(theme_root),
        f"$HOME/.config/conky/{theme_name}": str(theme_root),
        f"{HOME}/.config/conky/{theme_name}": str(theme_root),
        f"~/.conky/{theme_name}": str(theme_root),
        f"$HOME/.conky/{theme_name}": str(theme_root),
        f"{HOME}/.conky/{theme_name}": str(theme_root),
    }
    for old, new in replacements.items():
        if old in updated:
            updated = updated.replace(old, new)
            fixes += 1

    if launch_dir:
        launch_dir = launch_dir.resolve()
        script_root = launch_dir / "scripts"
        if script_root.is_dir():
            for old, new in (
                (f"{theme_root}/scripts", str(script_root)),
                (str(theme_root / "scripts"), str(script_root)),
            ):
                if old in updated:
                    updated = updated.replace(old, new)
                    fixes += 1

    return updated, fixes, missing


def ensure_compat_install_symlink(theme_root: Path) -> list[tuple[str, str]]:
    settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
    if not settings.get("create_compat_symlinks", True):
        return []

    theme_root = theme_root.resolve()
    compat = HOME / ".config" / "conky" / theme_root.name
    links: list[tuple[str, str]] = []
    try:
        compat.parent.mkdir(parents=True, exist_ok=True)
        if compat.is_symlink():
            target = compat.resolve()
            if target == theme_root:
                return links
            compat.unlink()
        elif compat.exists():
            return links

        compat.symlink_to(theme_root, target_is_directory=True)
        links.append((str(compat), str(theme_root)))
        logging.info("Created compatibility symlink: %s -> %s", compat, theme_root)
    except OSError as exc:
        logging.warning("Could not create compatibility symlink %s: %s", compat, exc)
    return links


def _materialize_patched_scripts(
    theme_root: Path,
    launch_dir: Path,
    hex_remap: dict[str, str] | None = None,
) -> int:
    """Copy scripts with corrected asset paths so shell/Lua helpers find images/icons."""
    scripts_src = theme_root / "scripts"
    if not scripts_src.is_dir():
        return 0

    scripts_dest = launch_dir / "scripts"
    if scripts_dest.exists() or scripts_dest.is_symlink():
        if scripts_dest.is_symlink():
            scripts_dest.unlink()
        else:
            shutil.rmtree(scripts_dest)

    shutil.copytree(scripts_src, scripts_dest, symlinks=True)
    patched_files = 0
    for script_file in scripts_dest.rglob("*"):
        if not script_file.is_file():
            continue
        if script_file.suffix.lower() not in (".sh", ".lua"):
            continue
        original = script_file.read_text(encoding="utf-8", errors="ignore")
        updated, fixes, _missing = rewrite_theme_asset_paths(original, theme_root, launch_dir)
        if hex_remap and script_file.suffix.lower() == ".lua":
            updated, color_fixes = apply_hex_remap(updated, hex_remap)
            fixes += color_fixes
        if fixes:
            script_file.write_text(updated, encoding="utf-8")
            if script_file.suffix == ".sh":
                script_file.chmod(script_file.stat().st_mode | 0o111)
            patched_files += 1
    return patched_files


def _link_theme_assets(theme_root: Path, launch_dir: Path, skip_dirs: set[str] | None = None) -> None:
    skip = {name.lower() for name in (skip_dirs or set())}
    linked: set[str] = set()
    for item in theme_root.iterdir():
        if item.name.startswith("."):
            continue
        if item.is_dir() and item.name.lower() in {name.lower() for name in ASSET_DIR_NAMES}:
            if item.name.lower() in skip:
                continue
            dest = launch_dir / item.name
            if dest.exists() or dest.is_symlink():
                dest.unlink()
            dest.symlink_to(item.resolve(), target_is_directory=True)
            linked.add(item.name)
        elif item.is_file() and item.suffix.lower() in ASSET_FILE_SUFFIXES:
            dest = launch_dir / item.name
            if dest.exists() or dest.is_symlink():
                dest.unlink()
            dest.symlink_to(item.resolve())
            linked.add(item.name)

    nested_config = theme_root / ".config" / "conky"
    if nested_config.is_dir():
        dest = launch_dir / ".config"
        if not dest.exists():
            dest.mkdir(parents=True, exist_ok=True)
        target = dest / "conky"
        if target.exists() or target.is_symlink():
            target.unlink()
        target.symlink_to(nested_config.resolve(), target_is_directory=True)
        linked.add(".config/conky")


def build_launch_env(bundle: ThemeLaunchBundle) -> dict[str, str]:
    env = _conky_env()
    env["CONKY_THEME_ROOT"] = str(bundle.theme_root)
    env["CONKY_MANAGER_LAUNCH_DIR"] = str(bundle.launch_dir)
    if bundle.fonts_dir and bundle.fonts_dir.is_dir():
        fontconfig_dir = bundle.launch_dir / ".fontconfig"
        fontconfig_dir.mkdir(parents=True, exist_ok=True)
        fonts_conf = fontconfig_dir / "fonts.conf"
        fonts_conf.write_text(
            "\n".join([
                '<?xml version="1.0"?>',
                '<!DOCTYPE fontconfig SYSTEM "fonts.dtd">',
                "<fontconfig>",
                f"  <dir>{bundle.fonts_dir.resolve()}</dir>",
                "</fontconfig>",
                "",
            ]),
            encoding="utf-8",
        )
        env["FONTCONFIG_FILE"] = str(fonts_conf)
    return env


def create_theme_launch_bundle(
    cfg_path: Path,
    apply_desktop_optimize: bool = True,
    position_override: dict[str, Any] | None = None,
    color_override: dict[str, Any] | None = None,
) -> ThemeLaunchBundle:
    cfg_path = cfg_path.resolve()
    theme_root = resolve_theme_root(cfg_path)
    original_content = cfg_path.read_text(encoding="utf-8", errors="ignore")
    content = original_content
    original_colors = parse_theme_colors(original_content)
    hex_remap: dict[str, str] | None = None
    color_fixes = 0

    if apply_desktop_optimize:
        settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
        if settings.get("auto_optimize", True) or position_override:
            if position_override:
                settings = merge_position_settings(settings, cfg_path, position_override)
            if _is_new_syntax(content):
                content = _patch_new_syntax(content, settings)
            else:
                content = _patch_legacy_syntax(content, settings)
    elif position_override:
        content = apply_position_patch(content, position_override)

    if color_override and color_override.get("colors"):
        override_colors = color_override["colors"]
        if color_override.get("smart_lua", True):
            hex_remap = build_smart_color_remap(original_colors, override_colors)
        content, color_fixes = apply_color_patch(content, override_colors)

    content, path_fixes, missing = rewrite_theme_asset_paths(content, theme_root)
    compat_links = ensure_compat_install_symlink(theme_root)

    bundle_name = normalize_folder_name(f"{theme_root.name}_{cfg_path.stem}")
    launch_dir = LAUNCH_DIR / bundle_name
    if launch_dir.exists():
        shutil.rmtree(launch_dir)
    launch_dir.mkdir(parents=True, exist_ok=True)

    patched_scripts = _materialize_patched_scripts(theme_root, launch_dir, hex_remap=hex_remap)
    content, extra_fixes, missing = rewrite_theme_asset_paths(content, theme_root, launch_dir)
    path_fixes += extra_fixes + patched_scripts + color_fixes

    launch_config = launch_dir / cfg_path.name
    launch_config.write_text(content, encoding="utf-8")
    _link_theme_assets(theme_root, launch_dir, skip_dirs={"scripts"})

    fonts_dir = theme_root / "fonts"
    if not fonts_dir.is_dir():
        fonts_dir = None

    bundle = ThemeLaunchBundle(
        source_config=cfg_path,
        theme_root=theme_root,
        launch_dir=launch_dir,
        launch_config=launch_config,
        path_fixes=path_fixes,
        missing_assets=missing,
        fonts_dir=fonts_dir,
        compat_links=compat_links,
    )
    logging.info(
        "Prepared launch bundle for %s (fixes=%s, missing=%s, root=%s)",
        cfg_path,
        path_fixes,
        len(missing),
        theme_root,
    )
    return bundle


def scan_theme_assets(cfg_path: Path) -> tuple[list[str], list[str]]:
    cfg_path = cfg_path.resolve()
    theme_root = resolve_theme_root(cfg_path)
    content = cfg_path.read_text(encoding="utf-8", errors="ignore")
    _, _, missing = rewrite_theme_asset_paths(content, theme_root)
    found: list[str] = []
    for pattern in PATH_REFERENCE_PATTERNS:
        for match in pattern.finditer(content):
            resolved = _find_asset_in_theme(theme_root, match.group(1))
            if resolved:
                found.append(str(resolved))
    return sorted(set(found)), sorted(set(missing))


def optimize_for_desktop(cfg_path: Path):
    bundle = create_theme_launch_bundle(cfg_path, apply_desktop_optimize=True)
    return bundle.launch_config


def optimize_for_kde(cfg_path: Path):
    """Backward-compatible alias."""
    return optimize_for_desktop(cfg_path)


def _conky_env() -> dict[str, str]:
    env = os.environ.copy()
    if "DISPLAY" not in env and "WAYLAND_DISPLAY" not in env:
        env["DISPLAY"] = ":0"
    return env


def validate_conky_config(
    config_path: Path,
    timeout_sec: int = 12,
    launch_dir: Path | None = None,
    env: dict[str, str] | None = None,
) -> bool:
    """Validate a config by running conky once (-i 1). Works on Conky 1.10+."""
    if not config_path.is_file():
        return False
    run_env = env or _conky_env()
    cwd = launch_dir if launch_dir and launch_dir.is_dir() else config_path.parent
    try:
        check = subprocess.run(
            ["conky", "-c", str(config_path), "-i", "1", "-q"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            check=False,
            timeout=timeout_sec,
            env=run_env,
            cwd=str(cwd),
        )
        return check.returncode == 0
    except Exception:
        return False


def kill_running_conky():
    result = subprocess.run(
        ["pkill", "-x", "conky"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    return result.returncode in (0, 1)


def _start_conky_with_healthcheck(
    config_path: Path,
    launch_dir: Path | None = None,
    env: dict[str, str] | None = None,
):
    settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
    health_seconds = float(settings.get("healthcheck_seconds", 2.0))
    nice_level = int(settings.get("nice_level", 10))
    run_env = env or _conky_env()
    cwd = launch_dir if launch_dir and launch_dir.is_dir() else config_path.parent
    log_handle = LOG_FILE.open("a", encoding="utf-8")

    cmd = ["conky", "-q", "-c", str(config_path)]
    if shutil.which("nice"):
        cmd = ["nice", "-n", str(nice_level), *cmd]

    process = subprocess.Popen(
        cmd,
        stdout=log_handle,
        stderr=log_handle,
        env=run_env,
        cwd=str(cwd),
        start_new_session=True,
    )
    waited = 0.0
    while waited < health_seconds:
        if process.poll() is not None:
            return process, False
        time.sleep(0.2)
        waited += 0.2
    return process, process.poll() is None


def run_conky(
    cfg_path: Path,
    position_override: dict[str, Any] | None = None,
    color_override: dict[str, Any] | None = None,
):
    cfg_path = Path(cfg_path)
    if not cfg_path.is_file():
        raise RuntimeError(f"Config not found: {cfg_path}")

    settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
    full_visual = settings.get("full_visual_launch", True)
    candidates: list[tuple[str, Path, ThemeLaunchBundle | None]] = []

    if full_visual:
        try:
            bundle = create_theme_launch_bundle(
                cfg_path,
                apply_desktop_optimize=True,
                position_override=position_override,
                color_override=color_override,
            )
            candidates.append(("full-visual", bundle.launch_config, bundle))
        except Exception as exc:
            logging.warning("Full visual launch bundle failed for %s: %s", cfg_path, exc)
            bundle = None

    if settings.get("auto_optimize", True) and not full_visual:
        try:
            optimized = optimize_for_desktop(cfg_path)
            fallback_bundle = create_theme_launch_bundle(
                cfg_path,
                apply_desktop_optimize=False,
                position_override=position_override,
                color_override=color_override,
            )
            candidates.append(("optimized", optimized, fallback_bundle))
        except Exception as exc:
            logging.warning("Optimization failed for %s: %s", cfg_path, exc)

    theme_root = resolve_theme_root(cfg_path)
    ensure_compat_install_symlink(theme_root)
    fallback_bundle = ThemeLaunchBundle(
        source_config=cfg_path,
        theme_root=theme_root,
        launch_dir=theme_root,
        launch_config=cfg_path,
        fonts_dir=(theme_root / "fonts") if (theme_root / "fonts").is_dir() else None,
    )
    candidates.append(("original-root", cfg_path, fallback_bundle))

    last_error = "unknown error"
    for mode_name, path, launch_bundle in candidates:
        if mode_name == "optimized" and not validate_conky_config(path):
            logging.warning("Skipping invalid optimized config: %s", path)
            continue

        launch_dir = launch_bundle.launch_dir if launch_bundle else resolve_theme_root(cfg_path)
        env = build_launch_env(launch_bundle) if launch_bundle else build_launch_env(
            ThemeLaunchBundle(
                source_config=cfg_path,
                theme_root=resolve_theme_root(cfg_path),
                launch_dir=launch_dir,
                launch_config=path,
            )
        )

        if launch_bundle and launch_bundle.missing_assets:
            logging.warning(
                "Theme %s has missing assets: %s",
                cfg_path,
                ", ".join(launch_bundle.missing_assets[:5]),
            )

        process, healthy = _start_conky_with_healthcheck(path, launch_dir=launch_dir, env=env)
        if healthy:
            logging.info(
                "Started Conky (%s): %s (pid=%s, root=%s, fixes=%s)",
                mode_name,
                path,
                process.pid,
                launch_dir,
                launch_bundle.path_fixes if launch_bundle else 0,
            )
            return process.pid, path, mode_name
        last_error = f"{mode_name} process exited during startup"

    raise RuntimeError(f"Theme failed to start: {cfg_path} ({last_error})")


@dataclass(frozen=True)
class ThemeItem:
    path: Path

    @property
    def label(self):
        return f"{self.path.parent.name}/{self.path.name}"


@dataclass(frozen=True)
class ThemeSource:
    id: str
    name: str
    description: str
    author: str
    url: str
    homepage: str
    tags: tuple[str, ...] = ()


THEME_CATALOG: tuple[ThemeSource, ...] = (
    ThemeSource(
        "harmony",
        "Harmony Light/Dark Global",
        "Versatile global theme with light and dark variants.",
        "Closebox73",
        "https://github.com/closebox73/Harmony-Light-Dark-Global-Conky/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Harmony-Light-Dark-Global-Conky",
        ("modern", "global", "popular"),
    ),
    ThemeSource(
        "deneb",
        "Deneb",
        "Part of the Ursa Major Conky themes pack.",
        "Closebox73",
        "https://github.com/closebox73/Deneb/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Deneb",
        ("ursa-major", "elegant"),
    ),
    ThemeSource(
        "vega",
        "Vega",
        "Clean monitoring layout from Closebox73.",
        "Closebox73",
        "https://github.com/closebox73/Vega/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Vega",
        ("ursa-major", "minimal"),
    ),
    ThemeSource(
        "constellation",
        "Constellation",
        "Stylish desktop widget theme.",
        "Closebox73",
        "https://github.com/closebox73/Constellation/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Constellation",
        ("ursa-major",),
    ),
    ThemeSource(
        "ormix",
        "Ormix",
        "Modular widgets: clock, weather, music, and more.",
        "Closebox73",
        "https://github.com/closebox73/Ormix/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Ormix",
        ("modular", "widgets"),
    ),
    ThemeSource(
        "helium",
        "Helium",
        "Lightweight rounded Conky layout.",
        "Closebox73",
        "https://github.com/closebox73/Helium/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Helium",
        ("light",),
    ),
    ThemeSource(
        "hydra",
        "Hydra",
        "Multi-panel system monitor theme.",
        "Closebox73",
        "https://github.com/closebox73/Hydra/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Hydra",
        ("monitor",),
    ),
    ThemeSource(
        "fuchsia",
        "Fuchsia",
        "Colorful modern Conky skin.",
        "Closebox73",
        "https://github.com/closebox73/Fuchsia/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Fuchsia",
        ("colorful",),
    ),
    ThemeSource(
        "mimosa-light",
        "Mimosa Light",
        "Bright minimal Conky theme.",
        "Closebox73",
        "https://github.com/closebox73/Mimosa-Light/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Mimosa-Light",
        ("light", "minimal"),
    ),
    ThemeSource(
        "gotham",
        "Gotham Conky",
        "Classic Gotham-style system monitor.",
        "flobz",
        "https://github.com/flobz/gotham-conky/archive/refs/heads/main.zip",
        "https://github.com/flobz/gotham-conky",
        ("classic", "monitor"),
    ),
    ThemeSource(
        "os-monitoring",
        "Conky OS Monitoring",
        "Modern OS monitoring layout by EliverLara.",
        "EliverLara",
        "https://github.com/EliverLara/conky-os-monitoring/archive/refs/heads/main.zip",
        "https://github.com/EliverLara/conky-os-monitoring",
        ("monitor", "modern"),
    ),
    ThemeSource(
        "conky-lua",
        "Conky Lua",
        "Lua-powered Conky configuration collection.",
        "bluedx",
        "https://github.com/bluedx/conky-lua/archive/refs/heads/master.zip",
        "https://github.com/bluedx/conky-lua",
        ("lua", "advanced"),
    ),
    ThemeSource(
        "mx-conky",
        "MX Conky Collection",
        "Popular Conky pack from MX Linux community.",
        "MX-Linux",
        "https://github.com/MX-Linux/mx-conky-data/archive/refs/heads/master.zip",
        "https://github.com/MX-Linux/mx-conky-data",
        ("collection", "distro"),
    ),
    ThemeSource(
        "quatrofusion",
        "Quatro Fusion",
        "Four-part fusion Conky theme pack.",
        "Closebox73",
        "https://github.com/closebox73/quatrofusion/archive/refs/heads/main.zip",
        "https://github.com/closebox73/quatrofusion",
        ("pack",),
    ),
    ThemeSource(
        "nebula",
        "Nebula",
        "Space-inspired Conky desktop theme.",
        "Closebox73",
        "https://github.com/closebox73/Nebula/archive/refs/heads/main.zip",
        "https://github.com/closebox73/Nebula",
        ("space", "dark"),
    ),
)


def normalize_folder_name(name: str) -> str:
    cleaned = re.sub(r"[^\w\s\-+.]", "", name).strip() or "conky-theme"
    for suffix in STRIP_FOLDER_SUFFIXES:
        if cleaned.endswith(suffix):
            cleaned = cleaned[: -len(suffix)]
    return cleaned.strip("-_") or "conky-theme"


def _archive_kind(path: Path) -> str | None:
    lower = path.name.lower()
    for ext in (".tar.gz", ".tar.bz2", ".tar.xz", ".tgz", ".tbz2", ".txz"):
        if lower.endswith(ext):
            return ext.lstrip(".")
    for ext in (".zip", ".tar", ".7z"):
        if lower.endswith(ext):
            return ext.lstrip(".")
    return None


def extract_archive(archive_path: Path, dest_dir: Path) -> None:
    kind = _archive_kind(archive_path)
    if not kind:
        raise RuntimeError(f"Unsupported archive format: {archive_path.name}")

    dest_dir.mkdir(parents=True, exist_ok=True)
    if kind == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            for member in zf.infolist():
                if member.filename.startswith("/") or ".." in member.filename:
                    raise RuntimeError(f"Unsafe path in archive: {member.filename}")
            zf.extractall(dest_dir)
        return

    if kind in ("tar", "tar.gz", "tar.bz2", "tar.xz", "tgz", "tbz2", "txz"):
        mode_map = {
            "tar": "r:",
            "tar.gz": "r:gz",
            "tgz": "r:gz",
            "tar.bz2": "r:bz2",
            "tbz2": "r:bz2",
            "tar.xz": "r:xz",
            "txz": "r:xz",
        }
        with tarfile.open(archive_path, mode_map[kind]) as tf:
            for member in tf.getmembers():
                if member.name.startswith("/") or ".." in member.name:
                    raise RuntimeError(f"Unsafe path in archive: {member.name}")
            try:
                tf.extractall(dest_dir, filter="data")
            except TypeError:
                tf.extractall(dest_dir)
        return

    if kind == "7z":
        if shutil.which("7z"):
            subprocess.run(["7z", "x", str(archive_path), f"-o{dest_dir}", "-y"], check=True)
            return
        if shutil.which("7za"):
            subprocess.run(["7za", "x", str(archive_path), f"-o{dest_dir}", "-y"], check=True)
            return
        raise RuntimeError("7z archives require p7zip (7z command) to be installed.")

    raise RuntimeError(f"Unsupported archive format: {archive_path.name}")


def _theme_folder_for_config(cfg: Path, extract_root: Path) -> Path:
    folder = cfg.parent
    if folder.name.lower() in ("config", "conky", "scripts") and folder.parent != extract_root:
        folder = folder.parent
    try:
        rel_parts = folder.relative_to(extract_root).parts
        if len(rel_parts) > 1:
            return extract_root / rel_parts[0]
    except ValueError:
        pass
    return folder


def discover_install_units(extract_root: Path) -> list[Path]:
    units: dict[str, Path] = {}
    candidates: list[Path] = list(extract_root.rglob("*.conf"))
    for name in ("conky.conf", "conkyrc"):
        candidates.extend(p for p in extract_root.rglob(name) if p.is_file())

    for cfg in candidates:
        if not cfg.is_file() or not is_valid_theme(cfg):
            continue
        unit = _theme_folder_for_config(cfg, extract_root)
        units[str(unit.resolve())] = unit

    if units:
        return sorted(units.values(), key=lambda p: p.name.lower())

    children = [p for p in extract_root.iterdir() if p.is_dir() and not p.name.startswith(".")]
    if len(children) == 1:
        return [children[0]]
    if children:
        return sorted(children, key=lambda p: p.name.lower())
    return [extract_root]


def _unique_destination(base: Path, name: str) -> Path:
    dest = base / name
    if not dest.exists():
        return dest
    index = 2
    while True:
        candidate = base / f"{name}-{index}"
        if not candidate.exists():
            return candidate
        index += 1


def install_tree(source: Path, target_base: Path, preferred_name: str | None = None) -> str:
    target_base.mkdir(parents=True, exist_ok=True)
    folder_name = normalize_folder_name(preferred_name or source.name)
    dest = _unique_destination(target_base, folder_name)
    shutil.copytree(source, dest, dirs_exist_ok=False, symlinks=True)
    ensure_compat_install_symlink(dest)
    logging.info("Installed theme folder: %s -> %s", source, dest)
    return dest.name


def import_folder_to_conky(source: Path, target_base: Path | None = None) -> list[str]:
    if not source.is_dir():
        raise RuntimeError(f"Not a folder: {source}")
    target = target_base or DEFAULT_IMPORT_DIR
    return [install_tree(source, target, source.name)]


def import_archive_to_conky(archive_path: Path, target_base: Path | None = None) -> list[str]:
    if not archive_path.is_file():
        raise RuntimeError(f"Archive not found: {archive_path}")
    if _archive_kind(archive_path) is None:
        raise RuntimeError(
            f"Unsupported archive: {archive_path.name}\n"
            f"Supported: {', '.join(ARCHIVE_EXTENSIONS)}"
        )

    target = target_base or DEFAULT_IMPORT_DIR
    installed: list[str] = []

    with tempfile.TemporaryDirectory(prefix="conky-import-", dir=BASE_DIR) as tmp:
        tmp_path = Path(tmp)
        extract_archive(archive_path, tmp_path)

        entries = [p for p in tmp_path.iterdir() if not p.name.startswith(".")]
        work_root = entries[0] if len(entries) == 1 and entries[0].is_dir() else tmp_path
        default_name = normalize_folder_name(work_root.name)

        units = discover_install_units(work_root)
        if len(units) == 1:
            installed.append(install_tree(units[0], target, default_name))
        else:
            for unit in units:
                installed.append(install_tree(unit, target, unit.name))

    return installed


def validate_download_url(url: str) -> None:
    parsed = urlparse(url)
    if parsed.scheme != "https":
        raise RuntimeError("Only HTTPS downloads are allowed.")
    host = parsed.netloc.lower()
    if re.fullmatch(r"files\d+\.pling\.com", host):
        return
    if not any(host == allowed or host.endswith("." + allowed) for allowed in ALLOWED_DOWNLOAD_HOSTS):
        raise RuntimeError(f"Download host not allowed: {host}")


def download_file(
    url: str,
    dest_path: Path,
    progress_callback: Callable[[float, str], None] | None = None,
) -> Path:
    validate_download_url(url)
    dest_path.parent.mkdir(parents=True, exist_ok=True)
    request = Request(url, headers={"User-Agent": f"{APP_NAME}/{APP_VERSION}"})

    with urlopen_secure(request, timeout=120) as response:
        total = int(response.headers.get("Content-Length") or 0)
        downloaded = 0
        chunk_size = 1024 * 64
        with dest_path.open("wb") as handle:
            while True:
                chunk = response.read(chunk_size)
                if not chunk:
                    break
                handle.write(chunk)
                downloaded += len(chunk)
                if progress_callback and total > 0:
                    progress_callback(downloaded / total, f"Downloading… {downloaded // 1024} KB")

    if progress_callback:
        progress_callback(1.0, "Download complete")
    return dest_path


def download_and_install_theme(
    source: ThemeSource,
    target_base: Path | None = None,
    progress_callback: Callable[[float, str], None] | None = None,
) -> list[str]:
    target = target_base or DEFAULT_IMPORT_DIR
    archive_name = f"{source.id}-{int(time.time())}.zip"
    archive_path = DOWNLOADS_DIR / archive_name

    if progress_callback:
        progress_callback(0.05, f"Connecting to {urlparse(source.url).netloc}…")

    download_file(source.url, archive_path, progress_callback)

    if progress_callback:
        progress_callback(0.9, "Extracting and installing…")

    installed = import_archive_to_conky(archive_path, target)

    if progress_callback:
        progress_callback(1.0, f"Installed {len(installed)} theme(s)")

    return installed


def get_theme_source_by_id(source_id: str) -> ThemeSource | None:
    for item in THEME_CATALOG:
        if item.id == source_id:
            return item
    return None


@dataclass(frozen=True)
class OnlineStore:
    id: str
    label: str
    site_url: str
    browse_url: str
    api_host: str
    details_api_host: str
    page_url_template: str


ONLINE_STORES: tuple[OnlineStore, ...] = (
    OnlineStore(
        "gnome-look",
        "gnome-look.org",
        "https://www.gnome-look.org",
        "https://www.gnome-look.org/browse?cat=124&ord=downloads",
        "api.pling.com",
        "api.gnome-look.org",
        "https://www.gnome-look.org/p/{id}",
    ),
    OnlineStore(
        "kde-look",
        "KDE Look",
        "https://www.kde-look.org",
        "https://www.kde-look.org/browse?cat=124&ord=downloads",
        "api.kde-look.org",
        "api.kde-look.org",
        "https://www.kde-look.org/p/{id}",
    ),
    OnlineStore(
        "pling",
        "Pling.com",
        "https://www.pling.com",
        "https://www.pling.com/browse?cat=124&ord=downloads",
        "api.pling.com",
        "api.pling.com",
        "https://www.pling.com/p/{id}",
    ),
    OnlineStore(
        "opendesktop",
        "OpenDesktop.org",
        "https://www.opendesktop.org",
        "https://www.opendesktop.org/browse?cat=124&ord=downloads",
        "api.pling.com",
        "api.pling.com",
        "https://www.pling.com/p/{id}",
    ),
)


@dataclass
class OnlineProduct:
    product_id: int
    name: str
    summary: str
    author: str
    downloads: int
    score: float
    version: str
    preview_url: str
    page_url: str
    store_id: str


def _strip_html(text: str) -> str:
    cleaned = re.sub(r"<[^>]+>", " ", text or "")
    cleaned = re.sub(r"\s+", " ", cleaned)
    return cleaned.strip()


def _ocs_request(api_host: str, endpoint: str, params: dict[str, Any]) -> dict[str, Any]:
    query = dict(params)
    query.setdefault("format", "json")
    url = f"https://{api_host}/ocs/v1/content/{endpoint}?{urlencode(query)}"
    request = Request(url, headers={"User-Agent": f"{APP_NAME}/{APP_VERSION}"})
    with urlopen_secure(request, timeout=45) as response:
        payload = json.load(response)
    if payload.get("status") != "ok":
        message = payload.get("message") or "Store API request failed"
        raise RuntimeError(message)
    return payload


def get_online_store(store_id: str) -> OnlineStore:
    for store in ONLINE_STORES:
        if store.id == store_id:
            return store
    return ONLINE_STORES[0]


def browse_online_store(
    store: OnlineStore,
    query: str = "",
    page: int = 1,
    per_page: int = 20,
    sort: str = "downloads",
) -> tuple[list[OnlineProduct], int]:
    params: dict[str, Any] = {
        "page": page,
        "itemsperpage": per_page,
        "ord": sort,
        "categories": CONKY_STORE_CATEGORY,
    }
    search = query.strip()
    if search:
        params["search"] = search
    elif store.id == "kde-look":
        params["search"] = "conky"

    payload = _ocs_request(store.api_host, "data", params)
    total = int(payload.get("totalitems") or 0)
    products: list[OnlineProduct] = []

    for item in payload.get("data", []):
        if item.get("typename") != "Conky" and item.get("xdg_type") != "conky":
            continue
        product_id = int(item["id"])
        preview = (
            item.get("previewpic1")
            or item.get("smallpreviewpic1")
            or item.get("previewpic2")
            or ""
        )
        products.append(
            OnlineProduct(
                product_id=product_id,
                name=str(item.get("name") or "Unnamed theme"),
                summary=_strip_html(str(item.get("summary") or "")),
                author=str(item.get("personid") or "unknown"),
                downloads=int(item.get("downloads") or 0),
                score=float(item.get("score") or 0),
                version=str(item.get("version") or ""),
                preview_url=preview,
                page_url=store.page_url_template.format(id=product_id),
                store_id=store.id,
            )
        )

    return products, total


def fetch_online_product_details(store: OnlineStore, product_id: int) -> dict[str, Any]:
    payload = _ocs_request(store.details_api_host, f"data/{product_id}", {})
    items = payload.get("data") or []
    if not items:
        raise RuntimeError(f"Theme not found on {store.label}: {product_id}")
    return items[0]


def extract_product_downloads(details: dict[str, Any]) -> list[tuple[str, str]]:
    links: list[tuple[str, str]] = []
    for index in range(1, 6):
        link = details.get(f"downloadlink{index}")
        name = details.get(f"downloadname{index}") or f"download-{index}"
        if link:
            links.append((str(name), str(link)))
    return links


def pick_theme_download(details: dict[str, Any]) -> tuple[str, str]:
    links = extract_product_downloads(details)
    archive_ext = (".zip", ".tar.gz", ".tar.xz", ".tar.bz2", ".tar", ".tgz", ".txz", ".7z")
    for name, url in links:
        lower = name.lower()
        if any(lower.endswith(ext) for ext in archive_ext):
            return name, url
    if links:
        return links[0]
    raise RuntimeError("No downloadable archive found for this theme.")


def download_and_install_online_product(
    store: OnlineStore,
    product_id: int,
    progress_callback: Callable[[float, str], None] | None = None,
) -> list[str]:
    if progress_callback:
        progress_callback(0.05, f"Reading theme info from {store.label}…")

    details = fetch_online_product_details(store, product_id)
    download_name, download_url = pick_theme_download(details)

    if progress_callback:
        progress_callback(0.12, f"Downloading {download_name}…")

    suffix = Path(download_name).suffix or ".zip"
    archive_path = DOWNLOADS_DIR / f"store-{product_id}-{int(time.time())}{suffix}"
    download_file(download_url, archive_path, progress_callback)

    if progress_callback:
        progress_callback(0.9, "Extracting and installing into ~/.conky…")

    if _archive_kind(archive_path):
        installed = import_archive_to_conky(archive_path)
    else:
        folder_name = normalize_folder_name(str(details.get("name") or download_name))
        dest = _unique_destination(DEFAULT_IMPORT_DIR, folder_name)
        dest.mkdir(parents=True, exist_ok=True)
        shutil.copy2(archive_path, dest / archive_path.name)
        installed = [dest.name]

    if progress_callback:
        progress_callback(1.0, f"Installed {len(installed)} item(s)")
    return installed


# ---------------------------------------------------------------------------
# GTK application
# ---------------------------------------------------------------------------

class ConkyManagerGTK:
    """Professional Conky theme manager with GTK 3 interface."""

    def __init__(self):
        self.session, self.desktop = detect_environment()
        self.desktop_env = detect_desktop_environment()
        self.settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
        self.profiles = load_json(PROFILES_FILE, {"profiles": {}, "last_profile": ""})

        self.themes: list[ThemeItem] = []
        self.filtered: list[ThemeItem] = []
        self.theme_status: dict[str, str] = {}
        self._worker_lock = threading.Lock()
        self._worker_busy = False
        self._preview_timer_id: int | None = None
        self._busy_buttons: list[Gtk.Widget] = []
        self._position_loading = False
        self._position_buttons: dict[str, Gtk.ToggleButton] = {}
        self._active_position_theme: Path | None = None
        self._color_loading = False
        self._color_pickers: dict[str, Gtk.ColorButton] = {}
        self._theme_color_slots: dict[str, str] = {}
        self._active_color_theme: Path | None = None

        self.setup_ui()
        self.apply_ui_theme()
        self.load_themes()
        self.refresh_profiles()
        self.refresh_log_view()
        self.show_wayland_notice()

    # ---------------- UI setup ----------------

    def setup_ui(self):
        self.win = Gtk.Window(title=f"{APP_NAME} {APP_VERSION}")
        self.win.set_default_size(1024, 680)
        self.win.set_resizable(True)
        self.win.connect("destroy", Gtk.main_quit)

        main_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=0)
        self.win.add(main_box)

        header = Gtk.HeaderBar()
        header.set_show_close_button(True)
        header.props.title = APP_NAME
        header.props.subtitle = (
            f"{APP_VERSION} | {self.desktop_env.upper()} | {self.session} | {self.desktop}"
        )
        self.win.set_titlebar(header)

        menu_btn = Gtk.MenuButton()
        menu_icon = Gtk.Image.new_from_icon_name("open-menu-symbolic", Gtk.IconSize.BUTTON)
        menu_btn.add(menu_icon)
        menu = Gtk.Menu()

        settings_item = Gtk.MenuItem(label="⚙ Settings")
        settings_item.connect("activate", self.show_settings_dialog)
        menu.append(settings_item)

        refresh_item = Gtk.MenuItem(label="🔄 Refresh Themes")
        refresh_item.connect("activate", lambda *_: self.load_themes())
        menu.append(refresh_item)

        store_item = Gtk.MenuItem(label="🌐 Get Themes Online")
        store_item.connect("activate", self.show_theme_store_dialog)
        menu.append(store_item)

        menu.append(Gtk.SeparatorMenuItem())

        about_item = Gtk.MenuItem(label="About")
        about_item.connect("activate", self.show_about)
        menu.append(about_item)

        menu.show_all()
        menu_btn.set_popup(menu)
        header.pack_end(menu_btn)

        content_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=0)
        main_box.pack_start(content_box, True, True, 0)

        self.setup_themes_sidebar(content_box)
        self.setup_main_panel(content_box)
        self.setup_statusbar(main_box)

        self.win.show_all()

    def setup_themes_sidebar(self, parent: Gtk.Box):
        sidebar_frame = Gtk.Frame()
        sidebar_frame.set_shadow_type(Gtk.ShadowType.IN)
        sidebar_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        sidebar_box.set_margin_top(10)
        sidebar_box.set_margin_bottom(10)
        sidebar_box.set_margin_start(10)
        sidebar_box.set_margin_end(10)
        sidebar_frame.add(sidebar_box)
        sidebar_frame.set_size_request(300, -1)
        parent.pack_start(sidebar_frame, False, False, 0)

        title = Gtk.Label()
        title.set_markup("<b>Themes</b>")
        title.set_xalign(0)
        sidebar_box.pack_start(title, False, False, 0)

        self.search_entry = Gtk.SearchEntry()
        self.search_entry.set_placeholder_text("Search themes...")
        self.search_entry.connect("search-changed", self.on_search_changed)
        sidebar_box.pack_start(self.search_entry, False, False, 0)

        scroll = Gtk.ScrolledWindow()
        scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        scroll.set_min_content_height(200)

        self.theme_store = Gtk.ListStore(str, str, str)  # label, status, path
        self.theme_tree = Gtk.TreeView(model=self.theme_store)
        self.theme_tree.set_headers_visible(False)
        self.theme_tree.get_selection().set_mode(Gtk.SelectionMode.MULTIPLE)
        self.theme_tree.get_selection().connect("changed", self.on_theme_selection_changed)
        self.theme_tree.connect("button-press-event", self.on_theme_tree_button_press)

        name_renderer = Gtk.CellRendererText()
        name_renderer.set_property("ellipsize", 3)  # PANGO_ELLIPSIZE_END
        name_col = Gtk.TreeViewColumn("Theme", name_renderer, text=0)
        name_col.set_expand(True)
        self.theme_tree.append_column(name_col)

        status_renderer = Gtk.CellRendererText()
        status_col = Gtk.TreeViewColumn("Status", status_renderer, text=1)
        status_col.set_fixed_width(80)
        self.theme_tree.append_column(status_col)

        scroll.add(self.theme_tree)
        sidebar_box.pack_start(scroll, True, True, 0)

        toolbar = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=5)
        self.btn_refresh = Gtk.Button.new_with_label("🔄")
        self.btn_refresh.set_tooltip_text("Refresh theme list")
        self.btn_refresh.connect("clicked", lambda *_: self.load_themes())

        import_menu_btn = Gtk.MenuButton()
        import_menu_btn.add(Gtk.Label(label="📥 Import"))
        import_menu_btn.set_tooltip_text("Import folder or archive into ~/.conky")
        import_menu = Gtk.Menu()
        import_folder_item = Gtk.MenuItem(label="Import folder…")
        import_folder_item.connect("activate", self.import_theme_folder)
        import_archive_item = Gtk.MenuItem(label="Import archive (zip, tar…)…")
        import_archive_item.connect("activate", self.import_theme_archive)
        import_menu.append(import_folder_item)
        import_menu.append(import_archive_item)
        import_menu.show_all()
        import_menu_btn.set_popup(import_menu)
        self.btn_import = import_menu_btn

        self.btn_download = Gtk.Button.new_with_label("🌐 Get Themes")
        self.btn_download.set_tooltip_text(
            "Download themes from GitHub, gnome-look.org, KDE Look, and Pling"
        )
        self.btn_download.get_style_context().add_class("suggested-action")
        self.btn_download.connect("clicked", self.show_theme_store_dialog)

        self.btn_health = Gtk.Button.new_with_label("🔍 Scan")
        self.btn_health.set_tooltip_text("Health scan all themes")
        self.btn_health.connect("clicked", self.health_scan)
        for btn in (self.btn_refresh, self.btn_import, self.btn_download, self.btn_health):
            toolbar.pack_start(btn, True, True, 0)
        sidebar_box.pack_start(toolbar, False, False, 0)

        actions = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        self.btn_run_selected = self._action_btn("▶ Run Selected", "suggested-action", self.run_selected)
        self.btn_preview = self._action_btn("👁 Preview", None, self.preview_selected)
        self.btn_edit = self._action_btn("✏ Edit", None, self.edit_selected_theme)
        self.btn_open_folder = self._action_btn("📁 Open Folder", None, self.open_selected_theme_folder)
        self.btn_repair = self._action_btn("🔧 Smart Repair", None, self.smart_repair)
        for btn in (self.btn_run_selected, self.btn_preview, self.btn_edit, self.btn_open_folder, self.btn_repair):
            actions.pack_start(btn, False, False, 0)
        sidebar_box.pack_start(actions, False, False, 0)

        self._busy_buttons.extend([
            self.btn_refresh, self.btn_import, self.btn_download, self.btn_health,
            self.btn_run_selected, self.btn_preview, self.btn_edit,
            self.btn_open_folder, self.btn_repair,
        ])

    def setup_main_panel(self, parent: Gtk.Box):
        main_panel = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        main_panel.set_margin_top(12)
        main_panel.set_margin_bottom(12)
        main_panel.set_margin_start(12)
        main_panel.set_margin_end(12)
        parent.pack_start(main_panel, True, True, 0)

        self.notebook = Gtk.Notebook()
        main_panel.pack_start(self.notebook, True, True, 0)

        self.build_tab_control()
        self.build_tab_position()
        self.build_tab_colors()
        self.build_tab_profiles()
        self.build_tab_diagnostics()

    def build_tab_control(self):
        tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        tab.set_margin_top(10)
        tab.set_margin_bottom(10)
        tab.set_margin_start(10)
        tab.set_margin_end(10)

        status_frame = Gtk.Frame(label="Runtime Status")
        status_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        status_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        status_box.set_margin_top(12)
        status_box.set_margin_bottom(12)
        status_box.set_margin_start(12)
        status_box.set_margin_end(12)
        status_frame.add(status_box)

        status_header = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.status_indicator = Gtk.Label()
        self.status_indicator.set_markup("<span size='large' foreground='#64748b'>●</span> Idle")
        status_header.pack_start(self.status_indicator, False, False, 0)
        self.runtime_info = Gtk.Label(label="No Conky instance running")
        self.runtime_info.set_xalign(0)
        self.runtime_info.set_line_wrap(True)
        status_header.pack_start(self.runtime_info, True, True, 0)
        status_box.pack_start(status_header, False, False, 0)

        env_label = Gtk.Label()
        env_label.set_markup(
            f"<span foreground='#888888'>Desktop: {self.desktop_env} | "
            f"Session: {self.session} | {self.desktop}</span>"
        )
        env_label.set_xalign(0)
        status_box.pack_start(env_label, False, False, 0)

        tab.pack_start(status_frame, False, False, 0)

        controls_frame = Gtk.Frame(label="Quick Actions")
        controls_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        controls_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        controls_box.set_margin_top(12)
        controls_box.set_margin_bottom(12)
        controls_box.set_margin_start(12)
        controls_box.set_margin_end(12)
        controls_frame.add(controls_box)

        self.btn_run_all = self._action_btn("▶ Run All", "suggested-action", self.run_all)
        self.btn_stop = self._action_btn("⏹ Stop All", "destructive-action", self.stop_all)
        self.btn_validate = Gtk.Button.new_with_label("✓ Validate Selected")
        self.btn_validate.connect("clicked", self.validate_selected_theme)
        self.btn_open_log = Gtk.Button.new_with_label("📄 Open Log")
        self.btn_open_log.connect("clicked", lambda *_: subprocess.Popen(["xdg-open", str(LOG_FILE)]))
        self.btn_open_opt = Gtk.Button.new_with_label("📂 Optimized")
        self.btn_open_opt.connect("clicked", lambda *_: subprocess.Popen(["xdg-open", str(OPTIMIZED_DIR)]))

        for btn in (self.btn_run_all, self.btn_stop, self.btn_validate, self.btn_open_log, self.btn_open_opt):
            controls_box.pack_start(btn, False, False, 0)

        tab.pack_start(controls_frame, False, False, 0)

        hint_frame = Gtk.Frame(label="Tips")
        hint_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        hint_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        hint_box.set_margin_top(10)
        hint_box.set_margin_bottom(10)
        hint_box.set_margin_start(12)
        hint_box.set_margin_end(12)
        hint_frame.add(hint_box)
        for text in (
            "• Select themes in the sidebar (Ctrl+click for multiple).",
            "• Preview runs a theme temporarily without saving.",
            "• Smart Repair rebuilds launch bundles with images, fonts, and scripts.",
            "• Full Visual Launch fixes broken ~/.config/conky paths automatically.",
            "• Use the Position tab to control theme alignment and screen offset.",
            "• Use the Colors tab to recolor themes safely via launch bundles.",
            "• Import folders or archives (zip, tar.gz, tar.xz…) into ~/.conky.",
            "• Use 🌐 Get Themes for GitHub, gnome-look.org, KDE Look, and Pling.",
            f"• Detected environment: {self.desktop_env} ({self.session}).",
        ):
            lbl = Gtk.Label(label=text)
            lbl.set_xalign(0)
            lbl.get_style_context().add_class("dim-label")
            hint_box.pack_start(lbl, False, False, 0)
        tab.pack_start(hint_frame, True, True, 0)

        self._busy_buttons.extend([self.btn_run_all, self.btn_validate])

        self.notebook.append_page(tab, Gtk.Label(label="Control"))

    def build_tab_position(self):
        tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        tab.set_margin_top(10)
        tab.set_margin_bottom(10)
        tab.set_margin_start(10)
        tab.set_margin_end(10)

        intro = Gtk.Label(
            label="Select a theme from the sidebar, then choose where it appears on your desktop.",
            xalign=0,
            wrap=True,
        )
        intro.get_style_context().add_class("dim-label")
        tab.pack_start(intro, False, False, 0)

        self._build_position_panel(tab)

        hint_frame = Gtk.Frame(label="Position Tips")
        hint_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        hint_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        hint_box.set_margin_top(10)
        hint_box.set_margin_bottom(10)
        hint_box.set_margin_start(12)
        hint_box.set_margin_end(12)
        hint_frame.add(hint_box)
        for text in (
            "• Click a grid cell to set alignment (top-left, center, bottom-right, etc.).",
            "• gap_x / gap_y are pixel offsets from the chosen screen edge.",
            "• Use arrow buttons for fine-tuning, then Apply & Restart to preview live.",
            "• Save for Theme keeps a unique position for each Conky theme.",
        ):
            lbl = Gtk.Label(label=text)
            lbl.set_xalign(0)
            lbl.get_style_context().add_class("dim-label")
            hint_box.pack_start(lbl, False, False, 0)
        tab.pack_start(hint_frame, True, True, 0)

        self._busy_buttons.extend([
            self.btn_save_position,
            self.btn_reset_position,
            self.btn_apply_position,
        ])
        self.notebook.append_page(tab, Gtk.Label(label="Position"))

    def _get_screen_size(self) -> tuple[int, int]:
        try:
            display = Gdk.Display.get_default()
            if display:
                monitor = display.get_primary_monitor()
                if monitor is not None:
                    geometry = display.get_monitor_geometry(monitor)
                    return geometry.width, geometry.height
        except Exception:
            pass
        return 1920, 1080

    def _build_position_panel(self, parent: Gtk.Box):
        screen_w, screen_h = self._get_screen_size()
        pos_frame = Gtk.Frame(label="Alignment & Offset")
        pos_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        pos_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        pos_box.set_margin_top(10)
        pos_box.set_margin_bottom(10)
        pos_box.set_margin_start(12)
        pos_box.set_margin_end(12)
        pos_frame.add(pos_box)

        header = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.pos_enabled_switch = Gtk.Switch()
        self.pos_enabled_switch.set_active(True)
        self.pos_enabled_switch.set_tooltip_text(
            "When enabled, the manager controls where the theme appears on your desktop."
        )
        self.pos_enabled_switch.connect("notify::active", self._on_position_enabled_toggled)
        header.pack_start(Gtk.Label(label="Control position:", xalign=0), False, False, 0)
        header.pack_start(self.pos_enabled_switch, False, False, 0)
        self.pos_screen_label = Gtk.Label(
            label=f"Screen: {screen_w}×{screen_h}px",
            xalign=1,
        )
        self.pos_screen_label.get_style_context().add_class("dim-label")
        header.pack_end(self.pos_screen_label, False, False, 0)
        pos_box.pack_start(header, False, False, 0)

        grid_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=12)
        grid = Gtk.Grid(column_spacing=4, row_spacing=4)
        for idx, (align_id, symbol) in enumerate(POSITION_ALIGNMENTS):
            row = idx // 3
            col = idx % 3
            btn = Gtk.ToggleButton(label=symbol)
            btn.set_size_request(42, 36)
            btn.set_tooltip_text(align_id.replace("_", " ").title())
            btn.connect("toggled", self._on_alignment_button_toggled, align_id)
            grid.attach(btn, col, row, 1, 1)
            self._position_buttons[align_id] = btn
        grid_row.pack_start(grid, False, False, 0)

        offset_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.pos_gap_x_spin = Gtk.SpinButton.new_with_range(-4000, 4000, 1)
        self.pos_gap_x_spin.set_value(30)
        self.pos_gap_x_spin.set_tooltip_text("Horizontal offset from screen edge (gap_x)")
        self.pos_gap_x_spin.connect("value-changed", self._on_gap_spin_changed)
        offset_box.pack_start(
            self._settings_row("Horizontal (gap_x):", self.pos_gap_x_spin),
            False,
            False,
            0,
        )

        self.pos_gap_y_spin = Gtk.SpinButton.new_with_range(-4000, 4000, 1)
        self.pos_gap_y_spin.set_value(50)
        self.pos_gap_y_spin.set_tooltip_text("Vertical offset from screen edge (gap_y)")
        self.pos_gap_y_spin.connect("value-changed", self._on_gap_spin_changed)
        offset_box.pack_start(
            self._settings_row("Vertical (gap_y):", self.pos_gap_y_spin),
            False,
            False,
            0,
        )

        self.pos_nudge_spin = Gtk.SpinButton.new_with_range(1, 100, 1)
        self.pos_nudge_spin.set_value(10)
        nudge_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=4)
        for label, handler in (
            ("←", lambda *_: self._nudge_position(-1, 0)),
            ("→", lambda *_: self._nudge_position(1, 0)),
            ("↑", lambda *_: self._nudge_position(0, -1)),
            ("↓", lambda *_: self._nudge_position(0, 1)),
        ):
            btn = Gtk.Button(label=label)
            btn.connect("clicked", handler)
            nudge_box.pack_start(btn, False, False, 0)
        nudge_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        nudge_row.pack_start(Gtk.Label(label="Fine tune:", xalign=0), False, False, 0)
        nudge_row.pack_start(nudge_box, False, False, 0)
        nudge_row.pack_start(Gtk.Label(label="Step:", xalign=0), False, False, 0)
        nudge_row.pack_start(self.pos_nudge_spin, False, False, 0)
        offset_box.pack_start(nudge_row, False, False, 0)
        grid_row.pack_start(offset_box, True, True, 0)
        pos_box.pack_start(grid_row, False, False, 0)

        self.pos_status_label = Gtk.Label(label="Select a theme to adjust its position.", xalign=0)
        self.pos_status_label.get_style_context().add_class("dim-label")
        pos_box.pack_start(self.pos_status_label, False, False, 0)

        action_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self.btn_save_position = Gtk.Button.new_with_label("💾 Save for Theme")
        self.btn_save_position.connect("clicked", lambda *_: self._save_position_for_theme())
        self.btn_reset_position = Gtk.Button.new_with_label("↺ Reset")
        self.btn_reset_position.connect("clicked", lambda *_: self._reset_position_for_theme())
        self.btn_apply_position = Gtk.Button.new_with_label("▶ Apply & Restart")
        self.btn_apply_position.get_style_context().add_class("suggested-action")
        self.btn_apply_position.connect("clicked", lambda *_: self._apply_position_and_restart())
        for btn in (self.btn_save_position, self.btn_reset_position, self.btn_apply_position):
            action_row.pack_start(btn, False, False, 0)
        pos_box.pack_start(action_row, False, False, 0)

        self._set_position_controls_sensitive(False)
        parent.pack_start(pos_frame, False, False, 0)

    def _set_position_controls_sensitive(self, enabled: bool):
        for widget in (
            self.pos_gap_x_spin,
            self.pos_gap_y_spin,
            self.pos_nudge_spin,
            self.btn_save_position,
            self.btn_reset_position,
            self.btn_apply_position,
        ):
            widget.set_sensitive(enabled)
        for btn in self._position_buttons.values():
            btn.set_sensitive(enabled)

    def _set_position_controls(self, position: dict[str, Any], enabled: bool = True):
        self._position_loading = True
        self.pos_enabled_switch.set_active(enabled)
        alignment = position.get("alignment", "top_left")
        for align_id, btn in self._position_buttons.items():
            btn.set_active(align_id == alignment)
        self.pos_gap_x_spin.set_value(int(position.get("gap_x", 30)))
        self.pos_gap_y_spin.set_value(int(position.get("gap_y", 50)))
        self._position_loading = False

    def _get_position_from_controls(self) -> dict[str, Any] | None:
        if not self.pos_enabled_switch.get_active():
            return None
        alignment = "top_left"
        for align_id, btn in self._position_buttons.items():
            if btn.get_active():
                alignment = align_id
                break
        return {
            "alignment": alignment,
            "gap_x": int(self.pos_gap_x_spin.get_value()),
            "gap_y": int(self.pos_gap_y_spin.get_value()),
        }

    def _update_position_status(self, text: str):
        self.pos_status_label.set_text(text)

    def on_theme_selection_changed(self, _selection):
        self._load_position_for_selected_theme()
        self._load_colors_for_selected_theme()

    def _load_position_for_selected_theme(self):
        theme = self._selected_theme_item()
        if not theme:
            self._active_position_theme = None
            self._set_position_controls_sensitive(False)
            self._update_position_status("Select a theme to adjust its position.")
            return

        self._active_position_theme = theme.path
        self._set_position_controls_sensitive(True)
        try:
            content = theme.path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            content = ""

        saved = resolve_theme_position(theme.path)
        if saved:
            self._set_position_controls(saved, enabled=True)
            self._update_position_status(
                f"{theme.path.parent.name}: saved position "
                f"({saved['alignment']}, X={saved['gap_x']}, Y={saved['gap_y']})"
            )
            return

        parsed = parse_theme_position(content)
        defaults = load_positions().get("default", DEFAULT_POSITIONS["default"])
        self._set_position_controls(parsed, enabled=bool(defaults.get("enabled", True)))
        self._update_position_status(
            f"{theme.path.parent.name}: from theme file "
            f"({parsed['alignment']}, X={parsed['gap_x']}, Y={parsed['gap_y']})"
        )

    def _on_position_enabled_toggled(self, switch, _pspec):
        if self._position_loading:
            return
        enabled = switch.get_active()
        self._set_position_controls_sensitive(bool(self._active_position_theme) and enabled)
        if self._active_position_theme:
            state = "enabled" if enabled else "disabled"
            self._update_position_status(
                f"{self._active_position_theme.parent.name}: position control {state}."
            )

    def _on_alignment_button_toggled(self, button: Gtk.ToggleButton, align_id: str):
        if self._position_loading:
            return
        if not button.get_active():
            self._position_loading = True
            button.set_active(True)
            self._position_loading = False
            return
        for other_id, other_btn in self._position_buttons.items():
            if other_id != align_id and other_btn.get_active():
                self._position_loading = True
                other_btn.set_active(False)
                self._position_loading = False
        self._update_position_status(
            f"Alignment: {align_id.replace('_', ' ')} | "
            f"X={int(self.pos_gap_x_spin.get_value())}, "
            f"Y={int(self.pos_gap_y_spin.get_value())}"
        )

    def _on_gap_spin_changed(self, _spin):
        if self._position_loading:
            return
        position = self._get_position_from_controls()
        if not position:
            return
        self._update_position_status(
            f"Offset: X={position['gap_x']}, Y={position['gap_y']} "
            f"({position['alignment'].replace('_', ' ')})"
        )

    def _nudge_position(self, dx: int, dy: int):
        if not self._active_position_theme or not self.pos_enabled_switch.get_active():
            return
        step = int(self.pos_nudge_spin.get_value())
        self._position_loading = True
        self.pos_gap_x_spin.set_value(self.pos_gap_x_spin.get_value() + dx * step)
        self.pos_gap_y_spin.set_value(self.pos_gap_y_spin.get_value() + dy * step)
        self._position_loading = False
        self._on_gap_spin_changed(None)

    def _save_position_for_theme(self, show_message: bool = True):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return None
        position = self._get_position_from_controls()
        if not position:
            self.show_message("Info", "Enable position control first.")
            return None

        data = load_positions()
        data.setdefault("themes", {})[str(theme.path.resolve())] = {
            "enabled": True,
            **position,
        }
        save_positions(data)
        self._update_position_status(
            f"Saved for {theme.path.parent.name}: "
            f"{position['alignment']}, X={position['gap_x']}, Y={position['gap_y']}"
        )
        if show_message:
            self.push_status(f"Position saved for {theme.path.parent.name}.")
        return position

    def _reset_position_for_theme(self):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        data = load_positions()
        data.get("themes", {}).pop(str(theme.path.resolve()), None)
        save_positions(data)
        self._load_position_for_selected_theme()
        self.push_status(f"Position reset for {theme.path.parent.name}.")

    def _capture_run_position(self) -> dict[str, Any] | None:
        position = self._get_position_from_controls()
        if position and self.settings.get("remember_position", True):
            self._save_position_for_theme(show_message=False)
        return position

    def _apply_position_and_restart(self):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        position = self._save_position_for_theme(show_message=False)
        if not position:
            return
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return
        self.set_busy(True)
        self.push_status(f"Applying position for {theme.path.parent.name}...")

        def worker():
            try:
                kill_running_conky()
                pid, _, mode = run_conky(
                    theme.path,
                    position_override=position,
                    color_override=self._get_colors_from_controls()
                    if self.color_enabled_switch.get_active()
                    else None,
                )
                GLib.idle_add(
                    self.update_runtime_status,
                    True,
                    f"{theme.path.name} [{mode}] @ {position['alignment']} "
                    f"X={position['gap_x']} Y={position['gap_y']}",
                )
                GLib.idle_add(
                    self.push_status,
                    f"Position applied: {theme.path.parent.name} "
                    f"({position['alignment']}, X={position['gap_x']}, Y={position['gap_y']})",
                )
            except Exception as exc:
                GLib.idle_add(
                    self.show_message,
                    "Position Error",
                    str(exc),
                    Gtk.MessageType.ERROR,
                )

            GLib.idle_add(lambda: self.set_busy(False))

        threading.Thread(target=worker, daemon=True).start()

    def build_tab_colors(self):
        tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        tab.set_margin_top(10)
        tab.set_margin_bottom(10)
        tab.set_margin_start(10)
        tab.set_margin_end(10)

        intro = Gtk.Label(
            label="Change theme colors without editing original files. "
            "Lua ring colors are synced automatically when Smart Lua Match is enabled.",
            xalign=0,
            wrap=True,
        )
        intro.get_style_context().add_class("dim-label")
        tab.pack_start(intro, False, False, 0)

        header = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.color_enabled_switch = Gtk.Switch()
        self.color_enabled_switch.set_active(True)
        self.color_enabled_switch.connect("notify::active", self._on_color_enabled_toggled)
        header.pack_start(Gtk.Label(label="Recolor theme:", xalign=0), False, False, 0)
        header.pack_start(self.color_enabled_switch, False, False, 0)

        self.color_preset_combo = Gtk.ComboBoxText()
        self.color_preset_combo.append("custom", "Custom")
        for preset_id, preset in COLOR_PRESETS.items():
            self.color_preset_combo.append(preset_id, preset["label"])
        self.color_preset_combo.set_active_id("custom")
        self.color_preset_combo.connect("changed", self._on_color_preset_changed)
        header.pack_end(self.color_preset_combo, False, False, 0)
        header.pack_end(Gtk.Label(label="Palette:", xalign=1), False, False, 0)
        tab.pack_start(header, False, False, 0)

        smart_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.color_smart_lua_switch = Gtk.Switch()
        self.color_smart_lua_switch.set_active(True)
        self.color_smart_lua_switch.set_tooltip_text(
            "Automatically recolor matching hex values inside Lua ring scripts."
        )
        smart_row.pack_start(Gtk.Label(label="Smart Lua match:", xalign=0), False, False, 0)
        smart_row.pack_start(self.color_smart_lua_switch, False, False, 0)
        tab.pack_start(smart_row, False, False, 0)

        colors_frame = Gtk.Frame(label="Theme Colors")
        colors_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        colors_scroll = Gtk.ScrolledWindow()
        colors_scroll.set_policy(Gtk.PolicyType.NEVER, Gtk.PolicyType.AUTOMATIC)
        colors_scroll.set_min_content_height(180)
        self.color_slots_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.color_slots_box.set_margin_top(10)
        self.color_slots_box.set_margin_bottom(10)
        self.color_slots_box.set_margin_start(12)
        self.color_slots_box.set_margin_end(12)
        colors_scroll.add(self.color_slots_box)
        colors_frame.add(colors_scroll)
        tab.pack_start(colors_frame, True, True, 0)

        self.color_status_label = Gtk.Label(
            label="Select a theme to detect and customize its colors.",
            xalign=0,
        )
        self.color_status_label.get_style_context().add_class("dim-label")
        tab.pack_start(self.color_status_label, False, False, 0)

        action_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self.btn_save_colors = Gtk.Button.new_with_label("💾 Save for Theme")
        self.btn_save_colors.connect("clicked", lambda *_: self._save_colors_for_theme())
        self.btn_reset_colors = Gtk.Button.new_with_label("↺ Reset")
        self.btn_reset_colors.connect("clicked", lambda *_: self._reset_colors_for_theme())
        self.btn_apply_colors = Gtk.Button.new_with_label("▶ Apply & Restart")
        self.btn_apply_colors.get_style_context().add_class("suggested-action")
        self.btn_apply_colors.connect("clicked", lambda *_: self._apply_colors_and_restart())
        for btn in (self.btn_save_colors, self.btn_reset_colors, self.btn_apply_colors):
            action_row.pack_start(btn, False, False, 0)
        tab.pack_start(action_row, False, False, 0)

        hint_frame = Gtk.Frame(label="Color Tips")
        hint_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        hint_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        hint_box.set_margin_top(10)
        hint_box.set_margin_bottom(10)
        hint_box.set_margin_start(12)
        hint_box.set_margin_end(12)
        hint_frame.add(hint_box)
        for text in (
            "• Original theme files are never modified — colors apply in launch bundles only.",
            "• Palettes remap only the color slots detected in the selected theme.",
            "• Smart Lua Match updates ring meters when they share the same hex values.",
            "• Save for Theme keeps custom colors for each Conky theme separately.",
        ):
            lbl = Gtk.Label(label=text)
            lbl.set_xalign(0)
            lbl.get_style_context().add_class("dim-label")
            hint_box.pack_start(lbl, False, False, 0)
        tab.pack_start(hint_frame, False, False, 0)

        self._set_color_controls_sensitive(False)
        self._busy_buttons.extend([
            self.btn_save_colors,
            self.btn_reset_colors,
            self.btn_apply_colors,
        ])
        self.notebook.append_page(tab, Gtk.Label(label="Colors"))

    def _hex_to_rgba(self, color: str) -> Gdk.RGBA:
        rgba = Gdk.RGBA()
        rgba.alpha = 1.0
        normalized = normalize_color(color)
        if not normalized:
            rgba.red = rgba.green = rgba.blue = 1.0
            return rgba
        body = normalized[1:]
        rgba.red = int(body[0:2], 16) / 255.0
        rgba.green = int(body[2:4], 16) / 255.0
        rgba.blue = int(body[4:6], 16) / 255.0
        return rgba

    def _rgba_to_hex(self, rgba: Gdk.RGBA) -> str:
        return (
            f"#{int(rgba.red * 255):02X}"
            f"{int(rgba.green * 255):02X}"
            f"{int(rgba.blue * 255):02X}"
        )

    def _set_color_controls_sensitive(self, enabled: bool):
        recolor_on = self.color_enabled_switch.get_active()
        for widget in (
            self.color_preset_combo,
            self.color_smart_lua_switch,
            self.btn_save_colors,
            self.btn_reset_colors,
            self.btn_apply_colors,
        ):
            widget.set_sensitive(enabled and recolor_on)
        for picker in self._color_pickers.values():
            picker.set_sensitive(enabled and recolor_on)

    def _clear_color_pickers(self):
        for child in self.color_slots_box.get_children():
            self.color_slots_box.remove(child)
        self._color_pickers.clear()

    def _populate_color_pickers(self, colors: dict[str, str]):
        self._clear_color_pickers()
        self._theme_color_slots = dict(colors)
        if not colors:
            empty = Gtk.Label(label="No color slots detected in this theme.", xalign=0)
            empty.get_style_context().add_class("dim-label")
            self.color_slots_box.pack_start(empty, False, False, 0)
            self.color_slots_box.show_all()
            return

        for slot in ordered_theme_color_slots(colors):
            row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
            label = COLOR_SLOT_LABELS.get(slot, slot)
            row.pack_start(Gtk.Label(label=f"{label} ({slot}):", xalign=0), False, False, 0)
            picker = Gtk.ColorButton()
            picker.set_rgba(self._hex_to_rgba(colors[slot]))
            picker.set_tooltip_text(f"Original: {colors[slot]}")
            picker.connect("color-set", self._on_color_picker_changed, slot)
            row.pack_end(picker, False, False, 0)
            self.color_slots_box.pack_start(row, False, False, 0)
            self._color_pickers[slot] = picker
        self.color_slots_box.show_all()

    def _get_colors_from_controls(self) -> dict[str, Any] | None:
        if not self.color_enabled_switch.get_active() or not self._color_pickers:
            return None
        colors: dict[str, str] = {}
        for slot, picker in self._color_pickers.items():
            colors[slot] = self._rgba_to_hex(picker.get_rgba())
        return {
            "colors": colors,
            "smart_lua": self.color_smart_lua_switch.get_active(),
        }

    def _load_colors_for_selected_theme(self):
        theme = self._selected_theme_item()
        if not theme:
            self._active_color_theme = None
            self._clear_color_pickers()
            self._set_color_controls_sensitive(False)
            self.color_status_label.set_text("Select a theme to detect and customize its colors.")
            return

        self._active_color_theme = theme.path
        try:
            content = theme.path.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            content = ""

        base_colors = parse_theme_colors(content)
        saved = resolve_theme_colors(theme.path)
        if saved:
            display_colors = dict(base_colors)
            display_colors.update(saved["colors"])
            self._color_loading = True
            self.color_enabled_switch.set_active(True)
            self.color_smart_lua_switch.set_active(saved.get("smart_lua", True))
            self.color_preset_combo.set_active_id("custom")
            self._populate_color_pickers(display_colors)
            self._color_loading = False
            self._set_color_controls_sensitive(True)
            self.color_status_label.set_text(
                f"{theme.path.parent.name}: saved custom colors ({len(saved['colors'])} slots)"
            )
            return

        self._color_loading = True
        self.color_enabled_switch.set_active(True)
        self.color_smart_lua_switch.set_active(True)
        self.color_preset_combo.set_active_id("custom")
        self._populate_color_pickers(base_colors)
        self._color_loading = False
        self._set_color_controls_sensitive(bool(base_colors))
        if base_colors:
            self.color_status_label.set_text(
                f"{theme.path.parent.name}: detected {len(base_colors)} color slot(s) from theme file"
            )
        else:
            self.color_status_label.set_text(
                f"{theme.path.parent.name}: no standard color slots found in config"
            )

    def _on_color_enabled_toggled(self, _switch, _pspec):
        if self._color_loading:
            return
        self._set_color_controls_sensitive(bool(self._active_color_theme))
        if self._active_color_theme:
            state = "enabled" if self.color_enabled_switch.get_active() else "disabled"
            self.color_status_label.set_text(
                f"{self._active_color_theme.parent.name}: color customization {state}"
            )

    def _on_color_preset_changed(self, combo: Gtk.ComboBoxText):
        if self._color_loading or not self._theme_color_slots:
            return
        preset_id = combo.get_active_id()
        if not preset_id or preset_id == "custom":
            return
        merged = merge_preset_colors(self._theme_color_slots, preset_id)
        self._color_loading = True
        for slot, picker in self._color_pickers.items():
            if slot in merged:
                picker.set_rgba(self._hex_to_rgba(merged[slot]))
        self._color_loading = False
        self.color_status_label.set_text(
            f"Applied palette: {COLOR_PRESETS[preset_id]['label']}"
        )

    def _on_color_picker_changed(self, picker: Gtk.ColorButton, slot: str):
        if self._color_loading:
            return
        self._color_loading = True
        self.color_preset_combo.set_active_id("custom")
        self._color_loading = False
        self.color_status_label.set_text(
            f"Updated {COLOR_SLOT_LABELS.get(slot, slot)} → {self._rgba_to_hex(picker.get_rgba())}"
        )

    def _save_colors_for_theme(self, show_message: bool = True) -> dict[str, Any] | None:
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return None
        colors_payload = self._get_colors_from_controls()
        if not colors_payload:
            self.show_message("Info", "Enable recoloring and select at least one color.")
            return None

        data = load_colors()
        data.setdefault("themes", {})[str(theme.path.resolve())] = {
            "enabled": True,
            "smart_lua": colors_payload["smart_lua"],
            "colors": colors_payload["colors"],
        }
        save_colors(data)
        self.color_status_label.set_text(
            f"Saved colors for {theme.path.parent.name} ({len(colors_payload['colors'])} slots)"
        )
        if show_message:
            self.push_status(f"Colors saved for {theme.path.parent.name}.")
        return colors_payload

    def _reset_colors_for_theme(self):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        data = load_colors()
        data.get("themes", {}).pop(str(theme.path.resolve()), None)
        save_colors(data)
        self._load_colors_for_selected_theme()
        self.push_status(f"Colors reset for {theme.path.parent.name}.")

    def _capture_run_colors(self) -> dict[str, Any] | None:
        colors_payload = self._get_colors_from_controls()
        if colors_payload and self.settings.get("remember_colors", True):
            self._save_colors_for_theme(show_message=False)
        return colors_payload

    def _apply_colors_and_restart(self):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        colors_payload = self._save_colors_for_theme(show_message=False)
        if not colors_payload:
            return
        position = self._get_position_from_controls() if self.pos_enabled_switch.get_active() else None
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return
        self.set_busy(True)
        self.push_status(f"Applying colors for {theme.path.parent.name}...")

        def worker():
            try:
                kill_running_conky()
                pid, _, mode = run_conky(
                    theme.path,
                    position_override=position,
                    color_override=colors_payload,
                )
                GLib.idle_add(
                    self.update_runtime_status,
                    True,
                    f"{theme.path.name} [{mode}] with custom colors",
                )
                GLib.idle_add(
                    self.push_status,
                    f"Colors applied for {theme.path.parent.name}.",
                )
            except Exception as exc:
                GLib.idle_add(
                    self.show_message,
                    "Color Error",
                    str(exc),
                    Gtk.MessageType.ERROR,
                )
            GLib.idle_add(lambda: self.set_busy(False))

        threading.Thread(target=worker, daemon=True).start()

    def build_tab_profiles(self):
        tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        tab.set_margin_top(10)
        tab.set_margin_bottom(10)
        tab.set_margin_start(10)
        tab.set_margin_end(10)

        name_frame = Gtk.Frame(label="Profile Name")
        name_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        name_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        name_box.set_margin_top(10)
        name_box.set_margin_bottom(10)
        name_box.set_margin_start(10)
        name_box.set_margin_end(10)
        name_frame.add(name_box)
        self.profile_name_entry = Gtk.Entry()
        self.profile_name_entry.set_placeholder_text("My profile name...")
        self.profile_name_entry.set_text(self.profiles.get("last_profile", ""))
        name_box.pack_start(self.profile_name_entry, False, False, 0)
        tab.pack_start(name_frame, False, False, 0)

        content = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)

        left = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        left.pack_start(Gtk.Label(label="Saved Profiles", xalign=0), False, False, 0)
        profile_scroll = Gtk.ScrolledWindow()
        profile_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        profile_scroll.set_min_content_height(120)
        self.profile_listbox = Gtk.ListBox()
        self.profile_listbox.set_selection_mode(Gtk.SelectionMode.SINGLE)
        self.profile_listbox.connect("row-selected", self.on_profile_selected)
        profile_scroll.add(self.profile_listbox)
        left.pack_start(profile_scroll, True, True, 0)
        content.pack_start(left, True, True, 0)

        right = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        self.btn_profile_save = Gtk.Button.new_with_label("💾 Save Profile")
        self.btn_profile_save.get_style_context().add_class("suggested-action")
        self.btn_profile_save.connect("clicked", self.save_profile)
        self.btn_profile_add = Gtk.Button.new_with_label("➕ Add Themes")
        self.btn_profile_add.connect("clicked", self.add_to_profile)
        self.btn_profile_run = Gtk.Button.new_with_label("▶ Run Profile")
        self.btn_profile_run.connect("clicked", self.run_profile)
        self.btn_autostart_on = Gtk.Button.new_with_label("🚀 Enable Autostart")
        self.btn_autostart_on.connect("clicked", self.enable_autostart)
        self.btn_autostart_off = Gtk.Button.new_with_label("⏹ Disable Autostart")
        self.btn_autostart_off.connect("clicked", self.disable_autostart)
        self.btn_profile_delete = Gtk.Button.new_with_label("🗑 Delete Profile")
        self.btn_profile_delete.connect("clicked", self.delete_profile)
        self.btn_profile_reload = Gtk.Button.new_with_label("🔄 Reload")
        self.btn_profile_reload.connect("clicked", lambda *_: self.refresh_profiles())
        for btn in (
            self.btn_profile_save, self.btn_profile_add, self.btn_profile_run,
            self.btn_autostart_on, self.btn_autostart_off,
            self.btn_profile_delete, self.btn_profile_reload,
        ):
            right.pack_start(btn, False, False, 0)
        content.pack_start(right, False, False, 0)
        tab.pack_start(content, True, True, 0)

        details_frame = Gtk.Frame(label="Profile Contents")
        details_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        details_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        details_box.set_margin_top(10)
        details_box.set_margin_bottom(10)
        details_box.set_margin_start(10)
        details_box.set_margin_end(10)
        details_frame.add(details_box)
        self.profile_details_title = Gtk.Label(label="No profile selected")
        self.profile_details_title.set_xalign(0)
        self.profile_details_title.get_style_context().add_class("dim-label")
        details_box.pack_start(self.profile_details_title, False, False, 0)
        details_scroll = Gtk.ScrolledWindow()
        details_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        details_scroll.set_min_content_height(80)
        self.profile_details_listbox = Gtk.ListBox()
        details_scroll.add(self.profile_details_listbox)
        details_box.pack_start(details_scroll, True, True, 0)
        tab.pack_start(details_frame, False, False, 0)

        self._busy_buttons.extend([
            self.btn_profile_save, self.btn_profile_add, self.btn_profile_run,
            self.btn_autostart_on, self.btn_autostart_off, self.btn_profile_delete,
        ])

        self.notebook.append_page(tab, Gtk.Label(label="Profiles"))

    def build_tab_diagnostics(self):
        tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        tab.set_margin_top(10)
        tab.set_margin_bottom(10)
        tab.set_margin_start(10)
        tab.set_margin_end(10)

        top = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        self.btn_log_refresh = Gtk.Button.new_with_label("🔄 Refresh Log")
        self.btn_log_refresh.connect("clicked", lambda *_: self.refresh_log_view())
        top.pack_start(self.btn_log_refresh, False, False, 0)
        tab.pack_start(top, False, False, 0)

        logs_frame = Gtk.Frame(label="Application Log")
        logs_frame.set_shadow_type(Gtk.ShadowType.ETCHED_IN)
        logs_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=5)
        logs_box.set_margin_top(10)
        logs_box.set_margin_bottom(10)
        logs_box.set_margin_start(10)
        logs_box.set_margin_end(10)
        logs_frame.add(logs_box)

        logs_scroll = Gtk.ScrolledWindow()
        logs_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        self.logs_textview = Gtk.TextView()
        self.logs_textview.set_editable(False)
        self.logs_textview.set_monospace(True)
        self.logs_buffer = self.logs_textview.get_buffer()
        logs_scroll.add(self.logs_textview)
        logs_box.pack_start(logs_scroll, True, True, 0)
        tab.pack_start(logs_frame, True, True, 0)

        self.notebook.append_page(tab, Gtk.Label(label="Diagnostics"))

    def setup_statusbar(self, parent: Gtk.Box):
        self.statusbar = Gtk.Statusbar()
        self.statusbar_context = self.statusbar.get_context_id("main")
        parent.pack_start(self.statusbar, False, False, 0)
        self.push_status("Ready")

    def _action_btn(self, label: str, style_class: str | None, handler: Callable):
        btn = Gtk.Button.new_with_label(label)
        if style_class:
            btn.get_style_context().add_class(style_class)
        btn.connect("clicked", handler)
        return btn

    def apply_ui_theme(self):
        theme = self.settings.get("ui_theme", "auto")
        settings = Gtk.Settings.get_default()
        if theme == "dark":
            settings.set_property("gtk-application-prefer-dark-theme", True)
        elif theme == "light":
            settings.set_property("gtk-application-prefer-dark-theme", False)

    def push_status(self, message: str):
        self.statusbar.pop(self.statusbar_context)
        self.statusbar.push(self.statusbar_context, message)

    def show_message(self, title: str, message: str, msg_type=Gtk.MessageType.INFO):
        dialog = Gtk.MessageDialog(
            transient_for=self.win,
            modal=True,
            message_type=msg_type,
            buttons=Gtk.ButtonsType.OK,
            text=title,
        )
        dialog.format_secondary_text(message)
        dialog.run()
        dialog.destroy()

    def show_wayland_notice(self):
        if "wayland" in self.session.lower():
            self.show_message(
                "Wayland Notice",
                "Conky on Wayland may behave differently depending on your compositor.\n"
                "If themes fail to appear, try an X11 session or adjust window settings\n"
                "(Layer / Window type) in Settings.",
                Gtk.MessageType.WARNING,
            )

    def show_about(self, _menuitem=None):
        about = Gtk.AboutDialog(transient_for=self.win)
        about.set_program_name(APP_NAME)
        about.set_version(APP_VERSION)
        about.set_comments(
            "Universal Conky theme manager for Linux\n"
            "(KDE, GNOME, XFCE, Cinnamon, MATE, LXQt, and more)"
        )
        about.set_website("https://github.com/almezali/conky-manager-g")
        about.run()
        about.destroy()

    # ---------------- Settings dialog ----------------

    def show_settings_dialog(self, _menuitem=None):
        dialog = Gtk.Dialog(title="Settings", transient_for=self.win, flags=0)
        dialog.set_default_size(520, 520)
        dialog.set_resizable(True)

        content = dialog.get_content_area()
        content.set_spacing(10)
        content.set_margin_top(15)
        content.set_margin_bottom(15)
        content.set_margin_start(15)
        content.set_margin_end(15)

        scroll = Gtk.ScrolledWindow()
        scroll.set_policy(Gtk.PolicyType.NEVER, Gtk.PolicyType.AUTOMATIC)
        scroll.set_min_content_height(400)
        inner = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        scroll.add(inner)
        content.pack_start(scroll, True, True, 0)

        # Desktop environment
        de_frame = Gtk.Frame(label="Desktop Environment")
        de_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        de_box.set_margin_top(10)
        de_box.set_margin_bottom(10)
        de_box.set_margin_start(10)
        de_box.set_margin_end(10)
        de_frame.add(de_box)

        preset_combo = Gtk.ComboBoxText()
        for preset_id, preset_label in (
            ("auto", f"Auto-detect ({self.desktop_env})"),
            ("kde", "KDE / Plasma"),
            ("gnome", "GNOME"),
            ("xfce", "XFCE"),
            ("cinnamon", "Cinnamon"),
            ("mate", "MATE"),
            ("lxqt", "LXQt"),
            ("lxde", "LXDE"),
            ("generic", "Generic / Other"),
        ):
            preset_combo.append(preset_id, preset_label)
        preset_combo.set_active_id(self.settings.get("desktop_preset", "auto"))

        optimize_switch = Gtk.Switch()
        optimize_switch.set_active(self.settings.get("auto_optimize", True))

        de_box.pack_start(self._settings_row("Environment preset:", preset_combo), False, False, 0)
        optimize_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        optimize_label = Gtk.Label(label="Auto-optimize themes:")
        optimize_label.set_xalign(0)
        optimize_label.set_size_request(160, -1)
        optimize_row.pack_start(optimize_label, False, False, 0)
        optimize_row.pack_start(optimize_switch, False, False, 0)
        de_box.pack_start(optimize_row, False, False, 0)

        visual_switch = Gtk.Switch()
        visual_switch.set_active(self.settings.get("full_visual_launch", True))
        visual_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        visual_label = Gtk.Label(label="Full visual launch:")
        visual_label.set_xalign(0)
        visual_label.set_size_request(160, -1)
        visual_row.pack_start(visual_label, False, False, 0)
        visual_row.pack_start(visual_switch, False, False, 0)
        de_box.pack_start(visual_row, False, False, 0)

        compat_switch = Gtk.Switch()
        compat_switch.set_active(self.settings.get("create_compat_symlinks", True))
        compat_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        compat_label = Gtk.Label(label="Compat symlinks:")
        compat_label.set_xalign(0)
        compat_label.set_size_request(160, -1)
        compat_row.pack_start(compat_label, False, False, 0)
        compat_row.pack_start(compat_switch, False, False, 0)
        de_box.pack_start(compat_row, False, False, 0)

        remember_switch = Gtk.Switch()
        remember_switch.set_active(self.settings.get("remember_position", True))
        remember_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        remember_label = Gtk.Label(label="Remember position:")
        remember_label.set_xalign(0)
        remember_label.set_size_request(160, -1)
        remember_row.pack_start(remember_label, False, False, 0)
        remember_row.pack_start(remember_switch, False, False, 0)
        de_box.pack_start(remember_row, False, False, 0)

        remember_colors_switch = Gtk.Switch()
        remember_colors_switch.set_active(self.settings.get("remember_colors", True))
        remember_colors_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        remember_colors_label = Gtk.Label(label="Remember colors:")
        remember_colors_label.set_xalign(0)
        remember_colors_label.set_size_request(160, -1)
        remember_colors_row.pack_start(remember_colors_label, False, False, 0)
        remember_colors_row.pack_start(remember_colors_switch, False, False, 0)
        de_box.pack_start(remember_colors_row, False, False, 0)
        inner.pack_start(de_frame, False, False, 0)

        pos_frame = Gtk.Frame(label="Default Desktop Position")
        pos_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        pos_box.set_margin_top(10)
        pos_box.set_margin_bottom(10)
        pos_box.set_margin_start(10)
        pos_box.set_margin_end(10)
        pos_frame.add(pos_box)

        positions = load_positions()
        default_pos = positions.get("default", DEFAULT_POSITIONS["default"])
        default_enabled = Gtk.Switch()
        default_enabled.set_active(default_pos.get("enabled", True))
        default_align = Gtk.ComboBoxText()
        for align_id, symbol in POSITION_ALIGNMENTS:
            default_align.append(align_id, f"{symbol} {align_id.replace('_', ' ')}")
        default_align.set_active_id(default_pos.get("alignment", "top_left"))
        default_gap_x = Gtk.SpinButton.new_with_range(-4000, 4000, 1)
        default_gap_x.set_value(int(default_pos.get("gap_x", 30)))
        default_gap_y = Gtk.SpinButton.new_with_range(-4000, 4000, 1)
        default_gap_y.set_value(int(default_pos.get("gap_y", 50)))

        pos_box.pack_start(self._settings_row("Enable by default:", default_enabled), False, False, 0)
        pos_box.pack_start(self._settings_row("Default alignment:", default_align), False, False, 0)
        pos_box.pack_start(self._settings_row("Default gap_x:", default_gap_x), False, False, 0)
        pos_box.pack_start(self._settings_row("Default gap_y:", default_gap_y), False, False, 0)
        inner.pack_start(pos_frame, False, False, 0)

        # Window behavior
        win_frame = Gtk.Frame(label="Window Behavior")
        win_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        win_box.set_margin_top(10)
        win_box.set_margin_bottom(10)
        win_box.set_margin_start(10)
        win_box.set_margin_end(10)
        win_frame.add(win_box)

        layer_combo = Gtk.ComboBoxText()
        layer_combo.append("above", "Above windows (overlay)")
        layer_combo.append("below", "Below windows (background)")
        layer_combo.set_active_id(self.settings.get("layer", "above"))

        wtype_combo = Gtk.ComboBoxText()
        wtype_combo.append("dock", "Dock")
        wtype_combo.append("desktop", "Desktop (GNOME/XFCE/MATE)")
        wtype_combo.append("normal", "Normal window")
        wtype_combo.set_active_id(self.settings.get("window_type", "dock"))

        win_box.pack_start(self._settings_row("Layer:", layer_combo), False, False, 0)
        win_box.pack_start(self._settings_row("Window type:", wtype_combo), False, False, 0)
        inner.pack_start(win_frame, False, False, 0)

        # Performance
        perf_frame = Gtk.Frame(label="Performance")
        perf_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        perf_box.set_margin_top(10)
        perf_box.set_margin_bottom(10)
        perf_box.set_margin_start(10)
        perf_box.set_margin_end(10)
        perf_frame.add(perf_box)

        smooth_combo = Gtk.ComboBoxText()
        smooth_combo.append("ultra", "Ultra smooth (less CPU)")
        smooth_combo.append("balanced", "Balanced")
        smooth_combo.append("performance", "Performance (faster updates)")
        smooth_combo.set_active_id(self.settings.get("smoothness", "balanced"))

        nice_spin = Gtk.SpinButton.new_with_range(-5, 19, 1)
        nice_spin.set_value(int(self.settings.get("nice_level", 10)))

        max_spin = Gtk.SpinButton.new_with_range(1, 6, 1)
        max_spin.set_value(int(self.settings.get("max_instances", 1)))

        health_spin = Gtk.SpinButton.new_with_range(0.4, 5.0, 0.1)
        health_spin.set_digits(1)
        health_spin.set_value(float(self.settings.get("healthcheck_seconds", 2.0)))

        perf_box.pack_start(self._settings_row("Smoothness:", smooth_combo), False, False, 0)
        perf_box.pack_start(self._settings_row("Nice level:", nice_spin), False, False, 0)
        perf_box.pack_start(self._settings_row("Max instances:", max_spin), False, False, 0)
        perf_box.pack_start(self._settings_row("Healthcheck (sec):", health_spin), False, False, 0)
        inner.pack_start(perf_frame, False, False, 0)

        # Preview & UI
        tools_frame = Gtk.Frame(label="Preview & Interface")
        tools_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        tools_box.set_margin_top(10)
        tools_box.set_margin_bottom(10)
        tools_box.set_margin_start(10)
        tools_box.set_margin_end(10)
        tools_frame.add(tools_box)

        preview_spin = Gtk.SpinButton.new_with_range(2, 60, 1)
        preview_spin.set_value(int(self.settings.get("preview_seconds", 5)))

        ui_theme_combo = Gtk.ComboBoxText()
        ui_theme_combo.append("auto", "Auto (system)")
        ui_theme_combo.append("light", "Light")
        ui_theme_combo.append("dark", "Dark")
        ui_theme_combo.set_active_id(self.settings.get("ui_theme", "auto"))

        tools_box.pack_start(self._settings_row("Preview seconds:", preview_spin), False, False, 0)
        tools_box.pack_start(self._settings_row("UI theme:", ui_theme_combo), False, False, 0)
        inner.pack_start(tools_frame, False, False, 0)

        dialog.add_button("Reset Defaults", Gtk.ResponseType.REJECT)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        dialog.show_all()

        response = dialog.run()
        if response == Gtk.ResponseType.REJECT:
            self.settings = dict(DEFAULT_SETTINGS)
            save_json(SETTINGS_FILE, self.settings)
            save_positions(DEFAULT_POSITIONS)
            save_colors(DEFAULT_COLORS)
            self.apply_ui_theme()
            self.push_status("Settings reset to defaults.")
        elif response == Gtk.ResponseType.OK:
            self.settings = {
                "layer": layer_combo.get_active_id() or "above",
                "window_type": wtype_combo.get_active_id() or "dock",
                "smoothness": smooth_combo.get_active_id() or "balanced",
                "nice_level": int(nice_spin.get_value()),
                "max_instances": int(max_spin.get_value()),
                "healthcheck_seconds": float(health_spin.get_value()),
                "preview_seconds": int(preview_spin.get_value()),
                "ui_theme": ui_theme_combo.get_active_id() or "auto",
                "desktop_preset": preset_combo.get_active_id() or "auto",
                "auto_optimize": optimize_switch.get_active(),
                "full_visual_launch": visual_switch.get_active(),
                "create_compat_symlinks": compat_switch.get_active(),
                "remember_position": remember_switch.get_active(),
                "remember_colors": remember_colors_switch.get_active(),
            }
            save_json(SETTINGS_FILE, self.settings)
            positions = load_positions()
            positions["default"] = {
                "enabled": default_enabled.get_active(),
                "alignment": default_align.get_active_id() or "top_left",
                "gap_x": int(default_gap_x.get_value()),
                "gap_y": int(default_gap_y.get_value()),
            }
            save_positions(positions)
            self.apply_ui_theme()
            self.push_status("Settings saved.")
            self.show_message("Success", "Settings saved successfully.")
            self._load_position_for_selected_theme()
            self._load_colors_for_selected_theme()

        dialog.destroy()

    def _settings_row(self, label_text: str, widget: Gtk.Widget) -> Gtk.Box:
        row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        label = Gtk.Label(label=label_text)
        label.set_xalign(0)
        label.set_size_request(160, -1)
        row.pack_start(label, False, False, 0)
        row.pack_start(widget, True, True, 0)
        return row

    # ---------------- Theme list ----------------

    def load_themes(self, *_args):
        self.themes = [ThemeItem(p) for p in find_themes()]
        for item in self.themes:
            ensure_compat_install_symlink(resolve_theme_root(item.path))
        self.apply_filter()
        self.push_status(f"Loaded {len(self.themes)} valid theme(s).")

    def on_search_changed(self, entry: Gtk.SearchEntry):
        self.apply_filter(entry.get_text())

    def apply_filter(self, query: str = ""):
        if not query and hasattr(self, "search_entry"):
            query = self.search_entry.get_text()
        q = query.strip().lower()
        self.filtered = [t for t in self.themes if q in t.label.lower()]
        self.theme_store.clear()
        for item in self.filtered:
            status = self.theme_status.get(item.label, "")
            status_text = {
                "stable": "✓ OK",
                "needs_fix": "⚠ Fix",
                "broken": "✗ Bad",
            }.get(status, "")
            self.theme_store.append([item.label, status_text, str(item.path)])

    def selected_theme_paths(self) -> list[Path]:
        paths: list[Path] = []
        selection = self.theme_tree.get_selection()
        _model, rows = selection.get_selected_rows()
        for row in rows:
            path_str = self.theme_store[row][2]
            paths.append(Path(path_str))
        return paths

    def _selected_theme_item(self) -> ThemeItem | None:
        paths = self.selected_theme_paths()
        if not paths:
            return None
        return ThemeItem(paths[0])

    def on_theme_tree_button_press(self, widget, event):
        if event.button == 3:
            path_info = widget.get_path_at_pos(int(event.x), int(event.y))
            if path_info:
                path, _col, _cx, _cy = path_info
                widget.get_selection().select_path(path)
                self.show_theme_context_menu(event)
                return True
        return False

    def show_theme_context_menu(self, event):
        theme = self._selected_theme_item()
        if not theme:
            return
        menu = Gtk.Menu()

        for label, handler in (
            ("Open theme file", lambda *_: subprocess.Popen(["xdg-open", str(theme.path)])),
            ("Edit theme file", lambda *_: subprocess.Popen(["xdg-open", str(theme.path)])),
            ("Open containing folder", lambda *_: subprocess.Popen(["xdg-open", str(theme.path.parent)])),
        ):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", handler)
            menu.append(item)

        menu.append(Gtk.SeparatorMenuItem())

        opt_item = Gtk.MenuItem(label="Open optimized file")
        opt_item.connect("activate", lambda *_: subprocess.Popen(["xdg-open", str(optimize_for_desktop(theme.path))]))
        menu.append(opt_item)

        menu.append(Gtk.SeparatorMenuItem())

        val_item = Gtk.MenuItem(label="Validate (original + optimized)")
        val_item.connect("activate", self.validate_selected_theme)
        menu.append(val_item)

        prev_item = Gtk.MenuItem(label="Preview (temporary)")
        prev_item.connect("activate", self.preview_selected)
        menu.append(prev_item)

        menu.show_all()
        menu.popup(None, None, None, None, event.button, event.time)

    # ---------------- Busy state & workers ----------------

    def set_busy(self, busy: bool):
        self._worker_busy = busy
        for btn in self._busy_buttons:
            btn.set_sensitive(not busy)

    def _run_in_thread(self, target: Callable, on_done: Callable | None = None):
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return

        self.set_busy(True)

        def wrapper():
            try:
                target()
            finally:
                if on_done:
                    GLib.idle_add(on_done)

        threading.Thread(target=wrapper, daemon=True).start()

    def _run_themes_worker(
        self,
        themes: list[Path],
        position: dict[str, Any] | None = None,
        colors: dict[str, Any] | None = None,
    ):
        settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
        max_instances = int(settings.get("max_instances", 1))
        run_list = themes[:max_instances]
        limited_to = len(run_list)

        kill_running_conky()
        started = 0
        fallback = 0
        position_info = ""

        for theme in run_list:
            try:
                pid, _, mode = run_conky(theme, position_override=position, color_override=colors)
                started += 1
                if mode != "full-visual":
                    fallback += 1
                if position:
                    position_info = (
                        f" @ {position['alignment']} "
                        f"X={position['gap_x']} Y={position['gap_y']}"
                    )
                GLib.idle_add(self.append_log, f"Started: {theme.name} (pid={pid}, mode={mode})")
            except Exception as exc:
                GLib.idle_add(self.append_log, f"Failed: {theme} ({exc})")

        def finish():
            self.set_busy(False)
            self.push_status(
                f"Started {started} theme(s). Fallback: {fallback}. Limited to {limited_to}."
            )
            self.update_runtime_status(True, f"{started} instance(s) running{position_info}")
            self.refresh_log_view()

        GLib.idle_add(finish)

    def run_selected(self, *_args):
        themes = self.selected_theme_paths()
        if not themes:
            self.show_message("Info", "Select at least one theme.")
            return
        self.run_themes(themes)

    def run_all(self, *_args):
        if not self.themes:
            self.show_message("Info", "No valid themes found.")
            return
        self.run_themes([t.path for t in self.themes])

    def run_themes(self, themes: list[Path]):
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return
        position = self._capture_run_position() if len(themes) == 1 else None
        colors = self._capture_run_colors() if len(themes) == 1 else None
        self.set_busy(True)
        self.push_status("Starting themes...")
        threading.Thread(
            target=lambda: self._run_themes_worker(themes, position, colors),
            daemon=True,
        ).start()

    def smart_repair(self, *_args):
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return
        self.set_busy(True)
        self.push_status("Running smart repair...")

        def worker():
            ok = 0
            fail = 0
            for theme in self.themes:
                try:
                    optimize_for_desktop(theme.path)
                    ok += 1
                except Exception as exc:
                    fail += 1
                    GLib.idle_add(self.append_log, f"Repair failed: {theme.path} ({exc})")

            def finish():
                self.set_busy(False)
                self.push_status(f"Smart repair complete. Success: {ok}, Failed: {fail}.")
                self.show_message(
                    "Repair",
                    f"Prepared full launch bundles for {ok} theme(s), failed {fail}.\n"
                    "Asset paths, fonts, and visuals are relinked automatically.",
                )

            GLib.idle_add(finish)

        threading.Thread(target=worker, daemon=True).start()

    def stop_all(self, *_args):
        kill_running_conky()
        self.update_runtime_status(False)
        self.push_status("Stopped all running Conky instances.")

    def update_runtime_status(self, running: bool, detail: str = ""):
        if running:
            self.status_indicator.set_markup(
                "<span size='large' foreground='#10b981'>●</span> Running"
            )
            self.runtime_info.set_text(detail or "Conky is active")
        else:
            self.status_indicator.set_markup(
                "<span size='large' foreground='#64748b'>●</span> Idle"
            )
            self.runtime_info.set_text("No Conky instance running")

    def preview_selected(self, *_args):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return

        seconds = int(load_json(SETTINGS_FILE, DEFAULT_SETTINGS).get("preview_seconds", 5))
        position = self._capture_run_position()
        colors = self._capture_run_colors()
        try:
            kill_running_conky()
            pid, _, mode = run_conky(
                theme.path,
                position_override=position,
                color_override=colors,
            )
        except Exception as exc:
            self.show_message("Preview Error", str(exc), Gtk.MessageType.ERROR)
            return

        pos_text = ""
        if position:
            pos_text = f" @ {position['alignment']} X={position['gap_x']} Y={position['gap_y']}"
        self.update_runtime_status(True, f"Preview: {theme.path.name} [{mode}]{pos_text}")
        self.push_status(f"Preview started ({seconds}s): {theme.path.name} [{mode}]")

        if self._preview_timer_id:
            GLib.source_remove(self._preview_timer_id)

        def stop_preview():
            try:
                os.kill(pid, 15)
            except Exception:
                pass
            kill_running_conky()
            self.update_runtime_status(False)
            self.push_status("Preview stopped.")
            return False

        self._preview_timer_id = GLib.timeout_add_seconds(seconds, stop_preview)

    def edit_selected_theme(self, *_args):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        subprocess.Popen(["xdg-open", str(theme.path)])

    def open_selected_theme_folder(self, *_args):
        theme = self._selected_theme_item()
        if not theme:
            self.show_message("Info", "Select a theme first.")
            return
        subprocess.Popen(["xdg-open", str(theme.path.parent)])

    def import_theme_folder(self, *_args):
        dialog = Gtk.FileChooserDialog(
            title="Import Theme Folder",
            transient_for=self.win,
            action=Gtk.FileChooserAction.SELECT_FOLDER,
        )
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        dialog.set_current_folder(str(HOME))
        response = dialog.run()
        folder = dialog.get_filename() if response == Gtk.ResponseType.OK else None
        dialog.destroy()
        if not folder:
            return
        self._perform_import(lambda: import_folder_to_conky(Path(folder)))

    def import_theme_archive(self, *_args):
        dialog = Gtk.FileChooserDialog(
            title="Import Theme Archive",
            transient_for=self.win,
            action=Gtk.FileChooserAction.OPEN,
        )
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        dialog.set_current_folder(str(HOME))
        filt = Gtk.FileFilter()
        filt.set_name("Archives")
        for ext in ARCHIVE_EXTENSIONS:
            filt.add_pattern(f"*{ext}")
        dialog.add_filter(filt)
        all_filter = Gtk.FileFilter()
        all_filter.set_name("All files")
        all_filter.add_pattern("*")
        dialog.add_filter(all_filter)
        response = dialog.run()
        archive = dialog.get_filename() if response == Gtk.ResponseType.OK else None
        dialog.destroy()
        if not archive:
            return
        self._perform_import(lambda: import_archive_to_conky(Path(archive)))

    def _perform_import(self, import_func: Callable[[], list[str]]):
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return

        self.set_busy(True)
        self.push_status(f"Importing into {DEFAULT_IMPORT_DIR}…")

        def worker():
            try:
                installed = import_func()
                names = ", ".join(installed)

                def finish_ok():
                    self.set_busy(False)
                    self.load_themes()
                    self.push_status(f"Imported {len(installed)} item(s) into ~/.conky")
                    self.show_message(
                        "Import Complete",
                        f"Installed into:\n{DEFAULT_IMPORT_DIR}\n\n"
                        f"Folders: {names}",
                    )

                GLib.idle_add(finish_ok)
            except Exception as exc:
                def finish_err():
                    self.set_busy(False)
                    self.show_message("Import Error", str(exc), Gtk.MessageType.ERROR)

                GLib.idle_add(finish_err)

        threading.Thread(target=worker, daemon=True).start()

    def show_theme_store_dialog(self, *_args):
        dialog = Gtk.Dialog(title="Get Conky Themes", transient_for=self.win, flags=0)
        dialog.set_default_size(820, 580)
        dialog.set_resizable(True)

        content = dialog.get_content_area()
        content.set_spacing(10)
        content.set_margin_top(12)
        content.set_margin_bottom(12)
        content.set_margin_start(12)
        content.set_margin_end(12)

        header = Gtk.Label()
        header.set_markup(
            "<b>Theme Gallery</b>\n"
            "<span foreground='#888888'>GitHub collections and OpenDesktop stores "
            "(gnome-look.org, KDE Look, Pling) — installed to ~/.conky</span>"
        )
        header.set_xalign(0)
        header.set_line_wrap(True)
        content.pack_start(header, False, False, 0)

        notebook = Gtk.Notebook()
        content.pack_start(notebook, True, True, 0)

        github_selection: dict[str, ThemeSource | None] = {"value": None}
        online_selection: dict[str, Any] = {"product": None, "store": ONLINE_STORES[0]}

        # ---- GitHub tab ----
        github_tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        github_tab.set_margin_top(8)
        github_tab.set_margin_bottom(8)
        github_tab.set_margin_start(8)
        github_tab.set_margin_end(8)

        github_search = Gtk.SearchEntry()
        github_search.set_placeholder_text("Search GitHub curated themes…")
        github_tab.pack_start(github_search, False, False, 0)

        github_paned = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        github_scroll = Gtk.ScrolledWindow()
        github_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        github_scroll.set_min_content_height(280)
        github_list = Gtk.ListBox()
        github_list.set_selection_mode(Gtk.SelectionMode.SINGLE)
        github_scroll.add(github_list)
        github_paned.pack_start(github_scroll, True, True, 0)

        github_details = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        github_details.set_size_request(320, -1)
        gh_title = Gtk.Label()
        gh_title.set_markup("<b>Select a theme</b>")
        gh_title.set_xalign(0)
        gh_title.set_line_wrap(True)
        github_details.pack_start(gh_title, False, False, 0)
        gh_desc = Gtk.Label()
        gh_desc.set_xalign(0)
        gh_desc.set_line_wrap(True)
        gh_desc.get_style_context().add_class("dim-label")
        github_details.pack_start(gh_desc, False, False, 0)
        gh_author = Gtk.Label()
        gh_author.set_xalign(0)
        github_details.pack_start(gh_author, False, False, 0)
        gh_tags = Gtk.Label()
        gh_tags.set_xalign(0)
        gh_tags.set_line_wrap(True)
        github_details.pack_start(gh_tags, False, False, 0)
        gh_link = Gtk.LinkButton(uri="https://github.com/almezali/conky-manager-g", label="Project homepage")
        gh_link.set_halign(Gtk.Align.START)
        github_details.pack_start(gh_link, False, False, 0)
        github_paned.pack_start(github_details, False, False, 0)
        github_tab.pack_start(github_paned, True, True, 0)
        notebook.append_page(github_tab, Gtk.Label(label="GitHub"))

        github_rows: dict[Gtk.ListBoxRow, ThemeSource] = {}

        def update_github_details(source: ThemeSource | None):
            github_selection["value"] = source
            if not source:
                gh_title.set_markup("<b>Select a theme</b>")
                gh_desc.set_text("Curated Conky themes from GitHub.")
                gh_author.set_text("")
                gh_tags.set_text("")
                gh_link.set_uri("https://github.com/almezali/conky-manager-g")
                gh_link.set_label("Project homepage")
                return
            gh_title.set_markup(f"<b>{source.name}</b>")
            gh_desc.set_text(source.description)
            gh_author.set_text(f"Author: {source.author}")
            gh_tags.set_text("Tags: " + ", ".join(source.tags))
            gh_link.set_uri(source.homepage)
            gh_link.set_label("View on GitHub")

        def populate_github(filter_text: str = ""):
            for row in github_list.get_children():
                github_list.remove(row)
            github_rows.clear()
            query = filter_text.strip().lower()
            first_row = None
            for source in THEME_CATALOG:
                haystack = " ".join(
                    [source.name, source.description, source.author, " ".join(source.tags)]
                ).lower()
                if query and query not in haystack:
                    continue
                row = Gtk.ListBoxRow()
                box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                box.set_margin_top(6)
                box.set_margin_bottom(6)
                box.set_margin_start(10)
                box.set_margin_end(10)
                title = Gtk.Label()
                title.set_markup(f"<b>{source.name}</b>")
                title.set_xalign(0)
                box.pack_start(title, False, False, 0)
                sub = Gtk.Label(label=f"{source.author}")
                sub.set_xalign(0)
                sub.get_style_context().add_class("dim-label")
                box.pack_start(sub, False, False, 0)
                row.add(box)
                github_rows[row] = source
                github_list.add(row)
                if first_row is None:
                    first_row = row
            github_list.show_all()
            if first_row is not None:
                github_list.select_row(first_row)

        github_list.connect("row-selected", lambda _lb, row: update_github_details(github_rows.get(row)))
        github_search.connect("search-changed", lambda entry: populate_github(entry.get_text()))
        populate_github()

        # ---- OpenDesktop tab ----
        store_tab = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        store_tab.set_margin_top(8)
        store_tab.set_margin_bottom(8)
        store_tab.set_margin_start(8)
        store_tab.set_margin_end(8)

        controls = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        store_combo = Gtk.ComboBoxText()
        for store in ONLINE_STORES:
            store_combo.append(store.id, store.label)
        store_combo.set_active_id(ONLINE_STORES[0].id)

        sort_combo = Gtk.ComboBoxText()
        for sort_id, sort_label in (
            ("downloads", "Most downloaded"),
            ("latest", "Latest"),
            ("rating", "Top rated"),
        ):
            sort_combo.append(sort_id, sort_label)
        sort_combo.set_active_id("downloads")

        store_search = Gtk.SearchEntry()
        store_search.set_placeholder_text("Search Conky themes…")
        controls.pack_start(store_combo, False, False, 0)
        controls.pack_start(sort_combo, False, False, 0)
        controls.pack_start(store_search, True, True, 0)
        store_tab.pack_start(controls, False, False, 0)

        store_status = Gtk.Label(label="Loading themes…")
        store_status.set_xalign(0)
        store_status.get_style_context().add_class("dim-label")
        store_tab.pack_start(store_status, False, False, 0)

        store_paned = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        store_scroll = Gtk.ScrolledWindow()
        store_scroll.set_policy(Gtk.PolicyType.AUTOMATIC, Gtk.PolicyType.AUTOMATIC)
        store_scroll.set_min_content_height(260)
        store_list = Gtk.ListBox()
        store_list.set_selection_mode(Gtk.SelectionMode.SINGLE)
        store_scroll.add(store_list)
        store_paned.pack_start(store_scroll, True, True, 0)

        store_details = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        store_details.set_size_request(320, -1)
        st_title = Gtk.Label()
        st_title.set_markup("<b>Select a theme</b>")
        st_title.set_xalign(0)
        st_title.set_line_wrap(True)
        store_details.pack_start(st_title, False, False, 0)
        st_desc = Gtk.Label()
        st_desc.set_xalign(0)
        st_desc.set_line_wrap(True)
        st_desc.set_max_width_chars(42)
        st_desc.get_style_context().add_class("dim-label")
        store_details.pack_start(st_desc, False, False, 0)
        st_meta = Gtk.Label()
        st_meta.set_xalign(0)
        st_meta.set_line_wrap(True)
        store_details.pack_start(st_meta, False, False, 0)
        st_link = Gtk.LinkButton(uri="https://www.gnome-look.org/browse?cat=124", label="Browse on web")
        st_link.set_halign(Gtk.Align.START)
        store_details.pack_start(st_link, False, False, 0)
        store_paned.pack_start(store_details, False, False, 0)
        store_tab.pack_start(store_paned, True, True, 0)

        load_more_btn = Gtk.Button.new_with_label("Load more")
        load_tab_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        load_tab_box.pack_start(load_more_btn, False, False, 0)
        store_tab.pack_start(load_tab_box, False, False, 0)
        notebook.append_page(store_tab, Gtk.Label(label="gnome-look & Stores"))

        store_rows: dict[Gtk.ListBoxRow, OnlineProduct] = {}
        store_state = {"page": 1, "total": 0, "loading": False, "items": []}

        def current_store() -> OnlineStore:
            return get_online_store(store_combo.get_active_id() or ONLINE_STORES[0].id)

        def update_store_details(product: OnlineProduct | None):
            store = current_store()
            online_selection["store"] = store
            online_selection["product"] = product
            if not product:
                st_title.set_markup("<b>Select a theme</b>")
                st_desc.set_text(f"Browse the Conky section from {store.label}.")
                st_meta.set_text("")
                st_link.set_uri(store.browse_url)
                st_link.set_label(f"Open {store.label}")
                return
            st_title.set_markup(f"<b>{product.name}</b>")
            st_desc.set_text(product.summary or "No description available.")
            st_meta.set_text(
                f"Author: {product.author} | Downloads: {product.downloads} | "
                f"Score: {product.score:.1f} | v{product.version}"
            )
            st_link.set_uri(product.page_url)
            st_link.set_label(f"View on {store.label}")

        def render_store_items(append: bool = False):
            start_index = 0
            if not append:
                for row in store_list.get_children():
                    store_list.remove(row)
                store_rows.clear()
            else:
                start_index = len(store_rows)
            first_row = None
            for product in store_state["items"][start_index:]:
                row = Gtk.ListBoxRow()
                box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                box.set_margin_top(6)
                box.set_margin_bottom(6)
                box.set_margin_start(10)
                box.set_margin_end(10)
                title = Gtk.Label()
                title.set_markup(f"<b>{product.name}</b>")
                title.set_xalign(0)
                box.pack_start(title, False, False, 0)
                sub = Gtk.Label(label=f"{product.author} · {product.downloads} downloads")
                sub.set_xalign(0)
                sub.get_style_context().add_class("dim-label")
                box.pack_start(sub, False, False, 0)
                row.add(box)
                store_rows[row] = product
                store_list.add(row)
                if first_row is None and not append:
                    first_row = row
            store_list.show_all()
            if first_row is not None and not append:
                store_list.select_row(first_row)
            shown = len(store_state["items"])
            total = store_state["total"]
            store_status.set_text(f"Showing {shown} of {total} Conky themes")
            load_more_btn.set_sensitive(shown < total and not store_state["loading"])

        def load_store_page(reset: bool = False):
            if store_state["loading"]:
                return
            if reset:
                store_state["page"] = 1
                store_state["items"] = []
            store_state["loading"] = True
            store_status.set_text("Loading themes from store…")
            load_more_btn.set_sensitive(False)

            store = current_store()
            query = store_search.get_text()
            page = store_state["page"]
            sort = sort_combo.get_active_id() or "downloads"

            def worker():
                try:
                    products, total = browse_online_store(
                        store, query=query, page=page, per_page=20, sort=sort
                    )
                    error = None
                except Exception as exc:
                    products, total, error = [], 0, str(exc)

                def finish():
                    store_state["loading"] = False
                    if error:
                        store_status.set_text(f"Failed to load store: {error}")
                        load_more_btn.set_sensitive(False)
                        return
                    if reset:
                        store_state["items"] = products
                    else:
                        store_state["items"].extend(products)
                    store_state["total"] = total
                    render_store_items(append=not reset)

                GLib.idle_add(finish)

            threading.Thread(target=worker, daemon=True).start()

        def on_store_changed(*_args):
            load_store_page(reset=True)

        def on_load_more(*_args):
            store_state["page"] += 1
            load_store_page(reset=False)

        store_combo.connect("changed", on_store_changed)
        sort_combo.connect("changed", on_store_changed)
        store_search.connect("search-changed", lambda *_: load_store_page(reset=True))
        store_list.connect("row-selected", lambda _lb, row: update_store_details(store_rows.get(row)))
        load_more_btn.connect("clicked", on_load_more)
        load_store_page(reset=True)

        dialog.add_button("Close", Gtk.ResponseType.CLOSE)
        download_btn = dialog.add_button("Download && Install", Gtk.ResponseType.OK)
        dialog.show_all()

        response = dialog.run()
        active_tab = notebook.get_current_page()
        dialog.destroy()

        if response != Gtk.ResponseType.OK or self._worker_busy:
            if response == Gtk.ResponseType.OK and self._worker_busy:
                self.show_message("Busy", "Please wait until the current operation finishes.")
            return

        if active_tab == 0:
            source = github_selection["value"]
            if source:
                self._download_theme_with_progress(source)
            return

        product = online_selection["product"]
        store = online_selection["store"]
        if product:
            self._download_online_product_with_progress(store, product)

    def _run_download_progress(self, title: str, worker_func: Callable[[Callable], list[str]]):
        progress_dialog = Gtk.Dialog(title="Downloading Theme", transient_for=self.win, flags=0)
        progress_dialog.set_default_size(460, 150)
        progress_dialog.set_deletable(False)
        box = progress_dialog.get_content_area()
        box.set_spacing(10)
        box.set_margin_top(15)
        box.set_margin_bottom(15)
        box.set_margin_start(15)
        box.set_margin_end(15)
        title_label = Gtk.Label()
        title_label.set_markup(f"<b>{title}</b>")
        title_label.set_xalign(0)
        box.pack_start(title_label, False, False, 0)
        progress = Gtk.ProgressBar()
        progress.set_show_text(True)
        box.pack_start(progress, False, False, 0)
        status_label = Gtk.Label(label="Connecting…")
        status_label.set_xalign(0)
        status_label.get_style_context().add_class("dim-label")
        box.pack_start(status_label, False, False, 0)
        progress_dialog.show_all()

        result: dict[str, Any] = {"installed": [], "error": None}
        done = {"finished": False}

        def progress_cb(fraction: float, message: str):
            pct = int(max(0.0, min(1.0, fraction)) * 100)
            GLib.idle_add(progress.set_fraction, fraction)
            GLib.idle_add(progress.set_text, f"{pct}%")
            GLib.idle_add(status_label.set_text, message)

        def worker():
            try:
                result["installed"] = worker_func(progress_cb)
            except Exception as exc:
                result["error"] = str(exc)
            finally:
                done["finished"] = True

        def poll_done():
            if not done["finished"]:
                return True
            progress_dialog.destroy()
            self.set_busy(False)
            if result["error"]:
                self.show_message("Download Error", result["error"], Gtk.MessageType.ERROR)
                return False
            self.load_themes()
            self.push_status(f"Installed theme: {title}")
            self.show_message(
                "Theme Installed",
                f"{title} was installed into ~/.conky\n\n"
                f"Folders: {', '.join(result['installed'])}",
            )
            return False

        self.set_busy(True)
        threading.Thread(target=worker, daemon=True).start()
        GLib.timeout_add(150, poll_done)
        progress_dialog.run()

    def _download_theme_with_progress(self, source: ThemeSource):
        self._run_download_progress(
            source.name,
            lambda progress_cb: download_and_install_theme(source, progress_callback=progress_cb),
        )

    def _download_online_product_with_progress(self, store: OnlineStore, product: OnlineProduct):
        self._run_download_progress(
            product.name,
            lambda progress_cb: download_and_install_online_product(
                store, product.product_id, progress_callback=progress_cb
            ),
        )

    def import_theme(self, *_args):
        """Legacy entry point — opens folder import."""
        self.import_theme_folder()

    # ---------------- Profiles ----------------

    def refresh_profiles(self):
        self.profiles = load_json(PROFILES_FILE, {"profiles": {}, "last_profile": ""})
        for row in self.profile_listbox.get_children():
            self.profile_listbox.remove(row)
        for name in sorted(self.profiles.get("profiles", {}).keys()):
            row = Gtk.ListBoxRow()
            box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
            box.set_margin_top(5)
            box.set_margin_bottom(5)
            box.set_margin_start(10)
            box.set_margin_end(10)
            lbl = Gtk.Label()
            lbl.set_markup(f"<b>{name}</b>")
            lbl.set_xalign(0)
            box.pack_start(lbl, False, False, 0)
            count = len(self.profiles["profiles"][name])
            sub = Gtk.Label(label=f"{count} theme(s)")
            sub.set_xalign(0)
            sub.get_style_context().add_class("dim-label")
            box.pack_start(sub, False, False, 0)
            row.add(box)
            row.profile_name = name
            self.profile_listbox.add(row)
        self.profile_listbox.show_all()
        self.show_profile_details()

    def on_profile_selected(self, listbox, row):
        if row:
            self.profile_name_entry.set_text(row.profile_name)
            self.show_profile_details()

    def selected_profile_name(self) -> str:
        row = self.profile_listbox.get_selected_row()
        if row:
            return row.profile_name
        return self.profile_name_entry.get_text().strip()

    def save_profile(self, *_args):
        name = self.profile_name_entry.get_text().strip()
        if not name:
            self.show_message("Error", "Enter a profile name.", Gtk.MessageType.ERROR)
            return
        themes = self.selected_theme_paths()
        if not themes:
            self.show_message("Error", "Select at least one theme in the sidebar.", Gtk.MessageType.ERROR)
            return
        self.profiles.setdefault("profiles", {})[name] = [str(p) for p in themes]
        self.profiles["last_profile"] = name
        save_json(PROFILES_FILE, self.profiles)
        self.refresh_profiles()
        self.push_status(f"Saved profile: {name}")

    def add_to_profile(self, *_args):
        name = self.selected_profile_name()
        if not name:
            self.show_message("Error", "Select or enter a profile name.", Gtk.MessageType.ERROR)
            return
        if name not in self.profiles.get("profiles", {}):
            self.show_message("Error", f"Profile not found: {name}", Gtk.MessageType.ERROR)
            return
        themes = self.selected_theme_paths()
        if not themes:
            self.show_message("Error", "Select themes in the sidebar first.", Gtk.MessageType.ERROR)
            return
        current = set(self.profiles["profiles"][name])
        for t in themes:
            current.add(str(t))
        self.profiles["profiles"][name] = sorted(current)
        save_json(PROFILES_FILE, self.profiles)
        self.refresh_profiles()
        self.push_status(f"Updated profile: {name} (+{len(themes)})")

    def show_profile_details(self):
        for row in self.profile_details_listbox.get_children():
            self.profile_details_listbox.remove(row)
        name = self.selected_profile_name()
        if not name:
            self.profile_details_title.set_text("No profile selected")
            return
        items = self.profiles.get("profiles", {}).get(name, [])
        self.profile_details_title.set_text(f"{name} — {len(items)} theme(s)")
        for p in items:
            row = Gtk.ListBoxRow()
            lbl = Gtk.Label(label=p)
            lbl.set_xalign(0)
            lbl.set_margin_top(4)
            lbl.set_margin_bottom(4)
            lbl.set_margin_start(10)
            row.add(lbl)
            self.profile_details_listbox.add(row)
        self.profile_details_listbox.show_all()

    def delete_profile(self, *_args):
        name = self.selected_profile_name()
        if not name:
            return
        confirm = Gtk.MessageDialog(
            transient_for=self.win,
            modal=True,
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.YES_NO,
            text="Delete Profile",
        )
        confirm.format_secondary_text(f"Delete profile '{name}'?")
        if confirm.run() != Gtk.ResponseType.YES:
            confirm.destroy()
            return
        confirm.destroy()
        self.profiles.get("profiles", {}).pop(name, None)
        save_json(PROFILES_FILE, self.profiles)
        self.refresh_profiles()

    def run_profile(self, *_args):
        name = self.selected_profile_name()
        if not name:
            self.show_message("Error", "Select a profile.", Gtk.MessageType.ERROR)
            return
        theme_paths = self.profiles.get("profiles", {}).get(name)
        if not theme_paths:
            self.show_message("Error", f"Profile not found: {name}", Gtk.MessageType.ERROR)
            return
        themes = [Path(p) for p in theme_paths if Path(p).exists()]
        if not themes:
            self.show_message("Error", "No existing themes in this profile.", Gtk.MessageType.ERROR)
            return
        self.profiles["last_profile"] = name
        save_json(PROFILES_FILE, self.profiles)
        self.run_themes(themes)

    def enable_autostart(self, *_args):
        name = self.selected_profile_name()
        if not name:
            self.show_message("Error", "Select a profile first.", Gtk.MessageType.ERROR)
            return
        if name not in self.profiles.get("profiles", {}):
            self.show_message("Error", f"Profile not found: {name}", Gtk.MessageType.ERROR)
            return
        AUTOSTART_DIR.mkdir(parents=True, exist_ok=True)
        exec_path = str(Path(__file__).resolve())
        desktop = "\n".join([
            "[Desktop Entry]",
            "Type=Application",
            f"Name={APP_NAME}",
            "Comment=Start Conky profile on login",
            f'Exec=python3 "{exec_path}" --run-profile "{name}"',
            "Terminal=false",
            "Hidden=false",
            "X-GNOME-Autostart-enabled=true",
            "X-GNOME-Autostart-Delay=3",
            "X-KDE-autostart-after=panel",
            "X-KDE-StartupNotify=false",
            "OnlyShowIn=GNOME;KDE;XFCE;Cinnamon;MATE;LXDE;LXQt;",
            "",
        ])
        try:
            AUTOSTART_FILE.write_text(desktop, encoding="utf-8")
            if OLD_AUTOSTART_FILE.exists() and OLD_AUTOSTART_FILE != AUTOSTART_FILE:
                OLD_AUTOSTART_FILE.unlink(missing_ok=True)
            self.show_message("Autostart", f"Enabled autostart for profile: {name}")
        except Exception as exc:
            self.show_message("Error", f"Failed to enable autostart: {exc}", Gtk.MessageType.ERROR)

    def disable_autostart(self, *_args):
        try:
            for path in (AUTOSTART_FILE, OLD_AUTOSTART_FILE):
                if path.exists():
                    path.unlink()
            self.show_message("Autostart", "Autostart disabled.")
        except Exception as exc:
            self.show_message("Error", f"Failed to disable autostart: {exc}", Gtk.MessageType.ERROR)

    # ---------------- Diagnostics ----------------

    def validate_selected_theme(self, *_args):
        themes = self.selected_theme_paths()
        if not themes:
            self.show_message("Info", "Select a theme first.")
            return
        cfg = themes[0]
        try:
            bundle = create_theme_launch_bundle(cfg, apply_desktop_optimize=True)
            ok_launch = validate_conky_config(
                bundle.launch_config,
                launch_dir=bundle.launch_dir,
                env=build_launch_env(bundle),
            )
            found, missing = scan_theme_assets(cfg)
            missing_text = "\n".join(missing[:8]) if missing else "None"
            if len(missing) > 8:
                missing_text += f"\n… and {len(missing) - 8} more"
            self.show_message(
                "Validation",
                f"Theme: {cfg.name}\n"
                f"Launch bundle: {'OK' if ok_launch else 'FAIL'}\n"
                f"Path fixes: {bundle.path_fixes}\n"
                f"Assets found: {len(found)}\n"
                f"Missing assets:\n{missing_text}",
            )
        except Exception as exc:
            self.show_message("Validation Error", str(exc), Gtk.MessageType.ERROR)

    def refresh_log_view(self, lines: int = 250):
        try:
            text = LOG_FILE.read_text(encoding="utf-8", errors="ignore")
            tail = "\n".join(text.splitlines()[-lines:])
        except Exception:
            tail = "(Log file not available yet.)"
        self.logs_buffer.set_text(tail)

    def append_log(self, line: str):
        end = self.logs_buffer.get_end_iter()
        self.logs_buffer.insert(end, line + "\n")

    def health_scan(self, *_args):
        if self._worker_busy:
            self.show_message("Busy", "Please wait until the current operation finishes.")
            return
        self.set_busy(True)
        self.push_status("Health scan running...")

        def worker():
            ok = warn = bad = 0
            for item in self.themes:
                GLib.idle_add(self.push_status, f"Checking: {item.label}")
                try:
                    bundle = create_theme_launch_bundle(item.path, apply_desktop_optimize=True)
                    ok_launch = validate_conky_config(
                        bundle.launch_config,
                        launch_dir=bundle.launch_dir,
                        env=build_launch_env(bundle),
                    )
                    if not ok_launch:
                        self.theme_status[item.label] = "broken"
                        bad += 1
                        continue
                    if bundle.missing_assets:
                        self.theme_status[item.label] = "needs_fix"
                        warn += 1
                    else:
                        self.theme_status[item.label] = "stable"
                        ok += 1
                except Exception:
                    self.theme_status[item.label] = "broken"
                    bad += 1

            def finish():
                self.set_busy(False)
                self.apply_filter()
                self.push_status(
                    f"Health scan complete. Stable: {ok}, Needs fix: {warn}, Broken: {bad}."
                )

            GLib.idle_add(finish)

        threading.Thread(target=worker, daemon=True).start()


# ---------------------------------------------------------------------------
# CLI & entry point
# ---------------------------------------------------------------------------

def run_profile_cli(profile_name: str) -> int:
    if not is_conky_installed():
        return 1
    profiles = load_json(PROFILES_FILE, {"profiles": {}})
    theme_paths = profiles.get("profiles", {}).get(profile_name, [])
    themes = [Path(p) for p in theme_paths if Path(p).exists()]
    if not themes:
        return 1
    kill_running_conky()
    settings = load_json(SETTINGS_FILE, DEFAULT_SETTINGS)
    max_instances = int(settings.get("max_instances", 1))
    for theme in themes[:max_instances]:
        try:
            run_conky(theme)
        except Exception:
            pass
    return 0


def check_gtk_dependencies() -> bool:
    try:
        gi.require_version("Gtk", "3.0")
        from gi.repository import Gtk  # noqa: F401
        return True
    except Exception as exc:
        print(f"Error: GTK3 dependencies not found: {exc}")
        print("Install: python3-gobject python3-gi-cairo gir1.2-gtk-3.0")
        return False


def main() -> int:
    if "--run-profile" in sys.argv:
        try:
            idx = sys.argv.index("--run-profile")
            profile_name = sys.argv[idx + 1]
        except Exception:
            return 1
        return run_profile_cli(profile_name)

    if not is_conky_installed():
        print("Conky is not installed. Install it first, then run this app again.")
        return 1

    if not check_gtk_dependencies():
        return 1

    ConkyManagerGTK()
    Gtk.main()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
