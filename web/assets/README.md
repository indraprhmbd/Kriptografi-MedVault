# MedVault Web Assets

Letakkan aset visual di folder ini. Semua berkas di sini ikut di-embed ke binary
Go dan disajikan dari URL `/assets/...`, jadi tidak perlu build tool atau CDN.

## Halaman login

Halaman login tidak memakai berkas gambar. Maskot dan seluruh ikon digambar
sebagai SVG inline di `web/index.html`, sehingga tampilan tetap utuh saat
aplikasi dipakai tanpa internet dan tidak ada gambar rusak.

## Catatan

- Nama berkas harus sama persis. Server VPS Linux membedakan huruf besar dan kecil.
- Jangan memakai spasi pada nama berkas.
- Ukuran berkas sebaiknya di bawah 1 MB per gambar agar hemat memori VPS.
- Aset lain boleh ditambahkan di folder ini dan diakses lewat `/assets/nama-berkas`.
