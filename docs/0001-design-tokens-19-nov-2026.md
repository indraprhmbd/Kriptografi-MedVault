# 0001 - Design Tokens dari Figma

Tanggal: 19 Nov 2026
Status: Active
Sumber: file Figma `MedVault - Redesign Dashboard UI`, file key `Wt4wwa2QJOIPVWbhesTjgX`, version `2423131685551667227`.

## Tujuan

Menyatukan nilai visual Figma ke dalam satu berkas token supaya halaman HTML tidak memakai warna, radius, atau durasi hasil tebakan. Berkas token juga menjadi jembatan sebelum halaman-halaman `web/pages/` ditulis ulang mengikuti desain.

## Lokasi

| File | Peran |
| :--- | :--- |
| `web/assets/tokens.css` | Sumber nilai CSS custom properties dengan prefiks `--mv-ui-`. |
| `web/layout.html` | Menautkan `tokens.css` dan mendaftarkan alias `--color-clinical-*` di dalam blok `@theme` Tailwind. |

`tokens.css` berada di bawah `/assets/`, sehingga sudah termasuk dalam `isStaticAsset` di `web/embed.go` dan otomatis mendapat query `?v={{.AssetVersion}}`. Tidak ada CDN yang ditambahkan.

## Ringkasan nilai

| Kelompok | Nilai utama |
| :--- | :--- |
| Canvas dan surface | `#f5faf8`, `#ffffff`, `#f7fbfa`, `#f1f8f6` |
| Gradient sidebar dan panel ledger | `#04302c` 0 persen, `#064e46` 50 persen, `#0b6b60` 100 persen |
| Ink | `#102a26`, `#5b716d`, `#9fc4bd`, `#7a908c`, `#d5f0eb` |
| Akcen | `#0f9f91`, `#087f75`, `#075e57`, `#2dd4bf`, `#99f6e4`, `#5eead4` |
| Border | `#dbe7e3`, `#b7ddd6` |
| Success | `#166534`, `#ecfdf3`, `#bbf7d0`, `#86efac` |
| Danger | `#9f1239`, `#e11d48`, `#fff1f2`, `#fecdd3` |
| Radius | 8, 10, 12, 14, 16, 18, 20, 24, 28, 999 px |
| Font | Inter dengan fallback sistem, weight 400, 500, 600, 700 |
| Ukuran teks | 11, 12, 13, 14, 15, 18, 20, 22, 24, 30, 32, 34 px |
| Layout 1440 | sidebar 264 px, inset 24 px, main offset 312 px, main 1104 px, gap 24 px, gap kartu 16 px |
| Motion | standard `cubic-bezier(0.4, 0, 0.2, 1)`, enter `cubic-bezier(0, 0, 0.2, 1)`, exit `cubic-bezier(0.4, 0, 1, 1)`, stagger 60 ms, total 700 ms |

## Keputusan

1. Prefiks `--mv-ui-` dipakai supaya token dashboard tidak menimpa token `--mv-*` yang dipakai halaman autentikasi di `layout.html`.
2. Font Inter tidak di-vendor. Token menyimpan nama Inter lebih dulu, lalu fallback ke font sistem. Ini menjaga aturan zero CDN di context/008. Jika kemudian ingin tipografi persis Figma, berkas font harus diunduh dan ditempatkan di `web/assets/` terlebih dahulu.
3. Durasi motion diubah menjadi `0ms` saat `prefers-reduced-motion: reduce`, mengikuti Motion & Interaction Spec frame `3:2`.
4. Gradient didefinisikan sekali sebagai kelas `.mv-ui-gradient-panel` karena sidebar dan panel ledger memakai tiga stop yang identik.

## Validasi

Jalankan `go test ./web/...` setelah perubahan. `assetVersion` akan menghitung ulang digest karena `/assets/tokens.css` ikut dipindai, sehingga URL berkas berubah otomatis setelah deploy.
