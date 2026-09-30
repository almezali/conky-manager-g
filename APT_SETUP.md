# Debian APT Repository Setup

The repository publishes the Debian package through GitHub Pages.

## 1. Create an APT signing key

Run locally:

```bash
gpg --full-generate-key
```

Choose:
- RSA and RSA
- 4096 bits
- A suitable expiration period
- Name: `Conky Manager GTK APT`
- Email: an address you control

List the key:

```bash
gpg --list-secret-keys --keyid-format LONG
```

Export the private key:

```gpg
gpg --armor --export-secret-keys YOUR_KEY_ID > conky-manager-apt-private.asc
```

## 2. Add GitHub Actions secrets

In the repository settings, add:

- `APT_GPG_PRIVATE_KEY` — the complete contents of `conky-manager-apt-private.asc`
- `APT_GPG_PASSPHRASE` — the passphrase used for the signing key

Never commit the private key to Git.

## 3. Enable GitHub Pages

In repository **Settings → Pages**, select **GitHub Actions** as the deployment source.

The workflow is:

`/.github/workflows/apt-repository.yml`

It runs automatically whenever a GitHub Release is published. It can also be started manually from the Actions page.

## 4. Install from APT

After the first successful publication:

```bash
sudo install -d -m 0755 /etc/apt/keyrings

curl -fsSL https://almezali.github.io/conky-manager-g/apt.gpg \
  | sudo gpg --dearmor -o /etc/apt/keyrings/conky-manager-g.gpg

echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/conky-manager-g.gpg] https://almezali.github.io/conky-manager-g stable main" \
  | sudo tee /etc/apt/sources.list.d/conky-manager-g.list

sudo apt update
sudo apt install conky-manager-g
```

Future releases are delivered through the same repository:

```bash
sudo apt update
sudo apt upgrade
```

The repository currently publishes the x86_64/amd64 Debian package.
