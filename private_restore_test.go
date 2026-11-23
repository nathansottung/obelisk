//go:build !guionly

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const privateFileName = "secret-location-plan.txt"

// privateChunk builds a real private package from one distinctively named file.
func privateChunk(t *testing.T, app *App, name string, spanned bool) (*Chunk, []byte) {
	t.Helper()
	if _, err := app.SaveConfig(map[string]any{"private_media": true}); err != nil {
		t.Fatal(err)
	}
	body := []byte(strings.Repeat("the bytes that must come back by hand\n", 1100))
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, privateFileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
	ch := Chunk{
		Name: name, Status: "PLANNED", MediaKind: "CUSTOM", TargetBytes: 1 << 30,
		DataBytes: int64(len(body)), FileCount: 1, SrcRoot: src, HashAlg: "SHA256", Par2: 10,
		Encrypted: true, Files: []ChunkFileRef{{RelPath: privateFileName, SizeBytes: int64(len(body))}},
	}
	if spanned {
		ch.TargetBytes, ch.Spanned = 16000, true
		ch.Segments = planSegments(int64(len(body))+4096, 16000)
	}
	c := app.Store.AddChunk(ch)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("build: %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if !c.PrivateManifest {
		t.Fatal("the package was not built as private")
	}
	return c, body
}

// checkPrivateMedium is the design rule for one medium's package folder.
func checkPrivateMedium(t *testing.T, dir string, c *Chunk) {
	t.Helper()
	mustExist(t, filepath.Join(dir, c.Name+".manifest.json.gpg"))
	mustExist(t, filepath.Join(dir, "RESTORE.txt"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		switch e.Name() {
		case c.Name + ".manifest.json", c.Name + ".tar", "filelist.txt", "manifest-sha256.txt",
			"tagmanifest-sha256.txt", "bag-info.txt", "bagit.txt":
			t.Errorf("%s is on the private medium", e.Name())
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(privateFileName)) {
			t.Errorf("%s shows a private file name", e.Name())
		}
	}
}

// checkRestoreNames requires every file RESTORE.txt names to be on the media, or to
// be the tar its own decrypt step makes.
func checkRestoreNames(t *testing.T, c *Chunk, restore string, onMedia map[string]bool) {
	t.Helper()
	made := c.Name + ".tar"
	for _, tok := range regexp.MustCompile(regexp.QuoteMeta(c.Name)+`\.[A-Za-z0-9.*]+`).FindAllString(restore, -1) {
		tok = strings.TrimRight(tok, ".")
		if tok == made || tok == c.Name+".segNNN" {
			continue
		}
		if c.Spanned && tok == payloadName(c) {
			continue // the rejoin step makes it from the segments
		}
		found := onMedia[tok]
		if strings.Contains(tok, "*") {
			for n := range onMedia {
				if ok, _ := filepath.Match(tok, n); ok {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("RESTORE.txt names %s, which is not on the medium", tok)
		}
	}
}

// byHand restores the package in dir with only par2, gpg and tar, after damaging
// the payload so that par2 has real work to do.
func byHand(t *testing.T, tools map[string]string, dir string, c *Chunk, pass string, want []byte) {
	t.Helper()
	home := t.TempDir()
	t.Cleanup(func() {
		gpgconf := filepath.Join(filepath.Dir(tools["gpg"]), "gpgconf")
		_ = exec.Command(gpgconf, "--homedir", home, "--kill", "gpg-agent").Run()
	})
	run := func(stdin, tool string, args ...string) (string, error) {
		cmd := exec.Command(tools[tool], args...)
		cmd.Dir, cmd.Stdin = dir, strings.NewReader(stdin)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		t.Logf("%s %s: exit ok=%v", tool, strings.Join(args, " "), err == nil)
		if err != nil {
			return out.String(), &handError{err, errb.String()}
		}
		return out.String(), nil
	}
	gpgArgs := func(rest ...string) []string {
		return append([]string{"--batch", "--yes", "--homedir", home, "--pinentry-mode", "loopback", "--passphrase-fd", "0"}, rest...)
	}
	payload := c.Name + ".tar.gpg"
	b, err := os.ReadFile(filepath.Join(dir, payload))
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)/2] ^= 0xFF
	if err := os.WriteFile(filepath.Join(dir, payload), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run("", "par2", "verify", payload+".par2"); err == nil {
		t.Fatal("par2 did not see the damage")
	}
	if _, err := run("", "par2", "repair", payload+".par2"); err != nil {
		t.Fatalf("par2 repair: %v", err)
	}
	listing, err := run(pass+"\n", "gpg", gpgArgs("-d", c.Name+".manifest.json.gpg")...)
	if err != nil {
		t.Fatalf("gpg on the manifest: %v", err)
	}
	if !strings.Contains(listing, privateFileName) {
		t.Fatal("the decrypted manifest does not list the file")
	}
	if _, err := run(pass+"\n", "gpg", gpgArgs("-d", "-o", c.Name+".tar", payload)...); err != nil {
		t.Fatalf("gpg on the payload: %v", err)
	}
	if _, err := run("", "tar", "-xf", c.Name+".tar"); err != nil {
		t.Fatalf("tar: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, privateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the restored file differs from the original")
	}
}

type handError struct {
	err    error
	stderr string
}

func (e *handError) Error() string { return e.err.Error() + ": " + e.stderr }

func copyFolderFiles(t *testing.T, from, to string, keep func(string) bool) map[string]bool {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
		if e.IsDir() || !keep(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return names
}

func all(string) bool { return true }

func TestPrivateMedium_WrittenPackageRestoresByHand(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	c, want := privateChunk(t, app, "PRIV-WRITE", false)
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(medium, c.Name)
	checkPrivateMedium(t, dir, c)
	work := filepath.Join(t.TempDir(), "hand")
	names := copyFolderFiles(t, dir, work, all)
	rt, err := os.ReadFile(filepath.Join(dir, "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	checkRestoreNames(t, c, string(rt), names)
	pass, err := app.Passphrase(c.KeyRef)
	if err != nil {
		t.Fatal(err)
	}
	byHand(t, tools, work, c, pass, want)
}

func TestPrivateMedium_BurnedPackageRestoresByHand(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	c, want := privateChunk(t, app, "PRIV-BURN", false)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	q := app.Store.AddBurnQueue(BurnQueue{Name: "Q", MediaKind: "CUSTOM", Discs: []*BurnDisc{{ChunkID: c.ID, Status: "PENDING"}}})
	disc := filepath.Join(t.TempDir(), "disc")
	prev := runBurnProgram
	runBurnProgram = func([]string) error { // the disc is what is in the folder now
		copyFolderFiles(t, c.StagedDir, disc, all)
		return nil
	}
	t.Cleanup(func() { runBurnProgram = prev })
	if _, err := app.BurnNext(q.ID, noProg); err != nil {
		t.Fatal(err)
	}
	checkPrivateMedium(t, disc, c)
	work := filepath.Join(t.TempDir(), "hand")
	names := copyFolderFiles(t, disc, work, all)
	rt, err := os.ReadFile(filepath.Join(disc, "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	checkRestoreNames(t, c, string(rt), names)
	pass, err := app.Passphrase(c.KeyRef)
	if err != nil {
		t.Fatal(err)
	}
	byHand(t, tools, work, c, pass, want)
}

func TestPrivateMedium_SpannedPackageRestoresByHand(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	c, want := privateChunk(t, app, "PRIV-SPAN", true)
	root := t.TempDir()
	var tapes []string
	for i := 0; i < 100; i++ {
		tape := filepath.Join(root, "tape"+string(rune('a'+i)))
		if err := os.MkdirAll(tape, 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := app.SpanWriteNext(c.ID, tape, 0, 0, 0, 0, noProg)
		if err != nil {
			t.Fatal(err)
		}
		tapes = append(tapes, filepath.Join(tape, c.Name))
		if done, _ := res["complete"].(bool); done {
			break
		}
	}
	if len(tapes) < 2 {
		t.Fatalf("expected several tapes, got %d", len(tapes))
	}
	work := filepath.Join(t.TempDir(), "hand")
	names := map[string]bool{}
	for _, tape := range tapes {
		checkPrivateMedium(t, tape, c)
		for n := range copyFolderFiles(t, tape, work, all) {
			names[n] = true
		}
	}
	rt, err := os.ReadFile(filepath.Join(tapes[0], "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	checkRestoreNames(t, c, string(rt), names)
	segs, err := filepath.Glob(filepath.Join(work, c.Name+".seg*"))
	if err != nil || len(segs) < 2 {
		t.Fatalf("segments %v, %v", segs, err)
	}
	sort.Strings(segs)
	var joined bytes.Buffer
	for _, s := range segs {
		b, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		joined.Write(b)
	}
	if err := os.WriteFile(filepath.Join(work, c.Name+".tar.gpg"), joined.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	pass, err := app.Passphrase(c.KeyRef)
	if err != nil {
		t.Fatal(err)
	}
	byHand(t, tools, work, c, pass, want)
}

// The recovery kit's guides must say what a private medium holds instead of the
// plaintext manifest, wherever they tell the reader to open one.
func TestRecoveryGuides_DescribeThePrivateManifest(t *testing.T) {
	for name, doc := range map[string]string{
		"kit readme": recoveryReadmeMD(1, 1),
		"runbook":    string(restoreRunbook),
	} {
		if !strings.Contains(doc, ".manifest.json.gpg") {
			t.Errorf("%s does not name the encrypted manifest", name)
		}
		if !strings.Contains(doc, "private media") && !strings.Contains(doc, "Private media") {
			t.Errorf("%s does not explain private media", name)
		}
		if !strings.Contains(doc, "gpg -d") && !strings.Contains(doc, "gpg --decrypt") {
			t.Errorf("%s does not show how to decrypt it", name)
		}
	}
	if !strings.Contains(recoveryReadmeMD(1, 1), "gpg -d NAME.manifest.json.gpg") {
		t.Error("the kit readme does not show how to read a private manifest")
	}
	if !strings.Contains(string(restoreRunbook), "gpg -d PHOTOS-C0003.manifest.json.gpg") {
		t.Error("the runbook does not show how to read a private manifest")
	}
}
