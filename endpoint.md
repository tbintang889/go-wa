**Bagus!** Sekarang Anda bisa melihat semua 19 endpoint yang aktif. Berikut daftar lengkapnya:

## Endpoint yang Tersedia:

### **Core & Auth**


| Method | Endpoint           | Fungsi                                 |
| ------ | ------------------ | -------------------------------------- |
| GET    | `/`                | Dashboard QR Code                      |
| GET    | `/api/qr`          | Get QR Code login                      |
| GET    | `/api/status`      | Status koneksi                         |
| GET    | `/api/status-full` | Status lengkap (connected + logged in) |
| GET    | `/ws`              | WebSocket connection                   |

### **Chat & Messaging**


| Method | Endpoint               | Fungsi                       |
| ------ | ---------------------- | ---------------------------- |
| GET    | `/chat`                | Chat Room UI                 |
| POST   | `/api/send-text`       | Kirim pesan teks             |
| POST   | `/api/send-message`    | Kirim pesan (alternatif)     |
| POST   | `/api/send-media`      | Kirim media (gambar/dokumen) |
| GET    | `/api/messages/:jid`   | Ambil riwayat pesan          |
| GET    | `/api/delivery-status` | Cek status pengiriman        |

### **Contacts & Groups**


| Method | Endpoint             | Fungsi                 |
| ------ | -------------------- | ---------------------- |
| GET    | `/api/chats`         | Daftar chat/kontak     |
| GET    | `/api/contacts`      | Daftar kontak WhatsApp |
| GET    | `/api/groups`        | Daftar grup            |
| GET    | `/api/group-members` | Anggota grup           |

### **Media & Debug**


| Method | Endpoint           | Fungsi                    |
| ------ | ------------------ | ------------------------- |
| GET    | `/api/media/:id`   | Ambil file media (gambar) |
| GET    | `/api/debug/chats` | Debug daftar chat         |
| GET    | `/api/routes`      | Lihat semua endpoint      |

### **Webhook**


| Method | Endpoint            | Fungsi                   |
| ------ | ------------------- | ------------------------ |
| POST   | `/webhook/incoming` | Webhook untuk sistem CRM |

## Cara Cek Endpoint Tertentu:

```bash
# Cek status
curl http://localhost:8080/api/status

# Cek daftar chat
curl http://localhost:8080/api/chats

# Cek kontak
curl http://localhost:8080/api/contacts

# Cek grup
curl http://localhost:8080/api/groups

# Kirim pesan
curl -X POST http://localhost:8080/api/send-text \
  -H "Content-Type: application/json" \
  -d '{"to":"628123456789","text":"halo"}'
```

## Catatan:

- **Total 19 endpoint** aktif
- **WebSocket** di `/ws` untuk real-time
- **Media** di `/api/media/:id` untuk akses gambar
- **Webhook** di `/webhook/incoming` untuk integrasi eksternal

Semua endpoint siap digunakan! 🚀

https://chat.deepseek.com/share/nrjr0a6xckpaapde06
