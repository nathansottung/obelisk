//go:build !guionly

package main

import (
	"fmt"
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
  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`

func TestBuildUI_AsksBeforeRebuildingOverCopies(t *testing.T) {
	runUIHarness(t, "rebuild_warning_ui.js", rebuildWarningUI,
		"SKIPPED, NOT PASSED: node is not on PATH, so the page's rebuild warning was not run.")
}
