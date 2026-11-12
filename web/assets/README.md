# MedVault Web Assets

Letakkan aset visual di folder ini. Semua berkas di sini ikut di-embed ke binary
Go dan disajikan dari URL `/assets/...`, jadi tidak perlu build tool atau CDN.

## Halaman autentikasi

Halaman masuk (`web/index.html`) dan halaman daftar dokter (`web/register.html`)
memakai `maskot.svg` di folder ini sebagai logo kecil di header dan sebagai
maskot di panel samping. Berkas disajikan dari URL `/assets/maskot.svg` dan
gambar cadangan teks tetap muncul bila aset gagal dimuat. Seluruh ikon digambar
sebagai SVG inline di masing-masing halaman, jadi tidak ada aset dari CDN dan
tidak ada ikon yang rusak saat aplikasi dipakai tanpa internet. Header dashboard
di `web/layout.html` juga memakai `maskot.svg` sebagai logo.

## Catatan

- Nama berkas harus sama persis. Server VPS Linux membedakan huruf besar dan kecil.
- Jangan memakai spasi pada nama berkas.
- Ukuran berkas sebaiknya di bawah 1 MB per gambar agar hemat memori VPS.
- Aset lain boleh ditambahkan di folder ini dan diakses lewat `/assets/nama-berkas`.
