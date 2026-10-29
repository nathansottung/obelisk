//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// openpgpPacket reads one packet header at b: its tag, and where its body
// starts. A partial (streamed) length is fine: only the first body bytes are
// read.
func openpgpPacket(t *testing.T, b []byte) (tag int, body []byte, next []byte) {
	t.Helper()
	if len(b) < 2 || b[0]&0x80 == 0 {
		t.Fatalf("not an OpenPGP packet: % x", b[:min(len(b), 8)])
	}
	var n, hl int
	partial := false
	if b[0]&0x40 != 0 { // new format
		tag = int(b[0] & 0x3f)
		switch l := int(b[1]); {
		case l < 192:
			n, hl = l, 2
		case l < 224:
			n, hl = (l-192)<<8+int(b[2])+192, 3
		case l == 255:
			n, hl = int(b[2])<<24|int(b[3])<<16|int(b[4])<<8|int(b[5]), 6
		default:
			n, hl, partial = 1<<(l&0x1f), 2, true
		}
	} else { // old format
		tag = int(b[0]>>2) & 0x0f
		switch b[0] & 3 {
		case 0:
			n, hl = int(b[1]), 2
		case 1:
			n, hl = int(b[1])<<8|int(b[2]), 3
		case 2:
			n, hl = int(b[1])<<24|int(b[2])<<16|int(b[3])<<8|int(b[4]), 5
		default:
			n, hl, partial = len(b)-1, 1, true
		}
	}
	end := min(hl+n, len(b))
	body = b[hl:end]
	if !partial {
		next = b[end:]
	}
	return tag, body, next
}

// assertPinnedGpg checks a symmetric gpg file: a version-4 symmetric-key packet
// for AES-256 with an iterated and salted SHA-512 S2K at the fixed (maximum)
// count, followed by integrity-protected data (SEIPD version 1), which every
// gpg since 1.4 can decrypt.
func assertPinnedGpg(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tag, body, next := openpgpPacket(t, b)
	if tag != 3 || len(body) < 13 {
		t.Fatalf("%s: first packet tag %d (% x), want a symmetric-key packet (3)", filepath.Base(path), tag, body)
	}
	if body[0] != 4 || body[1] != 9 {
		t.Errorf("%s: packet version %d, cipher %d; want version 4, AES-256 (9)", filepath.Base(path), body[0], body[1])
	}
	if body[2] != 3 || body[3] != 10 {
		t.Errorf("%s: S2K type %d hash %d; want iterated and salted (3), SHA-512 (10)", filepath.Base(path), body[2], body[3])
	}
	if c := body[12]; c != 0xff {
		t.Errorf("%s: S2K count byte 0x%02x, want 0xff (65011712)", filepath.Base(path), c)
	}
	if len(body) != 13 {
		t.Errorf("%s: symmetric-key packet carries %d extra bytes (an encrypted session key); want none", filepath.Base(path), len(body)-13)
	}
	if next == nil {
		t.Fatalf("%s: no packet after the symmetric-key packet", filepath.Base(path))
	}
	tag, body, _ = openpgpPacket(t, next)
	if tag != 18 || len(body) == 0 || body[0] != 1 {
		t.Errorf("%s: data packet tag %d version % x; want SEIPD (18) version 1", filepath.Base(path), tag, body[:min(len(body), 1)])
	}
}

// An encrypted package's payload and its private manifest are written with
// pinned gpg settings, not whatever the local gpg defaults to.
func TestBuildChunk_GpgOutputIsPinned(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	if _, err := app.SaveConfig(map[string]any{"private_media": true}); err != nil {
		t.Fatal(err)
	}
	src, refs := makeSource(t)
	c := app.Store.AddChunk(Chunk{
		Name: "GPG-PIN", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: true,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	assertPinnedGpg(t, filepath.Join(staging, c.Name, payloadName(c)))
	assertPinnedGpg(t, filepath.Join(staging, c.Name, c.Name+".manifest.json.gpg"))
}

// Every gpg run uses its own empty home folder and caches no passphrase, so
// neither the operator's gpg.conf nor their agent's cache takes part, and the
// home folder is gone afterwards.
func TestGpgSession_IsolatedAndUncached(t *testing.T) {
	gpgBin := nativeTools(t)["gpg"]
	s, err := newGpgSession(gpgBin)
	if err != nil {
		t.Fatal(err)
	}
	home := s.home
	args := s.args()
	for _, want := range []string{"--homedir", home, "--no-options", "--no-symkey-cache", "--batch", "--pinentry-mode"} {
		if !slices.Contains(args, want) {
			t.Errorf("gpg arguments %v lack %q", args, want)
		}
	}
	if fi, err := os.Stat(home); err != nil || !fi.IsDir() {
		t.Fatalf("session home %s: %v", home, err)
	}
	s.close()
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("session home %s is still there after close: %v", home, err)
	}

	// The operator's own gpg home is not touched by a build and a restore.
	userHome := t.TempDir()
	t.Setenv("GNUPGHOME", userHome)
	app, _ := newTestApp(t, nativeTools(t))
	src, refs := makeSource(t)
	c := app.Store.AddChunk(Chunk{
		Name: "GPG-HOME", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: true,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RestoreChunk(c.ID, filepath.Join(medium, c.Name), t.TempDir(), nil, noProg); err != nil {
		t.Fatal(err)
	}
	if ents, _ := os.ReadDir(userHome); len(ents) != 0 {
		t.Errorf("gpg wrote into the operator's GNUPGHOME: %v", ents)
	}
}
