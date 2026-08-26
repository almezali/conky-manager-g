# GTK Themes

A collection of GTK themes stored inside the `.themes` directory.

## 📦 Archive Structure

The `themes.tar.xz` archive contains:

```text
themes.tar.xz
└── .themes/
    ├── Theme-1/
    ├── Theme-2/
    ├── Theme-3/
    └── ...
```

The `.themes` directory is intended to be installed directly inside your Home directory.

---

## 🚀 Installation

### 1. Extract to Home

Open a terminal and run:

```bash
tar -xJf themes.tar.xz -C ~
```

This will create:

```text
~/.themes/
```

with all included themes.

### 2. Check the installed themes

```bash
ls ~/.themes
```

You can also check hidden directories with:

```bash
ls -la ~
```

---

## 🛠️ Create the Directory First

If `~/.themes` does not exist:

```bash
mkdir -p ~/.themes
tar -xJf themes.tar.xz -C ~
```

---

# 🎨 Using GTK Themes

GTK themes can be used by many Linux desktop environments. The exact method depends on your Desktop Environment.

---

## 🖥️ XFCE

Open:

**Settings Manager → Appearance → Style**

Select the installed GTK theme.

You can also change the icon theme from:

**Settings Manager → Appearance → Icons**

XFCE is one of the easiest environments for using GTK themes.

---

## 🟣 GNOME

Open:

**Settings → Appearance**

For themes that support GNOME's current GTK stack, select the appropriate options there.

For additional GTK theme management, tools such as **GNOME Tweaks** may be used when available.

> Note: Modern GNOME uses GTK4 and libadwaita for many system applications. A traditional GTK theme does not necessarily change libadwaita applications.

---

## 🔵 KDE Plasma

Open:

**System Settings → Colors & Themes → Application Style → Configure GNOME/GTK Application Style**

Depending on your Plasma version, the GTK configuration may be available under:

**System Settings → Appearance → Application Style → GNOME/GTK Application Style**

Choose the GTK theme from the available list.

> KDE Plasma itself uses Qt. GTK themes only affect GTK applications.

---

## 🟢 Cinnamon

Open:

**System Settings → Themes**

Choose the appropriate theme under the application/GTK theme settings.

Cinnamon provides good support for GTK3 themes.

---

## 🟤 MATE

Open:

**Control Center → Appearance**

Then select the desired theme.

You can also customize:

* GTK theme
* Icons
* Window borders
* Fonts

MATE works well with traditional GTK themes.

---

## 🟠 Budgie

Open the desktop's appearance/theme settings and select the installed GTK theme.

Budgie uses GTK extensively, so GTK themes can significantly change the appearance of applications.

---

## 🪶 LXDE

Open:

**Preferences → Customize Look and Feel**

Select the GTK theme under the appropriate widget/theme section.

LXDE primarily uses GTK2/GTK3 depending on the applications and distribution.

---

## 🟦 LXQt

LXQt itself is Qt-based, but GTK applications can still use GTK themes.

Depending on the distribution and configuration, GTK appearance can be configured through the desktop's appearance settings or GTK configuration tools.

> LXQt's own applications are Qt applications, so changing a GTK theme will not change the LXQt interface itself.

---

# ⚙️ Manual GTK Configuration

If your desktop environment does not provide a graphical GTK theme selector, GTK themes can be configured manually.

## GTK 3

Create:

```bash
mkdir -p ~/.config/gtk-3.0
```

Then create:

```text
~/.config/gtk-3.0/settings.ini
```

Example:

```ini
[Settings]
gtk-theme-name=Theme-Name
gtk-icon-theme-name=Icon-Theme
gtk-font-name=Sans 10
```

Replace:

```text
Theme-Name
```

with the exact directory name inside:

```text
~/.themes/
```

For example:

```ini
[Settings]
gtk-theme-name=Adwaita-dark
```

---

## GTK 2

Older GTK2 applications can use:

```text
~/.gtkrc-2.0
```

Example:

```text
gtk-theme-name="Theme-Name"
```

GTK2 is legacy technology, so modern applications generally use GTK3 or GTK4 instead.

---

# 🆕 GTK 4

GTK4 has a different theming model.

Create:

```bash
mkdir -p ~/.config/gtk-4.0
```

Some GTK themes provide GTK4 files that can be installed or linked into:

```text
~/.config/gtk-4.0/
```

For example, if the theme provides:

```text
gtk.css
assets/
```

they may be placed in:

```text
~/.config/gtk-4.0/
```

However, **not every GTK3 theme supports GTK4**.

---

# ⚠️ GTK4 & libadwaita

Modern GNOME applications increasingly use:

**GTK4 + libadwaita**

A traditional GTK theme may therefore affect GTK3 applications while having little or no effect on newer GNOME/libadwaita applications.

This is normal behavior and does not necessarily mean that the theme was installed incorrectly.

---

# 🔍 Find the Exact Theme Name

List installed themes:

```bash
find ~/.themes -maxdepth 1 -mindepth 1 -type d -printf '%f\n'
```

Example:

```text
Adwaita-dark
Arc-Dark
Nordic
Orchis
```

The directory name is normally the name used by GTK configuration.

---

# 📁 Install a Theme Manually

If you download a theme separately:

```bash
mkdir -p ~/.themes
cp -r Theme-Name ~/.themes/
```

Or:

```bash
cp -r Theme-Name ~/.themes/
```

After installation, select it from your desktop environment's appearance settings.

---

# 🌐 System-Wide Installation

If you want the theme available to **all users**, install it under:

```text
/usr/share/themes/
```

For example:

```bash
sudo cp -r Theme-Name /usr/share/themes/
```

User installation is generally preferable because it does not require root privileges.

---

# 👤 User vs System Themes

### Current user

```text
~/.themes/
```

Only your user can use the theme.

### All users

```text
/usr/share/themes/
```

The theme becomes available system-wide.

---

# 🧩 Useful Theme Locations

GTK themes can commonly be found in:

```text
~/.themes/
~/.local/share/themes/
```

System-wide:

```text
/usr/share/themes/
```

Icons are commonly installed in:

```text
~/.icons/
~/.local/share/icons/
```

or:

```text
/usr/share/icons/
```

---

# 🔄 Refresh the Desktop

Some desktop environments apply the theme immediately.

If the theme does not appear:

1. Log out and log back in.
2. Restart the affected application.
3. Reopen the desktop appearance settings.
4. Verify that the theme directory contains the expected GTK files.

---

# 🔎 Check Theme Contents

A typical GTK theme may contain directories such as:

```text
Theme-Name/
├── gtk-2.0/
├── gtk-3.0/
├── gtk-4.0/
├── index.theme
└── ...
```

The available directories indicate which GTK versions the theme supports.

For example:

```text
gtk-3.0/
```

means GTK3 support.

```text
gtk-4.0/
```

means GTK4 support.

A theme containing only `gtk-3.0` should not be expected to fully theme GTK4 applications.

---

# 💡 Recommended Installation

For a normal user installation, simply run:

```bash
tar -xJf themes.tar.xz -C ~
```

The result should be:

```text
~/.themes/
```

Then select the desired theme from your Desktop Environment's appearance settings.

---

## Supported Desktop Environments

GTK themes can be used with applications running on many Linux desktops, including:

* XFCE
* GNOME
* KDE Plasma
* Cinnamon
* MATE
* Budgie
* LXDE
* LXQt
* Openbox
* i3
* bspwm
* Sway
* Other GTK-based environments and window-manager setups

> The desktop environment itself may use GTK, Qt, or another toolkit. A GTK theme primarily controls **GTK applications**, not necessarily the entire desktop.

---

## 📌 Quick Install

```bash
mkdir -p ~/.themes && tar -xJf themes.tar.xz -C ~
```

That's it. Your themes will be available under:

```text
~/.themes/
```



<img width="1366" height="701" alt="pic-01" src="https://github.com/user-attachments/assets/858e7e3b-2714-4dba-aca4-fbe9241ebfb2" />

<img width="1366" height="768" alt="0-01-Scr" src="https://github.com/user-attachments/assets/f1fc130d-89c5-4b96-9b97-bbbf3ff994e1" />

<img width="1366" height="768" alt="0-06-Scr" src="https://github.com/user-attachments/assets/4db929bb-0cb3-41bf-82f7-3fab4b7b7840" />

<img width="1366" height="768" alt="0-02-Scr" src="https://github.com/user-attachments/assets/1f8f41df-f159-4975-ac22-786d2ca08804" />

<img width="1366" height="768" alt="017" src="https://github.com/user-attachments/assets/8d45317d-f61f-4f0e-bce4-b65e9c53e87e" />

<img width="1366" height="768" alt="016" src="https://github.com/user-attachments/assets/d7a27e29-86d1-4d80-8031-05d4092fe89b" />


<img width="1366" height="768" alt="023" src="https://github.com/user-attachments/assets/1e845922-57d0-44a0-a1bf-74f41b4aeb24" />

<img width="1366" height="768" alt="031" src="https://github.com/user-attachments/assets/40485e7c-ca5b-43ba-8070-8fd17433f86f" />

<img width="1366" height="768" alt="0C-Screenshot_2026-06-11_18-44-48" src="https://github.com/user-attachments/assets/a1416724-543f-451c-a3dc-7cf37d23d73e" />


<img width="1366" height="768" alt="0-0-Screenshot_2026-06-14_11-46-35" src="https://github.com/user-attachments/assets/0186ecde-746b-40b8-b65d-792d8d598a2e" />

<img width="1366" height="768" alt="Screenshot_2026-06-14_19-59-16" src="https://github.com/user-attachments/assets/b50fe6c4-972c-4f6d-b6d5-f37e757a48a2" />

<img width="1366" height="768" alt="pkger-d-0001" src="https://github.com/user-attachments/assets/83ed4409-b2b0-4570-8876-fec4e622b1db" />


<img width="1366" height="768" alt="Screenshot_2025-11-04_09-05-31" src="https://github.com/user-attachments/assets/0ff7e09f-d26d-46e5-adfd-12ddad5c5abf" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-08-35" src="https://github.com/user-attachments/assets/d30aa2c4-f0ee-4997-bdb7-7c90e6520bd9" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-10-13" src="https://github.com/user-attachments/assets/d3af0b74-422b-4d14-a013-7b1d712582af" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-11-24" src="https://github.com/user-attachments/assets/e97209d9-abdf-48c9-a2e5-82e309a4a3a8" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-13-37" src="https://github.com/user-attachments/assets/c5fc3462-0078-4af3-b0e3-c6357fae105b" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-18-21" src="https://github.com/user-attachments/assets/0ba62a98-46d8-4f7a-8887-2531fd821752" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-19-48" src="https://github.com/user-attachments/assets/d6794921-e9bd-407b-a6c0-0e17cf896793" />
<img width="1366" height="768" alt="Screenshot_2025-11-04_09-22-32" src="https://github.com/user-attachments/assets/14d1616a-b9f9-40b7-bbcb-268436faeba0" />


