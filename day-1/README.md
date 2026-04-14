# Laporan Implementasi Aplikasi Chat AI

## 1. Deskripsi singkat service yang dibuat

Service yang dibuat adalah aplikasi chat AI full-stack dengan komponen:

- Frontend: Next.js (App Router)
- Backend: Golang REST API
- Database: PostgreSQL
- LLM runtime: Docker Model Runner (OpenAI-compatible endpoint)
- Reverse proxy publik: Nginx di VPS

Fitur utama aplikasi:

- Registrasi dan login user
- Autentikasi JWT untuk endpoint terproteksi
- Pembuatan chat baru per user
- Penyimpanan riwayat chat dan pesan (user + AI) ke PostgreSQL
- Integrasi model Hugging Face melalui Docker Model Runner

Konfigurasi LLM saat ini pada environment:

- `LLM_MODEL_NAME=hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K`
- `LLM_CTX_SIZE=4096`
- `LLM_BASE_URL=http://model-runner.docker.internal:12434/engines/v1`

Arsitektur akses publik saat ini:

- Akses user masuk ke Nginx (port 80/443)
- Nginx meneruskan `/` ke frontend (`127.0.0.1:3000`)
- Nginx meneruskan `/api/*` dan `/health` ke backend (`127.0.0.1:8080`)

## 2. Penjelasan endpoint /health

Endpoint health digunakan untuk memantau kesiapan backend beserta dependency utama.

Endpoint:

- `GET /health`

Perilaku endpoint:

- Melakukan `PingContext` ke PostgreSQL
- Melakukan health check LLM service (melalui Docker Model Runner)
- Tetap merespons HTTP 200 agar kompatibel dengan orchestrator/monitoring, dengan detail status pada body

Contoh respons normal:

```json
{
  "status": "ok",
  "services": {
    "database": "ok",
    "llm": "ok"
  }
}
```

Mode respons sederhana:

- `GET /health?simple=true`

Contoh:

```json
{
  "status": "ok"
}
```

## 3. Screenshot atau bukti endpoint dapat diakses

Bukti akses endpoint pada VPS publik:

- URL: `https://152.42.223.24/health`

Hasil verifikasi dari terminal:

```bash
$ curl -vk https://152.42.223.24/health
...
< HTTP/2 200
< server: nginx/1.24.0 (Ubuntu)
< content-type: application/json
...
{"services":{"database":"ok","llm":"ok"},"status":"ok"}
```

Tambahan verifikasi redirect HTTP ke HTTPS:

```bash
$ curl -i http://152.42.223.24/health
HTTP/1.1 301 Moved Permanently
Location: https://152.42.223.24/health
```

Catatan TLS saat akses via IP:

- Sertifikat saat ini self-signed, sehingga `curl` tanpa `-k` akan menampilkan error verifikasi SSL.

## 4. Penjelasan proses build dan run Docker

Langkah build dan run service aplikasi:

1. Siapkan environment file.

```bash
cp .env.example .env
```

2. Sesuaikan nilai penting pada `.env` (minimal `JWT_SECRET`, dan parameter LLM jika diperlukan).

3. Login Hugging Face (agar model dapat di-pull oleh Docker Model Runner).

```bash
hf auth login
```

4. Jalankan model di Docker Model Runner.

```bash
./scripts/docker-model-run.sh hf.co/bartowski/Llama-3.2-3B-Instruct-GGUF:Q6_K
```

5. Build dan jalankan stack utama (DB + backend + frontend).

```bash
docker compose --env-file .env up -d --build db backend frontend
```

Alternatif one-command bootstrap:

```bash
./scripts/up-with-dmr.sh
```

6. Verifikasi status container.

```bash
docker compose ps
```

## 5. Penjelasan proses deployment ke VPS

Deployment aktual untuk VPS IP `152.42.223.24`:

1. Provision server Ubuntu di DigitalOcean.
2. Install Docker Engine, Docker Compose plugin, dan Nginx.
3. Clone repository ke VPS.
4. Salin `.env.example` menjadi `.env`, lalu sesuaikan environment produksi.
5. Login Hugging Face dan jalankan model lewat Docker Model Runner.
6. Jalankan service aplikasi dengan Docker Compose (db, backend, frontend).
7. Terapkan konfigurasi host Nginx dari `deploy/nginx/day1-ncc.conf` ke `/etc/nginx/sites-available/`, aktifkan di `sites-enabled`, lalu reload Nginx.
8. Pastikan sertifikat SSL untuk IP tersedia di:
   - `/etc/ssl/certs/day1-ncc-ip.crt`
   - `/etc/ssl/private/day1-ncc-ip.key`
9. Buka firewall port `80/tcp` dan `443/tcp`.
10. Uji endpoint publik:

```bash
curl -k https://152.42.223.24/health
```

Ringkasan alur trafik:

- Port publik: 80 (redirect) dan 443 (HTTPS)
- Nginx sebagai single entrypoint
- Service aplikasi tetap di loopback host (`127.0.0.1`) untuk membatasi eksposur langsung

## 6. Kendala yang dihadapi (jika ada)

Kendala yang muncul selama implementasi/deploy:

- Inisialisasi model LLM membutuhkan resource RAM besar dan waktu warm-up lebih lama.
- Akses HTTPS berbasis IP menggunakan sertifikat self-signed, sehingga klien standar akan memberi peringatan SSL.

Mitigasi yang dilakukan:

- Menjalankan model melalui Docker Model Runner agar lifecycle model lebih stabil.
- Menambahkan healthcheck antar service agar startup lebih terkoordinasi.
- Menjalankan aplikasi di balik host Nginx dengan redirect HTTP ke HTTPS.
