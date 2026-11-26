//go:build !guionly

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// copiedPackage is a real private package with one copy written to a new medium.
func copiedPackage(t *testing.T, app *App, name string) (*Chunk, string, []byte) {
	t.Helper()
	c, body := privateChunk(t, app, name, false)
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
		t.Fatal(err)
	}
	return app.Store.Chunk(c.ID), medium, body
}

// rebuildPackage makes a new build of a package that has copies.
func rebuildPackage(t *testing.T, app *App, c *Chunk) *Chunk {
	t.Helper()
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return app.Store.Chunk(c.ID)
}

func snapshotCopy(cp Copy) Copy {
	cp.Sidecars = cloneSidecars(cp.Sidecars)
	cp.Segments = append([]Segment(nil), cp.Segments...)
	return cp
}

func wantVerifies(t *testing.T, app *App, id int, medium string) {
	t.Helper()
	r, err := app.VerifyChunk(id, medium)
	if err != nil {
		t.Fatalf("verify %s: %v", medium, err)
	}
	if ok, _ := r["verify_ok"].(bool); !ok {
		t.Fatalf("the copy did not verify: %v", r)
	}
	if n, _ := r["credited"].(int); n != 1 {
		t.Fatalf("the verify credited %d copies, want 1", n)
	}
}

func wantRestores(t *testing.T, app *App, id int, source string, body []byte) {
	t.Helper()
	out := t.TempDir()
	if _, err := app.RestoreChunk(id, source, out, nil, noProg); err != nil {
		t.Fatalf("restore from %s: %v", source, err)
	}
	got, err := os.ReadFile(filepath.Join(out, privateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatal("the restored file differs from the original")
	}
}

// The problem itself: a copy on media keeps verifying after its package is rebuilt.
func TestPerCopy_OldCopyStillVerifiesAfterARebuild(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "OLD-VERIFIES")
	oldHash := c.EncHash
	c = rebuildPackage(t, app, c)
	if c.EncHash == oldHash {
		t.Fatal("the rebuild did not make a new payload")
	}
	wantVerifies(t, app, c.ID, medium)
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 {
		t.Fatalf("the old copy lost its credit: %+v", got.Copies)
	}
}

func TestPerCopy_CampaignChecksEachCopyAgainstItsOwnHash(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "OLD-CAMPAIGN")
	c = rebuildPackage(t, app, c)
	res, err := app.VerifyCampaign(medium, VerifyB, noProg)
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != 1 {
		t.Fatalf("the campaign failed the old copy: %v", res)
	}
}

// A rebuild adds new values and changes no existing copy.
func TestPerCopy_RebuildChangesNoExistingCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _, _ := copiedPackage(t, app, "UNCHANGED-COPY")
	before := snapshotCopy(c.Copies[0])
	if before.KeyRef != c.KeyRef || before.PayloadHash != c.EncHash || before.TarHash != c.TarHash || len(before.Sidecars) == 0 {
		t.Fatalf("the copy did not record its own values: %+v", before)
	}
	// The user makes a new key (rotation); the rebuild uses it.
	newKey := readyTestKey(t, app)
	c = rebuildPackage(t, app, c)
	if c.KeyRef == before.KeyRef || c.KeyRef != newKey {
		t.Fatal("the rebuild did not use the new current key")
	}
	if len(c.Copies) != 1 || !reflect.DeepEqual(snapshotCopy(c.Copies[0]), before) {
		t.Fatalf("the rebuild changed the copy:\n was %+v\n now %+v", before, c.Copies)
	}
}

// Old and new copies each restore in the app with their own key.
func TestPerCopy_RestoreUsesEachCopysOwnKey(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, oldMedium, body := copiedPackage(t, app, "OWN-KEY")
	c = rebuildPackage(t, app, c)
	newMedium := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "NEW-COPY", Kind: "HDD"})
	if _, err := app.WriteChunk(c.ID, newMedium, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	wantRestores(t, app, c.ID, filepath.Join(oldMedium, c.Name), body)
	wantRestores(t, app, c.ID, filepath.Join(newMedium, c.Name), body)
	wantVerifies(t, app, c.ID, oldMedium)
	wantVerifies(t, app, c.ID, newMedium)
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 2 {
		t.Fatalf("verified copies = %d, want 2", got.VerifiedCopyCount())
	}
}

// An old copy's encrypted file list opens with its own key.
func TestPerCopy_OldPrivateManifestOpensWithItsOwnKey(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "OWN-MANIFEST")
	c = rebuildPackage(t, app, c)
	if m, err := app.ReadMediumManifest(c.ID, medium); err != nil || m == nil {
		t.Fatalf("the old copy's manifest did not open: %v", err)
	}
}

// A spanned set keeps its own tapes, key and hash through a rebuild.
func TestPerCopy_SpannedSetKeepsItsOwnRecord(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, body := privateChunk(t, app, "OLD-SPAN", true)
	root := t.TempDir()
	pool := filepath.Join(root, "pool")
	for i := 0; i < 50; i++ {
		tape := filepath.Join(root, fmt.Sprintf("tape%02d", i))
		if err := os.MkdirAll(tape, 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := app.SpanWriteNext(c.ID, tape, 0, 0, 0, 0, noProg)
		if err != nil {
			t.Fatal(err)
		}
		copyFolderFiles(t, filepath.Join(tape, c.Name), pool, all)
		if done, _ := res["complete"].(bool); done {
			break
		}
	}
	c = app.Store.Chunk(c.ID)
	before := snapshotCopy(c.Copies[0])
	if len(before.Segments) == 0 {
		t.Fatal("the spanned copy did not record its segments")
	}
	c = rebuildPackage(t, app, c)
	if len(c.Copies) != 1 || !reflect.DeepEqual(snapshotCopy(c.Copies[0]), before) {
		t.Fatalf("the rebuild changed the spanned copy:\n was %+v\n now %+v", before, c.Copies)
	}
	if c.VerifiedCopyCount() != 1 {
		t.Fatal("the spanned copy lost its credit")
	}
	vols := map[int]*Volume{}
	for _, v := range app.Store.Volumes() {
		vols[v.ID] = v
	}
	if len(chunkPhysCopies(c, vols, nil)) != 1 {
		t.Fatal("the old spanned set no longer counts as a physical copy")
	}
	wantRestores(t, app, c.ID, pool, body)
}

// A copy's values survive a reopen, on whichever engine this run uses.
func TestPerCopy_ValuesSurviveAReopen(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	dir := app.DataDir
	c, _, _ := copiedPackage(t, app, "REOPENED")
	want := snapshotCopy(c.Copies[0])
	id := c.ID
	if err := app.Store.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := openTestStore(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	app.Store = s
	got := s.Chunk(id).Copies[0]
	if got.KeyRef != want.KeyRef || got.PayloadHash != want.PayloadHash || got.TarHash != want.TarHash ||
		!reflect.DeepEqual(got.Sidecars, want.Sidecars) {
		t.Fatalf("the copy's values were lost:\n was %+v\n now %+v", want, got)
	}
}

// A value a copy already holds is never overwritten by running the migration again.
func TestPerCopy_MigrationIsIdempotent(t *testing.T) {
	c := catalog{Chunks: []*Chunk{{KeyRef: "new-key", EncHash: "new-hash", Copies: []Copy{
		{Path: "old", KeyRef: "old-key", PayloadHash: "old-hash"}, {Path: "bare"}}}}}
	migrateCopyValuesV9(&c)
	migrateCopyValuesV9(&c)
	if cp := c.Chunks[0].Copies[0]; cp.KeyRef != "old-key" || cp.PayloadHash != "old-hash" {
		t.Fatalf("a copy's own values were replaced: %+v", cp)
	}
	if cp := c.Chunks[0].Copies[1]; cp.KeyRef != "new-key" || cp.PayloadHash != "new-hash" {
		t.Fatalf("a bare copy did not inherit: %+v", cp)
	}
}

// The old key stays readable from every keystore after a rebuild and a sync.
func TestPerCopy_KeyStaysWhileACopyNeedsIt(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _, _ := copiedPackage(t, app, "KEPT-KEY")
	oldKey := c.KeyRef
	c = rebuildPackage(t, app, c)
	if _, err := app.SyncKeystores(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Passphrase(oldKey); err != nil {
		t.Fatalf("the old copy's key is gone: %v", err)
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.KeystorePaths {
		ks, err := readStore(p)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, k := range ks.Keys {
			found = found || k["key_ref"] == oldKey
		}
		if !found {
			t.Fatalf("keystore %s no longer holds the old copy's key", p)
		}
	}
	if missing := app.keysCopiesNeedMissing(); len(missing) != 0 {
		t.Fatalf("keys reported missing: %v", missing)
	}
}

// A copy whose key is in no keystore is named, so nobody finds out at restore time.
func TestPerCopy_NamesAKeyNoKeystoreHolds(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	ok := true
	app.Store.AddChunk(Chunk{Name: "NO-KEY", Status: "VERIFIED", Encrypted: true, KeyRef: "gone",
		Copies: []Copy{{Path: "x", KeyRef: "gone", PayloadHash: "h", VerifyOK: &ok}}})
	if missing := app.keysCopiesNeedMissing(); len(missing) != 1 || missing[0] != "gone" {
		t.Fatalf("missing = %v", missing)
	}
}

// Staging gone: refill it from a verified copy, then write a new copy from it.
func TestPerCopy_RefillFromAVerifiedCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, body := copiedPackage(t, app, "REFILL")
	vol := c.Copies[0].VolumeID
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RefillStaging(c.ID, vol, "", noProg); err != nil {
		t.Fatalf("refill: %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if err := chunkStagedForWrite(c); err != nil {
		t.Fatalf("the refilled staging cannot be written: %v", err)
	}
	second := t.TempDir()
	other := app.Store.AddVolume(Volume{Label: "SECOND", Kind: "HDD"})
	if _, err := app.WriteChunk(c.ID, second, 0, 0, 0, other.ID, noProg); err != nil {
		t.Fatal(err)
	}
	wantVerifies(t, app, c.ID, medium)
	wantVerifies(t, app, c.ID, second)
	wantRestores(t, app, c.ID, filepath.Join(second, c.Name), body)
}

// A refill from an older copy puts that copy's key and hash back as the package's.
func TestPerCopy_RefillFromAnOlderCopyUsesItsValues(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _, _ := copiedPackage(t, app, "REFILL-OLD")
	vol, oldKey, oldHash := c.Copies[0].VolumeID, c.KeyRef, c.EncHash
	c = rebuildPackage(t, app, c)
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RefillStaging(c.ID, vol, "", noProg); err != nil {
		t.Fatalf("refill: %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if c.KeyRef != oldKey || c.EncHash != oldHash {
		t.Fatal("the package does not describe what is now in staging")
	}
	if err := chunkStagedForWrite(c); err != nil {
		t.Fatal(err)
	}
}

// A refill is checked against the copy's hash before use, and a bad one changes nothing.
func TestPerCopy_RefillRefusesADamagedCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "REFILL-BAD")
	vol, staged := c.Copies[0].VolumeID, c.StagedDir
	payload := filepath.Join(staged, payloadName(c))
	stagedBytes, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payload, append([]byte("damaged"), stagedBytes...), 0o644); err != nil {
		t.Fatal(err)
	}
	flipByte(t, filepath.Join(medium, c.Name, payloadName(c)))
	before, _ := os.ReadFile(payload)
	if _, err := app.RefillStaging(c.ID, vol, "", noProg); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a damaged copy was accepted: %v", err)
	}
	if after, _ := os.ReadFile(payload); string(after) != string(before) {
		t.Fatal("a refused refill changed staging")
	}
}

func TestPerCopy_RefillNeedsAVerifiedCopyAndABrokenStaging(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _, _ := copiedPackage(t, app, "REFILL-RULES")
	vol := c.Copies[0].VolumeID
	if _, err := app.RefillStaging(c.ID, vol, "", noProg); err == nil || !strings.Contains(err.Error(), "intact") {
		t.Fatalf("a refill over intact staging: %v", err)
	}
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	bad := false
	c.Copies[0].VerifyOK = &bad
	app.Store.UpdateChunk(c)
	if _, err := app.RefillStaging(c.ID, vol, "", noProg); err == nil || !strings.Contains(err.Error(), "verified") {
		t.Fatalf("a refill from an unverified copy: %v", err)
	}
}

// A failed rewrite never marks the good copy as replaced.
func TestPerCopy_FailedRewriteKeepsTheGoodCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "GOOD-REWRITE")
	vol := c.Copies[0].VolumeID
	// A damaged staged payload stops the write before anything reaches the medium.
	flipByte(t, filepath.Join(c.StagedDir, payloadName(c)))
	if _, err := app.RewriteCopy(c.ID, vol, 0, 0, 0, noProg); err == nil {
		t.Fatal("the rewrite did not fail")
	}
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.Copies[0].Superseded || got.VerifiedCopyCount() != 1 {
		t.Fatalf("the failed rewrite retired the good copy: %+v", got.Copies)
	}
	wantVerifies(t, app, c.ID, medium)
}

// A rewrite refused by a sidecar failure after an identical payload keeps the copy.
func TestPerCopy_RewriteFailingAtASidecarKeepsTheSameBytesCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "SIDECAR-REWRITE")
	vol := c.Copies[0].VolumeID
	prev := writeAfterPayloadHook
	writeAfterPayloadHook = func() error { return errors.New("the medium failed after the payload (injected)") }
	t.Cleanup(func() { writeAfterPayloadHook = prev })
	if _, err := app.RewriteCopy(c.ID, vol, 0, 0, 0, noProg); err == nil {
		t.Fatal("the rewrite did not fail")
	}
	writeAfterPayloadHook = prev
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.Copies[0].Superseded {
		t.Fatalf("a failed rewrite of the same bytes retired the copy: %+v", got.Copies)
	}
	wantVerifies(t, app, c.ID, medium)
}

// A successful rewrite retires the old record and adds a new one with new values.
func TestPerCopy_SuccessfulRewriteAfterARebuildRecordsNewValues(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "NEW-REWRITE")
	vol, oldKey := c.Copies[0].VolumeID, c.KeyRef
	c = rebuildPackage(t, app, c)
	if _, err := app.RewriteCopy(c.ID, vol, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	got := app.Store.Chunk(c.ID)
	if len(got.Copies) != 2 || !got.Copies[0].Superseded || got.Copies[0].KeyRef != oldKey ||
		got.Copies[1].Superseded || got.Copies[1].KeyRef != got.KeyRef {
		t.Fatalf("copies after the rewrite: %+v", got.Copies)
	}
	wantVerifies(t, app, c.ID, medium)
}

// Damage par2 can repair reads as repairable, not as lost.
func TestPerCopy_DamageWithinPar2IsRepairable(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, body := copiedPackage(t, app, "REPAIRABLE")
	flipByte(t, filepath.Join(medium, c.Name, payloadName(c)))
	r, err := app.VerifyChunk(c.ID, medium)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := r["verify_ok"].(bool); ok {
		t.Fatal("the damage was not found")
	}
	if rep, _ := r["repairable"].(bool); !rep {
		t.Fatalf("the verify did not say repairable: %v", r)
	}
	got := app.Store.Chunk(c.ID)
	if !got.Copies[0].Repairable || got.VerifiedCopyCount() != 0 {
		t.Fatalf("copy state: %+v", got.Copies[0])
	}
	if s := copyResult(got.Copies[0]); !strings.Contains(s, "repairable") || strings.Contains(s, "FAILED") {
		t.Fatalf("the copy reads %q", s)
	}
	wantRestores(t, app, c.ID, filepath.Join(medium, c.Name), body)
}

func TestPerCopy_DamageBeyondPar2IsNotRepairable(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "NOT-REPAIRABLE")
	dir := filepath.Join(medium, c.Name)
	matches, _ := filepath.Glob(filepath.Join(dir, "*.vol*.par2"))
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			t.Fatal(err)
		}
	}
	flipByte(t, filepath.Join(dir, payloadName(c)))
	r, err := app.VerifyChunk(c.ID, medium)
	if err != nil {
		t.Fatal(err)
	}
	if rep, _ := r["repairable"].(bool); rep {
		t.Fatal("damage with no recovery blocks was called repairable")
	}
	if app.Store.Chunk(c.ID).Copies[0].Repairable {
		t.Fatal("the copy was marked repairable")
	}
}

// A good verify clears "repairable".
func TestPerCopy_AGoodCheckClearsRepairable(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "CLEARS")
	payload := filepath.Join(medium, c.Name, payloadName(c))
	good, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	flipByte(t, payload)
	if _, err := app.VerifyChunk(c.ID, medium); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payload, good, 0o644); err != nil {
		t.Fatal(err)
	}
	wantVerifies(t, app, c.ID, medium)
	if app.Store.Chunk(c.ID).Copies[0].Repairable {
		t.Fatal("a clean copy still reads repairable")
	}
}

// The rebuild warning is gone: the API starts the build with no confirmation.
func TestPerCopy_RebuildNeedsNoConfirmation(t *testing.T) {
	s := newIT(t)
	s.setConfig(map[string]any{"keystore_paths": s.makeKeystores(2)})
	src := s.makeSource(map[string][]byte{"a.txt": []byte("kept on the medium\n")})
	c := s.scanPlanBuild(src, 1, true, 10)
	pid := int(c["id"].(float64))
	vid := s.addVolume("COPY-1", "vault")
	medium := t.TempDir()
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": medium, "volume_id": vid}))
	ch := s.app.Store.Chunk(pid)
	oldHash := ch.EncHash
	ch.Status = "FAILED"
	s.app.Store.UpdateChunk(ch)
	m := s.obj("POST", fmt.Sprintf("/api/chunks/%d/build", pid), nil)
	if s.status(m) == http.StatusConflict {
		t.Fatalf("the rebuild still asks: %v", m)
	}
	s.job(m)
	got := s.app.Store.Chunk(pid)
	if got.EncHash == oldHash || got.VerifiedCopyCount() != 1 {
		t.Fatalf("after the rebuild: new hash=%v verified=%d", got.EncHash != oldHash, got.VerifiedCopyCount())
	}
}

// The refill is reachable over the API.
func TestPerCopy_RefillAPI(t *testing.T) {
	s := newIT(t)
	s.setConfig(map[string]any{"keystore_paths": s.makeKeystores(2)})
	src := s.makeSource(map[string][]byte{"a.txt": []byte("kept on the medium\n")})
	c := s.scanPlanBuild(src, 1, true, 10)
	pid := int(c["id"].(float64))
	vid := s.addVolume("COPY-1", "vault")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": t.TempDir(), "volume_id": vid}))
	ch := s.app.Store.Chunk(pid)
	if err := os.RemoveAll(ch.StagedDir); err != nil {
		t.Fatal(err)
	}
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/refill", pid), map[string]any{"volume_id": vid}))
	if err := chunkStagedForWrite(s.app.Store.Chunk(pid)); err != nil {
		t.Fatalf("after the refill: %v", err)
	}
}

// An old copy and a new copy each come back by hand with par2, gpg and tar.
func TestPerCopy_HandRestoreOfOldAndNewCopies(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	c, oldMedium, want := copiedPackage(t, app, "BY-HAND")
	oldChunk := *c
	oldKey := c.KeyRef
	c = rebuildPackage(t, app, c)
	newMedium := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "NEW-HAND", Kind: "HDD"})
	if _, err := app.WriteChunk(c.ID, newMedium, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		medium, key string
		pkg         *Chunk
	}{{oldMedium, oldKey, &oldChunk}, {newMedium, c.KeyRef, c}} {
		pass, err := app.Passphrase(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		work := filepath.Join(t.TempDir(), "hand")
		copyFolderFiles(t, filepath.Join(tc.medium, c.Name), work, all)
		byHand(t, tools, work, tc.pkg, pass, want)
	}
}

// The field names are the on-disk contract.
func TestPerCopy_JSONFieldNames(t *testing.T) {
	b, err := json.Marshal(Copy{KeyRef: "k", PayloadHash: "h", TarHash: "t", Sidecars: map[string]string{"a": "b"},
		Segments: []Segment{{Index: 1}}, Repairable: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"key_ref", "payload_hash", "tar_hash", "sidecars", "segments", "repairable"} {
		if !strings.Contains(string(b), `"`+name+`"`) {
			t.Errorf("%s is not in %s", name, b)
		}
	}
}

func flipByte(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)/2] ^= 0xFF
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
