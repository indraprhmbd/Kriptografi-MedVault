# 0003 - Pemetaan Frame Figma ke Halaman Web

Tanggal: 19 Nov 2026
Status: Active
Sumber: file Figma `MedVault - Redesign Dashboard UI`, file key `Wt4wwa2QJOIPVWbhesTjgX`.

## Tujuan

Menentukan frame Figma mana yang menjadi halaman mana, bagian apa saja yang harus ada di tiap fragmen `web/pages/`, data apa yang dipakai, dan endpoint API mana yang dipanggil. Dokumen ini tidak mengubah perilaku aplikasi.

## Shell bersama

Elemen yang berulang di delapan frame modul dipetakan ke `web/layout.html`.

| Elemen Figma | Lokasi saat ini | Catatan |
| :--- | :--- | :--- |
| Brand `MV`, `MedVault`, `Secure Medical Records` | Sidebar `layout.html` | Logo sudah memakai `/assets/maskot.svg`. Tidak ada ikon lain yang perlu diekspor. |
| Sidebar delapan nav item | Sidebar kiri `layout.html` | Delapan item navigasi memakai ikon SVG lokal. Status aktif diambil dari `route`. |
| Profil `dr. Andi`, `dr_andi@medvault.id` | Kartu profil di sidebar | Data diambil dari `session`. |
| Kartu `USB Dongle terhubung`, `Key ID 7F3A` | Kartu dongle di sidebar | Data diambil dari `dongle` di `app.js`. Teks Figma memakai `·`, bukan tanda hubung. |
| Badge `Glass-Box` | Topbar dan drawer telemetry | Membuka Glass-Box Inspector yang sudah ada. |
| Tombol `Keluar` | Tombol `logout()` di sidebar | Sudah ada. |
| Footer `MedVault Go - demo ...` | Footer `layout.html` | Teks sudah sama dengan Figma. |
| Maskot melayang 8 px, loop 6000 ms | `.mv-mascot`, `@keyframes mv-float` | Sudah ada dan sudah mengikuti `prefers-reduced-motion`. |

## Pemetaan frame

| Frame Figma | ID | Route | File fragmen | Data utama | Endpoint |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Dashboard / Desktop 1440 | `2:2` | `/dashboard` | `web/pages/dashboard.html` | `patients`, `blocks`, `validation`, `session`, `health` | `GET /api/v1/patients`, `GET /api/v1/audit/blocks` |
| Modul / Pasien | `6:2` | `/patients` | `web/pages/patients.html` | `patients`, `patientForm`, `patientView` | `POST /api/v1/patients`, `GET /api/v1/patients?view=` |
| Modul / Rekam Medis | `6:238` | `/records` | `web/pages/records.html` | `recordForm`, `inspectId`, `inspection`, `pending` | `POST /api/v1/records`, `GET /api/v1/records/{id}/inspect` |
| Modul / Brankas File | `6:483` | `/vault` | `web/pages/vault.html` | `vaultRecordId`, `vaultFile`, `vaultInfo` | `POST /api/v1/vault/{record_id}/upload`, `GET /api/v1/vault/{record_id}/download` |
| Modul / Steganografi | `9:2` | `/steganography` | `web/pages/steganography.html` | `stegoRecordId`, `stegoResult`, `extracted` | `POST /api/v1/stego/{record_id}/embed`, `POST /api/v1/stego/extract` |
| Modul / Audit Ledger | `9:208` | `/audit` | `web/pages/audit.html` | `blocks`, `validation`, `tamperRecordId` | `GET /api/v1/audit/blocks`, `POST /api/v1/audit/simulate-tamper`, `POST /api/v1/audit/restore-tamper` |
| Modul / Telemetry | `9:501` | `/telemetry` | `web/pages/telemetry.html` | `telemetry`, `telemetryPretty` | Tidak ada. Hanya merender respons terakhir. |
| Modul / Keamanan | `9:664` | `/security` | `web/pages/security.html` | `session`, `dongle`, `health` | `GET /healthz`, endpoint status sesi yang sudah ada |
| Motion & Interaction Spec | `3:2` | tidak ada | `web/assets/tokens.css`, `web/layout.html` | Tidak ada data | Tidak ada. Menjadi durasi, easing, dan aturan reduced motion. |

Pemetaan route ini sudah sesuai dengan `pages` di `web/embed.go`, sehingga tidak perlu menambah route baru.

## Bagian per halaman

### Dashboard
- Eyebrow `Clinical workspace`, judul `Selamat datang, dr. Andi`, subjudul ringkasan
- Badge `API terhubung`, tombol `Pasien baru`
- Empat stat card: Pasien terdaftar, Blok audit, Validasi ledger, Sesi aktif
- Kartu `Akses cepat modul` dengan segmented `Semua / Kriptografi / Audit` dan lima baris modul
- Panel kanan `Status Ledger` dengan tiga blok, badge `Valid`, segmented `Rantai valid / Simulasi tamper`, tombol `Verifikasi rantai`

### Pasien
- Toggle `Plaintext / Raw SQLite`
- Form `Registrasi Pasien` dengan lima input
- Catatan AAD nama kolom
- Tabel `Daftar Pasien` dengan lima kolom
- Panel raw SQLite di bawah tabel

### Rekam Medis
- Form `Rekam Medis Baru` dengan pasien, diagnosis, resep, status kunjungan
- Panel `Inspeksi Rekam Medis` dengan empat tahap bernomor
- Blok proposal menunggu tanda tangan dan tombol `Tanda tangani`
- Tabel `Rekam terbaru`

### Brankas File
- Form unggah dengan ID rekam medis dan dropzone
- Progres potongan dan batas 50 MB
- Panel `Hasil enkripsi` dengan ciphertext, SHA-256, header, fingerprint kunci
- Panel `Catatan Brankas` dengan aksi unduh

### Steganografi
- Panel sisipkan dengan ID rekam medis dan metrik PSNR, MSE, kapasitas, tag HMAC
- Panel ekstraksi dengan dropzone, state sukses, dan state gagal
- Payload ringkas berisi nomor RM, pasien, dokter, ringkasan, waktu

### Audit Ledger
- Panel status rantai dan tiga metrik
- Panel simulasi manipulasi dengan tombol merah dan tombol pulihkan
- Rantai visual empat blok
- Tabel blok dengan enam kolom

### Telemetry
- Badge `POST /api/records`, `200 OK`, durasi
- Panel JSON `_telemetry`
- Panel `Waktu algoritma` dengan empat baris durasi

### Keamanan
- Empat gerbang autentikasi dengan status masing-masing
- Panel `Kondisi layanan` dengan empat layanan
- Panel `Sesi aktif` dengan tombol keluar

## Aset

Sesuai arahan terbaru, tidak ada ekspor ikon dari Figma. Satu-satunya aset branding yang dipakai adalah `web/assets/maskot.svg`, yang sudah dirujuk di `web/layout.html` dan ikut ter-embed. Tombol dan ikon lain memakai teks atau karakter CSS yang sudah ada.

## Catatan interpretasi

1. Figma Dashboard menulis `Valid`, sementara screenshot lama menulis `Belum diperiksa`. Teks Figma yang dipakai sebagai acuan.
2. Nama blok ledger di Figma adalah `EMBED_STEGO`, bukan `EVENT_STEGO`.
3. Jumlah validator aktif di Figma adalah `3`.
4. Threshold dHash di Figma tertulis `minimal 50%`. Ini berbeda dengan `MEDVAULT_DHASH_MAX_DISTANCE=14` di context/000 keduanya bisa benar karena satu persentase kemiripan dan satu jarak Hamming maksimum, tetapi perlu dicatat supaya tidak tertukar saat implementasi.

## Validasi

Pemetaan ini tidak menambah route atau mengubah handler. Jalankan `go test ./web/...` untuk memastikan `embed.FS`, daftar route, dan versi aset masih valid.
