package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits on image attachments. A message takes at most maxAttachments of all kinds.
const (
	maxImageBytes    = 5 << 20
	maxImagesPerChat = 500 // a backstop for the disk, counting images already sent
	imageDirName     = "chat-images"
	maxImageName     = 100
)

// imageTypes are the types a chat accepts, by what the bytes say (not by the file name).
var imageTypes = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

var imageExts = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}

// imageDir is where a chat keeps its uploaded images: <data>/chat-images/<chat>. The chat id
// has been checked against idPattern by the caller; "" means there is no data folder.
func (m *Manager) imageDir(chatID string) string {
	if m.DataDir == "" || !idPattern.MatchString(chatID) {
		return ""
	}
	return filepath.Join(m.DataDir, imageDirName, chatID)
}

// imageFile finds the stored file of an image id and the type its extension names.
func (m *Manager) imageFile(chatID, id string) (path, mime string, ok bool) {
	dir := m.imageDir(chatID)
	if dir == "" || !idPattern.MatchString(id) {
		return "", "", false
	}
	for ext, mt := range imageExts {
		p := filepath.Join(dir, id+ext)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p, mt, true
		}
	}
	return "", "", false
}

// cleanImageName keeps the last part of a file name, without control characters, cut to a sane length.
func cleanImageName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = name[strings.LastIndex(name, "/")+1:]
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > maxImageName {
		name = string(r[:maxImageName])
	}
	if name = strings.TrimSpace(name); name == "" {
		return "image"
	}
	return name
}

func jsonError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, code)
}

// ImageUploadHandler serves POST /api/chats/{id}/images?name=<file name>: the body is the
// image itself. The bytes never go through a command, so the ledger and the database do
// not see them. The caller checks the session, Host and Origin first.
func (m *Manager) ImageUploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatID := r.PathValue("id")
		if m.get(chatID) == nil {
			jsonError(w, http.StatusNotFound, "not_found")
			return
		}
		dir := m.imageDir(chatID)
		if dir == "" {
			jsonError(w, http.StatusServiceUnavailable, "no_data_folder")
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImageBytes))
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			jsonError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, "bad_request")
			return
		}
		mime := http.DetectContentType(data)
		ext, ok := imageTypes[mime]
		if !ok {
			jsonError(w, http.StatusUnsupportedMediaType, "not_image")
			return
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			m.log.Error("chat: image folder", "chat", chatID, "err", err)
			jsonError(w, http.StatusInternalServerError, "internal")
			return
		}
		if list, _ := os.ReadDir(dir); len(list) >= maxImagesPerChat {
			jsonError(w, http.StatusTooManyRequests, "too_many")
			return
		}
		id := randomID("i")
		if err := os.WriteFile(filepath.Join(dir, id+ext), data, 0o600); err != nil {
			m.log.Error("chat: save image", "chat", chatID, "err", err)
			jsonError(w, http.StatusInternalServerError, "internal")
			return
		}
		name := cleanImageName(r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": name, "size": len(data), "mime": mime})
	})
}

// ImageHandler serves GET /api/chats/{id}/images/{imageId} from the stored file. The type
// comes from the extension Mooring chose at upload, so it is always one of the four.
func (m *Manager) ImageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chatID := r.PathValue("id")
		path, mime, ok := m.imageFile(chatID, r.PathValue("imageId"))
		if !ok || m.get(chatID) == nil {
			jsonError(w, http.StatusNotFound, "not_found")
			return
		}
		f, err := os.Open(path)
		if err != nil {
			jsonError(w, http.StatusNotFound, "not_found")
			return
		}
		defer func() { _ = f.Close() }()
		h := w.Header()
		h.Set("Content-Type", mime)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "private, max-age=31536000, immutable")
		http.ServeContent(w, r, "", time.Time{}, f)
	})
}

// readImage loads a stored image for the agent and checks it again.
func (m *Manager) readImage(chatID, id string) (data []byte, mime string, err error) {
	path, _, ok := m.imageFile(chatID, id)
	if !ok {
		return nil, "", apiErr("NOT_FOUND", "An attached image is no longer kept. Attach it again.")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		m.log.Error("chat: read image", "chat", chatID, "err", err)
		return nil, "", apiErr("INTERNAL", "An attached image could not be read.")
	}
	mime = http.DetectContentType(data)
	if _, ok := imageTypes[mime]; !ok || len(data) > maxImageBytes {
		return nil, "", apiErr("BAD_REQUEST", "An attached image is not a PNG, JPEG, GIF or WebP of 5 MB or less.")
	}
	return data, mime, nil
}

// removeImages deletes every image of a chat, when the chat is deleted.
func (m *Manager) removeImages(chatID string) {
	dir := m.imageDir(chatID)
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		m.log.Warn("chat: images not removed", "chat", chatID, "err", err)
	}
}

// sweepImages runs once at start. It deletes the images of chats that no longer exist and
// the images no message of their chat refers to: uploads that were never sent.
func (m *Manager) sweepImages() {
	if m.DataDir == "" {
		return
	}
	root := filepath.Join(m.DataDir, imageDirName)
	chats, err := os.ReadDir(root)
	if err != nil {
		return
	}
	removed := 0
	for _, d := range chats {
		dir := filepath.Join(root, d.Name())
		if !d.IsDir() || m.chats[d.Name()] == nil {
			_ = os.RemoveAll(dir)
			removed++
			continue
		}
		used, ok := m.sentImages(d.Name())
		if !ok {
			continue
		}
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			id := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			if !used[id] && os.Remove(filepath.Join(dir, f.Name())) == nil {
				removed++
			}
		}
	}
	if removed > 0 {
		m.log.Info("chat: removed unused images", "count", removed)
	}
}

// sentImages is the set of image ids the saved messages of a chat refer to.
func (m *Manager) sentImages(chatID string) (map[string]bool, bool) {
	rows, err := m.db.Query(`SELECT body FROM chat_messages WHERE chat_id = ?`, chatID)
	if err != nil {
		m.log.Warn("chat: image sweep skipped a chat", "chat", chatID, "err", err)
		return nil, false
	}
	defer func() { _ = rows.Close() }()
	used := map[string]bool{}
	for rows.Next() {
		var body string
		if rows.Scan(&body) != nil || !strings.Contains(body, `"imageId"`) {
			continue
		}
		var msg Message
		if json.Unmarshal([]byte(body), &msg) == nil {
			for _, a := range msg.Attachments {
				if a.ImageID != "" {
					used[a.ImageID] = true
				}
			}
		}
	}
	return used, rows.Err() == nil
}
