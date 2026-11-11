#!/usr/bin/env bash
# hop-restore v0: memeriksa dan memulihkan arsip hop-backup.
#
#   hop-restore.sh verify <dir-arsip>          # cek checksum + integritas DB, tanpa menyentuh apa pun
#   hop-restore.sh apply  <dir-arsip>          # pulihkan ke server INI (butuh root); untuk migrasi ke server baru
#
# "apply" mengasumsikan paket dasar sudah terpasang (nginx, php8.3-fpm, mariadb,
# redis) dan aplikasi sudah di-clone/di-install di path yang sama dengan server
# lama. Langkah paket otomatis menyusul di versi berikutnya (lihat docs/02-PLANNING.md).
set -euo pipefail
MODE="${1:-}"; SRC="${2:-}"
[ -n "$MODE" ] && [ -d "$SRC" ] || { sed -n 2,8p "$0"; exit 2; }
PASS_FILE="${HOP_PASSPHRASE_FILE:-$HOME/.config/hop/backup-passphrase}"
say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { say "GAGAL: $*"; FAILS=$((FAILS+1)); }
FAILS=0

say "Arsip: $SRC"
[ -f "$SRC/MANIFEST.json" ] || { say "MANIFEST.json tidak ada"; exit 1; }

# ---------- 1. checksum ----------
say "Checksum semua file"
python3 -I - "$SRC" <<'EOF' || FAILS=$((FAILS+1))
import hashlib, json, os, sys
d = sys.argv[1]; m = json.load(open(os.path.join(d, "MANIFEST.json"))); bad = 0
for rel, info in m["files"].items():
    p = os.path.join(d, rel)
    if not os.path.exists(p): print("  hilang:", rel); bad += 1; continue
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""): h.update(chunk)
    if h.hexdigest() != info["sha256"]: print("  rusak:", rel); bad += 1
print(f"  {len(m['files'])} file, {bad} bermasalah"); sys.exit(1 if bad else 0)
EOF

# ---------- 2. integritas isi ----------
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
if [ -f "$SRC/linkhop/db.sql.gz" ]; then
  say "LinkHop: gzip + struktur dump"
  gunzip -t "$SRC/linkhop/db.sql.gz" || fail "db.sql.gz rusak"
  say "  $(gunzip -c "$SRC/linkhop/db.sql.gz" | grep -c '^CREATE TABLE') tabel di dump"
  [ -f "$SRC/linkhop/storage-public.tar.gz" ] && { tar -tzf "$SRC/linkhop/storage-public.tar.gz" >/dev/null || fail "storage-public.tar.gz rusak"; }
fi
for app in pixelhop notehop filehop payhop; do
  if [ -f "$SRC/$app/$app.db" ]; then
    say "$app: PRAGMA integrity_check"
    python3 -I -c "import sqlite3,sys; c=sqlite3.connect('file:'+sys.argv[1]+'?mode=ro',uri=True); r=c.execute('pragma integrity_check').fetchone()[0]; print('  ',r); sys.exit(0 if r=='ok' else 1)" "$SRC/$app/$app.db" || fail "$app.db tidak sehat"
  fi
done
[ -f "$SRC/pixelhop/images.tar.gz" ] && say "  PixelHop: $(tar -tzf "$SRC/pixelhop/images.tar.gz" | wc -l) entri gambar"

# ---------- 3. rahasia ----------
if [ -f "$SRC/secrets.tar.gz.gpg" ]; then
  if [ -s "$PASS_FILE" ]; then
    say "Rahasia: dekripsi uji"
    gpg --batch --quiet --passphrase-file "$PASS_FILE" -d "$SRC/secrets.tar.gz.gpg" > "$TMP/secrets.tar.gz" || fail "dekripsi rahasia gagal (passphrase salah?)"
    mkdir -p "$TMP/secrets" && tar -xzf "$TMP/secrets.tar.gz" -C "$TMP" && ls "$TMP/secrets" | sed 's/^/  /'
  else
    fail "secrets.tar.gz.gpg ada tapi passphrase tidak ditemukan di $PASS_FILE"
  fi
elif [ -d "$SRC/secrets" ]; then
  say "Rahasia: TIDAK terenkripsi di arsip ini"; cp -r "$SRC/secrets" "$TMP/secrets"
fi

say "Verifikasi selesai, masalah: $FAILS"
[ "$MODE" = verify ] && exit $((FAILS > 0 ? 1 : 0))
[ "$FAILS" = 0 ] || { say "Tidak melanjutkan apply karena ada masalah"; exit 1; }

# ---------- 4. apply (root) ----------
[ "$MODE" = apply ] || exit 2
[ "$(id -u)" = 0 ] || { say "apply butuh root: sudo -E $0 apply $SRC"; exit 1; }
if [ -d "/var/www/helink/linkhop" ]; then
  HELINK=/var/www/helink/linkhop
elif [ -d "/var/www/helink/hub" ]; then
  HELINK=/var/www/helink/hub
else
  HELINK=/var/www/helink/shortlink-app
fi; PIXELHOP=/home/carawin/apps/pixelhop
read -r -p "Ini akan MENIMPA database dan file di server ini. Ketik 'pulihkan' untuk lanjut: " ans
[ "$ans" = pulihkan ] || exit 1

say "Hentikan layanan"
systemctl stop pixelhop notehop filehop payhop linkhop-queue hub-queue biohop-queue queue-worker 2>/dev/null || true

if [ -f "$SRC/linkhop/db.sql.gz" ] && [ -f "$TMP/secrets/linkhop.env" ]; then
  say "LinkHop: env, DB, storage"
  install -o www-data -g www-data -m 640 "$TMP/secrets/linkhop.env" "$HELINK/.env"
  ev() { grep -E "^$1=" "$HELINK/.env" | tail -1 | cut -d= -f2- | sed -e 's/^"//' -e 's/"$//'; }
  mysql -e "CREATE DATABASE IF NOT EXISTS \`$(ev DB_DATABASE)\` CHARACTER SET utf8mb4;
            CREATE USER IF NOT EXISTS '$(ev DB_USERNAME)'@'localhost' IDENTIFIED BY '$(ev DB_PASSWORD)';
            GRANT ALL ON \`$(ev DB_DATABASE)\`.* TO '$(ev DB_USERNAME)'@'localhost'; FLUSH PRIVILEGES;"
  gunzip -c "$SRC/linkhop/db.sql.gz" | mysql "$(ev DB_DATABASE)"
  [ -f "$SRC/linkhop/storage-public.tar.gz" ] && tar -xzf "$SRC/linkhop/storage-public.tar.gz" -C "$HELINK/storage/app"
  chown -R www-data:www-data "$HELINK/storage" "$HELINK/bootstrap/cache"
  sudo -u www-data php "$HELINK/artisan" config:cache >/dev/null && sudo -u www-data php "$HELINK/artisan" storage:link >/dev/null || true
fi

if [ -f "$SRC/pixelhop/pixelhop.db" ]; then
  say "PixelHop: env, DB, gambar"
  mkdir -p "$PIXELHOP/data"
  install -o carawin -g carawin -m 600 "$TMP/secrets/pixelhop.env" "$PIXELHOP/pixelhop.env"
  install -o carawin -g carawin -m 644 "$SRC/pixelhop/pixelhop.db" "$PIXELHOP/data/pixelhop.db"
  rm -f "$PIXELHOP/data/pixelhop.db-wal" "$PIXELHOP/data/pixelhop.db-shm"
  [ -f "$SRC/pixelhop/images.tar.gz" ] && tar -xzf "$SRC/pixelhop/images.tar.gz" -C "$PIXELHOP/data" && chown -R carawin:carawin "$PIXELHOP/data"
  for u in "$SRC"/system/pixelhop*.service; do [ -f "$u" ] && cp "$u" /etc/systemd/system/; done
fi

if [ -f "$SRC/notehop/notehop.db" ]; then
  say "NoteHop: env, DB"
  id notehop >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin -d /var/lib/notehop notehop
  mkdir -p /var/lib/notehop /etc/notehop
  install -o root -g notehop -m 640 "$TMP/secrets/notehop.env" /etc/notehop/notehop.env
  install -o notehop -g notehop -m 640 "$SRC/notehop/notehop.db" /var/lib/notehop/notehop.db
  rm -f /var/lib/notehop/notehop.db-wal /var/lib/notehop/notehop.db-shm
  chown notehop:notehop /var/lib/notehop && chmod 750 /var/lib/notehop
  [ -f "$SRC/system/notehop.unit" ] && sed -n '/^# \/etc\/systemd/,$p' "$SRC/system/notehop.unit" | sed 1d > /etc/systemd/system/notehop.service
fi

# FileHop dan PayHop: env, unit, dan database (arsip lama tanpa DB: database mulai kosong, hanya sesi hilang).
# Untuk PayHop database berisi data yang dienkripsi dengan PAYHOP_PROFILE_KEY; env hasil restore membawa kunci itu.
restore_service() {  # slug
  local slug=$1
  [ -f "$TMP/secrets/$slug.env" ] || return 0
  say "$slug: env, unit, database"
  id "$slug" >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin -d "/var/lib/$slug" "$slug"
  mkdir -p "/etc/$slug" "/var/lib/$slug"
  install -o root -g "$slug" -m 640 "$TMP/secrets/$slug.env" "/etc/$slug/$slug.env"
  if [ -f "$SRC/$slug/$slug.db" ]; then
    install -o "$slug" -g "$slug" -m 640 "$SRC/$slug/$slug.db" "/var/lib/$slug/$slug.db"
    rm -f "/var/lib/$slug/$slug.db-wal" "/var/lib/$slug/$slug.db-shm"
  else
    say "  $slug: arsip tanpa database; mulai dari database kosong"
  fi
  chown "$slug:$slug" "/var/lib/$slug" && chmod 750 "/var/lib/$slug"
  [ -f "$SRC/system/$slug.unit" ] && sed -n '/^# \/etc\/systemd/,$p' "$SRC/system/$slug.unit" | sed 1d > "/etc/systemd/system/$slug.service"
  [ -x "/usr/local/bin/$slug" ] || say "  $slug: biner /usr/local/bin/$slug belum ada; bangun dari repo (deploy/install.sh)"
}
restore_service filehop
restore_service payhop

say "nginx + cron"
cp -n "$SRC"/system/nginx-sites-enabled/* /etc/nginx/sites-available/ 2>/dev/null || true
for f in "$SRC"/system/nginx-sites-enabled/*; do ln -sf "/etc/nginx/sites-available/$(basename "$f")" /etc/nginx/sites-enabled/; done
nginx -t && systemctl reload nginx
[ -f "$SRC/system/crontab-carawin.txt" ] && crontab -u carawin "$SRC/system/crontab-carawin.txt"

systemctl daemon-reload
systemctl start pixelhop notehop filehop payhop linkhop-queue hub-queue biohop-queue queue-worker 2>/dev/null || true
say "Healthcheck"
for u in "https://hel.ink/health/queue" "http://127.0.0.1:8080/api/v1/stats" "http://127.0.0.1:8090/healthz"; do
  printf '  %s -> ' "$u"; curl -s -o /dev/null -w '%{http_code}\n' -m 10 "$u" || echo "gagal"
done
say "Selesai. Ubah DNS (hel.ink, p.hel.ink, note.hel.ink) ke server ini bila migrasi, lalu pasang sertifikat Cloudflare origin."
