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
| Profil `dr. Andi`, `dr_andi@medvault.id` | Kartu profil di sidebar | Data diambil dari `session`; saat sesi belum siap tampil `Memuat sesi...`. |
| Kartu `USB Dongle terhubung`, `Key ID 7F3A` | Kartu dongle di sidebar | Data diambil dari `session.dongle_key_id`; tidak ada Key ID contoh yang ditulis mati. |
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
| Modul / Telemetry | `9:501` | `/telemetry` | `web/pages/telemetry.html` | `telemetry`, `telemetrySource`, `telemetryAt`, `telemetryPretty` | Tidak ada. Merender trace terakhir dari `sessionStorage`. |
| Modul / Keamanan | `9:664` | `/security` | `web/pages/security.html` | `session`, `sessionInfo`, `health` | `GET /healthz`, `GET /api/v1/auth/me` |
| Motion & Interaction Spec | `3:2` | tidak ada | `web/assets/tokens.css`, `web/layout.html` | Tidak ada data | Tidak ada. Menjadi durasi, easing, dan aturan reduced motion. |

Pemetaan route ini sudah sesuai dengan `pages` di `web/embed.go`, sehingga tidak perlu menambah route baru.

## Bagian per halaman

### Dashboard
- Eyebrow `Clinical workspace`, judul `Selamat datang` diikuti nama sesi bila tersedia
- Badge `API terhubung`, tombol `Pasien baru`
- Empat stat card: Pasien terdaftar, Blok audit, Validasi ledger, Sesi aktif
- Kartu `Akses cepat modul` dengan segmented `Semua / Kriptografi / Audit` dan lima baris modul
- Panel kanan `Status Ledger` dengan blok nyata dari ledger, badge status, segmented `Rantai valid / Simulasi tamper`, tombol `Verifikasi rantai`

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
- Catatan bahwa daftar rekam belum tersedia lewat API dan inspeksi memakai ID rekam

### Brankas File
- Form unggah dengan ID rekam medis dan dropzone
- Batas 50 MB dan penjelasan potongan 64 KiB, tanpa progres palsu
- Panel `Hasil enkripsi` dengan ciphertext, SHA-256, header, fingerprint kunci dari `vaultInfo`
- Catatan bahwa daftar berkas per rekam belum tersedia lewat API

### Steganografi
- Panel sisipkan dengan ID rekam medis dan metrik PSNR, MSE, kapasitas, tag HMAC dari `stegoResult`
- Panel ekstraksi dengan dropzone, state sukses, dan pesan gagal dari `toast`
- Payload ringkas berisi nomor RM, pasien, dokter, ringkasan, waktu

### Audit Ledger
- Panel status rantai dan tiga metrik dari `validation` dan `blocks`
- Panel simulasi manipulasi dengan tombol merah dan tombol pulihkan
- Rantai visual dari blok ledger yang dimuat
- Tabel blok dengan enam kolom dan empty state

### Telemetry
- Badge sumber permintaan, modul, durasi, dan waktu dari trace nyata
- Panel JSON `_telemetry`
- Ringkasan `module`, `duration_us`, jumlah langkah, metrik, dan peringatan
- Tabel langkah trace dari `steps`

### Keamanan
- Empat gerbang autentikasi dengan status dari `session`
- Panel `Kondisi layanan` dari payload `/healthz`
- Panel `Sesi aktif` dengan waktu dari `sessionInfo` dan tombol keluar

## Aset

Sesuai arahan terbaru, tidak ada ekspor ikon dari Figma. Satu-satunya aset branding yang dipakai adalah `web/assets/maskot.svg`, yang sudah dirujuk di `web/layout.html` dan ikut ter-embed. Tombol dan ikon lain memakai teks atau karakter CSS yang sudah ada.

## Catatan interpretasi

1. Figma Dashboard menulis `Valid`, tetapi aplikasi menampilkan `Belum diperiksa` sampai respons validasi ledger benar-benar diterima. Kejujuran state lebih penting daripada kecocokan teks.
2. Nama blok ledger di Figma adalah `EMBED_STEGO`, bukan `EVENT_STEGO`.
3. Jumlah validator aktif di Figma adalah `3`. Halaman audit tidak menampilkan angka itu karena API belum menyediakan daftar validator; yang ditampilkan adalah jumlah blok yang dimuat.
4. Threshold dHash di Figma tertulis `minimal 50%`. Deployment memakai `MEDVAULT_DHASH_MAX_DISTANCE=32` dari 64 bit, yang setara 50 persen.

## Validasi

Pemetaan ini tidak menambah route atau mengubah handler. Jalankan `go test ./web/...` untuk memastikan `embed.FS`, daftar route, dan versi aset masih valid.
