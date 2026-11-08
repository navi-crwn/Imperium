#!/usr/bin/env bash
# Tahap 0, bagian yang butuh root. Jalankan sebagai:  sudo bash ~/imperium/scripts/tahap0-sudo.sh
# Setiap langkah bertanya dulu (y/t) dan bisa diulang dengan aman.
#
# Yang dilakukan:
#  1. sudoers sempit supaya hop-backup.sh bisa membackup NoteHop tanpa password
#  2. kosongkan NOTEHOP_ADMINS (celah: siapa pun yang daftar duluan dengan nama itu jadi admin)
#  3. placeholder Imperium di 127.0.0.1:8100 (systemd)
#  4. pintu utama: DNS imperium.hel.ink -> IP Tailscale, sertifikat Let's Encrypt (DNS-01), vhost nginx
#  5. pintu cadangan: cloudflared tunnel "imperium" untuk citadel.hel.ink (+ Access policy manual di dashboard)
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "jalankan dengan sudo"; exit 1; }
REPO=/home/carawin/imperium
TS_IP=$(tailscale ip -4 | head -1)
ask() { read -r -p "$1 (y/t) " a; [ "${a:-t}" = y ]; }
say() { printf '\n== %s\n' "$*"; }

# ---------- 1. sudoers untuk backup NoteHop ----------
if ask "1) Pasang sudoers agar carawin boleh menjalankan 'notehop backup' sebagai notehop dan membaca env NoteHop dan FileHop?"; then
  cat > /etc/sudoers.d/hop-backup <<'EOF'
# hop-backup: hanya perintah ini, tanpa password
carawin ALL=(notehop) NOPASSWD: /usr/local/bin/notehop backup -db /var/lib/notehop/notehop.db /tmp/hop-notehop-*.db
carawin ALL=(root) NOPASSWD: /usr/bin/mv /tmp/hop-notehop-*.db /home/carawin/backups/hop/*/notehop/notehop.db
carawin ALL=(root) NOPASSWD: /usr/bin/chown carawin /home/carawin/backups/hop/*/notehop/notehop.db
carawin ALL=(root) NOPASSWD: /usr/bin/cat /etc/notehop/notehop.env
carawin ALL=(root) NOPASSWD: /usr/bin/cat /etc/filehop/filehop.env
EOF
  chmod 440 /etc/sudoers.d/hop-backup && visudo -cf /etc/sudoers.d/hop-backup && echo "   sudoers OK"
fi

# ---------- 2. NOTEHOP_ADMINS ----------
if grep -qE '^NOTEHOP_ADMINS=.+' /etc/notehop/notehop.env 2>/dev/null; then
  echo "   NOTEHOP_ADMINS saat ini: $(grep -E '^NOTEHOP_ADMINS=' /etc/notehop/notehop.env | cut -d= -f2-)"
  if ask "2) Kosongkan NOTEHOP_ADMINS dan restart notehop? (peran admin akan datang dari HOP ID di Tahap 3)"; then
    cp -p /etc/notehop/notehop.env /etc/notehop/notehop.env.bak-tahap0
    sed -i 's/^NOTEHOP_ADMINS=.*/NOTEHOP_ADMINS=/' /etc/notehop/notehop.env
    systemctl restart notehop && sleep 1 && curl -fsS http://127.0.0.1:8090/healthz >/dev/null && echo "   notehop OK"
  fi
else
  echo "2) NOTEHOP_ADMINS sudah kosong / tidak ada. Lewati."
fi

# ---------- 3. placeholder Imperium ----------
if ask "3) Pasang placeholder Imperium di 127.0.0.1:8100?"; then
  mkdir -p /var/lib/imperium-placeholder
  cat > /var/lib/imperium-placeholder/index.html <<'EOF'
<!doctype html><meta charset="utf-8"><title>Imperium</title>
<body style="font-family:system-ui;background:#0b0b10;color:#e8e8f0;display:grid;place-items:center;height:100vh;margin:0">
<div style="text-align:center"><h1 style="letter-spacing:.3em;font-weight:600">IMPERIUM</h1><p>HOP control center. Tahap 0: pintu sudah terpasang, isi menyusul.</p></div>
EOF
  chown -R www-data:www-data /var/lib/imperium-placeholder
  cp "$REPO/deploy/imperium-placeholder.service" /etc/systemd/system/imperium.service
  systemctl daemon-reload && systemctl enable --now imperium && sleep 1
  curl -fsS -o /dev/null http://127.0.0.1:8100/ && echo "   placeholder OK (port 8100)"
fi

# ---------- 4. pintu utama: Tailscale ----------
if ask "4) Siapkan imperium.hel.ink lewat Tailscale ($TS_IP): DNS, sertifikat, vhost?"; then
  CF=/root/.secrets/cloudflare.ini
  if [ ! -s "$CF" ]; then
    echo "   Butuh Cloudflare API Token dengan izin Zone:DNS:Edit untuk zona hel.ink."
    echo "   Buat di https://dash.cloudflare.com/profile/api-tokens (template 'Edit zone DNS')."
    read -r -s -p "   Tempel token: " TOKEN; echo
    mkdir -p /root/.secrets && printf 'dns_cloudflare_api_token = %s\n' "$TOKEN" > "$CF" && chmod 600 "$CF"
  fi
  TOKEN=$(sed -n 's/^dns_cloudflare_api_token *= *//p' "$CF")
  # IPv6 keluar di VPS ini tidak jalan (ada rute default, tapi koneksi tidak pernah sampai).
  # Python memilih IPv6 lebih dulu dan plugin certbot tidak punya timeout, jadi macet selamanya.
  # Utamakan IPv4 untuk semua program (glibc getaddrinfo); IPv6 tetap dipakai bila hanya itu yang ada.
  if ! grep -qE '^precedence ::ffff:0:0/96 +100' /etc/gai.conf 2>/dev/null; then
    cp -p /etc/gai.conf /etc/gai.conf.bak-tahap0 2>/dev/null || true
    printf '\n# tahap0: utamakan IPv4, IPv6 keluar rusak di VPS ini\nprecedence ::ffff:0:0/96  100\n' >> /etc/gai.conf
    echo "   /etc/gai.conf: IPv4 diutamakan"
  fi
  pkill -f 'certbot certonly' 2>/dev/null || true
  ZONE=$(curl -fsS -H "Authorization: Bearer $TOKEN" "https://api.cloudflare.com/client/v4/zones?name=hel.ink" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["result"][0]["id"])')
  EXIST=$(curl -fsS -H "Authorization: Bearer $TOKEN" "https://api.cloudflare.com/client/v4/zones/$ZONE/dns_records?name=imperium.hel.ink&type=A" | python3 -I -c 'import json,sys;r=json.load(sys.stdin)["result"];print(r[0]["id"] if r else "")')
  BODY=$(printf '{"type":"A","name":"imperium","content":"%s","ttl":300,"proxied":false,"comment":"Imperium, Tailscale only"}' "$TS_IP")
  if [ -n "$EXIST" ]; then
    curl -fsS -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' "https://api.cloudflare.com/client/v4/zones/$ZONE/dns_records/$EXIST" --data "$BODY" >/dev/null && echo "   DNS A imperium.hel.ink diperbarui -> $TS_IP"
  else
    curl -fsS -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' "https://api.cloudflare.com/client/v4/zones/$ZONE/dns_records" --data "$BODY" >/dev/null && echo "   DNS A imperium.hel.ink dibuat -> $TS_IP"
  fi
  apt-get install -y -qq python3-certbot-dns-cloudflare >/dev/null
  certbot certonly --non-interactive --agree-tos -m "ivan.smb787@gmail.com" \
    --dns-cloudflare --dns-cloudflare-credentials "$CF" --dns-cloudflare-propagation-seconds 30 \
    -d imperium.hel.ink && echo "   sertifikat OK"
  sed "s/100\.80\.122\.59/$TS_IP/g" "$REPO/deploy/nginx-imperium.hel.ink.conf" > /etc/nginx/sites-available/imperium.hel.ink
  ln -sf /etc/nginx/sites-available/imperium.hel.ink /etc/nginx/sites-enabled/imperium.hel.ink
  nginx -t && systemctl reload nginx && echo "   nginx OK. Uji dari perangkat Tailscale: https://imperium.hel.ink"
fi

# ---------- 5. pintu cadangan: Cloudflare Tunnel + Access ----------
if systemctl is-active --quiet cloudflared && [ ! -f /etc/cloudflared/config.yml ]; then
  echo "5) cloudflared sudah berjalan sebagai tunnel buatan dashboard (remotely managed). Lewati."
elif ask "5) Pasang cloudflared dan tunnel 'imperium' (dikelola lokal) untuk citadel.hel.ink?"; then
  if ! command -v cloudflared >/dev/null; then
    mkdir -p --mode=0755 /usr/share/keyrings
    curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
    echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared $(. /etc/os-release; echo "$VERSION_CODENAME") main" > /etc/apt/sources.list.d/cloudflared.list
    apt-get update -qq && apt-get install -y -qq cloudflared
  fi
  echo "   Login akan membuka URL; buka di browser dan pilih zona hel.ink."
  [ -f /root/.cloudflared/cert.pem ] || cloudflared tunnel login
  cloudflared tunnel list | grep -q ' imperium ' || cloudflared tunnel create imperium
  TID=$(cloudflared tunnel list -o json | python3 -I -c 'import json,sys;print([t["id"] for t in json.load(sys.stdin) if t["name"]=="imperium"][0])')
  mkdir -p /etc/cloudflared && cp "/root/.cloudflared/$TID.json" /etc/cloudflared/imperium.json
  cp "$REPO/deploy/cloudflared-config.yml" /etc/cloudflared/config.yml
  cloudflared tunnel route dns -f imperium citadel.hel.ink
  cloudflared service install 2>/dev/null || true
  systemctl enable --now cloudflared && systemctl restart cloudflared && echo "   tunnel OK"
  cat <<'EOF'
   LANGKAH MANUAL (sekali): Cloudflare Zero Trust -> Access -> Applications -> Add application
   -> Self-hosted -> domain citadel.hel.ink -> Policy "owner": Allow, Include: Emails = email Anda.
   Tanpa policy ini citadel.hel.ink terbuka untuk siapa pun yang tahu namanya.
EOF
fi

# ---------- 6. cron harian hop-backup (sebagai carawin) ----------
if ask "6) Jadwalkan hop-backup.sh tiap hari 03:45 di crontab carawin?"; then
  mkdir -p /home/carawin/backups/hop && chown carawin:carawin /home/carawin/backups/hop
  (sudo -u carawin crontab -l 2>/dev/null | grep -v hop-backup; echo "45 3 * * * /home/carawin/imperium/scripts/hop-backup.sh >> /home/carawin/backups/hop/cron.log 2>&1") | sudo -u carawin crontab -
  sudo -u carawin crontab -l | grep hop-backup && echo "   cron OK"
fi

say "Selesai. Uji backup lengkap: sudo -u carawin /home/carawin/imperium/scripts/hop-backup.sh"
