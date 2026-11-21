package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ryokusnadi/mooring/internal/password"
	"github.com/ryokusnadi/mooring/internal/store"
)

const (
	testUser = "ada"
	testPass = "correct horse battery"
)

// newPasswordServer is a server with a data folder, so passwords can be kept.
func newPasswordServer(t *testing.T, withPassword bool) (http.Handler, string) {
	t.Helper()
	t.Cleanup(password.UseLowCost())
	dir := t.TempDir()
	if withPassword {
		r, err := password.NewRecord(testUser, testPass)
		if err != nil {
			t.Fatal(err)
		}
		if err := password.Save(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	c := &clock{t: time.Unix(1_800_000_000, 0)}
	st, err := store.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Token: testToken, Static: fstest.MapFS{"index.html": {Data: []byte("x")}}, Now: c.now,
		Store: st, Home: t.TempDir(), DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close(); _ = st.Close() })
	return srv, dir
}

func passLogin(h http.Handler, user, pw string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]string{"username": user, "password": pw})
	return do(h, req{method: "POST", path: "/api/login", body: string(b), origin: "http://127.0.0.1:7340"})
}

func TestPasswordLoginOpensSession(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	w := passLogin(h, testUser, testPass)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	c := sessionCookieOf(t, w)
	if w := do(h, req{method: "GET", path: "/api/session", cookie: c}); w.Code != http.StatusNoContent {
		t.Fatalf("session: %d", w.Code)
	}
}

func TestWrongUsernameAndWrongPasswordLookTheSame(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	a := passLogin(h, "bob", testPass)
	b := passLogin(h, testUser, "wrong password")
	if a.Code != http.StatusUnauthorized || b.Code != http.StatusUnauthorized {
		t.Fatalf("codes %d %d", a.Code, b.Code)
	}
	if a.Body.String() != b.Body.String() {
		t.Fatalf("bodies differ: %q %q", a.Body, b.Body)
	}
	if len(a.Result().Cookies()) != 0 || len(b.Result().Cookies()) != 0 {
		t.Fatal("a refused login set a cookie")
	}
}

func TestPasswordLoginWithNoPasswordSetFailsLikeAWrongOne(t *testing.T) {
	h, _ := newPasswordServer(t, false)
	w := passLogin(h, testUser, testPass)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "wrong_credentials") {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
}

func TestPasswordAndTokenShareTheFailureLimit(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	for i := range maxFailures - 1 {
		if w := passLogin(h, testUser, "wrong password"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	if w := login(h, "wrong-token"); w.Code != http.StatusUnauthorized {
		t.Fatalf("token attempt: %d", w.Code)
	}
	w := passLogin(h, testUser, testPass)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("right password after 5 failures: %d", w.Code)
	}
	if w := login(h, testToken); w.Code != http.StatusTooManyRequests {
		t.Fatalf("right token after 5 failures: %d", w.Code)
	}
}

func TestPasswordLoginFromAnotherSiteIsRefused(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	body := `{"username":"ada","password":"` + testPass + `"}`
	for _, q := range []req{
		{method: "POST", path: "/api/login", body: body, origin: "https://evil.example"},
		{method: "POST", path: "/api/login", body: body, site: "cross-site"},
		{method: "POST", path: "/api/login", body: body, host: "evil.example"},
	} {
		w := do(h, q)
		if w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
			t.Fatalf("%+v: got %d", q, w.Code)
		}
	}
}

func TestPasswordLoginBodyIsLimited(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	big := `{"username":"ada","password":"` + strings.Repeat("a", 10000) + `"}`
	if w := do(h, req{method: "POST", path: "/api/login", body: big, origin: "http://127.0.0.1:7340"}); w.Code != http.StatusBadRequest {
		t.Fatalf("got %d", w.Code)
	}
}

func TestLoginStateOnlySaysWhetherAPasswordIsSet(t *testing.T) {
	for _, set := range []bool{false, true} {
		h, _ := newPasswordServer(t, set)
		w := do(h, req{method: "GET", path: "/api/login/state"})
		var got map[string]bool
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 1 || got["passwordSet"] != set {
			t.Fatalf("set=%v: %d %s", set, w.Code, w.Body)
		}
	}
	h, _ := newTestServer(t) // no data folder
	if w := do(h, req{method: "GET", path: "/api/login/state"}); !strings.Contains(w.Body.String(), `"passwordSet":false`) {
		t.Fatalf("no data folder: %s", w.Body)
	}
}

func TestUnknownVersionFileCountsAsSetButNeverSignsIn(t *testing.T) {
	h, dir := newPasswordServer(t, false)
	if err := os.WriteFile(filepath.Join(dir, password.FileName), []byte("v9:x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := do(h, req{method: "GET", path: "/api/login/state"}); !strings.Contains(w.Body.String(), `"passwordSet":true`) {
		t.Fatalf("state: %s", w.Body)
	}
	w := passLogin(h, testUser, testPass)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "password_file_unreadable") {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
}

func TestParallelWrongPasswordsGetOnlyTheLimit(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	const n = 40
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- passLogin(h, testUser, "wrong password").Code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for c := range codes {
		got[c]++
	}
	if got[http.StatusUnauthorized] != maxFailures || got[http.StatusTooManyRequests] != n-maxFailures {
		t.Fatalf("answers %v, want %d x 401 and the rest 429", got, maxFailures)
	}
}

func TestSuccessfulLoginsDoNotUseUpTheLimit(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	for range maxFailures + 3 {
		if w := passLogin(h, testUser, testPass); w.Code != http.StatusNoContent {
			t.Fatalf("got %d", w.Code)
		}
	}
}

func TestVeryLongPasswordIsRefusedAtLogin(t *testing.T) {
	h, _ := newPasswordServer(t, true)
	if w := passLogin(h, testUser, strings.Repeat("a", password.MaxPasswordBytes+1)); w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", w.Code)
	}
}
