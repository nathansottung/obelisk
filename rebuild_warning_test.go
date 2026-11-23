//go:build !guionly

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The server starts no rebuild over existing copies until the user has accepted what
// it does to them, and then it does start one: there is no dead end.
func TestBuildAPI_WarnsThenRebuildsOverExistingCopies(t *testing.T) {
	s := newIT(t)
	s.setConfig(map[string]any{"keystore_paths": s.makeKeystores(2)})
	src := s.makeSource(map[string][]byte{"a.txt": []byte("kept on the medium\n")})
	c := s.scanPlanBuild(src, 1, true, 10)
	pid := int(c["id"].(float64))
	vid := s.addVolume("COPY-1", "vault")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": t.TempDir(), "volume_id": vid}))
	ch := s.app.Store.Chunk(pid)
	oldKey := ch.KeyRef
	ch.Status = "FAILED"
	s.app.Store.UpdateChunk(ch)

	m := s.obj("POST", fmt.Sprintf("/api/chunks/%d/build", pid), nil)
	if s.status(m) != 409 || m["needs_confirmation"] != true {
		t.Fatalf("no warning before the rebuild: %v", m)
	}
	warning, _ := m["warning"].(string)
	if !strings.Contains(warning, "verifying") || !strings.Contains(warning, "hand restore") {
		t.Fatalf("the warning does not say what happens: %q", warning)
	}
	if got := s.app.Store.Chunk(pid); got.KeyRef != oldKey || got.Status != "FAILED" {
		t.Fatalf("an unaccepted rebuild changed the package: key changed=%v status=%s", got.KeyRef != oldKey, got.Status)
	}

	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/build", pid), map[string]any{"accept_existing_copies": true}))
	if got := s.app.Store.Chunk(pid); got.KeyRef == oldKey || got.Status != "STAGED" && got.Status != "VERIFIED" {
		t.Fatalf("the accepted rebuild did not run: key changed=%v status=%s", got.KeyRef != oldKey, got.Status)
	}
}

// writtenThenFailed is a private package with one verified copy, marked FAILED.
func writtenThenFailed(t *testing.T, app *App, name string) (*Chunk, string) {
	t.Helper()
	c, _ := privateChunk(t, app, name, false)
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	return c, medium
}

// After an accepted rebuild no old copy may count as protecting the new build, and
// the record of where it is must stay.
func TestRebuild_AcceptedRetiresOldCopiesAndKeepsWhereTheyAre(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := writtenThenFailed(t, app, "RETIRED-COPY")
	path := c.Copies[0].Path
	if err := app.BuildChunkOverCopies(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	if c.VerifiedCopyCount() != 0 || c.CurrentCopyCount() != 0 || c.VerifyOK != nil {
		t.Fatalf("an old copy still counts: verified=%d current=%d", c.VerifiedCopyCount(), c.CurrentCopyCount())
	}
	if len(c.Copies) != 1 || c.Copies[0].Path != path || !c.Copies[0].Superseded {
		t.Fatalf("the old copy's location was not kept: %+v", c.Copies)
	}
	if c.Status != "STAGED" {
		t.Fatalf("status %s", c.Status)
	}
}

// A rebuild that fails leaves the old copies usable: the old key and hash stay.
func TestRebuild_FailedAfterKeyGenerationKeepsTheOldKeyAndHash(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium := writtenThenFailed(t, app, "FAILED-REBUILD")
	key, hash, tarHash := c.KeyRef, c.EncHash, c.TarHash
	prev := buildAfterTarHook
	buildAfterTarHook = func(tarPath string) { // gpg cannot write its output over a folder
		if err := os.MkdirAll(filepath.Join(filepath.Dir(tarPath), c.Name+".tar.gpg"), 0o755); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { buildAfterTarHook = prev })
	if err := app.BuildChunkOverCopies(c.ID, noProg); err == nil {
		t.Fatal("the build did not fail")
	}
	c = app.Store.Chunk(c.ID)
	if c.KeyRef != key || c.EncHash != hash || c.TarHash != tarHash {
		t.Fatal("a failed rebuild replaced the key or hash the old copy needs")
	}
	if c.VerifiedCopyCount() != 1 {
		t.Fatalf("a failed rebuild retired the copy: %+v", c.Copies)
	}
	if len(c.Sidecars) == 0 {
		t.Fatal("a failed rebuild lost the sidecar hashes that check the old copy")
	}
	if m, err := app.ReadMediumManifest(c.ID, medium); err != nil || m == nil {
		t.Fatalf("the old copy no longer reads: %v", err)
	}
}

// A spanned set is recorded as one copy; every tape's location must survive.
func TestRebuild_SpannedKeepsEveryTapesLocation(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "SPAN-TAPES", true)
	root := t.TempDir()
	var tapes []string
	for i := 0; i < 50; i++ {
		tape := filepath.Join(root, fmt.Sprintf("tape%02d", i))
		if err := os.MkdirAll(tape, 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := app.SpanWriteNext(c.ID, tape, 0, 0, 0, 0, noProg)
		if err != nil {
			t.Fatal(err)
		}
		tapes = append(tapes, tape)
		if done, _ := res["complete"].(bool); done {
			break
		}
	}
	c = app.Store.Chunk(c.ID)
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if err := app.BuildChunkOverCopies(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	for _, tape := range tapes {
		found := false
		for _, cp := range c.Copies {
			found = found || (cp.Superseded && strings.Contains(normPath(cp.Path), normPath(tape)))
		}
		if !found {
			t.Errorf("the record of %s was lost: %+v", filepath.Base(tape), c.Copies)
		}
	}
	if c.CurrentCopyCount() != 0 {
		t.Fatalf("%d old copies still count", c.CurrentCopyCount())
	}
}

// The tapes' locations are on record before the build replaces their segments, so a
// crash during the build cannot lose them.
func TestRebuild_OldTapesAreRecordedBeforeTheBuildReplacesThem(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "SPAN-EARLY", true)
	tape := t.TempDir()
	if _, err := app.SpanWriteNext(c.ID, tape, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	recorded := false
	prev := buildAfterTarHook
	buildAfterTarHook = func(string) {
		for _, cp := range app.Store.Chunk(c.ID).Copies {
			recorded = recorded || (cp.Superseded && strings.Contains(normPath(cp.Path), normPath(tape)))
		}
	}
	t.Cleanup(func() { buildAfterTarHook = prev })
	if err := app.BuildChunkOverCopies(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if !recorded {
		t.Fatal("the tape was not on record while the build ran")
	}
}

// A rebuild the catalog cannot record did not happen: the old copies stay as they
// were, and running Build again works.
func TestRebuild_UnsavedResultLeavesTheOldCopiesInPlace(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := writtenThenFailed(t, app, "UNSAVED-REBUILD")
	key, hash, dest := c.KeyRef, c.EncHash, c.WrittenDest
	app.Store.failSave = func() error { return errors.New("disk full (injected)") }
	err := app.BuildChunkOverCopies(c.ID, noProg)
	clearFailSave(app.Store)
	if err == nil || !strings.Contains(err.Error(), "catalog could not record it as staged") {
		t.Fatalf("got %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if c.KeyRef != key || c.EncHash != hash || c.WrittenDest != dest || c.VerifiedCopyCount() != 1 || c.Status != "FAILED" {
		t.Fatalf("the unsaved rebuild changed the record: status=%s verified=%d", c.Status, c.VerifiedCopyCount())
	}
	if err := app.BuildChunkOverCopies(c.ID, noProg); err != nil {
		t.Fatalf("running Build again: %v", err)
	}
}

// After an accepted rebuild, search still says where the old copies are.
func TestSearch_ShowsRetiredCopiesWhenNoCurrentCopyExists(t *testing.T) {
	s := newIT(t)
	s.setConfig(map[string]any{"keystore_paths": s.makeKeystores(2)})
	src := s.makeSource(map[string][]byte{"find-me.txt": []byte("still on the old medium\n")})
	c := s.scanPlanBuild(src, 1, true, 10)
	pid := int(c["id"].(float64))
	vid := s.addVolume("OLD-1", "vault")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": t.TempDir(), "volume_id": vid}))
	ch := s.app.Store.Chunk(pid)
	ch.Status = "FAILED"
	s.app.Store.UpdateChunk(ch)
	if err := s.app.BuildChunkOverCopies(pid, noProg); err != nil {
		t.Fatal(err)
	}
	rows, err := s.app.Store.Search(SearchQuery{Text: "find-me", Limit: 5})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %v, %v", rows, err)
	}
	old, _ := rows[0]["retired_copies"].([]map[string]any)
	if len(old) != 1 || old[0]["volume_label"] != "OLD-1" {
		t.Fatalf("the old copy is not shown: %v", rows[0]["retired_copies"])
	}
}

// The warning says what really stops working, and speaks of a key only if there is one.
func TestRebuildWarning_MatchesThePackage(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	ok := true
	enc := app.Store.AddChunk(Chunk{Name: "ENC", Status: "FAILED", Encrypted: true, Copies: []Copy{{Path: "x", VerifyOK: &ok}}})
	plain := app.Store.AddChunk(Chunk{Name: "PLAIN", Status: "FAILED", Copies: []Copy{{Path: "x", VerifyOK: &ok}}})
	e := app.Store.rebuildWarning(enc.ID).Error()
	if !strings.Contains(e, "restoring") || !strings.Contains(e, "kept key") {
		t.Errorf("encrypted: %s", e)
	}
	p := app.Store.rebuildWarning(plain.ID).Error()
	if strings.Contains(p, "key") || !strings.Contains(p, "hand restore") {
		t.Errorf("unencrypted: %s", p)
	}
}

// A written package whose only copy failed and whose staging is gone can be rebuilt.
func TestRebuild_NoDeadEndWhenTheCopyFailedAndStagingIsGone(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := writtenThenFailed(t, app, "NO-DEAD-END")
	bad := false
	c.Copies[0].VerifyOK, c.VerifyOK, c.Status = &bad, &bad, "WRITTEN"
	app.Store.UpdateChunk(c)
	if err := app.BuildChunk(c.ID, noProg); err == nil || !strings.Contains(err.Error(), "still staged") {
		t.Fatalf("with staging present the answer should be to write it again: %v", err)
	}
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	var warning *RebuildWarning
	if err := app.BuildChunk(c.ID, noProg); !errors.As(err, &warning) {
		t.Fatalf("no warning: %v", err)
	}
	if err := app.BuildChunkOverCopies(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.Chunk(c.ID).Status; got != "STAGED" {
		t.Fatalf("status %s", got)
	}
}

const rebuildWarningUI = `
const __fail=[];const ck=(c,m)=>{if(!c)__fail.push(m)};
const W='Rebuilding will stop its copies from verifying';
let accept=false,posts=[],asked=[];
globalThis.confirm=m=>{asked.push(m);return accept};
const reply=(status,out)=>({status,ok:status<400,statusText:'x',json:async()=>out,text:async()=>JSON.stringify(out)});
globalThis.__fetch=async(u,o)=>{
  if(u!=='/api/chunks/7/build')return reply(200,{});
  const b=o.body?JSON.parse(o.body):{};posts.push(b);
  return b.accept_existing_copies?reply(200,{job_id:5}):reply(409,{error:W,needs_confirmation:true,warning:W,copies:1});
};
(async()=>{
  await doBuild(7);await dlgGo();
  ck(posts.length===1,'declining the warning must not send a second build; got '+JSON.stringify(posts));
  ck(asked.length===1&&asked[0].includes(W),'the user must be shown the warning; got '+JSON.stringify(asked));
  ck(__toasts.some(t=>t.text.includes('Nothing was rebuilt')),'declining must say nothing was rebuilt; got '+JSON.stringify(__toasts));
  posts=[];asked=[];accept=true;__toasts.length=0;
  await doBuild(7);await dlgGo();
  ck(posts.length===2&&posts[1].accept_existing_copies===true,'accepting must send the build again, accepted; got '+JSON.stringify(posts));
  ck(__toasts.some(t=>t.text.includes('Build started')),'accepting must start the build; got '+JSON.stringify(__toasts));
  const offers=c=>actions({id:7,name:'P',segments:[],...c}).includes('doBuild');
  ck(offers({status:'WRITTEN',verify_ok:false}),'a written package whose copy failed must offer Build');
  ck(!offers({status:'WRITTEN',verify_ok:false,adopted:true}),'an adopted package has no source to build from');
  ck(!offers({status:'VERIFIED',verify_ok:true}),'a verified package must not offer Build');
  const where=searchCopies({chunk:'P',copies:[],retired_copies:[{volume_label:'OLD-1',path:'E:/P'}]});
  ck(where.includes('OLD-1')&&where.includes('hand')&&!where.includes('no copies recorded'),'search must show where a retired copy is; got '+where);
  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

func TestBuildUI_AsksBeforeRebuildingOverCopies(t *testing.T) {
	runUIHarness(t, "rebuild_warning_ui.js", rebuildWarningUI,
		"SKIPPED, NOT PASSED: node is not on PATH, so the page's rebuild warning was not run.")
}
