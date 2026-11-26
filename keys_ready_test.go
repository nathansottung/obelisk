//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// keysAPI sends one request to the API and decodes the JSON reply.
func keysAPI(t *testing.T, app *App, method, path string, body map[string]any) (int, map[string]any, string) {
	t.Helper()
	mux := http.NewServeMux()
	api(mux, app)
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(b)))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.String()
}

// plannedEncrypted adds a small planned encrypted package with real files.
func plannedEncrypted(t *testing.T, app *App, name string) *Chunk {
	t.Helper()
	src, refs := makeSource(t)
	return app.Store.AddChunk(Chunk{Name: name, Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs), SrcRoot: src,
		HashAlg: "SHA256", Par2: 5, Encrypted: true, Files: append([]ChunkFileRef{}, refs...)})
}

// waitBuilt waits for package id to leave PLANNED/BUILDING after a build job.
func waitBuilt(t *testing.T, app *App, id int) *Chunk {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		c := app.Store.Chunk(id)
		if c != nil && c.Status != "PLANNED" && c.Status != "BUILDING" {
			return c
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("package %d was not built in time", id)
	return nil
}

// createKeyAPI creates a key through the API and returns its reference.
func createKeyAPI(t *testing.T, app *App, body map[string]any) string {
	t.Helper()
	code, out, raw := keysAPI(t, app, "POST", "/api/keys", body)
	if code != 200 {
		t.Fatalf("Create key: %d %s", code, raw)
	}
	if _, has := out["passphrase"]; has {
		t.Fatalf("Create key must not return the secret: %s", raw)
	}
	ref, _ := out["key_ref"].(string)
	if ref == "" || out["state"] != keyStateNotReady {
		t.Fatalf("a new key must be not ready: %s", raw)
	}
	return ref
}

// revealAPI reveals a key for an offline copy.
func revealAPI(t *testing.T, app *App, ref, kind string) map[string]any {
	t.Helper()
	code, out, raw := keysAPI(t, app, "POST", "/api/keys/"+ref+"/reveal", map[string]any{"kind": kind})
	if code != 200 {
		t.Fatalf("reveal %s: %d %s", kind, code, raw)
	}
	return out
}

func proveAPI(t *testing.T, app *App, ref, text, kind string) map[string]any {
	t.Helper()
	code, out, raw := keysAPI(t, app, "POST", "/api/keys/"+ref+"/prove", map[string]any{"text": text, "kind": kind})
	if code != 200 {
		t.Fatalf("prove: %d %s", code, raw)
	}
	return out
}

// An encrypted build with no ready key refuses before any job starts, and names
// where to go.
func TestKeyReady_EncryptedBuildRefusesWithNoReadyKey(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	c := plannedEncrypted(t, app, "NOKEY")
	code, out, raw := keysAPI(t, app, "POST", "/api/chunks/"+itoaForTest(c.ID)+"/build", nil)
	if code != 409 || out["key_not_ready"] != true {
		t.Fatalf("want a key-not-ready refusal, got %d %s", code, raw)
	}
	if !strings.Contains(raw, "Create key") {
		t.Fatalf("the refusal must name the step (Create key): %s", raw)
	}
	if got := app.Store.Chunk(c.ID); got.Status != "PLANNED" || got.KeyRef != "" {
		t.Fatalf("a refused build changed the package: %+v", got)
	}
	if len(app.Store.KeyMetas()) != 0 {
		t.Fatal("a refused build made a key")
	}
}

// A new key is not ready, and a build refuses naming the copy-and-prove step,
// until a copy is proven; then every build uses it.
func TestKeyReady_CreateCopyProveBuild(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, map[string]any{"name": "Family photos"})
	c := plannedEncrypted(t, app, "FIRST")
	code, out, raw := keysAPI(t, app, "POST", "/api/chunks/"+itoaForTest(c.ID)+"/build", nil)
	if code != 409 || out["key_not_ready"] != true || !strings.Contains(raw, "offline copy") || out["key_ref"] != ref {
		t.Fatalf("an unproven key must refuse naming the offline-copy step: %d %s", code, raw)
	}
	if err := app.BuildChunk(c.ID, noProg); err == nil || !strings.Contains(err.Error(), "offline copy") {
		t.Fatalf("BuildChunk itself must refuse too: %v", err)
	}
	sheet := revealAPI(t, app, ref, "printout")["sheet"].(string)
	if r := proveAPI(t, app, ref, sheet, "printout"); r["ok"] != true || r["state"] != keyStateReady {
		t.Fatalf("the printout typed back must prove the key: %v", r)
	}
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	second := plannedEncrypted(t, app, "SECOND")
	if err := app.BuildChunk(second.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if a, b := app.Store.Chunk(c.ID).KeyRef, app.Store.Chunk(second.ID).KeyRef; a != ref || b != ref {
		t.Fatalf("both builds must use the shared key %s, got %s and %s", ref, a, b)
	}
	if len(app.Store.KeyMetas()) != 1 {
		t.Fatalf("a build made a key of its own: %d keys", len(app.Store.KeyMetas()))
	}
	var states []KeyState
	_, _, raw = keysAPI(t, app, "GET", "/api/keys", nil)
	var page struct {
		Keys []KeyState `json:"keys"`
	}
	_ = json.Unmarshal([]byte(raw), &page)
	states = page.Keys
	if len(states) != 1 || states[0].Packages != 2 || !states[0].Current || states[0].Name != "Family photos" {
		t.Fatalf("the Keys page must show the key's name, that it is current, and that it opens 2 packages: %+v", states)
	}
	// RESTORE.txt names the key by reference, name and date.
	b, err := os.ReadFile(filepath.Join(app.Store.Chunk(c.ID).StagedDir, "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	date := app.Store.KeyMetaCopy(ref).CreatedAt.UTC().Format("2006-01-02")
	if !strings.Contains(string(b), ref) || !strings.Contains(string(b), "Family photos") || !strings.Contains(string(b), date) {
		t.Fatalf("RESTORE.txt must name key %s, its name and its date:\n%s", ref, b)
	}
}

// A typed-back sheet with its middle line missing, or one line changed, fails
// and names the line; the key stays not ready.
func TestKeyReady_ProofNamesMissingAndChangedLines(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, nil)
	sheet := revealAPI(t, app, ref, "printout")["sheet"].(string)
	short := dropSheetLine(t, sheet, 2)
	r := proveAPI(t, app, ref, short, "printout")
	if r["ok"] != false || !strings.Contains(strings.Join(anyStrings(asAnySlice(r["notes"])), " "), "L02") {
		t.Fatalf("a sheet missing line 2 must fail naming L02: %v", r)
	}
	// Change one character of line 1 and fix its check code, so only the
	// comparison with the key can find it.
	var lines []string
	for _, ln := range strings.Split(sheet, "\n") {
		if m := keySheetLineRe.FindStringSubmatch(ln); m != nil && sheetLineNo(ln) == 1 {
			chars := strings.ReplaceAll(m[3], " ", "")
			alt := "A"
			if chars[0] == 'A' {
				alt = "B"
			}
			chars = alt + chars[1:]
			ln = "  L01/" + m[2] + "  " + chars + "  [" + lineCode(chars) + "]"
		}
		lines = append(lines, ln)
	}
	r = proveAPI(t, app, ref, strings.Join(lines, "\n"), "printout")
	if r["ok"] != false || !strings.Contains(strings.Join(anyStrings(asAnySlice(r["notes"])), " "), "L01") {
		t.Fatalf("a changed line 1 must fail naming L01: %v", r)
	}
	if st := app.keyStateOf(ref); st != keyStateNotReady {
		t.Fatalf("a failed proof left the key %q", st)
	}
}

// The password-manager note is plain "Label: value" lines that any password
// manager keeps as typed (checked against Bitwarden's secure-note limits:
// plain text, at most 10,000 characters), and pasting it back proves the key.
func TestKeyReady_PasswordManagerNote(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, map[string]any{"name": "Archive key"})
	block := revealAPI(t, app, ref, "password-manager")["block"].(string)
	pass, _ := app.Passphrase(ref)
	for _, want := range []string{"Name: Archive key", "Key ref: " + ref, "Passphrase: " + pass,
		"SHA-256 fingerprint: " + passFingerprint(pass), "Opens: ", "gpg -d -o NAME.tar NAME.tar.gpg", "par2 verify", "tar -xf"} {
		if !strings.Contains(block, want) {
			t.Fatalf("the note lacks %q:\n%s", want, block)
		}
	}
	if len(block) > 10000 {
		t.Fatalf("the note is %d characters; a secure note holds 10,000", len(block))
	}
	for _, r := range block {
		if r != '\n' && (r < 0x20 || r > 0x7e) {
			t.Fatalf("the note holds a character a plain-text note may not keep: %q", r)
		}
	}
	for _, ln := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "gpg") || strings.HasPrefix(strings.TrimSpace(ln), "par2") || strings.HasPrefix(strings.TrimSpace(ln), "tar") {
			if strings.Contains(ln, pass) || strings.Contains(ln, "--passphrase") {
				t.Fatalf("a restore command carries the passphrase: %s", ln)
			}
		}
	}
	// Pasted back as a password manager might return it: extra spaces, CRLF.
	pasted := strings.ReplaceAll("  "+block, "\n", "\r\n")
	if r := proveAPI(t, app, ref, pasted, "password-manager"); r["ok"] != true {
		t.Fatalf("the pasted note must prove the key: %v", r)
	}
}

// Rotation is the user's choice: a new shared key is not used until it is
// proven, and the old one keeps opening its packages. No key is ever removed.
func TestKeyReady_RotationWaitsForProof(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	old := readyTestKey(t, app)
	a := plannedEncrypted(t, app, "OLDKEY")
	if err := app.BuildChunk(a.ID, noProg); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	newer := createKeyAPI(t, app, map[string]any{"name": "Second key"})
	b := plannedEncrypted(t, app, "STILLOLD")
	if err := app.BuildChunk(b.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if app.Store.Chunk(b.ID).KeyRef != old {
		t.Fatal("an unproven new key was used before it was proven")
	}
	block := revealAPI(t, app, newer, "password-manager")["block"].(string)
	proveAPI(t, app, newer, block, "password-manager")
	c := plannedEncrypted(t, app, "NEWKEY")
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if app.Store.Chunk(c.ID).KeyRef != newer {
		t.Fatal("the proven new key must be the current key")
	}
	if _, err := app.Passphrase(old); err != nil || app.Store.KeyMetaCopy(old) == nil {
		t.Fatal("the old key must stay")
	}
	for _, method := range []string{"DELETE"} {
		code, _, raw := keysAPI(t, app, method, "/api/keys/"+old, nil)
		if code == 200 {
			t.Fatalf("a key was removed from the page: %s", raw)
		}
	}
}

// "Use a separate key for this package": that package waits for its own key to
// be proven, and every other package keeps the shared key.
func TestKeyReady_SeparateKeyForOnePackage(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	shared := readyTestKey(t, app)
	gift := plannedEncrypted(t, app, "GIFT")
	sep := createKeyAPI(t, app, map[string]any{"purpose": "package", "package_id": gift.ID})
	if m := app.Store.KeyMetaCopy(sep); m.Name != "Key for package GIFT" || m.PackageID != gift.ID {
		t.Fatalf("the separate key must be named for its package: %+v", m)
	}
	code, out, raw := keysAPI(t, app, "POST", "/api/chunks/"+itoaForTest(gift.ID)+"/build", nil)
	if code != 409 || out["key_ref"] != sep {
		t.Fatalf("the package must wait for its own key, not fall back to the shared one: %d %s", code, raw)
	}
	other := plannedEncrypted(t, app, "OTHER")
	if err := app.BuildChunk(other.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if app.Store.Chunk(other.ID).KeyRef != shared {
		t.Fatal("another package must keep the shared key")
	}
	block := revealAPI(t, app, sep, "password-manager")["block"].(string)
	if !strings.Contains(block, "only package GIFT") {
		t.Fatalf("the separate key's note must say it opens only its package:\n%s", block)
	}
	proveAPI(t, app, sep, block, "password-manager")
	if err := app.BuildChunk(gift.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if app.Store.Chunk(gift.ID).KeyRef != sep {
		t.Fatal("the package must be encrypted with its own key")
	}
	for _, st := range app.KeyStates() {
		if st.Ref == sep && st.Current {
			t.Fatal("a package's own key must never become the shared current key")
		}
	}
	if code, _, _ := keysAPI(t, app, "POST", "/api/keys", map[string]any{"purpose": "package", "package_id": gift.ID}); code == 200 {
		t.Fatal("a package's own key must not be replaced")
	}
}

// A plain-text copy is saved only where the user chooses, never in the data
// folder, never on a drive holding backups or a package the key opens, never
// over an existing file, and never on a drive that cannot be identified.
func TestKeyReady_TextFilePlacement(t *testing.T) {
	app, staging := newBareTestApp(t, nil)
	ref := readyTestKey(t, app)
	c := plannedEncrypted(t, app, "PLACED")
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	save := func(p string) (int, string) {
		code, _, raw := keysAPI(t, app, "POST", "/api/keys/"+ref+"/save-file", map[string]any{"path": p})
		return code, raw
	}
	for name, p := range map[string]string{
		"the data folder":       filepath.Join(app.DataDir, "key.txt"),
		"the staging drive":     filepath.Join(staging, "key.txt"),
		"a drive the key opens": filepath.Join(app.Store.Chunk(c.ID).Copies[0].Path, "key.txt"),
		"a relative path":       "key.txt",
	} {
		if code, raw := save(p); code == 200 {
			t.Fatalf("a key file was saved to %s: %s", name, raw)
		}
		if _, err := os.Stat(p); err == nil && filepath.IsAbs(p) {
			t.Fatalf("a refused save left a file in %s", name)
		}
	}
	orig := keystoreDeviceOf
	keystoreDeviceOf = func(string) (string, error) { return "", errNoExistingParent }
	if code, raw := save(filepath.Join(t.TempDir(), "key.txt")); code == 200 {
		keystoreDeviceOf = orig
		t.Fatalf("a key file was saved to a drive Obelisk cannot identify: %s", raw)
	}
	keystoreDeviceOf = orig
	dest := filepath.Join(t.TempDir(), "obelisk-key.txt")
	if code, raw := save(dest); code != 200 {
		t.Fatalf("a key file on a separate drive must be saved: %s", raw)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if r := proveAPI(t, app, ref, string(b), "file"); r["ok"] != true {
		t.Fatalf("the saved file pasted back must prove the key: %v", r)
	}
	if code, _ := save(dest); code == 200 {
		t.Fatal("an existing key file was replaced")
	}
}

// A key made before this change shows "not proven", asks for a proof, is never
// used for a new build, and does not block a restore.
func TestKeyReady_OldKeysShowNotProven(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref, pass, _, err := app.GenerateKey("package OLD") // what a build did before
	if err != nil {
		t.Fatal(err)
	}
	var st KeyState
	for _, s := range app.KeyStates() {
		if s.Ref == ref {
			st = s
		}
	}
	if st.State != keyStateNotProven || !strings.Contains(st.Step, "Restores do not wait") {
		t.Fatalf("an old key must show not proven and ask for a proof: %+v", st)
	}
	if got, err := app.Passphrase(ref); err != nil || got != pass {
		t.Fatal("an old key must still be readable for restores")
	}
	c := plannedEncrypted(t, app, "NEW")
	if err := app.BuildChunk(c.ID, noProg); err == nil {
		t.Fatal("an old, unproven key was used for a new build")
	}
	if r := proveAPI(t, app, ref, pass, "password-manager"); r["ok"] != true || r["state"] != keyStateProven {
		t.Fatalf("an old key can be proven: %v", r)
	}
}

// The secret appears only in the reveal reply: never in the data folder (the
// catalog, its audit log, the job records), an app-state backup without keys,
// any other reply or error, or the key file's audit entry.
func TestKeyReady_SentinelPassphraseNeverLeaks(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, map[string]any{"name": "Sentinel"})
	pass, _ := app.Passphrase(ref)
	var replies []string
	call := func(method, path string, body map[string]any) map[string]any {
		_, out, raw := keysAPI(t, app, method, path, body)
		replies = append(replies, raw)
		return out
	}
	call("GET", "/api/keys", nil)
	c := plannedEncrypted(t, app, "SENT")
	call("POST", "/api/chunks/"+itoaForTest(c.ID)+"/build", nil)
	call("POST", "/api/keys/"+ref+"/prove", map[string]any{"text": "wrong", "kind": "printout"})
	call("POST", "/api/keys/"+ref+"/save-file", map[string]any{"path": filepath.Join(app.DataDir, "k.txt")})
	keyFile := filepath.Join(t.TempDir(), "k.txt")
	call("POST", "/api/keys/"+ref+"/save-file", map[string]any{"path": keyFile})
	block := revealAPI(t, app, ref, "password-manager")["block"].(string)
	call("POST", "/api/keys/"+ref+"/prove", map[string]any{"text": block, "kind": "password-manager"})
	call("POST", "/api/chunks/"+itoaForTest(c.ID)+"/build", nil)
	waitBuilt(t, app, c.ID)
	call("GET", "/api/keys", nil)
	call("GET", "/api/jobs", nil)
	for i, r := range replies {
		if strings.Contains(r, pass) {
			t.Fatalf("reply %d holds the passphrase: %s", i, r)
		}
	}
	if err := filepath.WalkDir(app.DataDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr == nil && bytes.Contains(b, []byte(pass)) {
			t.Errorf("%s holds the passphrase", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	backupDir := t.TempDir()
	res, err := app.ExportAppBackup(backupDir, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = res
	if err := filepath.WalkDir(backupDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if b, rerr := os.ReadFile(p); rerr == nil && bytes.Contains(b, []byte(pass)) {
			t.Errorf("the app-state backup %s holds the passphrase", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, j := range app.Store.Jobs() {
		jb, _ := json.Marshal(j)
		if bytes.Contains(jb, []byte(pass)) {
			t.Fatalf("job %d holds the passphrase", j.ID)
		}
	}
}

func asAnySlice(v any) []any { xs, _ := v.([]any); return xs }
