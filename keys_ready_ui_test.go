//go:build !guionly

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runUIAgainstServer runs the real ui/index.html script under node with its
// fetch sent to base, a real API server, so the page drives the app itself.
func runUIAgainstServer(t *testing.T, base, assertions string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the page flow from first start to an encrypted build is not exercised")
	}
	wire := fmt.Sprintf(`globalThis.__realFetch=globalThis.fetch;const __base=%q;`, base)
	run := `
const __fail=[];const ck=(c,m)=>{if(!c)__fail.push(m)};
process.on('unhandledRejection',e=>{__fail.push('unhandled rejection: '+((e&&e.message)||e))});
globalThis.__fetch=(u,o)=>globalThis.__realFetch(__base+u,o);
globalThis.navigator=globalThis.navigator||{};
(async()=>{try{` + assertions + `}catch(e){__fail.push('threw: '+(e&&e.stack||e))}
  if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
  console.log('PASS');process.exit(0)})();`
	harness := filepath.Join(t.TempDir(), "ui_keys_flow.js")
	if err := os.WriteFile(harness, []byte(wire+uiPrelude+"\n"+uiScript(t)+"\n"+run), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("page flow failed:\n%s", out)
	}
	return string(out)
}

// On a fresh data folder the page alone takes the user from a first encrypted
// build attempt to a finished encrypted build: the refusal leads to Create key,
// which asks for two keystores, then an offline copy (a printout), then the
// proof by typing it back. No API call is made by hand.
func TestUI_FirstStartToEncryptedBuildThroughThePage(t *testing.T) {
	dataDir := t.TempDir()
	store, err := openTestStore(t, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	app := initializedTestApp(t, &App{DataDir: dataDir, Store: store})
	if _, err := app.SaveConfig(map[string]any{"staging_dir": t.TempDir(), "par2_redundancy": 5}); err != nil {
		t.Fatal(err)
	}
	if n := len(mustConfig(t, app).KeystorePaths); n != 0 {
		t.Fatalf("a fresh data folder already has %d keystores", n)
	}
	c := plannedEncrypted(t, app, "FIRSTSTART")
	mux := http.NewServeMux()
	api(mux, app)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ks1, _ := json.Marshal(filepath.Join(t.TempDir(), "keys.json"))
	ks2, _ := json.Marshal(filepath.Join(t.TempDir(), "keys.json"))

	assertions := fmt.Sprintf(`
const PKG=%d, KS1=%s, KS2=%s;
const dlgHTML=()=>__el('#dlg').innerHTML;
// The checklist leads to the key before the first encrypted build.
let gs=gettingStarted({staging_dir:'x'},{keystores:{ok:false},key_ready:false},[],[],null);
ck(gs.includes('Create your key')&&gs.includes("loc('keys')"),'onboarding must lead to Keys before the first encrypted build');
// 1. The Keys page offers Create key, and says nothing is ready.
await vKeys();
ck(__el('#view').innerHTML.includes('createKeyDlg()'),'the Keys page must offer Create key');
ck(__el('#view').innerHTML.includes('No key is ready'),'the Keys page must say no key is ready');
// 2. A build is refused, and the refusal links straight to Create key.
await doBuild(PKG); await dlgGo(); await sleep(50);
ck(dlgHTML().includes('No key is ready for this build')&&dlgHTML().includes('Create key'),'the build refusal must lead to Create key: '+dlgHTML().slice(0,300));
await dlgGo(); await sleep(300);
ck(location.hash==='#keys','the refusal must open the Keys page');
// 3. Create key asks for the two keystores first.
ck(dlgHTML().includes('ckks1')&&dlgHTML().includes('ckks2'),'Create key must ask for two keystore locations');
__el('#ckks1').value=KS1; __el('#ckks2').value=KS2; __el('#ckname').value='Family archive';
await dlgGo(); await sleep(400);
let h=dlgHTML();
ck(h.includes('Make an offline copy of Family archive'),'creating the key must go on to the offline copy: '+h.slice(0,200));
ck(h.includes('can never be opened'),'the copy step must say why it matters');
const m=/data-call="keyPrintout" data-args="\[&quot;([^&]+)&quot;\]"/.exec(h);
ck(!!m,'the copy dialog must offer the printout');
const ref=m?m[1]:'';
ck(!__el('#kcprintout').innerHTML.includes('<tr>'),'the key must not be on screen before the click');
// 4. The printout, shown after a click.
await keyPrintout(ref);
const rows=[...__el('#kcprintout').innerHTML.matchAll(/<td class="mono">(L\d+\/\d+)<\/td><td class="mono"[^>]*>([^<]+)<\/td><td class="mono">([0-9A-F]{4})<\/td>/g)];
ck(rows.length>=2,'the printout must have short numbered lines with check codes');
ck(/SHA-256 [0-9a-f]{64}/.test(__el('#kcprintout').innerHTML),'the printout must carry the full SHA-256 fingerprint');
// 5. Prove it by typing it back.
await dlgGo(); await sleep(100);
ck(dlgHTML().includes('Prove your copy'),'the proof must follow the copy');
__el('#kpkind').value='printout';
__el('#kptext').value=rows.map(r=>r[1]+'  '+r[2]+'  ['+r[3]+']').join('\n');
await keyProve(ref);
ck(__el('#kpout').innerHTML.includes('Proven'),'typing the printout back must prove the key: '+__el('#kpout').innerHTML);
ck(__el('#kptext').value==='','the typed key must be cleared from the page once proven');
// 6. The build now runs to the end.
await doBuild(PKG); await dlgGo();
let st='';
for(let i=0;i<600&&!['STAGED','FAILED'].includes(st);i++){await sleep(100);
  const cs=await api('GET','/api/chunks');st=((cs||[]).find(x=>x.id===PKG)||{}).status||''}
ck(st==='STAGED','the encrypted build must finish: '+st);
console.log('REF='+ref);
`, c.ID, ks1, ks2)
	out := runUIAgainstServer(t, srv.URL, assertions)
	got := app.Store.Chunk(c.ID)
	if got.Status != "STAGED" || !got.Encrypted || got.KeyRef == "" || !strings.Contains(out, "REF="+got.KeyRef) {
		t.Fatalf("the package was not built with the key made on the page: %+v\n%s", got, out)
	}
	if m := app.Store.KeyMetaCopy(got.KeyRef); m == nil || m.ProvenAt == nil || m.Name != "Family archive" {
		t.Fatalf("the key must be the proven one named on the page: %+v", m)
	}
}
