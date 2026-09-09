package database

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)


var DB *sql.DB

// InitDB - inisialisasi database dengan auto migrasi
func InitDB(dbPath string) error {
    var err error
    DB, err = sql.Open("sqlite", dbPath+"?_timeout=5000&_busy_timeout=5000")
    if err != nil {
        return err
    }
    
    DB.SetMaxOpenConns(1)
    
    // Buat tabel messages
    createTableSQL := `
    CREATE TABLE IF NOT EXISTS messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        from_jid TEXT NOT NULL,
        to_jid TEXT NOT NULL,
        content TEXT,
        media_path TEXT,
        media_type TEXT,
        is_from_me BOOLEAN DEFAULT 0,
        status TEXT DEFAULT 'pending',
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE INDEX IF NOT EXISTS idx_messages_from_jid ON messages(from_jid);
    CREATE INDEX IF NOT EXISTS idx_messages_to_jid ON messages(to_jid);
    CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
    CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
    `
    
    _, err = DB.Exec(createTableSQL)
    if err != nil {
        return err
    }
    
    // Cek dan tambahkan kolom media_type jika belum ada
    var hasMediaType int
    DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'media_type'").Scan(&hasMediaType)
    if hasMediaType == 0 {
        _, err = DB.Exec("ALTER TABLE messages ADD COLUMN media_type TEXT")
        if err != nil {
            return err
        }
        println("✅ Added media_type column")
    }
    
    // Cek dan tambahkan kolom media_path jika belum ada
    var hasMediaPath int
    DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'media_path'").Scan(&hasMediaPath)
    if hasMediaPath == 0 {
        _, err = DB.Exec("ALTER TABLE messages ADD COLUMN media_path TEXT")
        if err != nil {
            return err
        }
        println("✅ Added media_path column")
    }
    
    // Bersihkan entri pesan status, broadcast, dan newsletter (saluran) yang tersimpan sebelumnya
    _, _ = DB.Exec(`
        DELETE FROM messages WHERE 
        from_jid LIKE '%@broadcast' OR to_jid LIKE '%@broadcast' OR 
        from_jid LIKE '%@newsletter' OR to_jid LIKE '%@newsletter' OR 
        from_jid LIKE 'status@%' OR to_jid LIKE 'status@%' OR
        from_jid LIKE '%@call' OR to_jid LIKE '%@call';
    `)

    return nil
}

// Update struct Message
type Message struct {
    ID        int       `json:"id"`
    FromJID   string    `json:"from_jid"`
    ToJID     string    `json:"to_jid"`
    Content   string    `json:"content"`
    MediaPath string    `json:"media_path,omitempty"`
    MediaType string    `json:"media_type,omitempty"`
    IsFromMe  bool      `json:"is_from_me"`
    Status    string    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
}

// SaveMessage - untuk pesan teks biasa
func SaveMessage(fromJID, toJID, content string, isFromMe bool) error {
    query := `INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) 
              VALUES (?, ?, ?, ?, 'sent', ?)`
    _, err := DB.Exec(query, fromJID, toJID, content, isFromMe, time.Now())
    return err
}

// SaveMessageWithTimestamp - untuk simpan riwayat pesan dengan timestamp tertentu
func SaveMessageWithTimestamp(fromJID, toJID, content string, isFromMe bool, ts time.Time) error {
    query := `INSERT INTO messages (from_jid, to_jid, content, is_from_me, status, timestamp) 
              VALUES (?, ?, ?, ?, 'sent', ?)`
    _, err := DB.Exec(query, fromJID, toJID, content, isFromMe, ts)
    return err
}

// SaveMessageWithMedia - untuk pesan dengan media
// SaveMessageWithMedia - untuk pesan dengan media
// SaveMessageWithMedia - untuk pesan dengan media
func SaveMessageWithMedia(fromJID, toJID, content, mediaPath, mediaType string, isFromMe bool) error {
    query := `INSERT INTO messages (from_jid, to_jid, content, media_path, media_type, is_from_me, status, timestamp) 
              VALUES (?, ?, ?, ?, ?, ?, 'sent', ?)`
    _, err := DB.Exec(query, fromJID, toJID, content, mediaPath, mediaType, isFromMe, time.Now())
    return err
}
// UpdateMessageStatus - update status pesan
func UpdateMessageStatus(id int, status string) error {
    query := `UPDATE messages SET status = ? WHERE id = ?`
    _, err := DB.Exec(query, status, id)
    return err
}

// GetPendingMessages - ambil pesan yang belum terkirim
func GetPendingMessages() ([]Message, error) {
    query := `SELECT id, from_jid, to_jid, content, media_path, is_from_me, status, timestamp 
              FROM messages 
              WHERE status = 'pending' AND is_from_me = 1
              ORDER BY id ASC`
    
    rows, err := DB.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var messages []Message
    for rows.Next() {
        var msg Message
        var mediaPath sql.NullString
        err := rows.Scan(&msg.ID, &msg.FromJID, &msg.ToJID, &msg.Content, &mediaPath, &msg.IsFromMe, &msg.Status, &msg.Timestamp)
        if err != nil {
            continue
        }
        if mediaPath.Valid {
            msg.MediaPath = mediaPath.String
        }
        messages = append(messages, msg)
    }
    return messages, nil
}

func GetMessagesByJID(jid string, limit int) ([]Message, error) {
	query := `SELECT id, from_jid, to_jid, content, is_from_me, timestamp 
              FROM messages 
              WHERE from_jid = ? OR to_jid = ?
              ORDER BY timestamp DESC LIMIT ?`

	rows, err := DB.Query(query, jid, jid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.ID, &msg.FromJID, &msg.ToJID, &msg.Content, &msg.IsFromMe, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append([]Message{msg}, messages...)
	}
	return messages, nil
}

func GetAllChats() ([]string, error) {
	query := `SELECT DISTINCT jid FROM (
		SELECT from_jid AS jid FROM messages 
		WHERE from_jid != '' AND from_jid != 'me' 
		  AND from_jid NOT LIKE '%@broadcast' 
		  AND from_jid NOT LIKE '%@newsletter' 
		  AND from_jid NOT LIKE 'status@%'
		UNION 
		SELECT to_jid AS jid FROM messages 
		WHERE to_jid != '' AND to_jid != 'me' 
		  AND to_jid NOT LIKE '%@broadcast' 
		  AND to_jid NOT LIKE '%@newsletter' 
		  AND to_jid NOT LIKE 'status@%'
	)`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []string
	for rows.Next() {
		var jid string
		if err := rows.Scan(&jid); err != nil {
			return nil, err
		}
		chats = append(chats, jid)
	}
	return chats, nil
}

// GetMessagesByJIDWithPagination ambil pesan dengan pagination (offset)
func GetMessagesByJIDWithPagination(jid string, limit int, offset int) ([]Message, error) {
    query := `SELECT id, from_jid, to_jid, content, media_path, is_from_me, status, timestamp 
              FROM messages 
              WHERE (from_jid = ? OR to_jid = ?) 
              AND from_jid != to_jid
              AND content NOT LIKE '%@%'
              ORDER BY timestamp DESC 
              LIMIT ? OFFSET ?`
    
    rows, err := DB.Query(query, jid, jid, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var msg Message
        var mediaPath sql.NullString
        err := rows.Scan(&msg.ID, &msg.FromJID, &msg.ToJID, &msg.Content, &mediaPath, &msg.IsFromMe, &msg.Status, &msg.Timestamp)
        if err != nil {
            continue
        }
        if mediaPath.Valid {
            msg.MediaPath = mediaPath.String
        }
        messages = append(messages, msg)
    }
    
    // Balik urutan
    for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
        messages[i], messages[j] = messages[j], messages[i]
    }
    
    return messages, nil
}

// GetTotalMessagesCount ambil total pesan untuk suatu JID
func GetTotalMessagesCount(jid string) (int, error) {
	query := `SELECT COUNT(*) FROM messages WHERE from_jid = ? OR to_jid = ?`
	var count int
	err := DB.QueryRow(query, jid, jid).Scan(&count)
	return count, err
}
