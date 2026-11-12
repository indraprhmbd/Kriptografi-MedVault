# MedVault Web Assets

Letakkan aset visual di folder ini. Semua berkas di sini ikut di-embed ke binary
Go dan disajikan dari URL `/assets/...`, jadi tidak perlu build tool atau CDN.

## Halaman login

Maskot halaman login memakai `maskot.svg` di folder ini dan disajikan dari URL
`/assets/maskot.svg`. Seluruh ikon digambar sebagai SVG inline di
`web/index.html`, jadi tidak ada aset dari CDN dan tidak ada ikon yang rusak saat
aplikasi dipakai tanpa internet.

## Catatan

- Nama berkas harus sama persis. Server VPS Linux membedakan huruf besar dan kecil.
- Jangan memakai spasi pada nama berkas.
- Ukuran berkas sebaiknya di bawah 1 MB per gambar agar hemat memori VPS.
- Aset lain boleh ditambahkan di folder ini dan diakses lewat `/assets/nama-berkas`.
