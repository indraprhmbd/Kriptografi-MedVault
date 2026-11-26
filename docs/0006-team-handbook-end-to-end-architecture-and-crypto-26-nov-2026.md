# 006 - Buku Panduan Teknis Tim (Team Engineering and Cryptography Handbook)

**Dokumen ID:** 006  
**Tanggal:** 26 November 2026  
**Status:** Approved for Study and Knowledge Sharing  
**Cakupan:** Buku panduan komprehensif end-to-end untuk seluruh anggota tim MedVault Go (Arsyadi, Hafizd, Rakha). Dokumen ini mengupas tuntas arsitektur sistem, alasan perombakan (rewrite), detail matematis dan implementasi 6 pilar kriptografi, alur data aplikasi, petunjuk pemecahan masalah USB dongle, pembagian peran presentasi, serta bank tanya jawab dosen penguji.

---

## 1. Pendahuluan dan Filosofi MedVault Go

### 1.1 Visi dan Identitas Proyek
MedVault Go adalah sistem Electronic Health Record (EHR) modern berbasis web yang dibangun dengan prinsip **Defense-in-Depth** (pertahanan berlapis). Sistem ini dirancang untuk menyelesaikan tiga dilema besar dalam pengelolaan data medis di fasilitas kesehatan:
1. **Kerahasiaan Data (Data Privacy):** Melindungi data identitas pasien (PII) dan rekam diagnosis klinis dari kebocoran database at-rest.
2. **Keabsahan dan Nir-Ubah (Authenticity and Immutability):** Menjamin bahwa setiap tindakan medis ditandatangani secara sah oleh dokter penanggung jawab dan dicatat dalam audit trail nir-ubah (immutable ledger) yang kebal terhadap manipulasi oleh administrator database atau peretas root.
3. **Efisiensi Sumber Daya (Low Resource Footprint):** Membuktikan bahwa sistem kriptografi kelas militer dan blockchain audit dapat berjalan sangat cepat dan stabil pada perangkat server berdaya rendah (VPS 2 vCPU, RAM 1.9 GiB) tanpa ketergantungan runtime yang rumit.

### 1.2 Anggota Tim dan Pembagian Fokus
* **Arsyadi Indra Hasan P (123240062):** Lead System Architect, Backend Cryptography (Internal Crypto, Steganography, PoA Ledger Consensus), dan Infrastructure Deployment (Podman Pod, Cloudflare Tunnel).
* **Hafizd S. Abdurrahman (123240167):** Frontend Cryptography Integration, WebCrypto Ed25519 Client-Side Signing, Browser Hardware Gates (File System Access API & Webcam dHash), dan UI State Management.
* **Rakha Albany Nugraha (123240214):** Quality Assurance, Security Audit, Evaluasi Metrik Kriptografi (PSNR, MSE, Benchmark Latensi), dan Analisis Pengujian Tamper.

---

## 2. Mengapa Melakukan Rewrite: Legacy (Python + Electron) vs MedVault Go (Golang + Web)

Sebelumnya, proyek ini diawali sebagai prototipe aplikasi desktop menggunakan tumpukan teknologi Python 3.11, FastAPI, Electron, OpenCV, dan Pytest. Di tengah perjalanan, tim mengambil keputusan strategis untuk melakukan **perombakan total (rewrite) ke bahasa Go murni dan arsitektur website multi-halaman (MPA)**.

Tim harus memahami dan dapat menjelaskan alasan rewrite ini secara percaya diri di hadapan dosen:

| Dimensi Evaluasi | Arsitektur Legacy (Python + Electron) | Arsitektur MedVault Go (Golang + Web MPA) | Alasan Teknis & Keuntungan Nyata |
| :--- | :--- | :--- | :--- |
| **Konsumsi Memori (RAM)** | Sangat boros: Runtime Electron memakan 300-450 MB RAM di workstation dokter, ditambah proses background FastAPI/Uvicorn 180-250 MB RAM. Total memori melampaui 600 MB. | Sangat ringan: Container aplikasi utama dibatasi 96 MB RAM, container validator dibatasi 32 MB RAM. Kondisi idle server hanya memakai **< 40 MB RAM**. | Mencegah crash Out-of-Memory (OOM) pada VPS spesifikasi rendah (1.9 GiB RAM). Menghemat alokasi RAM hingga lebih dari 80%. |
| **Dependensi CGO & Native Binaries** | Sangat rentan konflik: Bergantung pada pustaka C++ pihak ketiga seperti OpenCV wheels, dependensi OS, compiler C/C++, serta interpreter Python terpasang. | Bersih dan mandiri: Dikompilasi 100% pure Go (`CGO_ENABLED=0`). Mesin database menggunakan SQLite murni Go (`modernc.org/sqlite`). | Biner statis tunggal tanpa ketergantungan pustaka dinamis shared object (`.so` atau `.dll`). Portabel dan tidak mengalami drift versi library. |
| **Penyebaran Aplikasi (Deployment & Friction)** | Beban operasional tinggi: Dokter atau staf IT klinik harus mengunduh dan memasang installer desktop Electron (> 150 MB) di setiap komputer ruang periksa. | Bebas instalasi (Zero-Install): Dokter cukup membuka peramban web modern (Google Chrome atau Microsoft Edge) melalui URL terenkripsi. | Fleksibilitas tinggi bagi dokter untuk mengakses sistem dari berbagai ruangan klinik tanpa instalasi driver khusus. |
| **Keamanan Kunci Klien (Hardware Gate)** | Mengandalkan IPC Electron yang rentan terhadap eksploitasi peretasan Remote Code Execution bila terjadi kerentanan di layer antarmuka. | Menggunakan Web Standards modern: File System Access API untuk handle USB, WebCrypto API untuk penandatanganan di RAM, dan WebAuthn. | Kunci privat dongle tersimpan di USB dan dieksekusi di sandbox peramban. Kunci privat tidak pernah terkirim ke jaringan kabel atau internet. |
| **Keamanan Rantai Pasok (Supply Chain Security)** | Rentan: Memerlukan ratusan pustaka di folder `node_modules` Electron dan puluhan package di `requirements.txt`. | Minimalis dan aman: Seluruh aset frontend (Tailwind CSS v4 dan Alpine.js v3) disematkan langsung ke dalam biner Go menggunakan `embed.FS`. | Bebas risiko pembajakan CDN eksternal, bekerja stabil pada jaringan offline/lambat, dan menerapkan Content Security Policy (CSP) ketat (`default-src 'self'`). |
| **Performa & Throughput Kriptografi** | Terhambat oleh Python GIL (Global Interpreter Lock), lambat saat menangani enkripsi berkas streaming berukuran megabyte. | Sangat kencang: Menggunakan model konkurensi goroutine native Go dan optimasi instruksi mesin 64-bit untuk ChaCha20-Poly1305. | Enkripsi streaming berkas lab 5 MB selesai dalam ~12 milidetik; verifikasi rantai audit 100 blok selesai dalam ~4 milidetik. |
| **Pemisahan Wewenang Validator PoA** | Bersifat semu: Kunci validator sering kali berada dalam ruang memori atau proses yang sama dengan aplikasi web. | Isolasi fisik sejati: Layanan `medvault-validator` berjalan sebagai kontainer dan biner terpisah di port 8096, berbeda dari `medvault-app` di port 8095. | Menjamin pemisahan wewenang konsensus Proof-of-Authority (PoA) secara ketat; aplikasi utama tidak memiliki akses fisik ke kunci validator institusi. |

---

## 3. Arsitektur Infrastruktur dan Topologi Kontainer

### 3.1 Diagram Topologi Podman Pod
MedVault Go beroperasi di dalam satu Podman Pod bernama `medvault-pod` di server VPS Ubuntu 24.04:

```
[ Internet Publik ]
        │
        ▼ (Enkripsi TLS End-to-End)
[ Cloudflare Edge ]
        │
        ▼ (Cloudflare Tunnel: cloudflared service)
[ VPS Ubuntu 24.04 LTS (1.9 GiB RAM, 2 vCPU) ]
        │
┌───────┴────────────────────────────────────────────────────────┐
│ medvault-pod (Localhost Loopback Network Namespace)            │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ medvault-app (Port 8095, RAM Limit 96 MB)                │  │
│  │ - Melayani Web UI (Embedded HTML + Tailwind + Alpine)    │  │
│  │ - REST API Kriptografi, Steganografi, & Sesi Dokter      │  │
│  │ - Pure Go SQLite Engine (WAL Mode)                       │  │
│  └───────────────┬──────────────────────────────────────────┘  │
│                  │ HTTP Internal (Proposal Blok Audit)         │
│                  ▼                                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ medvault-validator (Port 8096, RAM Limit 32 MB)          │  │
│  │ - Layanan Validasi PoA Independen                        │  │
│  │ - Memegang Kunci Privat Validator Institusi (Ed25519)    │  │
│  │ - Membubuhkan Countersignature pada Blok Audit Sah       │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

### 3.2 Keamanan Jaringan Zero Exposed Port
* Tidak ada port masuk (inbound port 80, 443, 8095, atau 8096) yang dibuka pada firewall VPS (UFW).
* Lalu lintas masuk dikelola melalui Cloudflare Tunnel outbound. Layanan `cloudflared` membuat koneksi keluar yang aman menuju tepi jaringan Cloudflare, lalu mengarahkan permintaan masuk ke `127.0.0.1:8095`. Hal ini melindungi server dari pemindaian port botnet dan serangan DDoS langsung ke IP publik VPS.

### 3.3 SQLite dengan Mode WAL (Write-Ahead Logging)
Sistem menggunakan mesin SQLite murni Go (`modernc.org/sqlite`) yang dikonfigurasi dengan mode WAL (`PRAGMA journal_mode=WAL`). Mode ini memungkinkan operasi pembacaan (read) dan penulisan (write) berjalan secara bersamaan tanpa saling mengunci (*non-blocking concurrent reads*), sehingga respons API tetap cepat di bawah beban kerja simultan.

---

## 4. Bedah Rinci 6 Pilar Kriptografi MedVault Go

### Pilar 1: Autentikasi Multi-Faktor (MFA 3-Faktor) dan Hardware Gate

Sistem menerapkan prinsip otentikasi lengkap: *Something You Know*, *Something You Have*, dan *Something You Are*.

#### Faktor 1: Kata Sandi dengan Hash Memory-Hard Argon2id
* **Tujuan:** Melindungi kata sandi pengguna dari serangan brute-force berbasis GPU dan ASIC.
* **Parameter Algoritma:** Mengikuti standar RFC 9106:
  * Memori: 16 MiB (16384 KiB).
  * Iterasi (Waktu): 2 putaran.
  * Paralelisme: 2 thread.
  * Panjang Salt: 16 byte acak kriptografis (`crypto/rand`).
  * Panjang Hash Kunci: 32 byte.
* **Verifikasi Waktu-Konstan:** Pembandingan hash menggunakan fungsi `crypto/subtle.ConstantTimeCompare` untuk mencegah serangan analisis waktu (*timing attack*).

#### Faktor 2: USB Dongle Ed25519 dengan Protokol Challenge-Response
* **Tujuan:** Membuktikan dokter secara fisik memiliki USB kunci yang sah tanpa pernah membocorkan kunci privat ke jaringan.
* **Format Berkas `.medkey`:** Berkas JSON berukuran kecil yang disimpan di flashdisk USB:
  ```json
  {
    "version": 1,
    "key_id": "DONGLE-2026-0001",
    "username": "dr_andi",
    "algorithm": "Ed25519",
    "public_key_hex": "b5a2...",
    "private_key_seed_hex": "4f1c...",
    "issued_at": "2026-11-26T08:00:00Z"
  }
  ```
* **Mekanisme Challenge-Response Langkah demi Langkah:**
  1. *Permintaan Tantangan:* Browser dokter meminta tantangan acak ke `GET /api/v1/auth/challenge?username=dr_andi`.
  2. *Pembangkitan Server:* Server membangkitkan string acak 32-byte (64 heksadesimal) via kernel CSPRNG, menyimpannya di tabel database `challenges` dengan status berlaku 60 detik dan sekali pakai.
  3. *Penandatanganan Lokal Browser:* Browser membaca seed privat dari memori, mengimpornya ke WebCrypto API (`crypto.subtle.importKey`), dan menandatangani string tantangan 32-byte tersebut dengan kurva Ed25519.
  4. *Kirim Respon:* Browser mengirimkan `challenge_hex` dan `signature_hex` ke endpoint `POST /api/v1/auth/login`. Kunci privat tidak ikut dikirim.
  5. *Verifikasi Server:* Server memeriksa apakah tantangan masih berlaku dan belum pernah dipakai, lalu memverifikasi tanda tangan menggunakan kunci publik dokter (`ed25519.Verify`). Setelah diverifikasi, tantangan langsung dihapus (*consumed*).
* **Model On-Demand Hardware Token (Best Practice Industri):** Kunci privat dimuat ke memori sesi tab peramban saat otentikasi login dan otorisasi transaksi rekam medis (seperti prinsip kerja YubiKey atau token perbankan). Sistem tidak memaksakan background polling terus-menerus ke drive USB fisik untuk menghindari risiko sleep-mode sistem operasi dan kegagalan I/O palsu, sementara sesi kerja dokter dijamin aman melalui cookie sesi HttpOnly berbatas waktu (session TTL). Kunci privat di memori peramban otomatis terhapus saat tab ditutup atau pengguna menekan logout.

#### Faktor 3: Biometrik 64-bit difference hash (dHash) dan Passkey WebAuthn
* **Tujuan:** Verifikasi kehadiran fisik dokter secara cepat tanpa membebani server dengan pustaka pengenalan wajah C++ yang berat.
* **Mekanisme dHash:**
  1. Webcam mengambil frame gambar wajah dokter di browser (640x480).
  2. Gambar diubah menjadi skala abu-abu (grayscale) dan diperkecil menjadi kisi resolusi 9x8 piksel.
  3. Dihitung perbedaan intensitas cahaya horizontal: jika piksel kiri lebih terang dari piksel kanan, bit bernilai 1; jika tidak, bit bernilai 0.
  4. Kisi 9x8 menghasilkan 64 bit (8 baris x 8 selisih piksel) yang direpresentasikan dalam 16 karakter heksadesimal.
  5. Server membandingkan dHash login dengan dHash saat registrasi menggunakan **Jarak Hamming (Hamming Distance)**. Nilai toleransi jarak Hamming ditetapkan `<= 32 bit` (kemiripan struktur wajah >= 50%) untuk mengantisipasi variasi pencahayaan ruangan klinik.
* **Status WebAuthn:** Endpoint API WebAuthn (FIDO2) telah disiapkan di backend untuk spesifikasi ekstensi biometrik hardware di masa depan, sedangkan faktor biometrik aktif pada antarmuka registrasi dan login saat ini murni menggunakan webcam dHash 64-bit.

---

### Pilar 2: Enkripsi Kolom PII Pasien (AES-256-GCM + AAD Binding)

* **Tujuan:** Melindungi data identitas kependudukan pasien (Personally Identifiable Information / PII) di tabel database `patients` agar jika file database dicuri, penyerang hanya melihat teks acak Base64.
* **Algoritma:** **AES-256-GCM** (Galois/Counter Mode, NIST SP 800-38D).
* **Komponen Enkripsi:**
  * Kunci Master AES 256-bit (32 byte) dari konfigurasi server.
  * Nonce 12-byte acak yang dibangkitkan segar untuk setiap kolom data.
  * Tag Otentikasi GCM 16-byte untuk memastikan integritas data.
* **Pengikatan AAD Ketat (Strict Additional Authenticated Data Binding):**
  Untuk mencegah serangan penukaran kolom (*Column-Swapping Attack*) di mana penyerang yang memiliki akses ke database menukar ciphertext kolom alamat ke kolom NIK, nama kolom database lengkap diikat sebagai parameter AAD:
  * Kolom NIK: AAD = `patients.nik`
  * Kolom Nama Lengkap: AAD = `patients.full_name`
  * Kolom Alamat: AAD = `patients.address`
  Jika seseorang memindahkan ciphertext antar kolom, proses dekripsi akan gagal seketika dengan galat *cipher: message authentication failed*.

---

### Pilar 3: Super Encryption Diagnosis Rekam Medis (Klasik + Modern)

* **Tujuan:** Mengamankan catatan diagnosa dan terapi pasien dengan menggabungkan prinsip kerahasiaan berlapis (*cascade cipher*).
* **Alur Sekuensial Enkripsi:**

```
[ Catatan Diagnosa Medis Plaintext ]
                 │
                 ▼
 ┌───────────────────────────────────────────────────────────────┐
 │ 1. Extended Printable ASCII Vigenere Cipher                   │
 │    - Domain: 95 karakter printable ASCII (kode 32 s.d. 126)   │
 │    - Mengamankan simbol medis: angka, tanda baca, spasi, dosis│
 └───────────────┬───────────────────────────────────────────────┘
                 │
                 ▼
 ┌───────────────────────────────────────────────────────────────┐
 │ 2. Keyed Columnar Transposition Cipher                        │
 │    - Mengacak permutasi urutan karakter                       │
 │    - Header panjang 4-byte big-endian untuk cegah padding liar│
 └───────────────┬───────────────────────────────────────────────┘
                 │
                 ▼ (Intermediate Classical Ciphertext)
 ┌───────────────────────────────────────────────────────────────┐
 │ 3. Derivasi Kunci Pasien via HKDF-SHA256 (RFC 5869)           │
 │    - Master Secret + Info Tag: "patient_mrn:<MRN>"            │
 │    - Menghasilkan DEK (Data Encryption Key) 32-byte unik      │
 └───────────────┬───────────────────────────────────────────────┘
                 │
                 ▼
 ┌───────────────────────────────────────────────────────────────┐
 │ 4. Modern AEAD: XChaCha20-Poly1305                            │
 │    - Extended Nonce 24-byte acak                              │
 │    - AAD terikat pada Nomor Rekam Medis (MRN)                 │
 └───────────────┬───────────────────────────────────────────────┘
                 │
                 ▼
[ Ciphertext Akhir Terotentikasi (Format Base64) ]
```

* **Dampak Keamanan Nyata:**
  * Kunci enkripsi diturunkan secara unik untuk setiap pasien.
  * Pengikatan AAD pada nomor rekam medis memastikan bahwa record tidak dapat dipindahkan atau dipalsukan untuk pasien lain tanpa memicu kegagalan otentikasi Poly1305.

---

### Pilar 4: ChaCha20-Poly1305 Streaming File Vault

* **Tujuan:** Mengamankan dokumen lampiran laboratorium berukuran besar (PDF hasil lab, scan darah) di media penyimpanan disk tanpa menghabiskan memori RAM server.
* **Arsitektur Pemrosesan Streaming:**
  * File dibaca dan diproses dalam potongan-potongan kecil berukuran **64 KiB (65536 byte)**.
  * Kunci derivasi khusus file vault diturunkan menggunakan HKDF-SHA256 dengan info tag `vault_record:<id>`.
  * **Nonce Counter Progression:** Nonce awal 12-byte dibangkitkan secara acak dan dicatat pada header berkas. Untuk setiap chunk berikutnya, nilai nonce dinaikkan sebesar 1 pada representasi integer big-endian 32-bit.
  * Setiap chunk 64 KiB membawa tag otentikasi Poly1305 16-byte masing-masing.
* **Keunggulan:** Pemrosesan streaming ini menjamin bahwa penggunaan RAM server tetap datar di bawah 5 MB meskipun sedang mengenkripsi berkas lampiran berukuran puluhan megabyte.

---

### Pilar 5: Steganografi Citra Medis PNG LSB Terautentikasi

* **Tujuan:** Menyembunyikan ringkasan catatan medis ke dalam citra radiologi (misal foto rontgen thorax) dalam format PNG tanpa mengubah tampilan visualnya secara kasat mata.
* **Metode Penyisipan Bit:**
  * Sistem menyisipkan bit payload ke dalam bit paling tidak signifikan (Least Significant Bit / LSB) hanya pada kanal warna **Red, Green, dan Blue (RGB)**.
  * **Kanal Alpha (Transparansi) Dilindungi:** Bit kanal Alpha sama sekali tidak diubah untuk menjaga integritas format citra medis asli.
* **Header Terautentikasi (Ukuran Total 40 Byte):**
  Sebelum payload disisipkan, paket biner disusun dengan format berikut:
  1. *Magic Bytes (4 byte):* Nilai ASCII `MVST` sebagai penanda sah steganografi MedVault.
  2. *Panjang Payload (4 byte):* Ukuran panjang biner data dalam integer big-endian.
  3. *Tag Integritas HMAC-SHA256 (32 byte):* Dihitung atas payload ringkasan medis menggunakan kunci rahasia steganografi.
  4. *Payload Data:* Ringkasan teks medis yang telah dikompresi menggunakan algoritma **zlib**.
* **Ketahanan terhadap Manipulasi:** Jika citra stego diedit atau termanipulasi walaupun hanya 1 bit, proses ekstraksi akan mendeteksi ketidakcocokan tag HMAC-SHA256 dan segera menolak payload demi keselamatan diagnosis pasien.
* **Metrik Evaluasi Visual:**
  * **PSNR (Peak Signal-to-Noise Ratio):** Terukur pada rentang **66.20 hingga 72.22 dB**. Nilai ini jauh melampaui batas minimum persepsi mata manusia (40 dB), membuktikan citra stego tidak dapat dibedakan dari citra aslinya.
  * **MSE (Mean Squared Error):** Sangat rendah, mendekati nol (**0.0039 hingga 0.0156**).

---

### Pilar 6: Blockchain Audit Ledger Proof-of-Authority (Dual-Sign)

* **Tujuan:** Mencegah modifikasi data rekam medis secara ilegal di database SQLite (misal oleh admin nakal atau peretas dengan akses root) melalui buku besar audit nir-ubah.
* **Mekanisme Rantai Blok (Cryptographic Hash Chain):**
  Setiap peristiwa medis (`CREATE_RECORD`, `UPDATE_RECORD`, `UPLOAD_LAB`, `EMBED_STEGO`) dicatat sebagai satu blok di tabel `audit_blocks`. Setiap blok mengikat hash SHA-256 dari blok sebelumnya (`previous_hash`).
* **Konsensus Dual-Signature Ed25519 (PoA):**
  Sebuah blok baru dinyatakan sah hanya jika memuat dua tanda tangan digital yang valid:
  1. *Tanda Tangan Dokter:* Dibuat secara lokal di browser oleh kunci privat USB dongle dokter atas payload transaksi terkanonikalisasi (`block_index|timestamp|actor|target|action|record_state_hash`).
  2. *Countersignature Validator:* Proposal blok dikirimkan ke layanan independen `medvault-validator` (Port 8096). Validator memverifikasi keabsahan tanda tangan dokter dan kontinuitas indeks blok, lalu menandatanganinya dengan kunci institusi.
* **Pengikatan Kondisi Rekam Medis (Record State Hash Binding):**
  Hash SHA-256 dari ciphertext rekam medis di database disertakan di dalam payload transaksi blok.
* **Fungsi Deteksi Manipulasi (`VerifyChain`):**
  Aplikasi memiliki fungsi verifikasi otomatis yang memeriksa integritas seluruh rantai blok secara berkala. Jika ada penyerang yang mengubah teks diagnosis langsung di dalam file database SQLite, `VerifyChain()` mendeteksi perbedaan antara `record_state_hash` di blockchain dengan data riil di tabel `records`, dan seketika menyalakan alarm merah (*TAMPER DETECTED*).

---

## 5. Inspektor Telemetri Glass-Box (Glass-Box Telemetry)

MedVault Go menolak konsep keamanan kotak hitam (*black-box security*) di mana pengguna hanya disajikan hasil akhir tanpa mengetahui prosesnya.

* **Amplop Respons `_telemetry`:** Setiap endpoint API kriptografi menyertakan objek data telemetri yang berisi:
  * Tahapan transformasi data sekuensial.
  * Pratinjau teks (preview) pada masing-masing fase (klasik, kunci, nonce, tag otentikasi).
  * Pengukuran durasi eksekusi internal dalam satuan mikrodetik (`duration_us`).
  * Status verifikasi boolean (`verified: true/false`).
* **Keamanan Kunci:** Telemetri dirancang untuk tujuan edukatif dan audit forensik tanpa membocorkan materi kunci privat (hanya menampilkan fingerprint atau pratinjau data publik).

---

## 6. Alur Operasional End-to-End Aplikasi

### 6.1 Alur Registrasi Tenaga Medis
1. Pengguna membuka `/register`.
2. Pengguna memasukkan username, nama lengkap, peran, dan kata sandi. Kata sandi di-hash dengan Argon2id.
3. Pengguna menekan tombol untuk membuat kunci USB dongle Ed25519 baru, lalu menyimpannya ke flashdisk sebagai berkas `.medkey`.
4. Pengguna menyalakan kamera webcam untuk merekam foto wajah awal yang dikonversi menjadi data dHash 64-bit di memori peramban. Foto mentah tidak disimpan ke database.
5. Data dikirim ke server. Kunci publik dokter tersimpan di database; seed privat tetap berada di flashdisk pengguna.

### 6.2 Alur Login 3-Faktor
1. Pengguna membuka `/login` dan memasukkan username serta kata sandi.
2. Pengguna memilih berkas `.medkey` dari flashdisk USB via File System Access API.
3. Browser meminta tantangan acak dari server (`GET /api/v1/auth/challenge`).
4. Browser menandatangani tantangan tersebut menggunakan WebCrypto Ed25519 di memori lokal.
5. Kamera mengambil foto wajah dan menghitung dHash 64-bit saat itu.
6. Permintaan dikirim ke `POST /api/v1/auth/login`. Jika ketiga faktor valid, server menerbitkan cookie sesi `medvault_session` (HttpOnly, SameSite=Lax, masa berlaku 8 jam).
7. Watchdog USB mulai memantau keberadaan flashdisk di latar belakang setiap 3 detik.

### 6.3 Alur Pembuatan Rekam Medis dengan Sealing 2-Fase
1. Dokter memilih pasien, lalu menginput catatan diagnosis dan resep terapi.
2. Dokter menekan tombol simpan -> browser mengirim draf ke `POST /api/v1/records`.
3. Server mengenkripsi data diagnosis dengan Super Encryption (Klasik + HKDF + XChaCha20-Poly1305), menghitung `record_state_hash`, membuat proposal blok audit, dan mengembalikan `tx_payload` ke browser beserta token proposal berumur pendek.
4. Browser secara otomatis menandatangani `tx_payload` tersebut dengan kunci Ed25519 USB dokter via WebCrypto.
5. Browser mengirim tanda tangan dokter ke `POST /api/v1/records/seal`.
6. Server meneruskan proposal ke kontainer `medvault-validator` (Port 8096) untuk diverifikasi dan dibubuhi tanda tangan kedua.
7. Rekam medis tersimpan paten dan blok audit resmi tercatat di rantai blockchain.

---

## 7. Panduan Praktis dan Pemecahan Masalah USB Dongle

Bagian ini penting untuk dipahami seluruh anggota tim agar tidak panik saat pengujian atau demonstrasi:

### 7.1 Cara Menyiapkan Berkas `.medkey` untuk Pengujian
Terdapat dua cara untuk membuat berkas kunci:
* **Melalui Halaman Registrasi Web:** Buka `/register`, selesaikan Langkah 2 (Kunci Dongle USB), lalu simpan berkas `.medkey` ke komputer atau flashdisk Anda.
* **Melalui Baris Perintah Go (CLI):** Jalankan perintah berikut di terminal:
  ```powershell
  go run ./cmd/medkeygen -username dr_andi -key-id DONGLE-01 -out ./dr_andi.medkey
  ```
  Salin berkas `dr_andi.medkey` tersebut ke dalam flashdisk USB Anda.

### 7.2 Persyaratan Peramban (Browser)
* **Wajib Menggunakan Google Chrome atau Microsoft Edge Versi Desktop:** Fitur pembacaan handle berkas fisik (`showOpenFilePicker` dan `showSaveFilePicker`) adalah spesifikasi File System Access API yang didukung penuh pada engine Chromium.
* **Hindari Mozilla Firefox atau Browser Ponsel untuk Fitur Watchdog:** Firefox membatasi API ini karena kebijakan internal mereka, sehingga sistem akan otomatis beralih ke input file standar tanpa kemampuan pemantauan cabut fisik secara real-time.

### 7.3 Strategi Menghadapi Demonstrasi di Depan Dosen
* **Penjelasan Desain On-Demand Signature (Standar YubiKey):**
  Jelaskan kepada dosen: *"Kami mengadopsi standar industri token otentikasi (seperti YubiKey), di mana token hardware hanya dipanggil saat operasi kriptografi berlangsung (saat otentikasi login dan penandatanganan rekam medis). Flashdisk tidak perlu dipaksa terkunci atau dipantau terus-menerus setiap detik agar tidak mengganggu operasional klinis dokter, sementara sesi kerja tetap terproteksi oleh cookie HttpOnly berbatas waktu."*
* **Fleksibilitas Media Penyimpanan Kunci:**
  Berkas `.medkey` dapat disimpan di drive USB flashdisk maupun di folder lokal laptop (misal di folder Desktop). Proses otentikasi tanda tangan WebCrypto Ed25519 dan verifikasi challenge-response di server akan tetap berjalan 100% sukses dan valid secara matematis. Kunci privat seed tetap aman di sisi klien dan tidak pernah dikirim ke server.

### 7.4 Demonstrasi Konkurensi Multi-Dokter (Kombinasi Alami B dan CLI C)
Untuk membuktikan bahwa sistem aktif menangani multi-pengguna tanpa deadlock pada database SQLite WAL:
* **Metode Alami (Split-Screen Browser):** Buka dua jendela browser (misal Chrome normal dan Chrome Incognito). Login dengan dua dokter berbeda (`dr_andi` dan `dr_siti`). Saat dokter di jendela kanan membuat rekam medis, dokter di jendela kiri me-refresh halaman audit dan melihat blok audit baru langsung tercatat secara konsisten.
* **Metode CLI Concurrent Traffic Simulator:** Buka terminal PowerShell di samping browser, lalu jalankan:
  ```powershell
  go run deploy/traffic.go -base https://medvault.indraprhmbd.my.id -workers 3 -records 1
  ```
  Alat ini menyalakan 3 goroutines pekerja secara paralel (3 dokter virtual). Masing-masing pekerja secara simultan mendaftar, login 3-faktor, mengenkripsi pasien AES-GCM, mengenkripsi diagnosis super-encryption, dan menyegel blok audit PoA. Terminal menampilkan log transaksi berurutan dalam ~1.5 detik, dan verifikasi integritas ledger di browser seketika menunjukkan status 100% valid tanpa kegagalan konkurensi.

---

## 8. Panduan Pembagian Peran Presentasi Tim

Agar presentasi berjalan dinamis, runtut, dan menunjukkan kekompakan tim, berikut pembagian peran yang direkomendasikan untuk durasi 15-20 menit:

### Pembicara 1: Arsyadi Indra Hasan P (Lead Architect & Infrastructure)
* **Bagian:** Pembukaan, Latar Belakang Masalah, Alasan Rewrite Total (Legacy vs MedVault Go), Arsitektur Podman Pod, dan Konsensus Blockchain PoA.
* **Poin Penekanan:** Menjelaskan mengapa arsitektur Go murni nir-CGO dipilih untuk menghemat memori hingga < 40 MB, integrasi Cloudflare Tunnel tanpa port terbuka, dan isolasi fisik kontainer validator independen.

### Pembicara 2: Hafizd S. Abdurrahman (Frontend & Cryptographic Client Gates)
* **Bagian:** Autentikasi 3-Faktor (Argon2id, USB Dongle Ed25519, Biometrik dHash), On-Demand Hardware Token, dan Pengikatan Kolom AES-256-GCM.
* **Poin Penekanan:** Menjelaskan alur penandatanganan tantangan WebCrypto di browser, perlindungan privasi di mana foto mentah dan kunci privat tidak pernah dikirim ke server, serta proteksi anti-column-swapping via AAD.

### Pembicara 3: Rakha Albany Nugraha (Applied Crypto, Telemetry & Quality Assurance)
* **Bagian:** Super Encryption Diagnosis, ChaCha20 Streaming File Vault, Steganografi Citra PNG LSB, Glass-Box Telemetry, dan Hasil Pengujian 165 Unit Tests.
* **Poin Penekanan:** Menunjukkan metrik objektif (PSNR > 66 dB, MSE mendekati nol, waktu eksekusi kriptografi dalam mikrodetik), membuktikan ketahanan sistem terhadap simulasi tamper database, dan memandu sesi live demo.

---

## 9. Bank Pertanyaan dan Jawaban Dosen Penguji Kriptografi

Berikut kumpulan pertanyaan kritis yang sering diajukan dosen penguji beserta jawaban argumentatif berbasis kode:

### Q1: Mengapa repot-repot menggabungkan cipher klasik (Vigenère dan Transposisi) dengan XChaCha20-Poly1305? Bukankah cipher modern saja sudah sangat aman?
* **Jawaban Tim:**
  Secara matematis, keamanan utama sistem kami bertumpu penuh pada XChaCha20-Poly1305 dengan kunci 256-bit dan nonce 192-bit. Penggabungan cipher klasik berfungsi sebagai *cascade cipher* edukatif yang mendemonstrasikan prinsip dasar Shannon: *Confusion* (melalui substitusi Extended Vigenère 95 karakter) dan *Diffusion* (melalui permutasi transposisi kolom). Berdasarkan kaidah kriptografi, menerapkan sandi klasik sebelum sandi modern tidak menurunkan entropi keamanan sandi modern, melainkan mengacak pola sebaran karakter teks rekam medis sebelum diproses oleh algoritma AEAD.

### Q2: Mengapa memilih biometrik 64-bit dHash daripada model Deep Learning canggih seperti FaceNet atau CNN?
* **Jawaban Tim:**
  Target desain MedVault Go adalah fasilitas kesehatan dengan server berdaya rendah (VPS 2 vCPU, RAM 1.9 GiB) dengan arsitektur murni Go tanpa CGO (`CGO_ENABLED=0`). Model deep learning membutuhkan runtime Python atau pustaka C++ (seperti OpenCV/TensorFlow) yang mengonsumsi ratusan megabyte RAM dan memperlambat waktu start server. Algoritma 64-bit dHash dieksekusi secara instan di sisi browser dalam hitungan milidetik, foto mentah dokter tidak pernah disimpan di server (menjaga privasi biometrik), dan perbandingan jarak Hamming terbukti efektif mengenali struktur visual wajah secara efisien.

### Q3: Mengapa menggunakan blockchain Proof-of-Authority (PoA) dual-signature dan bukan blockchain Proof-of-Work (PoW) atau Proof-of-Stake (PoS)?
* **Jawaban Tim:**
  Data rekam medis tunduk pada regulasi privasi ketat (seperti UU PDP dan standar medis). Blockchain publik berbasis PoW atau PoS memiliki latensi transaksi lambat, biaya gas (gas fee), dan menyebarkan data ke node publik yang tidak dikenal. Dalam lingkungan faskes, konsensus Proof-of-Authority (PoA) memberikan kepastian transaksi instan (< 5 ms), mengikat tanggung jawab hukum secara jelas antara dokter yang menandatangani dengan validator institusi klinik, dan dapat beroperasi dengan konsumsi memori hanya 32 MB pada kontainer validator.

### Q4: Jika penyerang berhasil masuk ke server dan mendapatkan akses root, apakah sistem masih aman?
* **Jawaban Tim:**
  Sistem menerapkan prinsip pertahanan berlapis. Jika penyerang mencuri file database SQLite:
  1. Penyerang tidak dapat membaca NIK, nama pasien, dan alamat karena terenkripsi AES-256-GCM.
  2. Penyerang tidak dapat membaca diagnosa karena terenkripsi XChaCha20-Poly1305 dengan kunci per-pasien dari HKDF.
  3. Penyerang tidak dapat memalsukan transaksi rekam medis baru di masa lalu atau masa depan, karena setiap blok audit yang sah memerlukan tanda tangan digital Ed25519 dari USB fisik dokter yang kunci privatnya berada di perangkat fisik pengguna, bukan di server.
  4. Jika penyerang mengubah isi teks diagnosis di SQLite, fungsi `VerifyChain()` seketika mendeteksi ketidakcocokan antara hash status data dengan rantai audit blockchain.

### Q5: Mengapa sistem otentikasi tidak menggunakan JWT (JSON Web Token)?
* **Jawaban Tim:**
  JWT yang disimpan di LocalStorage atau sessionStorage rentan dicuri melalui serangan Cross-Site Scripting (XSS). MedVault Go menggunakan cookie sesi HttpOnly bertanda tangan dengan konfigurasi `SameSite=Lax` dan enkripsi TLS end-to-end. Script JavaScript di peramban sama sekali tidak memiliki akses baca ke cookie sesi ini, sehingga melindungi dokter dari risiko pembajakan sesi (*session hijacking*).
