package chat

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type uploaded struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int    `json:"size"`
	Mime string `json:"mime"`
}

func (r *rig) upload(chatID, name string, body []byte) (*httptest.ResponseRecorder, uploaded) {
	r.t.Helper()
	q := httptest.NewRequest("POST", "/api/chats/"+chatID+"/images?name="+name, bytes.NewReader(body))
	q.SetPathValue("id", chatID)
	w := httptest.NewRecorder()
	r.m.ImageUploadHandler().ServeHTTP(w, q)
	var u uploaded
	_ = json.Unmarshal(w.Body.Bytes(), &u)
	return w, u
}

func (r *rig) image(chatID, id string) *httptest.ResponseRecorder {
	q := httptest.NewRequest("GET", "/api/chats/"+chatID+"/images/"+id, nil)
	q.SetPathValue("id", chatID)
	q.SetPathValue("imageId", id)
	w := httptest.NewRecorder()
	r.m.ImageHandler().ServeHTTP(w, q)
	return w
}

func imageAtt(u uploaded) map[string]any {
	return map[string]any{"kind": "image", "imageId": u.ID, "name": u.Name}
}

func TestUploadKeepsPNGAndRefusesTheRest(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=image")
	c := r.start("c1")
	w, u := r.upload(c.id, "shot.png", pngBytes(t))
	if w.Code != 200 || u.ID == "" || u.Mime != "image/png" || u.Name != "shot.png" || u.Size != len(pngBytes(t)) {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(r.data, "chat-images", "c1", u.ID+".png")); err != nil {
		t.Fatalf("not stored: %v", err)
	}
	// What the bytes say counts, not the name.
	if w, _ := r.upload(c.id, "notes.png", []byte("just some text")); w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text named .png: %d", w.Code)
	}
	if w, _ := r.upload(c.id, "x.png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>1</script></svg>`)); w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("svg: %d", w.Code)
	}
	big := append(pngBytes(t), make([]byte, maxImageBytes)...)
	if w, _ := r.upload(c.id, "big.png", big); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over 5 MB: %d", w.Code)
	}
	if w, _ := r.upload("nochat", "a.png", pngBytes(t)); w.Code != http.StatusNotFound {
		t.Fatalf("unknown chat: %d", w.Code)
	}
	if got := cleanImageName("../../etc/pa\x00ss\nwd.png"); got != "passwd.png" {
		t.Fatalf("name %q", got)
	}
}

func TestPromptSendsTheImageAndKeepsARef(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=image")
	c := r.start("c1")
	_, u := r.upload(c.id, "shot.png", pngBytes(t))
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": "look", "attachments": []any{imageAtt(u)}}); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	r.waitState(c, StateIdle)
	waitFor(t, "the agent's answer", func() bool { m := c.find(KindAgent); return m != nil && m.Text == "got 1 image(s)" })
	user := c.find(KindUser)
	if len(user.Attachments) != 1 || user.Attachments[0].Kind != "image" || user.Attachments[0].ImageID != u.ID || user.Attachments[0].Name != "shot.png" || user.Attachments[0].Size != u.Size {
		t.Fatalf("refs %+v", user.Attachments)
	}
	raw, _ := json.Marshal(user)
	if strings.Contains(string(raw), "iVBOR") {
		t.Fatal("the message holds image bytes")
	}
	w := r.image(c.id, u.ID)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(w.Body.Bytes(), pngBytes(t)) {
		t.Fatalf("serve: %d %v", w.Code, w.Header())
	}
	for _, id := range []string{"missing", "..%2F..%2Fx", "a/b"} {
		if w := r.image(c.id, id); w.Code != 404 {
			t.Fatalf("%q: %d", id, w.Code)
		}
	}
	if w := r.image("other", u.ID); w.Code != 404 {
		t.Fatalf("other chat: %d", w.Code)
	}
}

func TestPromptRefusesAnImageWhenTheAgentTakesNone(t *testing.T) {
	r := newRig(t) // no "image" in FAKE_CAPS
	c := r.start("c1")
	if c.Info().Caps.Image {
		t.Fatal("the fake advertises images without being asked")
	}
	_, u := r.upload(c.id, "shot.png", pngBytes(t))
	_, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": "look", "attachments": []any{imageAtt(u)}})
	if err == nil || !strings.Contains(err.Error(), "Fake does not take images. Start a chat with an agent that does, such as Claude.") {
		t.Fatalf("err %v", err)
	}
	if c.find(KindUser) != nil || c.Info().State != StateIdle {
		t.Fatal("the refused message started a turn")
	}
}

func TestPromptRefusesNinthAttachmentAndUnknownImage(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=image")
	c := r.start("c1")
	_, u := r.upload(c.id, "a.png", pngBytes(t))
	nine := make([]any, 9)
	for i := range nine {
		nine[i] = imageAtt(u)
	}
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": "x", "attachments": nine}); err == nil || !strings.Contains(err.Error(), "at most 8 attachments") {
		t.Fatalf("nine: %v", err)
	}
	gone := map[string]any{"kind": "image", "imageId": "inothere", "name": "x.png"}
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": "x", "attachments": []any{gone}}); err == nil {
		t.Fatal("an image that was never uploaded was accepted")
	}
}

func TestDeletingAChatDeletesItsImages(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=image")
	c := r.start("c1")
	r.upload(c.id, "a.png", pngBytes(t))
	dir := filepath.Join(r.data, "chat-images", "c1")
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run("chat.remove", map[string]any{"id": "c1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("images stay after the chat is deleted: %v", err)
	}
}

func TestStartRemovesImagesThatWereNeverSent(t *testing.T) {
	r := newRig(t, "FAKE_CAPS=image")
	c := r.start("c1")
	_, unsent := r.upload(c.id, "unsent.png", pngBytes(t))
	_, sentImg := r.upload(c.id, "sent.png", pngBytes(t))
	if _, err := r.run("chat.prompt", map[string]any{"id": c.id, "text": "look", "attachments": []any{imageAtt(sentImg)}}); err != nil {
		t.Fatal(err)
	}
	r.waitState(c, StateIdle)
	orphan := filepath.Join(r.data, "chat-images", "gone-chat")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	r.m.Shutdown()
	r.boot()
	if w := r.image("c1", unsent.ID); w.Code != 404 {
		t.Fatalf("unsent image kept: %d", w.Code)
	}
	if w := r.image("c1", sentImg.ID); w.Code != 200 {
		t.Fatalf("sent image lost: %d", w.Code)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("the folder of a chat that does not exist stays")
	}
}
