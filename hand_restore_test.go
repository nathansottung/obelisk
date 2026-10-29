//go:build !guionly

package main

// hand_restore_test.go — a package restores by hand, with nothing but the
// steps its RESTORE.txt gives and the stock tools: par2, gpg, and the
// system's own tar (bsdtar on Windows, GNU tar on Linux). No Obelisk code runs
// in the restore. Each command line is taken from RESTORE.txt and run through
// the platform shell (cmd on Windows, sh elsewhere) in a folder holding only
// what the media hold; the restored files must equal the source byte for byte.
//
// The one thing added is how the passphrase is typed: a person types it at
// gpg's prompt, the test gives it on stdin. For that, and only on gpg lines,
// the test adds options that read the passphrase from stdin and keep gpg away
// from the machine's own gpg home.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// handRestoreSteps are the commands of RESTORE.txt's numbered steps, in order,
// for this platform: the indented command lines up to the integrity line,
// without the repair (run only when verify fails) and the one-file example.
// For a spanned package, the rejoin line for this platform is taken.
func handRestoreSteps(t *testing.T, txt string) []string {
	t.Helper()
	var steps []string
	for _, line := range strings.Split(txt, "\n") {
		if strings.HasPrefix(line, "Integrity") {
			break
		}
		if !strings.HasPrefix(line, "     ") {
			continue
		}
		cmd := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(cmd, "Windows:"):
			if runtime.GOOS != "windows" {
				continue
			}
			cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "Windows:"))
		case strings.HasPrefix(cmd, "Unix/mac:"):
			if runtime.GOOS == "windows" {
				continue
			}
			cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "Unix/mac:"))
		}
		if strings.Contains(cmd, "(only if verify fails)") || strings.Contains(cmd, "path/to/one/file") {
			continue
		}
		first := strings.Fields(cmd)[0]
		switch first {
		case "par2", "gpg", "tar", "copy", "cat":
			steps = append(steps, cmd)
		default:
			t.Fatalf("RESTORE.txt step %q runs %q, which is not one of the stock tools", cmd, first)
		}
	}
	if len(steps) == 0 {
		t.Fatalf("no restore steps found in RESTORE.txt:\n%s", txt)
	}
	return steps
}

// runHandStep runs one RESTORE.txt command in dir through the platform shell,
// with the stock tools first on PATH.
func runHandStep(t *testing.T, dir, cmd, pass string, toolDirs []string) {
	t.Helper()
	line := cmd
	var stdin string
	if strings.HasPrefix(cmd, "gpg ") {
		home := t.TempDir()
		t.Cleanup(func() { (&gpgSession{bin: filepath.Join(toolDirs[1], "gpg"), home: home}).close() })
		line = fmt.Sprintf("gpg --homedir %s --batch --pinentry-mode loopback --passphrase-fd 0 %s", shellQuote(home), strings.TrimPrefix(cmd, "gpg "))
		stdin = pass
	}
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command(os.Getenv("ComSpec"), "/c", line)
	} else {
		c = exec.Command("/bin/sh", "-c", line)
	}
	c.Dir = dir
	c.Env = append(os.Environ(), "PATH="+strings.Join(append(toolDirs, os.Getenv("PATH")), string(os.PathListSeparator)))
	c.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	if err := c.Run(); err != nil {
		t.Fatalf("RESTORE.txt step %q failed: %v\n%s", cmd, err, out.String())
	}
}

// shellQuote quotes p for sh. On Windows the test folders hold no spaces, and
// a quoted argument would be escaped again on its way to cmd.
func shellQuote(p string) string {
	if runtime.GOOS == "windows" {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// toolDirs are the folders of the stock tools, put first on PATH.
func toolDirs(tools map[string]string) []string {
	var dirs []string
	for _, n := range []string{"tar", "gpg", "par2"} {
		dirs = append(dirs, filepath.Dir(tools[n]))
	}
	return dirs
}

// gatherMedia copies every file on the media into one folder, as a person
// gathering a package's tapes or drives into one folder would.
func gatherMedia(t *testing.T, into string, media ...string) {
	t.Helper()
	for _, m := range media {
		err := filepath.WalkDir(m, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			dst := filepath.Join(into, d.Name())
			if old, err := os.ReadFile(dst); err == nil {
				if !bytes.Equal(old, b) {
					return fmt.Errorf("two media hold different files named %s", d.Name())
				}
				return nil
			}
			return os.WriteFile(dst, b, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// handSource writes a tree with Unicode and nested names, and returns its refs.
func handSource(t *testing.T, big int) (string, []ChunkFileRef) {
	t.Helper()
	src := t.TempDir()
	files := map[string]string{
		"hello.txt":           "the quick brown fox\n",
		"sub/notes.txt":       "a second file in a subfolder\n",
		"café.txt":            "accented\n",
		"日本語/写真.txt":          "japanese folder and name\n",
		"\U0001d54fstar★.txt": "supplementary plane and a symbol\n",
	}
	if big > 0 {
		files["big.dat"] = strings.Repeat("spanning-payload-", big)
	}
	var refs []ChunkFileRef
	for rel, body := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body))
		refs = append(refs, ChunkFileRef{RelPath: rel, SizeBytes: int64(len(body)), Hash: hex.EncodeToString(sum[:])})
	}
	return src, refs
}

// assertHandRestored compares every source file with the one restored by hand.
func assertHandRestored(t *testing.T, src, out string, refs []ChunkFileRef) {
	t.Helper()
	for _, r := range refs {
		want, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(r.RelPath)))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(r.RelPath)))
		if err != nil {
			t.Errorf("the hand restore did not produce %q: %v", r.RelPath, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("the hand-restored %q differs from the source", r.RelPath)
		}
	}
}

func TestHandRestore_FromRESTORE_txt(t *testing.T) {
	tools := nativeTools(t)
	for _, encrypted := range []bool{false, true} {
		t.Run(fmt.Sprintf("encrypted=%v", encrypted), func(t *testing.T) {
			app, _ := newTestApp(t, tools)
			src, refs := handSource(t, 0)
			c := app.Store.AddChunk(Chunk{
				Name: "HAND", Status: "PLANNED", MediaKind: "CUSTOM",
				TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
				SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: encrypted,
				Files: append([]ChunkFileRef{}, refs...),
			})
			if err := app.BuildChunk(c.ID, noProg); err != nil {
				t.Fatal(err)
			}
			medium := t.TempDir()
			if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
				t.Fatal(err)
			}
			c = app.Store.Chunk(c.ID)
			pass := ""
			if encrypted {
				var err error
				if pass, err = app.Passphrase(c.KeyRef); err != nil {
					t.Fatal(err)
				}
			}
			out := t.TempDir()
			gatherMedia(t, out, filepath.Join(medium, c.Name))
			txt, err := os.ReadFile(filepath.Join(out, "RESTORE.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range handRestoreSteps(t, string(txt)) {
				runHandStep(t, out, step, pass, toolDirs(tools))
			}
			assertHandRestored(t, src, out, refs)
		})
	}
}

// A spanned package restores by hand from its tapes gathered into one folder.
// Its RESTORE.txt names the par2 set and says to gather it with the segments:
// without it, step 1's par2 verify has nothing to read.
func TestHandRestore_SpannedFromRESTORE_txt(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	src, refs := handSource(t, 2500)
	var data int64
	for _, r := range refs {
		data += r.SizeBytes
	}
	c := app.Store.AddChunk(Chunk{
		Name: "HAND-SPAN", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 12000, DataBytes: data, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Spanned: true,
		Segments: planSegments(data+8192, 12000),
		Files:    append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	var media []string
	for i := 0; i < 50; i++ {
		m := t.TempDir()
		media = append(media, m)
		res, err := app.SpanWriteNext(c.ID, m, 0, 0, 0, 0, noProg)
		if err != nil {
			t.Fatal(err)
		}
		if done, _ := res["complete"].(bool); done {
			break
		}
	}
	c = app.Store.Chunk(c.ID)
	if len(media) < 2 {
		t.Fatalf("setup: the package fit on %d medium", len(media))
	}
	out := t.TempDir()
	gatherMedia(t, out, media...)
	b, err := os.ReadFile(filepath.Join(out, "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	txt := string(b)
	pl := payloadName(c)
	step0 := txt
	if i := strings.Index(txt, "STEP 0"); i >= 0 {
		step0 = txt[i:]
		if j := strings.Index(step0, "1)"); j >= 0 {
			step0 = step0[:j]
		}
	}
	for _, want := range []string{pl + ".par2", pl + ".vol", "par2 set"} {
		if !strings.Contains(step0, want) {
			t.Errorf("spanned RESTORE.txt step 0 does not mention %q:\n%s", want, step0)
		}
	}
	for _, step := range handRestoreSteps(t, txt) {
		runHandStep(t, out, step, "", toolDirs(tools))
	}
	assertHandRestored(t, src, out, refs)
}
