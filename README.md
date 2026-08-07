<p align="center">
  <img src="frontend/src/assets/images/scanify-logo.png" alt="Scanify" width="128">
</p>

<h1 align="center">Scanify</h1>

<p align="center">
  Aplikasi desktop pemindai dokumen yang cepat, lokal, dan portabel untuk Windows.
</p>

Scanify adalah aplikasi open-source berbasis Go dan Wails untuk memindai, menyusun, dan mengekspor dokumen melalui Windows Image Acquisition (WIA). Antarmukanya menggunakan React dan seluruh aplikasi produksi dikemas menjadi satu file EXE.

> Scanify v2 merupakan migrasi dari aplikasi WPF/.NET. Fitur unggah SIREKA belum disertakan dalam versi ini.

## Fitur

- Mendeteksi dan memilih scanner WIA yang terhubung.
- Mode warna, abu-abu, dan hitam putih.
- Resolusi 150, 300, dan 600 DPI.
- Galeri pratinjau hasil scan.
- Pemilihan halaman dengan urutan ekspor yang terlihat.
- Penghapusan halaman dengan pemadatan ulang nomor urut.
- Ekspor setiap halaman sebagai JPG yang valid.
- Penggabungan halaman terpilih menjadi PDF A4.
- Nama file unik sehingga hasil lama tidak ditimpa.
- Pembersihan file sesi sementara saat aplikasi ditutup.
- Antarmuka berbahasa Indonesia dengan tema gelap.

## Kompatibilitas

| Komponen | Dukungan |
| --- | --- |
| Sistem operasi | Windows 10/11 x64 |
| Scanner | Perangkat dengan driver WIA |
| UI runtime | Microsoft Edge WebView2 |
| Perangkat teruji | Canon MP280 series |

Binary rilis menyertakan bootstrapper WebView2. Jika WebView2 Runtime belum tersedia, Windows dapat meminta pemasangannya sebelum aplikasi dijalankan.

## Unduh dan gunakan

1. Buka halaman **Releases** repositori.
2. Unduh `Scanify-vX.Y.Z-windows-amd64.exe`.
3. Cocokkan SHA-256 file dengan `checksums.txt` jika diperlukan.
4. Pastikan driver WIA scanner sudah terpasang dan scanner sudah menyala.
5. Jalankan EXE, pilih scanner dan pengaturan, lalu tekan **Mulai scan**.

Tidak ada installer dan tidak ada layanan latar belakang. Hasil scan hanya disimpan ke lokasi yang dipilih pengguna.

## Pengembangan

### Prasyarat

- Windows 10/11 x64
- Go sesuai versi pada `go.mod`
- Node.js 24 atau versi LTS kompatibel
- Wails CLI v2.12.0
- Driver WIA untuk pengujian perangkat fisik

Pasang Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

Pasang dependensi dan jalankan aplikasi:

```powershell
git clone <URL-REPOSITORI-ANDA>
cd scanify
npm ci --prefix frontend
wails dev
```

## Pengujian

Jalankan seluruh tes yang tidak memerlukan perangkat fisik:

```powershell
go test ./...
npm test --prefix frontend
```

Uji deteksi scanner fisik:

```powershell
$env:SCANIFY_HARDWARE_TEST='1'
go test -run TestWIAHardwareEnumeration -v
```

Uji berikut benar-benar menggerakkan scanner satu kali pada mode warna 300 DPI:

```powershell
$env:SCANIFY_SCAN_HARDWARE_TEST='1'
go test -run TestWIAHardwareScan -v
```

## Build lokal

```powershell
wails build -clean -trimpath -platform windows/amd64 -webview2 embed -o scanify.exe
```

Binary akan dibuat di `build/bin/scanify.exe`.

## Rilis dan versioning

Rilis menggunakan [Semantic Versioning](https://semver.org/) dengan format tag `vMAJOR.MINOR.PATCH`. Workflow `.github/workflows/release.yml` hanya dipicu oleh tag dan akan:

1. Memvalidasi format tag.
2. Menjalankan tes Go dan React.
3. Memasukkan versi tag ke judul aplikasi dan metadata Windows.
4. Membangun EXE Windows x64 portabel.
5. Menghasilkan checksum SHA-256.
6. Membuat GitHub Release dan mengunggah artefaknya.

Contoh membuat rilis:

```powershell
git tag -a v2.0.0 -m "Release v2.0.0"
git push origin v2.0.0
```

Tag selain format seperti `v2.0.0` akan ditolak oleh workflow.

## Arsitektur

```text
React/TypeScript UI
        |
   Wails bindings
        |
Go application service
   |             |
WIA worker    Session/exporter
(COM STA)     (JPG dan PDF)
```

Komponen penting:

- `wia_windows.go` — komunikasi COM/WIA pada satu thread Windows khusus.
- `session.go` — halaman sementara, pilihan, urutan, dan pembersihan.
- `exporter.go` — ekspor JPG serta pembuatan dan validasi PDF.
- `frontend/src/App.tsx` — antarmuka React berbahasa Indonesia.

## Kontribusi

Kontribusi, laporan bug, dan usulan fitur dipersilakan.

1. Fork repositori.
2. Buat branch perubahan, misalnya `feature/adf-support`.
3. Tambahkan atau perbarui tes yang relevan.
4. Pastikan `go test ./...` dan `npm test --prefix frontend` berhasil.
5. Buat pull request dengan penjelasan perubahan dan cara mengujinya.

Untuk masalah scanner, sertakan versi Windows, nama perangkat, versi driver WIA, mode warna, DPI, dan pesan error lengkap. Jangan unggah dokumen hasil scan yang mengandung data pribadi.

## Privasi

Pemindaian dan ekspor dilakukan secara lokal. Scanify tidak mengunggah dokumen, telemetri, atau data scanner ke server. Koneksi internet hanya mungkin diperlukan oleh bootstrapper ketika WebView2 Runtime belum terpasang.

## Lisensi

Lisensi open-source belum ditetapkan. Sebelum publikasi, tambahkan file `LICENSE`; MIT direkomendasikan untuk penggunaan dan kontribusi yang sederhana serta permisif.
