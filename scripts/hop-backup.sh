#!/usr/bin/env bash
# hop-backup v0: satu arsip untuk seluruh keluarga HOP di VPS ini.
#
# Isi: dump MariaDB LinkHop, salinan SQLite PixelHop + gambar, salinan SQLite
# NoteHop (butuh sudo), env dan versi FileHop (butuh sudo), semua .env (dienkripsi), vhost nginx, unit systemd,
# crontab, daftar paket, dan MANIFEST.json (versi, commit, checksum).
# Opsional: snapshot restic ke R2 (memakai ~/.config/hop/backup.env).
#
# Pemakaian:  hop-backup.sh [--out DIR] [--no-images] [--no-remote]
# Deploy hop-backup-latest.py sebagai sibling skrip ini di checkout yang sama.
# Helper membutuhkan Python 3.10+; tidak ada dependency tambahan.
# Rahasia tidak pernah dicetak. Jalankan sebagai user carawin.
set -euo pipefail

OUT_ROOT="${HOP_BACKUP_DIR:-$HOME/backups/hop}"
WITH_IMAGES=1; WITH_REMOTE=1
while [ $# -gt 0 ]; do
  case "$1" in
    --out) OUT_ROOT="$2"; shift 2;;
    --no-images) WITH_IMAGES=0; shift;;
    --no-remote) WITH_REMOTE=0; shift;;
    *) echo "argumen tidak dikenal: $1"; exit 2;;
  esac
done

# Cek sudo credential cache bila tersedia
sudo -v 2>/dev/null || true

if [ -d "/var/www/helink/linkhop" ]; then
  HELINK=/var/www/helink/linkhop
elif [ -d "/var/www/helink/hub" ]; then
  HELINK=/var/www/helink/hub
else
  HELINK=/var/www/helink/shortlink-app
fi
PIXELHOP=/home/carawin/apps/pixelhop
NOTEHOP_DB=/var/lib/notehop/notehop.db
NOTEHOP_ENV=/etc/notehop/notehop.env
FILEHOP_ENV=/etc/filehop/filehop.env
PASS_FILE="$HOME/.config/hop/backup-passphrase"
KEEP_DAYS="${HOP_BACKUP_KEEP_DAYS:-14}"

STAMP=$(date +%Y%m%d-%H%M%S)
DEST="$OUT_ROOT/hop-$STAMP"
umask 077
mkdir -p "$DEST"/{linkhop,pixelhop,notehop,filehop,system,secrets}
LOG="$DEST/backup.log"
say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*" | tee -a "$LOG"; }
warn() { say "PERINGATAN: $*"; WARNINGS=$((WARNINGS+1)); }
WARNINGS=0
env_val() { grep -E "^$2=" "$1" 2>/dev/null | tail -1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//'; }

say "hop-backup v0 -> $DEST"

# ---------- LinkHop (hel.ink): MariaDB + storage publik ----------
if [ -f "$HELINK/.env" ]; then
  say "LinkHop: dump MariaDB"
  CNF=$(mktemp); trap 'rm -f "$CNF"' EXIT
  printf '[client]\nuser=%s\npassword=%s\nhost=%s\nport=%s\n' \
    "$(env_val "$HELINK/.env" DB_USERNAME)" "$(env_val "$HELINK/.env" DB_PASSWORD)" \
    "$(env_val "$HELINK/.env" DB_HOST)" "$(env_val "$HELINK/.env" DB_PORT)" > "$CNF"
  mysqldump --defaults-extra-file="$CNF" --single-transaction --quick --routines --triggers \
    --no-tablespaces "$(env_val "$HELINK/.env" DB_DATABASE)" | gzip > "$DEST/linkhop/db.sql.gz"
  say "LinkHop: storage/app/public"
  tar -czf "$DEST/linkhop/storage-public.tar.gz" -C "$HELINK/storage/app" public 2>/dev/null || warn "storage/app/public kosong"
  cp "$HELINK/.env" "$DEST/secrets/linkhop.env"
  git -C "$HELINK" rev-parse HEAD > "$DEST/linkhop/git-commit.txt" 2>/dev/null || true
else
  warn "LinkHop .env tidak ditemukan, dilewati"
fi

# ---------- PixelHop: SQLite + gambar ----------
if [ -f "$PIXELHOP/data/pixelhop.db" ]; then
  say "PixelHop: salinan SQLite (online backup)"
  python3 -I -c "import sqlite3,sys; sqlite3.connect(sys.argv[1]).backup(sqlite3.connect(sys.argv[2]))" \
    "$PIXELHOP/data/pixelhop.db" "$DEST/pixelhop/pixelhop.db"
  if [ "$WITH_IMAGES" = 1 ]; then
    say "PixelHop: gambar (tanpa varian cache)"
    tar -czf "$DEST/pixelhop/images.tar.gz" -C "$PIXELHOP/data" --exclude='images/variants' images
  fi
  cp "$PIXELHOP/pixelhop.env" "$DEST/secrets/pixelhop.env"
  cp "$PIXELHOP"/*.service "$DEST/system/" 2>/dev/null || true
  "$PIXELHOP/pixelhop" version > "$DEST/pixelhop/version.txt" 2>/dev/null || true
else
  warn "PixelHop DB tidak ditemukan, dilewati"
fi

# ---------- NoteHop: butuh sudo (user notehop memiliki /var/lib/notehop) ----------
# sudoers /etc/sudoers.d/hop-backup hanya mengizinkan perintah persis ini (tanpa password)
if sudo -n -u notehop /usr/local/bin/notehop backup -db "$NOTEHOP_DB" /tmp/hop-notehop-$STAMP.db 2>/dev/null; then
  say "NoteHop: salinan SQLite lewat notehop backup"
  sudo -n /usr/bin/mv /tmp/hop-notehop-$STAMP.db "$DEST/notehop/notehop.db" \
    && sudo -n /usr/bin/chown carawin "$DEST/notehop/notehop.db" || warn "NoteHop: memindahkan salinan gagal"
  sudo -n /usr/bin/cat "$NOTEHOP_ENV" > "$DEST/secrets/notehop.env" 2>/dev/null || warn "NoteHop env tidak terbaca"
  /usr/local/bin/notehop -version > "$DEST/notehop/version.txt" 2>&1 || true
else
  warn "NoteHop dilewati: sudoers belum terpasang (jalankan scripts/tahap0-sudo.sh langkah 1)"
fi

# ---------- FileHop: hanya env (rahasia SSO) dan versi ----------
# Database FileHop hanya berisi akun HOP ID yang pernah masuk dan sesi; file tidak pernah sampai
# ke server. Setelah restore, sesi lama memang sebaiknya hilang, jadi DB tidak dicadangkan.
if [ -f /usr/local/bin/filehop ]; then
  sudo -n /usr/bin/cat "$FILEHOP_ENV" > "$DEST/secrets/filehop.env" 2>/dev/null || { rm -f "$DEST/secrets/filehop.env"; warn "FileHop env tidak terbaca (jalankan scripts/tahap0-sudo.sh langkah 1)"; }
  /usr/local/bin/filehop -version > "$DEST/filehop/version.txt" 2>&1 || true
  say "FileHop: env dan versi"
fi

# ---------- Sistem ----------
say "Sistem: nginx, systemd, cron, paket"
cp -r /etc/nginx/sites-enabled "$DEST/system/nginx-sites-enabled" 2>/dev/null || warn "nginx sites tidak terbaca"
for u in notehop filehop pixelhop pixelhop-ai pixelhop-rembg linkhop-queue hub-queue biohop-queue imperium hop-agent notehop-backup.timer notehop-backup.service; do
  systemctl cat "$u" > "$DEST/system/$u.unit" 2>/dev/null || rm -f "$DEST/system/$u.unit"
done
crontab -l > "$DEST/system/crontab-$USER.txt" 2>/dev/null || true
dpkg --get-selections > "$DEST/system/dpkg-selections.txt" 2>/dev/null || true
{ php -v | head -1; node -v; mariadb --version 2>/dev/null || mysql --version; nginx -v 2>&1; /home/carawin/sdk/go/bin/go version 2>/dev/null; } > "$DEST/system/versions.txt" 2>&1 || true
cp /home/carawin/.config/pixelhop/backup.env "$DEST/secrets/pixelhop-restic.env" 2>/dev/null || true

# ---------- Rahasia: enkripsi ----------
if [ -s "$PASS_FILE" ]; then
  say "Rahasia: enkripsi dengan gpg (passphrase di $PASS_FILE)"
  tar -czf "$DEST/secrets.tar.gz" -C "$DEST" secrets
  gpg --batch --yes --symmetric --cipher-algo AES256 --passphrase-file "$PASS_FILE" -o "$DEST/secrets.tar.gz.gpg" "$DEST/secrets.tar.gz"
  rm -rf "$DEST/secrets" "$DEST/secrets.tar.gz"
else
  warn "Rahasia TIDAK dienkripsi: buat $PASS_FILE (chmod 600) berisi passphrase panjang, simpan salinannya di luar server"
fi

# ---------- Manifest ----------
say "MANIFEST.json"
python3 -I - "$DEST" <<'EOF'
import hashlib, json, os, sys, subprocess, datetime
d = sys.argv[1]; files = {}
for root, _, names in os.walk(d):
    for n in names:
        p = os.path.join(root, n)
        if n in ("MANIFEST.json", "backup.log"): continue
        h = hashlib.sha256()
        with open(p, "rb") as f:
            for chunk in iter(lambda: f.read(1 << 20), b""): h.update(chunk)
        files[os.path.relpath(p, d)] = {"sha256": h.hexdigest(), "bytes": os.path.getsize(p)}
def read(p):
    try: return open(os.path.join(d, p)).read().strip()
    except Exception: return None
m = {"format": "hop-backup/v0", "created": datetime.datetime.now(datetime.UTC).isoformat(),
     "host": os.uname().nodename,
     "apps": {"linkhop": {"commit": read("linkhop/git-commit.txt")},
              "pixelhop": {"version": read("pixelhop/version.txt")},
              "notehop": {"version": read("notehop/version.txt")},
              "filehop": {"version": read("filehop/version.txt")}},
     "files": files}
json.dump(m, open(os.path.join(d, "MANIFEST.json"), "w"), indent=1)
EOF

# ---------- Off-site (restic) ----------
REMOTE_ENV="$HOME/.config/hop/backup.env"
if [ "$WITH_REMOTE" = 1 ] && [ -f "$REMOTE_ENV" ]; then
  say "Off-site: restic"
  set -a; . "$REMOTE_ENV"; set +a
  R=${RESTIC_BIN:-/home/carawin/apps/pixelhop/bin/restic}
  "$R" snapshots >/dev/null 2>&1 || "$R" init
  "$R" backup --quiet --tag hop "$DEST"
  "$R" forget --quiet --tag hop --keep-daily 14 --keep-weekly 8 --keep-monthly 12 --prune
  if OFFSITE_SUMMARY=$("$R" snapshots --tag hop --json 2>/dev/null | python3 -I "$(dirname "$0")/hop-backup-latest.py"); then
    say "Off-site: $OFFSITE_SUMMARY"
  else
    warn "Off-site: ringkasan snapshot terbaru tidak tersedia"
  fi
elif [ "$WITH_REMOTE" = 1 ]; then
  warn "Off-site dilewati: buat $REMOTE_ENV (RESTIC_REPOSITORY, RESTIC_PASSWORD_FILE, AWS_*) seperti milik PixelHop tapi dengan path repo berbeda"
fi

# ---------- Retensi lokal ----------
find "$OUT_ROOT" -maxdepth 1 -name 'hop-2*' -type d -mtime +"$KEEP_DAYS" -exec rm -rf {} + 2>/dev/null || true

say "Selesai. Ukuran: $(du -sh "$DEST" | cut -f1). Peringatan: $WARNINGS"
[ "$WARNINGS" = 0 ] || exit 3
