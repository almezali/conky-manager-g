##  Install from APT


```bash
sudo install -d -m 0755 /etc/apt/keyrings

curl -fsSL https://almezali.github.io/conky-manager-g/apt.gpg \
  | sudo gpg --dearmor -o /etc/apt/keyrings/conky-manager-g.gpg

echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/conky-manager-g.gpg] https://almezali.github.io/conky-manager-g stable main" \
  | sudo tee /etc/apt/sources.list.d/conky-manager-g.list

sudo apt update
sudo apt install conky-manager-g
```

```bash
sudo apt update
sudo apt upgrade
```

The repository currently publishes the x86_64/amd64 Debian package.
