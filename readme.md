# GOWA - WhatsApp Bot API

<div align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white" alt="WhatsApp">
  <img src="https://img.shields.io/badge/WebSocket-010101?style=for-the-badge&logo=socket.io&logoColor=white" alt="WebSocket">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="MIT License">
</div>

<p align="center">
  <strong>WhatsApp Bot dengan Real-Time Chat Room, WebSocket, dan REST API</strong><br>
  Bot WhatsApp modern dengan antarmuka web, dukungan grup, media, dan notifikasi real-time.
</p>

---

## 📋 Daftar Isi

- [Fitur Utama](#-fitur-utama)
- [Teknologi yang Digunakan](#-teknologi-yang-digunakan)
- [Instalasi](#-instalasi)
- [Konfigurasi](#-konfigurasi)
- [Menjalankan Aplikasi](#-menjalankan-aplikasi)
- [Struktur Proyek](#-struktur-proyek)
- [API Endpoints](#-api-endpoints)
- [WebSocket](#-websocket)
- [Database](#-database)
- [Troubleshooting](#-troubleshooting)
- [Lisensi](#-lisensi)

---

## 🚀 Fitur Utama


| Fitur                      | Deskripsi                                                                 |
| -------------------------- | ------------------------------------------------------------------------- |
| **🔐 Autentikasi QR Code** | Login WhatsApp dengan scan QR code sekali, sesi tersimpan secara permanen |
| **💬 Real-Time Chat**      | Kirim dan terima pesan secara instan dengan WebSocket                     |
| **📱 Chat Room Web**       | Antarmuka web mirip WhatsApp Web untuk mengelola percakapan               |
| **📊 REST API**            | Endpoint lengkap untuk integrasi dengan sistem lain                       |
| **👥 Grup & Kontak**       | Kelola grup, lihat anggota, dan daftar kontak                             |
| **🖼️ Media Support**     | Kirim dan terima gambar, dokumen, stiker                                  |
| **🔔 Notifikasi**          | Notifikasi desktop dan suara saat pesan baru masuk                        |
| **📦 Database SQLite**     | Penyimpanan pesan dan kontak dengan performa ringan                       |
| **🔄 Auto-Reconnect**      | Koneksi otomatis pulih jika terputus                                      |

---

## 🛠️ Teknologi yang Digunakan


| Teknologi                                                 | Versi  | Kegunaan                      |
| --------------------------------------------------------- | ------ | ----------------------------- |
| [Go](https://golang.org/)                                 | 1.21+  | Bahasa pemrograman utama      |
| [whatsmeow](https://github.com/tulir/whatsmeow)           | latest | Library WhatsApp Multi-Device |
| [Gin](https://gin-gonic.com/)                             | v1.9+  | Web framework                 |
| [Gorilla WebSocket](https://github.com/gorilla/websocket) | v1.5+  | WebSocket server              |
| [SQLite](https://www.sqlite.org/)                         | 3      | Database penyimpanan pesan    |
| [modernc.org/sqlite](https://modernc.org/sqlite)          | latest | SQLite driver pure Go         |

---

## 📦 Instalasi

### Prasyarat

- **Go** 1.21 atau lebih baru ([Download](https://golang.org/dl/))
- **Git** ([Download](https://git-scm.com/))
- Akun WhatsApp aktif

### Langkah Instalasi

```bash
# 1. Clone repository
git clone https://github.com/github.com/tbintang889/go-wa.git
cd gowa

# 2. Download dependencies
go mod download
go mod tidy

# 3. Build aplikasi (opsional)
go build -o gowa.exe

# 4. Jalankan aplikasi
go run main.go
```
