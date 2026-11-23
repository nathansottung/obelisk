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
	if err := app.BuildChunk(c.ID, noProg); err == nil {
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
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	if c.CurrentCopyCount() != 1 {
		t.Fatalf("the spanned copy is no longer current: %+v", c.Copies)
	}
	for _, tape := range tapes {
		found := false
		for _, sg := range c.Copies[0].Segments {
			found = found || normPath(sg.Dest) == normPath(tape)
		}
		if !found {
			t.Errorf("the record of %s was lost: %+v", filepath.Base(tape), c.Copies)
		}
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
	if err := app.BuildChunk(c.ID, noProg); err != nil {
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
	err := app.BuildChunk(c.ID, noProg)
	clearFailSave(app.Store)
	if err == nil || !strings.Contains(err.Error(), "catalog could not record it as staged") {
		t.Fatalf("got %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if c.KeyRef != key || c.EncHash != hash || c.WrittenDest != dest || c.VerifiedCopyCount() != 1 || c.Status != "FAILED" {
		t.Fatalf("the unsaved rebuild changed the record: status=%s verified=%d", c.Status, c.VerifiedCopyCount())
	}
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("running Build again: %v", err)
	}
}

// A check recorded while a rebuild runs survives that rebuild's failure, and the old
// copy's sidecar hashes stay on record throughout, so a crash cannot lose them.
func TestRebuild_FailureKeepsAResultRecordedDuringTheBuild(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := writtenThenFailed(t, app, "CHECKED-DURING")
	hadSidecars := false
	prev := buildAfterTarHook
	buildAfterTarHook = func(tarPath string) {
		live := app.Store.Chunk(c.ID)
		hadSidecars = len(live.Sidecars) > 0
		bad := false
		live.Copies[0].VerifyOK = &bad // a verify found the copy damaged
		if err := os.MkdirAll(filepath.Join(filepath.Dir(tarPath), c.Name+".tar.gpg"), 0o755); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { buildAfterTarHook = prev })
	if err := app.BuildChunk(c.ID, noProg); err == nil {
		t.Fatal("the build did not fail")
	}
	if !hadSidecars {
		t.Error("the sidecar hashes were cleared while the old copy still depends on them")
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 0 || len(got.Copies) != 1 {
		t.Fatalf("the failed rebuild undid the damaged-copy result: verified=%d copies=%d", got.VerifiedCopyCount(), len(got.Copies))
	}
}

// A verify must not compare a copy with a hash a running build may yet take back:
// it waits its turn, and a campaign reports the package as skipped.
func TestVerify_DoesNotRunAgainstAPackageBeingBuilt(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium := writtenThenFailed(t, app, "VERIFY-DURING")
	var single error
	var campaign map[string]any
	prev := buildAfterTarHook
	buildAfterTarHook = func(tarPath string) {
		live := app.Store.Chunk(c.ID)
		good := live.EncHash
		live.EncHash = "a hash this build may take back"
		_, single = app.VerifyChunk(c.ID, medium)
		campaign, _ = app.VerifyCampaign(medium, VerifyB, noProg)
		live.EncHash = good
		if err := os.MkdirAll(filepath.Join(filepath.Dir(tarPath), c.Name+".tar.gpg"), 0o755); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { buildAfterTarHook = prev })
	if err := app.BuildChunk(c.ID, noProg); err == nil {
		t.Fatal("the build did not fail")
	}
	if single == nil || !strings.Contains(single.Error(), "being built") {
		t.Fatalf("a verify ran during the build: %v", single)
	}
	if campaign == nil || campaign["skipped"] != 1 {
		t.Fatalf("the campaign did not skip the package: %v", campaign)
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 {
		t.Fatalf("a check against the build's hash marked a good copy bad: %+v", got.Copies)
	}
}

// With no current copy, search still says where the old copies are.
func TestSearch_ShowsRetiredCopiesWhenNoCurrentCopyExists(t *testing.T) {
	s := newIT(t)
	s.setConfig(map[string]any{"keystore_paths": s.makeKeystores(2)})
	src := s.makeSource(map[string][]byte{"find-me.txt": []byte("still on the old medium\n")})
	c := s.scanPlanBuild(src, 1, true, 10)
	pid := int(c["id"].(float64))
	vid := s.addVolume("OLD-1", "vault")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": t.TempDir(), "volume_id": vid}))
	ch := s.app.Store.Chunk(pid)
	s.app.Store.SupersedeCopy(ch, vid)
	rows, err := s.app.Store.Search(SearchQuery{Text: "find-me", Limit: 5})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %v, %v", rows, err)
	}
	old, _ := rows[0]["retired_copies"].([]map[string]any)
	if len(old) != 1 || old[0]["volume_label"] != "OLD-1" {
		t.Fatalf("the old copy is not shown: %v", rows[0]["retired_copies"])
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
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.Chunk(c.ID).Status; got != "WRITTEN" {
		t.Fatalf("status %s: the failed copy is still on record, so the package is WRITTEN", got)
	}
}

const rebuildWarningUI = `
const __fail=[];const ck=(c,m)=>{if(!c)__fail.push(m)};
let posts=[],asked=[];
globalThis.confirm=m=>{asked.push(m);return false};
const reply=(status,out)=>({status,ok:status<400,statusText:'x',json:async()=>out,text:async()=>JSON.stringify(out)});
globalThis.__fetch=async(u,o)=>{
  if(u!=='/api/chunks/7/build')return reply(200,{});
  const b=o.body?JSON.parse(o.body):{};posts.push(b);
  return reply(200,{job_id:5});
};
(async()=>{
  await doBuild(7);await dlgGo();
  ck(posts.length===1&&!posts[0].accept_existing_copies,'a build needs one request and no acceptance; got '+JSON.stringify(posts));
  ck(asked.length===0,'a rebuild must not ask; got '+JSON.stringify(asked));
  ck(__toasts.some(t=>t.text.includes('Build started')),'the build must start; got '+JSON.stringify(__toasts));
  const offers=c=>actions({id:7,name:'P',segments:[],...c}).includes('doBuild');
  ck(offers({status:'WRITTEN',verify_ok:false}),'a written package whose copy failed must offer Build');
  ck(!offers({status:'WRITTEN',verify_ok:false,adopted:true}),'an adopted package has no source to build from');
  ck(!offers({status:'VERIFIED',verify_ok:true}),'a verified package must not offer Build');
  const where=searchCopies({chunk:'P',copies:[],retired_copies:[{volume_label:'OLD-1',path:'E:/P'}]});
  ck(where.includes('OLD-1')&&where.includes('hand')&&!where.includes('no copies recorded'),'search must show where a retired copy is; got '+where);
  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  const camp=(skipped,passed)=>({status:'COMPLETED',kind:'verify',result:{checked:3,ok:passed,passed,skipped,results:[{chunk:'BUSY-PKG',ok:false,skipped:'being built'}]}});
  ck(jobStampText(camp(1,2))==='COMPLETED','a campaign whose only shortfall is a skipped package is not CHECK FAILED; got '+jobStampText(camp(1,2)));
  ck(jobStamp(camp(1,2))!=='VERIFIED','a campaign with a skipped package is not VERIFIED');
  ck(jobStampText(camp(1,1))==='CHECK FAILED','a real failure beside a skip is still CHECK FAILED; got '+jobStampText(camp(1,1)));
  ck(jobSkippedNote(camp(1,2)).includes('BUSY-PKG')&&jobSkippedNote(camp(1,2)).includes('again'),'the job must name the skipped package and say to verify again');
  ck(jobArtifact(camp(1,2)).includes('BUSY-PKG'),'the Jobs row must show the skipped package');
  if(__fail.length){console.error('FAIL: '+__fail.join(' | '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

func TestBuildUI_RebuildsWithoutAsking(t *testing.T) {
	runUIHarness(t, "rebuild_warning_ui.js", rebuildWarningUI,
		"SKIPPED, NOT PASSED: node is not on PATH, so the page's build flow was not run.")
}
