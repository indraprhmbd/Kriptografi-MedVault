# MedVault Web Assets

Letakkan aset visual di folder ini. Semua berkas di sini ikut di-embed ke binary
Go dan disajikan dari URL `/assets/...`, jadi tidak perlu build tool atau CDN.

## Berkas opsional yang sudah terhubung ke halaman login

| Nama berkas | Dipakai di | Rekomendasi |
| :--- | :--- | :--- |
| `login-hero.jpg` | Panel kanan halaman login sebagai foto latar | Lanskap atau potret, minimal 1200 x 1600 px, gelap atau bernuansa teal agar teks tetap terbaca. Format JPG. |
| `doctor.png` | Kartu kecil "Dokter terverifikasi" di panel kanan | Foto dokter dengan latar transparan, rasio 3:4, minimal 600 x 800 px. Format PNG. |
| `mascot.png` | Badge maskot di kanan atas panel login | Maskot dengan latar transparan, rasio 1:1, minimal 512 x 512 px. Format PNG. |

Jika salah satu berkas belum ada, halaman tetap tampil rapi: gambar akan
disembunyikan otomatis dan panel memakai gradasi serta ikon bawaan.

## Catatan

- Nama berkas harus sama persis. Server VPS Linux membedakan huruf besar dan kecil.
- Jangan memakai spasi pada nama berkas.
- Ukuran berkas sebaiknya di bawah 1 MB per gambar agar hemat memori VPS.
- Aset lain boleh ditambahkan di folder ini dan diakses lewat `/assets/nama-berkas`.
