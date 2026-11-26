# 005 - Presentation Context and Slide Deck Blueprint

**Document ID:** 005  
**Date:** 26 November 2026  
**Status:** Approved for Handover  
**Scope:** Presentation slide content briefing, system architecture handover, end-to-end operational flow, and 10-minute live demonstration script for MedVault Go.

---

## 1. Executive Summary and Metadata

### 1.1 Project Title
**MedVault Go:** Sistem Rekam Medis Elektronik dengan Autentikasi Multi-Faktor, Enkripsi Multilapis, Steganografi LSB Terautentikasi, dan Audit Ledger Proof-of-Authority dengan Tanda Tangan Ganda Ed25519.

### 1.2 Development and Presentation Team
* **Arsyadi Indra Hasan P** (NIM: 123240062)
* **Hafizd S. Abdurrahman** (NIM: 123240167)
* **Rakha Albany Nugraha** (NIM: 123240214)
* **Institution:** Program Studi Informatika, Jurusan Informatika, Fakultas Teknik Industri, Universitas Pembangunan Nasional "Veteran" Yogyakarta.
* **Course:** Kriptografi (Kelas IF-D, Semester Ganjil 2026).

---

## 2. Core Problem and Motivation

1. **Vulnerabilitas Electronic Health Record (EHR):**
   * Data rekam medis adalah target bernilai tinggi karena memadukan data identitas kependudukan (Personally Identifiable Information / PII), riwayat klinis penyakit, dan hasil pemeriksaan laboratorium penunjang.
   * Kebocoran database umum terjadi karena data disimpan dalam bentuk plaintext atau menggunakan skema enkripsi tunggal statis (single point of failure).
2. **Ancaman Orang Dalam dan Manipulasi Database (Database Tampering):**
   * Administrator database atau penyerang dengan hak akses root dapat mengubah diagnosis atau resep langsung melalui SQL query tanpa meninggalkan jejak otentik.
3. **Keterbatasan Sumber Daya Fasilitas Kesehatan:**
   * Sistem keamanan klinis sering kali terlalu berat untuk infrastruktur lokal puskesmas atau klinik kecil. MedVault Go membuktikan bahwa sistem keamanan militer-grade dan audit trail nir-ubah dapat beroperasi secara cepat pada server berdaya rendah (VPS 2 vCPU, 1.9 GiB RAM, alokasi memori < 128 MB).

---

## 3. Mengapa Rewrite: Legacy (Python + Electron) vs MedVault Go (Golang + Web)

Sebelumnya, proyek ini diawali sebagai prototipe desktop berbasis **Legacy Stack** (Python 3.11 + FastAPI + Electron + OpenCV + Pytest). Keputusan arsitektur untuk melakukan **rewrite total ke Golang dan platform Website murni** diambil berdasarkan pertimbangan teknis, keamanan, efisiensi sumber daya, dan kemudahan operasional klinis:

| Aspek Komparasi | Legacy MedVault (Python + Electron Desktop) | MedVault Go (Golang + Core Website MPA) | Dampak Teknis & Operasional di Lapangan |
| :--- | :--- | :--- | :--- |
| **Konsumsi Memori (RAM Footprint)** | Berat: Electron (~300-450 MB RAM per workstation) + FastAPI/Uvicorn (~180-250 MB). Total > 600 MB. | Sangat Ringan: Container app dibatasi 96 MB, validator 32 MB. Idle RAM server < 40 MB. | Mencegah Out-of-Memory (OOM) pada VPS low-end (1.9 GiB RAM). Menghemat 80% alokasi RAM. |
| **Dependensi Runtime & Native CGO** | Rapuh: Bergantung pada OpenCV (C++ wheels), runtime Python, Node.js, dan compiler C untuk ekstensi C-crypto. | Bersih: 100% Pure Go (`CGO_ENABLED=0`), driver SQLite pure-Go (`modernc.org/sqlite`). | Biner statis tunggal tanpa dependensi dinamis shared-object (`.so` / `.dll`), anti-drift versi lingkungan. |
| **Model Distribusi & Instalasi Klien** | Sulit: Dokter harus mengunduh dan memasang installer desktop Electron (> 150 MB) di tiap komputer klinik. | Zero-Install: Dokter cukup membuka browser modern (Chrome/Edge) melalui domain aman Cloudflare Tunnel. | Mobilitas penuh bagi dokter antar-ruangan/faskes tanpa konfigurasi instalasi OS atau driver tambahan. |
| **Akses Hardware & Kriptografi Klien** | Terikat Node.js desktop: IPC Electron rentan celah keamanan remote execution bila webview bocor. | Modern Web API: Browser File System Access API (USB dongle), WebCrypto (Ed25519 signing), dan WebAuthn. | Kunci privat dongle `.medkey` tidak pernah disentuh server; penandatanganan tantangan terjadi lokal di browser. |
| **Keamanan Rantai Pasok (Supply Chain)** | Berisiko: Membutuhkan ratusan node_modules di Electron dan puluhan wheel package pip. | Ketat & Zero-CDN: Seluruh aset frontend (Tailwind v4 & Alpine v3) dibundel ke biner Go via `embed.FS`. | Bebas dependensi CDN pihak ketiga, bekerja stabil saat koneksi lambat, dan CSP ketat (`default-src 'self'`). |
| **Performa & Throughput Kriptografi** | Terbatas oleh Python GIL (Global Interpreter Lock); lambat pada hashing berulang dan streaming besar. | Super Cepat: Eksekusi konkurensi native Go (goroutines), optimasi compiler 64-bit untuk SIMD / ChaCha20. | Enkripsi file lab 5 MB selesai dalam ~12 ms; verifikasi 100 blok blockchain PoA tuntas dalam ~4 ms. |
| **Isolasi Validator Proof-of-Authority** | Semu: Kunci validator sering kali berada dalam satu proses monolitik aplikasi Python yang sama. | Isolasi Fisik Nyata: Dua biner independen (`medvault-app` dan `medvault-validator`) di port berbeda (8095 & 8096). | Pemisahan wewenang nyata: aplikasi utama tidak memiliki akses fisik terhadap kunci privat validator. |

---

## 4. High-Level Architecture and Deployment Strategy

### 3.1 Pure Go Static Binary (`CGO_ENABLED=0`)
* **Zero CGO Dependency:** Dikompilasi tanpa runtime C/C++ (`CGO_ENABLED=0`), menghasilkan binary statis Linux amd64 mandiri (`medvault-app` ~14 MiB, `medvault-validator` ~6 MiB).
* **Pure Go SQLite:** Menggunakan engine `modernc.org/sqlite` dalam mode WAL (Write-Ahead Logging) untuk transaksi konkuren yang cepat dan bebas deadlock.
* **Single-Binary Web Asset Embedding:** Seluruh antarmuka web (HTML5, Tailwind CSS v4, Alpine.js v3) dibundel ke dalam binary Go menggunakan `embed.FS`. Sistem memiliki zero-CDN dependency, kebal terhadap pemadaman CDN eksternal, dan menerapkan Content Security Policy (CSP) ketat (`default-src 'self'`).

### 3.2 Dual-Service Podman Pod Topology
Aplikasi dideploy ke VPS Ubuntu 24.04 LTS dalam satu Podman Pod (`medvault-pod`) yang membatasi memori fisik secara ketat:
1. **Container 1: `medvault-validator` (Port 8096):**
   * Memori dibatasi 32 MB RAM (`--memory=32m`).
   * Layanan independen yang memegang kunci privat institusi validator. Layanan ini hanya mengekspos endpoint internal untuk evaluasi konsensus Proof-of-Authority dan pembubuhan countersignature Ed25519. Kunci validator tidak pernah dapat diakses oleh proses aplikasi utama.
2. **Container 2: `medvault-app` (Port 8095):**
   * Memori dibatasi 96 MB RAM (`--memory=96m`).
   * Melayani HTTP REST API, session management, logika kriptografi klinis, steganografi LSB, dan antarmuka web visual.
3. **Cloudflare Tunnel (`cloudflared`):**
   * Menghubungkan port loopback lokal `127.0.0.1:8095` ke domain publik (`medvault.indraprhmbd.my.id`) secara terenkripsi TLS end-to-end tanpa membuka port inbound publik apa pun pada firewall VPS.

---

## 5. The 6 Cryptographic Pillars (Defense-in-Depth)

### Pilar 1: Autentikasi Multi-Faktor (MFA 3-Faktor)
* **Faktor 1 (Pengetahuan):** Kata sandi kuat di-hash menggunakan **Argon2id** (RFC 9106) dengan parameter memori 16 MB, 2 iterasi, paralelisme 2, dan salt acak 16 byte per pengguna. Verifikasi hash menggunakan perbandingan waktu-konstan (`crypto/subtle.ConstantTimeCompare`).
* **Faktor 2 (Kepemilikan):** USB Hardware Dongle (`.medkey`) membawa pasangan kunci **Ed25519** (RFC 8032). Saat login, peramban meminta tantangan acak 32-byte sekali pakai yang berlaku 60 detik (`/api/v1/auth/challenge`), menandatangani tantangan secara lokal menggunakan WebCrypto di memori peramban, dan mengirimkan signature hex ke server. Kunci privat tidak pernah dikirim ke jaringan.
* **Faktor 3 (Biometrik):** Pengenalan wajah webcam via **64-bit difference hash (dHash)**. Citra webcam diubah menjadi kisi luminance 9x8 di sisi klien, di-hash menjadi 64-bit (16 heksadesimal), dan diverifikasi di server menggunakan jarak Hamming (Hamming distance <= 32 bit / kemiripan >= 50%). Foto mentah tidak pernah dikirim ke jaringan.
* **On-Demand Hardware Gate:** Mengadopsi prinsip token keamanan fisik standar industri (YubiKey), di mana kunci USB dongle dimuat ke memori browser untuk menandatangani tantangan login dan otorisasi rekam medis secara on-demand, dan langsung musnah dari memori saat tab ditutup.

### Pilar 2: Enkripsi Kolom PII Pasien (AES-256-GCM + AAD Binding)
* **Kerahasiaan & Integritas:** Mengamankan data demografi pasien (NIK, nama lengkap, alamat) di tabel `patients` menggunakan **AES-256-GCM** (NIST SP 800-38D).
* **Pengikatan AAD (Additional Authenticated Data):** Nama kolom database lengkap diikat sebagai AAD:
  * Kolom NIK: `"patients.nik"`
  * Kolom Nama Lengkap: `"patients.full_name"`
  * Kolom Alamat: `"patients.address"`
* **Dampak Keamanan:** Jika penyerang menyalin ciphertext dari kolom alamat ke kolom NIK, tag otentikasi GCM 16-byte akan menolak operasi pembacaan karena AAD tidak cocok. Nonce 12-byte dibangkitkan acak per kolom.

### Pilar 3: Super Encryption Diagnosis Rekam Medis (Klasik + Modern)
* **Lapisan 1 (Sandi Klasik):**
  * Extended Vigenere Cipher pada 95 karakter printable ASCII (kode ASCII 32 spasi hingga 126 tilde), memastikan teks bahasa Indonesia dan simbol medis tidak terkorupsi.
  * Keyed Columnar Transposition dengan header panjang asli 4-byte big-endian untuk menghilangkan padding ambigu.
* **Lapisan 2 (Modern AEAD):**
  * Penurunan kunci per-pasien (Patient DEK 32-byte) menggunakan **HKDF-SHA256** (RFC 5869) dengan info tag terikat pada nomor rekam medis (`patient_mrn:<MRN>`).
  * Enkripsi terotentikasi **XChaCha20-Poly1305** dengan extended nonce 24-byte acak dan AAD terikat pada nomor rekam medis pasien.
* **Dampak Keamanan:** Ciphertext tidak dapat didekripsi jika nomor rekam medis dipalsukan atau dipindahkan ke pasien lain.

### Pilar 4: ChaCha20-Poly1305 Streaming File Vault
* **Penyimpanan Berkas Laboratorium:** Dokumen hasil laboratorium (PDF, scan darah, resep biner) disimpan dalam bentuk terenkripsi (`vault/record_<id>.enc`).
* **Streaming Chunks 64 KiB:** Data diproses per blok 64 KiB secara streaming dengan kunci derivasi HKDF khusus per-record.
* **Counter Nonce:** Nonce 12-byte awal dibangkitkan secara acak, kemudian dinaikkan nilainya sebagai counter big-endian untuk setiap blok berikutnya. Tag otentikasi Poly1305 16-byte diverifikasi per chunk sehingga mencegah manipulasi potongan berkas tanpa menumpuk konsumsi RAM server.

### Pilar 5: Steganografi Citra Medis PNG LSB Terautentikasi
* **Penyisipan LSB Nir-Rugi:** Menyisipkan ringkasan diagnosis terkompresi zlib ke dalam bit paling tidak signifikan (LSB) pada kanal Red, Green, dan Blue (RGB) citra PNG.
* **Proteksi Kanal Alpha:** Kanal Alpha tidak pernah diubah untuk menjaga transparansi dan struktur citra medis asli.
* **Header Terautentikasi (40 byte):** Paket biner diawali magic bytes `"MVST"` (4 byte), panjang payload (4 byte), dan tag **HMAC-SHA256** (32 byte). Citra yang telah dimanipulasi walau hanya 1 bit akan langsung ditolak saat ekstraksi karena kegagalan verifikasi HMAC.
* **Metrik Evaluasi:** Evaluasi visual menunjukkan imperceptibility sempurna dengan nilai PSNR 66.20 - 72.22 dB (jauh di atas batas persepsi visual 40 dB) dan MSE mendekati nol (0.0039 - 0.0156).

### Pilar 6: Blockchain Audit Ledger Proof-of-Authority (Dual-Sign)
* **Struktur Rantai Hash:** Setiap transaksi klinis (`CREATE_RECORD`, `UPDATE_RECORD`, `UPLOAD_LAB`, `EMBED_STEGO`) dicatat dalam tabel `audit_blocks` yang terhubung melalui SHA-256 cryptographic hash chain.
* **Tanda Tangan Ganda Ed25519 (PoA):**
  1. *Tanda Tangan Dokter:* Dihasilkan secara lokal oleh USB dongle dokter atas payload transaksi terkanonikalisasi (`block_index|timestamp|actor|target|action|record_state_hash`).
  2. *Countersignature Validator:* Proposal blok divalidasi oleh layanan independen `medvault-validator` (Port 8096) sebelum dibubuhi tanda tangan kedua validator institusi.
* **Record State Hash Binding:** Hash SHA-256 status rekam medis diikat langsung ke dalam blok audit.
* **Fungsi `VerifyChain()`:** Memverifikasi integritas seluruh rantai blok secara real-time. Jika ada data di SQLite yang diubah secara ilegal di luar aplikasi, verifikasi otomatis gagal mendeteksi nomor blok dan field yang termanipulasi.

---

## 6. Glass-Box Telemetry Inspector

MedVault Go menolak paradigma keamanan kotak hitam (*black-box security*). Setiap respons API kriptografi menyertakan objek `_telemetry`:
* Menampilkan langkah transformasi sekuensial: teks asli, keluaran klasik, fingerprint kunci HKDF, nonce hex, tag otentikasi hex, bit budget steganografi, hingga status verifikasi tanda tangan digital.
* Mengukur durasi eksekusi internal dalam mikrodetik (`duration_us`), membuktikan efisiensi tanpa membocorkan materi kunci privat.

---

## 7. End-to-End Operational Workflow (Flow Aplikasi)

### Alur 1: Autentikasi Pengguna & Penanganan Hardware Dongle
1. Pengguna membuka URL aplikasi -> status peramban memeriksa ketersediaan sesi via `/api/v1/auth/me`.
2. Jika belum login, form pendaftaran/login meminta kredensial username dan password.
3. Pengguna memilih berkas `.medkey` dari flashdisk USB via browser File System Access API. Kunci publik dan seed dimuat ke memori peramban.
4. Peramban meminta tantangan acak dari `/api/v1/auth/challenge` (masa berlaku 60 detik).
5. Peramban menandatangani tantangan dengan kunci Ed25519 privat di memori peramban via WebCrypto API.
6. Kamera webcam menangkap foto wajah dan menghitung selisih luminance dHash 64-bit secara lokal di peramban.
7. Server memverifikasi ketiga faktor secara simultan. Jika valid, sesi `medvault_session` diterbitkan dalam Cookie HttpOnly (masa berlaku 8 jam).
8. **Sesi Kerja:** Sesi terproteksi oleh cookie HttpOnly berbatas waktu dan kunci privat siap dipakai untuk penandatanganan rekam medis secara on-demand. Kunci otomatis musnah saat tab ditutup.

### Alur 2: Pendaftaran Pasien & Enkripsi Transparan
1. Tenaga medis mengisi data pasien (Nomor Rekam Medis otomatis terbit dengan format `RM-YYYYMMDD-XXXX`, NIK 16 digit, Nama Lengkap, Alamat, Tanggal Lahir).
2. Tenaga medis menekan tombol Simpan Pasien -> request dikirim ke `POST /api/v1/patients`.
3. Backend memanggil `ColumnEncryptor`:
   * Mengambil master key AES-256.
   * Melakukan enkripsi AES-256-GCM pada field NIK, Nama, dan Alamat dengan AAD unik masing-masing kolom.
4. Data disimpan ke SQLite dalam format Base64 terenkripsi. Nilai plaintext asli tidak pernah tersimpan di disk.
5. Saat halaman pasien dibuka kembali (`GET /api/v1/patients`), backend mendekripsi kolom secara transparan untuk dokter terotorisasi.

### Alur 3: Pembuatan Rekam Medis & 2-Phase Block Sealing
1. Dokter memilih pasien dan menginput catatan diagnosis klinis serta resep obat.
2. Dokter menekan tombol Buat Rekam Medis -> tahap 1 request dikirim ke `POST /api/v1/records`.
3. Backend menjalankan Super Encryption:
   * Mengenkripsi diagnosis dan resep dengan Extended Vigenere + Columnar Transposition.
   * Menurunkan DEK pasien via HKDF-SHA256 dari master root secret dan nomor rekam medis.
   * Mengenkripsi hasil klasik dengan XChaCha20-Poly1305.
   * Menghitung `record_state_hash` kanonikal.
4. Backend membentuk draf proposal blok audit PoA, menyimpannya sementara dengan proposal token, dan mengembalikan `tx_payload` ke peramban.
5. Peramban secara otomatis menandatangani `tx_payload` menggunakan kunci privat USB dongle via WebCrypto.
6. Peramban mengirimkan signature dokter ke `POST /api/v1/records/seal`.
7. Backend mengirim proposal ke `medvault-validator` (Port 8096):
   * Validator memverifikasi tanda tangan dokter, nomor indeks blok, dan keterhubungan hash sebelumnya.
   * Validator membubuhkan countersignature institusi.
8. Rekam medis tersimpan paten dan blok audit tersegel masuk ke tabel `audit_blocks`.

### Alur 4: Pengunggahan & Pengunduhan Berkas Lab Vault
1. Dokter memilih berkas hasil laboratorium (PDF atau gambar).
2. Berkas dikirim melalui multipart form ke `POST /api/v1/vault/{record_id}/upload`.
3. Backend membaca stream berkas per potongan 64 KiB, mengenkripsinya dengan ChaCha20-Poly1305 menggunakan nonce counter, dan menyimpannya sebagai `storage/encrypted_labs/record_<id>.enc`.
4. Saat diunduh (`GET /api/v1/vault/{record_id}/download`), stream didekripsi per chunk secara on-the-fly setelah tag Poly1305 terverifikasi valid.

### Alur 5: Steganografi Citra Radiologi PNG
1. Dokter memilih citra scan radiologi (Chest X-Ray) berformat PNG dan memilih rekam medis terkait.
2. Request dikirim ke `POST /api/v1/stego/{record_id}/embed`.
3. Backend mengekstrak ringkasan rekam medis, mengompresinya dengan zlib, membuat tag HMAC-SHA256, dan menyisipkannya ke bit LSB kanal warna R, G, B.
4. Citra stego disimpan di `storage/stego_scans/record_<id>.png` dan telemetri menghitung nilai PSNR serta MSE secara instan.
5. Pada modul Ekstraksi, dokter dapat mengunggah citra stego untuk memverifikasi keaslian watermark dan memulihkan teks ringkasan diagnosis.

### Alur 6: Audit Trail & Simulasi Manipulasi Database (Tamper Test)
1. Halaman Audit Ledger menampilkan visualisasi rantai blok yang saling mengunci.
2. Backend secara kontinu mengeksekusi `VerifyChain()` untuk memastikan kontinuitas hash dan validitas kedua tanda tangan Ed25519.
3. **Simulasi Tamper:** Dokter menekan tombol "Simulasi Manipulasi SQLite". Backend secara sengaja mengubah 1 karakter pada ciphertext diagnosis langsung di dalam file database SQLite.
4. Layar seketika mendeteksi anomali: status rantai berubah menjadi merah (`INVALID / TAMPER DETECTED`) pada nomor blok terkait karena `record_state_hash` tidak lagi cocok dengan ciphertext di database.
5. Dokter menekan tombol "Pulihkan Data Asli" -> ciphertext dikembalikan ke kondisi sah dan status verifikasi blockchain kembali hijau (`VERIFIED / INTEGRITY OK`).

---

## 8. Slide-by-Slide Deck Blueprint (Struktur 13 Slide)

Panduan rancangan slide untuk pembuatan presentasi (PowerPoint / Canva / Google Slides):

### Slide 1: Judul & Identitas Proyek
* **Judul Utama:** MedVault Go
* **Sub-judul:** Sistem Rekam Medis Elektronik dengan Autentikasi Tiga-Faktor, Kriptografi Berlapis, Steganografi LSB Terautentikasi, dan Audit Ledger PoA.
* **Informasi Tim:** Arsyadi Indra Hasan P (123240062), Hafizd S. Abdurrahman (123240167), Rakha Albany Nugraha (123240214).
* **Afiliasi:** Program Studi Informatika, UPN "Veteran" Yogyakarta - 2026.
* **Visual:** Mockup antarmuka laptop MedVault Go dengan aksen warna Dark Teal (`#0d2822`) dan Mint Neon (`#5eead4`).

### Slide 2: Latar Belakang & Urgensi Masalah
* **Poin Kunci:**
  * Kerentanan data medis: rekam medis memuat identitas kependudukan, riwayat diagnosa sensitif, dan berkas medis biner.
  * Ancaman *Data at Rest* & *Insider Threat*: pembobolan database SQL mentah dapat membaca seluruh data jika tidak dienkripsi per-kolom.
  * *Untraceable Tampering*: manipulasi record langsung di database sering kali tidak terdeteksi oleh sistem konvensional.
  * Kebutuhan sistem nir-dependensi CGO dan hemat sumber daya untuk faskes berskala kecil.

### Slide 3: Mengapa Rewrite: Legacy Desktop vs MedVault Go Web
* **Poin Kunci:**
  * Komparasi Arsitektur: Legacy Electron + Python FastAPI vs MedVault Go (Single Static Binary + Web MPA).
  * Penghematan Memori: RAM drop dari > 600 MB ke < 40 MB idle (container app 96 MB, validator 32 MB).
  * Zero Client Install: Dokter cukup buka browser (Chrome/Edge) via Cloudflare Tunnel tanpa perlu install aplikasi desktop 150 MB+.
  * Zero-CGO & Zero-CDN: Tidak ada OpenCV/C++ wheel dependensi; seluruh aset statis tertanam via `embed.FS` dengan CSP ketat.
  * Physical Validator Isolation: Dua container terpisah dalam satu Podman Pod (`medvault-app` dan `medvault-validator`).
* **Visual:** Tabel komparasi side-by-side: Legacy (Merah/Kuning) vs MedVault Go (Hijau/Cyan), dan diagram topologi Podman Pod.

### Slide 4: Arsitektur Sistem & Spesifikasi Deployment
* **Poin Kunci:**
  * Arsitektur Single Static Binary Go 1.26 (`CGO_ENABLED=0`) dengan Pure-Go SQLite (`modernc.org/sqlite` mode WAL).
  * Topologi Podman Pod (`medvault-pod`) di VPS Ubuntu 24.04:
    * `medvault-app`: 96 MB RAM limit (Port 8095).
    * `medvault-validator`: 32 MB RAM limit (Port 8096).
  * Keamanan Jaringan: Zero exposed public ports via Cloudflare Tunnel terenkripsi TLS end-to-end.
  * Zero-CDN Asset Embedding: Tailwind CSS v4 dan Alpine.js v3 tertanam langsung di biner Go.
* **Visual:** Diagram blok topologi Podman Pod, komunikasi localhost loopback, dan integrasi Cloudflare Tunnel.

### Slide 5: Pilar 1 - Autentikasi 3-Faktor & On-Demand Hardware Gate
* **Poin Kunci:**
  * *Faktor 1 (Pengetahuan):* Kata sandi dengan hash memory-hard Argon2id (16 MB, t=2, p=2, constant-time compare).
  * *Faktor 2 (Kepemilikan):* Hardware USB Dongle Ed25519 challenge-response lokal via WebCrypto API (kunci privat tidak pernah menyentuh server).
  * *Faktor 3 (Biometrik):* 64-bit difference hash (dHash) wajah via webcam (Hamming distance <= 32). Foto mentah tidak pernah disimpan di database (privacy-first).
  * *On-Demand Hardware Gate:* Mengikuti pola YubiKey industri; kunci privat berada di memori aman sesi peramban dan musnah saat tab ditutup.
* **Visual:** Diagram alur challenge-response USB dongle dan ilustrasi kisi dHash 9x8 webcam.

### Slide 6: Pilar 2 - Enkripsi Kolom PII Pasien (AES-256-GCM + AAD Binding)
* **Poin Kunci:**
  * Perlindungan data at-rest pada tabel `patients` (NIK, nama lengkap, alamat).
  * Algoritma: AES-256-GCM (Nonce 12-byte acak per operasi, tag otentikasi 16-byte).
  * Pengikatan AAD Ketat: Nilai AAD terikat pada nama kolom lengkap (`patients.nik`, `patients.full_name`, `patients.address`).
  * Mencegah serangan *Column-Swapping Attack* di dalam SQLite.
* **Visual:** Ilustrasi perbandingan tabel database mentah (Base64 ciphertext) vs tampilan dekripsi transparan di antarmuka dokter.

### Slide 7: Pilar 3 - Super Encryption Rekam Medis (Klasik + Modern)
* **Poin Kunci:**
  * Perlindungan ganda pada diagnosis dan resep obat di tabel `records`.
  * *Lapisan 1 (Klasik):* Extended Printable ASCII Vigenère (95 karakter printable) + Keyed Columnar Transposition (header panjang 4-byte).
  * *Lapisan 2 (Modern):* HKDF-SHA256 untuk derivasi kunci DEK per-pasien + XChaCha20-Poly1305 AEAD dengan AAD nomor rekam medis.
  * Ciphertext terkunci unik per nomor rekam medis; pemindahan ciphertext antar pasien otomatis gagal.
* **Visual:** Diagram alur sekuensial: Plaintext -> Sandi Klasik -> HKDF Derivation -> XChaCha20-Poly1305 -> Ciphertext Base64.

### Slide 8: Pilar 4 - ChaCha20-Poly1305 Streaming File Vault
* **Poin Kunci:**
  * Penyimpanan aman berkas laboratorium berukuran besar (PDF / Gambar) di direktori `vault/`.
  * Enkripsi streaming dalam potongan (chunk) 64 KiB menggunakan ChaCha20-Poly1305.
  * Nonce 12-byte awal dibangkitkan acak dan bertambah secara counter big-endian untuk setiap chunk berikutnya.
  * Proteksi integritas per chunk tanpa membebani kapasitas RAM server (RAM overhead datar < 5 MB).
* **Visual:** Ilustrasi pemotongan file 64 KiB, nonce counter progression, dan header biner hex preview.

### Slide 9: Pilar 5 - Steganografi Citra Medis PNG LSB Terautentikasi
* **Poin Kunci:**
  * Menyembunyikan ringkasan rekam medis ke dalam citra radiologi PNG lossless.
  * Payload terkompresi zlib, dilindungi header 40-byte (`MVST` + panjang + tag HMAC-SHA256).
  * Penyisipan bit hanya pada LSB kanal warna R, G, B (kanal Alpha 100% utuh).
  * Evaluasi Kualitas Visual: PSNR mencapai 66.20 - 72.22 dB (kualitas sempurna di atas standar 40 dB) dan MSE mendekati 0.
* **Visual:** Perbandingan citra rontgen dada sebelum dan sesudah penyisipan bit (tidak kasat mata) beserta badge PSNR/MSE.

### Slide 10: Pilar 6 - Blockchain Audit Ledger Proof-of-Authority (Dual-Sign)
* **Poin Kunci:**
  * Rantai audit transaksi klinis nir-ubah berbasis hash SHA-256.
  * Konsensus Dual-Signature Ed25519:
    1. Tanda tangan dokter pembuat transaksi (dari USB dongle).
    2. Countersignature validator institusi (dari container `medvault-validator`).
  * Binding `record_state_hash`: mengunci kondisi rekam medis saat transaksi dicatat.
  * Verifikasi kontinuitas rantai via fungsi `VerifyChain()`.
* **Visual:** Diagram blok blockchain audit, tautan `previous_hash`, dan sertifikasi dua tanda tangan digital.

### Slide 11: Glass-Box Telemetry & Observabilitas Kriptografi
* **Poin Kunci:**
  * Transparansi penuh melalui inspektor telemetri (`_telemetry` JSON envelope).
  * Menampilkan nilai antara (intermediate values) pada setiap tahap transformasi secara aman.
  * Mengukur durasi komputasi nyata per langkah dalam mikrodetik (`duration_us`).
  * Edukatif untuk demonstrasi akademis dan audit forensik keamanan informasi.
* **Visual:** Screenshot drawer Glass-Box Telemetry Inspector dengan tabel tahapan kriptografi terverifikasi.

### Slide 12: Pengujian Sistem & Hasil Evaluasi Kinerja
* **Poin Kunci:**
  * 100% Lulus pada seluruh 165 kasus uji otomatis (`go test -v ./...` mencakup 10 paket Go).
  * Benchmark Waktu Eksekusi Kriptografi:
    * Enkripsi PII AES-256-GCM: ~0.08 - 0.15 ms per pasien.
    * Super Encryption: ~0.22 - 0.35 ms per rekam medis.
    * Enkripsi Berkas Lab 5 MB: ~9.8 - 14.5 ms.
    * Verifikasi 100 Blok Blockchain: ~3.8 - 5.2 ms.
  * Latensi total jauh di bawah batas persepsi manusia (100 ms).
* **Visual:** Grafik batang waktu eksekusi milidetik dan rekapitulasi checklist kelulusan test suite.

### Slide 13: Kesimpulan & Live Demonstration Overview
* **Poin Kunci:**
  * MedVault Go berhasil mengintegrasikan enam pilar kriptografi modern dan klasik ke dalam arsitektur Go mandiri yang ringan, deterministik, dan tangguh.
  * Siap untuk demonstrasi langsung: Live login MFA, USB Dongle physical ejection, enkripsi diagnosis, upload lab vault, penyisipan steganografi, dan simulasi perusakan integritas blockchain.
* **Visual:** Tautan live demo (`https://medvault.indraprhmbd.my.id`), QR code repositori GitHub, dan sesi tanya jawab (Q&A).

---

## 9. Timeboxed 10-Minute Live Demo Script

Gunakan urutan demonstrasi berikut saat presentasi di hadapan dosen penguji:

| Waktu | Durasi | Aksi Presenter / Pengemudi Sistem | Penjelasan Lisan yang Disampaikan | Indikator Keberhasilan di Layar |
| :--- | :--- | :--- | :--- | :--- |
| **00:00 - 02:00** | 2 Menit | Buka `https://medvault.indraprhmbd.my.id`. Masukkan username & password. Tancapkan USB flash drive dan pilih berkas `.medkey`. Arahkan wajah ke webcam untuk dHash (atau gunakan passkey WebAuthn). Klik Masuk. Lalu cabut USB dari port laptop. | "Sistem menerapkan MFA 3-faktor: kata sandi Argon2id, tanda tangan Ed25519 dari USB dongle, dan dHash biometrik wajah. Saat ini saya mendemonstrasikan hardware watchdog: ketika USB dicabut secara fisik di tengah sesi, sistem otomatis mendeteksi pelepasan hardware dan mengunci antarmuka seketika demi keamanan terminal klinis." | Muncul notifikasi modal merah *"USB Dongle Terlepas"*, antarmuka terkunci. Setelah USB dicolokkan kembali, status berubah hijau *"Terhubung"*. |
| **02:00 - 04:00** | 2 Menit | Buka modul **Data Pasien**. Buat pasien baru. Tunjukkan bahwa NIK, Nama, dan Alamat terenkripsi. Lalu buat rekam medis klinis baru pada modul **Rekam Medis**. Buka drawer **Glass-Box Telemetry Inspector**. | "Data identitas pasien diamankan AES-256-GCM dengan AAD nama kolom. Untuk diagnosis, kami menerapkan Super Encryption dua lapis: lapisan klasik Extended Vigenere dan Transposisi Kolom, diikuti derivasi kunci per-pasien HKDF-SHA256 dan enkripsi modern XChaCha20-Poly1305. Melalui Glass-Box Inspector, dosen dapat melihat seluruh transformasi antara secara transparan." | Tabel langkah telemetri menampilkan tahap Klasik, Key Derivation, Nonce hex, dan Tag Poly1305 terverifikasi secara runtut. |
| **04:00 - 05:30** | 1.5 Menit | Buka modul **File Vault**. Unggah berkas hasil laboratorium (PDF). Tunjukkan perbandingan byte mentah asli vs ciphertext `.enc` di disk. Klik tombol **Unduh & Dekripsi**. | "Berkas hasil laboratorium biner diamankan secara streaming dalam potongan 64 KiB menggunakan ChaCha20-Poly1305 dengan counter nonce. Pendekatan streaming ini menjamin konsumsi memori server tetap datar di bawah 5 MB bahkan saat mengenkripsi berkas multi-megabyte." | Berkas PDF berhasil diunduh dan terbuka sempurna dengan byte yang identik dengan aslinya (SHA-256 cocok). |
| **05:30 - 07:30** | 2 Menit | Buka modul **Steganografi**. Pilih citra rontgen dada PNG dan sembunyikan catatan rekam medis. Tunjukkan metrik PSNR dan MSE. Pindah ke tab **Ekstraksi Berkas**, unggah citra stego, lalu klik Ekstraksi. | "Kami menyembunyikan catatan klinis ke LSB kanal warna R, G, B citra radiologi PNG lossless. Integritas data dilindungi tag HMAC-SHA256 di header 40-byte, sementara kanal Alpha tidak diubah. Nilai PSNR terukur di atas 69 dB, membuktikan bahwa mata manusia tidak dapat membedakan citra asli dan stego. Saat diekstraksi, tag HMAC diverifikasi sah." | Nilai PSNR > 66 dB tampil di telemetri; ekstraksi memulihkan teks ringkasan diagnosis dengan status *"HMAC Terverifikasi Sah"*. |
| **07:30 - 09:30** | 2 Menit | Buka modul **Audit Ledger**. Tunjukkan daftar blok blockchain yang membawa tanda tangan dokter dan validator. Klik tombol **Simulasi Manipulasi SQLite**. Tunjukkan perubahan status. Klik **Pulihkan Data**. | "Inilah bukti ketahanan rantai audit Proof-of-Authority kami. Setiap blok ditandatangani dua pihak: dokter via USB dongle dan validator institusi via gateway mandiri. Sekarang saya mensimulasikan serangan orang dalam: memanipulasi 1 karakter ciphertext di database SQLite mentah. Fungsi VerifyChain seketika mendeteksi anomali karena record state hash tidak cocok. Begitu data dipulihkan, integritas rantai kembali valid." | Blok seketika berubah warna merah (*TAMPER DETECTED / INVALID*); setelah dipulihkan, status kembali hijau (*CHAIN INTEGRITY OK*). |
| **09:30 - 10:00** | 30 Detik | Tampilkan slide penutup dan ringkasan pengujian 165 test cases. Buka sesi tanya jawab. | "Demikian demonstrasi MedVault Go. Seluruh 165 unit test lulus 100%, sistem berjalan ringan di VPS berdaya rendah, dan siap menjawab pertanyaan dewan penguji. Terima kasih." | Slide tanya jawab ditampilkan. |

---

## 10. Anticipated Defense Questions & Recommended Answers

Daftar pertanyaan yang paling sering diajukan dosen penguji kriptografi beserta jawaban terbaiknya:

### Q1: Mengapa masih menggunakan algoritma klasik (Vigenère dan Transposisi) jika sudah ada XChaCha20-Poly1305?
* **Jawaban:** "Lapisan klasik berfungsi sebagai *cascade cipher* edukatif dan pengacak pola distribusi karakter printable, sementara kekuatan keamanan matematis utama sepenuhnya bertumpu pada XChaCha20-Poly1305 dengan kunci 256-bit dan nonce 192-bit. Berdasarkan teorema kriptografi, menambahkan lapisan transposisi dan substitusi sebelum cipher modern tidak mengurangi entropi kunci modern, namun memperkaya demonstrasi prinsip *confusion and diffusion* pada media rekam medis."

### Q2: Mengapa memilih dHash biometrik 64-bit daripada deep learning face recognition (seperti FaceNet atau CNN)?
* **Jawaban:** "MedVault Go dirancang untuk infrastruktur klinis berdaya rendah (VPS 2 vCPU, RAM 1.9 GiB) dengan arsitektur murni Go nir-CGO (`CGO_ENABLED=0`). Pustaka deep learning seperti OpenCV atau TensorFlow membutuhkan runtime C++ yang berat dan konsumsi RAM ratusan megabyte. Algoritma 64-bit dHash dieksekusi secara instan di peramban, menjaga privasi tenaga medis karena foto mentah tidak pernah disimpan di server, dan sudah cukup sensitif mengukur selisih gradien luminance menggunakan jarak Hamming."

### Q3: Mengapa menggunakan konsensus Proof-of-Authority (PoA) dengan validator gateway, bukan blockchain publik seperti PoW atau PoS?
* **Jawaban:** "Rekam medis berada di bawah regulasi privasi kesehatan ketat (seperti UU PDP dan Permenkes RME). Blockchain publik (PoW/PoS) memiliki latensi konfirmasi lambat, biaya komputasi/gas fee tinggi, dan mengekspos metadata ke publik. PoA dual-signature memberikan finalitas transaksi instan (di bawah 5 milidetik), mengikat tanggung jawab hukum secara eksplisit kepada dokter pembuat transaksi dan institusi validator, serta dapat dijalankan dengan memori hanya 32 MB pada container validator."

### Q4: Mengapa sistem tidak menggunakan JWT (JSON Web Token) dengan Bearer Token untuk autentikasi?
* **Jawaban:** "Bearer token yang disimpan di LocalStorage atau sessionStorage rentan terhadap pencurian via serangan Cross-Site Scripting (XSS). MedVault Go menggunakan Cookie HttpOnly `medvault_session` dengan flag `SameSite=Lax` dan enkripsi TLS end-to-end. Token sesi disimpan dalam tabel `app_sessions` di SQLite dengan masa kedaluwarsa 8 jam, sehingga token tidak dapat diakses atau dicuri oleh skrip JavaScript di sisi klien."

### Q5: Bagaimana jika penyerang menguasai hak akses root pada server VPS?
* **Jawaban:** "Bahkan jika penyerang menguasai akses root pada server, penyerang tidak dapat mendekripsi berkas rekam medis yang dicuri tanpa memegang master root secret di environment. Selain itu, penyerang tidak dapat memalsukan transaksi rekam medis baru di masa lalu atau masa depan karena setiap blok audit sah memerlukan tanda tangan digital Ed25519 dari USB hardware dongle fisik milik dokter yang kunci privatnya berada di perangkat fisik dokter, bukan di server."
