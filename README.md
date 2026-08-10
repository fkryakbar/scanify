<p align="center">
  <img src="frontend/src/assets/images/scanify-logo.png" alt="Logo Scanify" width="128">
</p>

<h1 align="center">Scanify</h1>

<p align="center">
  Pindai, susun, simpan, dan kirim dokumen dari satu aplikasi Windows.
</p>

Scanify membantu Anda mengubah dokumen kertas menjadi file digital dengan lebih praktis. Hasil pemindaian dapat ditinjau dan disusun terlebih dahulu, kemudian disimpan sebagai JPG atau PDF. Dokumen juga dapat langsung dikirim ke <a href="https://arsipin.fkr.web.id">Workspace Arsipin</a> tanpa perlu mengunggahnya secara manual melalui browser.

## Yang dapat dilakukan

- Memindai dokumen menggunakan scanner yang mendukung WIA.
- Memilih hasil berwarna, abu-abu, atau hitam putih.
- Mengatur kualitas pemindaian ke 150, 300, atau 600 DPI.
- Meninjau, memilih, mengurutkan, dan menghapus halaman sebelum disimpan.
- Menyimpan setiap halaman sebagai JPG atau menggabungkannya menjadi satu PDF A4.
- Mengirim halaman terpilih langsung ke Workspace Arsipin.
- Memeriksa ketersediaan versi baru secara otomatis.
- Menjaga proses pemindaian tetap lokal sampai Anda memilih untuk mengunggah dokumen.

## Mulai menggunakan Scanify

1. Buka halaman [Releases](https://github.com/fkryakbar/scanify/releases).
2. Unduh file `Scanify-vX.Y.Z-windows-amd64.exe` dari versi terbaru.
3. Pastikan scanner sudah menyala dan driver-nya telah terpasang.
4. Jalankan Scanify, lalu pilih scanner, warna, dan kualitas pemindaian.
5. Tekan **Mulai scan**.
6. Tinjau hasilnya, pilih halaman yang dibutuhkan, lalu simpan sebagai JPG/PDF atau kirim ke Arsipin.

Scanify bersifat portabel sehingga tidak memerlukan proses instalasi. Pada komputer yang belum memiliki Microsoft Edge WebView2 Runtime, Windows mungkin akan meminta pemasangannya saat Scanify pertama kali dijalankan.

## Hubungkan dengan Workspace Arsipin

Scanify dapat terhubung ke workspace yang Anda gunakan di [arsipin.fkr.web.id](https://arsipin.fkr.web.id). Siapkan dua informasi dari Workspace Arsipin Anda:

- **Workspace API Link**
- **Password workspace**

Kemudian hubungkan workspace dengan langkah berikut:

1. Di Scanify, klik ikon **gear/pengaturan** di samping tombol **Upload ke Arsipin**.
2. Tempel **Workspace API Link** ke kolom **URL upload lengkap**.
3. Masukkan **password workspace**.
4. Klik **Simpan konfigurasi**.

Setelah tersambung, pilih halaman hasil scan yang ingin dikirim dan klik **Upload ke Arsipin**. Scanify akan menggabungkan halaman tersebut menjadi satu PDF dan mengirimkannya ke antrean arsip pada workspace Anda.

> Workspace API Link berbeda untuk setiap workspace. Salin link secara lengkap dari Workspace Arsipin dan jangan membagikan password workspace kepada orang lain.

## Kebutuhan perangkat

| Kebutuhan | Keterangan |
| --- | --- |
| Sistem operasi | Windows 10 atau Windows 11 (64-bit) |
| Scanner | Scanner dengan driver WIA |
| Komponen tampilan | Microsoft Edge WebView2 Runtime |
| Scanner yang telah diuji | Canon MP280 series |

## Privasi dan penyimpanan data

Pemindaian, penyusunan halaman, dan pembuatan file dilakukan di komputer Anda. Dokumen hanya dikirim ke Arsipin ketika Anda menekan **Upload ke Arsipin**.

Workspace API Link dan password workspace disimpan di komputer pada `%APPDATA%\Scanify\arsipin.json`. Password tersimpan sebagai teks biasa, jadi pastikan akun Windows dan perangkat Anda tidak digunakan oleh pihak yang tidak berwenang. File hasil scan sementara akan dibersihkan ketika aplikasi ditutup.

Scanify tidak mengirim telemetri atau informasi scanner. Koneksi internet digunakan untuk mengunggah dokumen ke Arsipin, memeriksa pembaruan aplikasi, dan memasang WebView2 Runtime jika komponen tersebut belum tersedia.

## Untuk pengembang

Scanify dibuat dengan Go, Wails, dan React. Untuk menjalankan proyek dari kode sumber, siapkan Go sesuai versi pada `go.mod`, Node.js, Wails CLI v2.12.0, serta driver WIA.

```powershell
git clone https://github.com/fkryakbar/scanify.git
cd scanify
npm ci --prefix frontend
wails dev
```

Menjalankan pengujian:

```powershell
npm test --prefix frontend
npm run build --prefix frontend
go test ./...
```

Membuat aplikasi Windows:

```powershell
wails build -clean -trimpath -platform windows/amd64 -webview2 embed -o scanify.exe
```

Hasil build tersedia di `build/bin/scanify.exe`.

## Kontribusi

Laporan bug dan usulan fitur dipersilakan melalui [GitHub Issues](https://github.com/fkryakbar/scanify/issues). Untuk kendala scanner, sertakan versi Windows, nama perangkat, versi driver, mode warna, DPI, dan pesan error. Jangan menyertakan dokumen hasil scan yang berisi data pribadi.

## Lisensi

Lisensi proyek belum ditetapkan. Tambahkan file `LICENSE` sebelum mendistribusikan atau menggunakan kode sumber di luar ketentuan yang diberikan pemilik repositori.
