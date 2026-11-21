//go:build !guionly

package main

// session_lows_test.go: download links opened outside a normal click, recovery of an
// old tab, the forwarding page's lifetime, the parent folder's ownership, and the
// construction that protects the sign-in code.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTabRequest(t *testing.T, target, site string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Site", site)
	return req
}

func readAll(t *testing.T, req *http.Request) (*http.Response, string) {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// A middle-click arrives with the cookie and no ticket. It gets a page that fetches
// a ticket from this origin and continues.
func TestDownloadLink_NewTabGetsAPageThatContinues(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	s := signInCookie(t, ts.URL, auth)
	req := newTabRequest(t, ts.URL+"/api/x/report?format=csv", "same-origin")
	req.AddCookie(s.cookie)
	res, body := readAll(t, req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("content type %q: %s", res.Header.Get("Content-Type"), body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control %q", res.Header.Get("Cache-Control"))
	}
	for _, want := range []string{"click the link normally", "obelisk -signin", sessionTicketPath} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not say %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, s.key) || strings.Contains(body, s.cookie.Value) {
		t.Error("the page holds a session credential")
	}
}

// Only a link followed from Obelisk itself may continue on its own. A typed address,
// a link from another program, or a used ticket gets the instructions alone.
func TestDownloadLink_OnlySameOriginContinues(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	for name, target := range map[string][2]string{
		"typed or from another program": {"/api/x/report", "none"},
		"a ticket that failed":          {"/api/x/report?ticket=spent", "same-origin"},
	} {
		res, body := readAll(t, newTabRequest(t, ts.URL+target[0], target[1]))
		if res.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "click the link normally") {
			t.Errorf("%s: %d %s", name, res.StatusCode, body)
		}
		if strings.Contains(body, "<script") {
			t.Errorf("%s: the page runs a script", name)
		}
	}
	// A call from code, not a navigation, keeps the JSON answer the UI reads.
	req := uiCall(http.MethodGet, ts.URL+"/api/x", nil)
	res, body := readAll(t, req)
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") || !strings.Contains(body, `"sign_in"`) {
		t.Errorf("a fetch got %q: %s", res.Header.Get("Content-Type"), body)
	}
}

// Runs the continuation page's own script under node.
func TestDownloadLink_PageScriptFetchesATicketAndGoes(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("SKIPPED, NOT PASSED: node is not on PATH, so the page script was not run")
	}
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	_, body := readAll(t, newTabRequest(t, ts.URL+"/api/x/report?format=csv", "same-origin"))
	m := regexp.MustCompile(`(?s)<script>(.*)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the page has no script:\n%s", body)
	}
	harness := `
let went=null,sent=null;
globalThis.document={body:{textContent:'This address has to be opened from Obelisk.'}};
globalThis.localStorage={getItem:k=>k==='obelisk_page_key'?'page-key-value':null};
globalThis.location={pathname:'/api/x/report',search:'?format=csv',replace:u=>{went=u}};
globalThis.fetch=async(u,o)=>{sent={u,o};return {ok:true,json:async()=>({url:'/api/x/report?format=csv&ticket=t'})}};
` + m[1] + `
setTimeout(()=>{
  const b=sent&&JSON.parse(sent.o.body);
  if(!sent||sent.u!=='/api/session/ticket'||sent.o.method!=='POST'){console.error('no ticket request');process.exit(1)}
  if(sent.o.headers['X-Obelisk-Page-Key']!=='page-key-value'||sent.o.headers['X-Requested-By']!=='obelisk-ui'){console.error('wrong headers');process.exit(1)}
  if(b.url!=='/api/x/report?format=csv'){console.error('wrong url '+b.url);process.exit(1)}
  if(went!=='/api/x/report?format=csv&ticket=t'){console.error('did not continue: '+went);process.exit(1)}
  if(!document.body.textContent.includes('download has started')){console.error('the tab still reads as a failure: '+document.body.textContent);process.exit(1)}
  console.log('PASS');process.exit(0);
},50);
`
	file := filepath.Join(t.TempDir(), "continue.js")
	if err := os.WriteFile(file, []byte(harness), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("page script: %v\n%s", err, out)
	}
}

const oldTabAssertions = `
const __fail=[];const ck=(c,m)=>{if(!c)__fail.push(m)};
const fire=e=>(globalThis.__win.storage||[]).forEach(f=>f(e));
const d=__el('#dsignin');
d.open=true;fire({key:'obelisk_page_key',newValue:'new-key'});
ck(globalThis.__reloads===1,'a sign-in in another tab must recover a signed-out tab, reloads='+globalThis.__reloads);
fire({key:'something_else',newValue:'x'});
ck(globalThis.__reloads===1,'another storage key must not reload');
fire({key:'obelisk_page_key',newValue:null});
ck(globalThis.__reloads===1,'a removed key must not reload');
d.open=false;fire({key:'obelisk_page_key',newValue:'newer-key'});
ck(globalThis.__reloads===1,'a working tab must not be reloaded');
if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
console.log('PASS');process.exit(0);
`

func TestUI_OldTabRecoversAsSoonAsAnotherTabSignsIn(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("SKIPPED, NOT PASSED: node is not on PATH, so tab recovery was not run")
	}
	capture := `globalThis.__win={};globalThis.__reloads=0;
globalThis.addEventListener=(n,f)=>{(globalThis.__win[n]=globalThis.__win[n]||[]).push(f)};
location.reload=()=>{globalThis.__reloads++};
`
	file := filepath.Join(t.TempDir(), "old_tab.js")
	if err := os.WriteFile(file, []byte(uiPrelude+capture+uiScript(t)+oldTabAssertions), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("old tab: %v\n%s", err, out)
	}
}

// signedInServerForCommand is a real session server with this account's sign-in file.
func signedInServerForCommand(t *testing.T) (*sessionAuth, string) {
	t.Helper()
	isolateSigninFolder(t)
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	auth.port, auth.addr = port, u.Host
	if _, err := auth.publishSigninFile(); err != nil {
		t.Fatal(err)
	}
	return auth, u.Host
}

// A slow browser must still find the page, so -signin neither waits nor deletes it.
func TestSignInCommand_LeavesTheForwardingPageForTheBrowser(t *testing.T) {
	_, host := signedInServerForCommand(t)
	dir, err := signinDir()
	if err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "open-test.html")
	old := signInOpener
	signInOpener = func(string) (string, error) { return page, os.WriteFile(page, []byte("page"), 0o600) }
	defer func() { signInOpener = old }()
	start := time.Now()
	var out, errOut bytes.Buffer
	if code := runSignIn(host, false, &out, &errOut); code != 0 {
		t.Fatalf("-signin: %s", errOut.String())
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("-signin waited %v for the browser", took)
	}
	if _, err := os.Stat(page); err != nil {
		t.Fatalf("the forwarding page was removed before the browser could read it: %v", err)
	}
}

// Left-over pages go at the next -signin or start, once their links are dead.
func TestSignInCommand_ClearsOnlyExpiredForwardingPages(t *testing.T) {
	_, host := signedInServerForCommand(t)
	dir, err := signinDir()
	if err != nil {
		t.Fatal(err)
	}
	stale, fresh := filepath.Join(dir, "open-stale.html"), filepath.Join(dir, "open-fresh.html")
	for _, p := range []string{stale, fresh} {
		if err := os.WriteFile(p, []byte("page"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-3 * signInLinkLifetime)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatal(err)
	}
	old := signInOpener
	signInOpener = func(string) (string, error) { return "", nil }
	defer func() { signInOpener = old }()
	var out, errOut bytes.Buffer
	if code := runSignIn(host, false, &out, &errOut); code != 0 {
		t.Fatalf("-signin: %s", errOut.String())
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an expired page was left behind: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a page whose link may still work was removed: %v", err)
	}
}

func TestSigninDir_RefusesAParentThatFailsItsCheck(t *testing.T) {
	isolateSigninFolder(t)
	refused := errors.New("the parent is open to others")
	old := checkSigninParent
	checkSigninParent = func(string) error { return refused }
	defer func() { checkSigninParent = old }()
	if _, err := signinDir(); !errors.Is(err, refused) {
		t.Fatalf("signinDir: %v", err)
	}
	base := os.Getenv("LOCALAPPDATA")
	if _, err := os.Stat(filepath.Join(base, "Obelisk", "signin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the signin folder was created under a refused parent: %v", err)
	}
}

// A refused folder must lead to the real fix: repair it, then restart Obelisk.
func TestSignIn_RefusedFolderLeadsToARestart(t *testing.T) {
	isolateSigninFolder(t)
	t.Setenv("OBELISK_NO_BROWSER", "1")
	old := checkSigninParent
	checkSigninParent = func(string) error { return errors.New("the folder is open to others") }
	auth, _ := newSessionAuth("", 7821)
	var logged []string
	auth.startOwnerSignIn("http://127.0.0.1:7821", func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) })
	checkSigninParent = old
	joined := strings.Join(logged, "\n")
	if !strings.Contains(joined, "the folder is open to others") || !strings.Contains(joined, "stop and restart Obelisk") {
		t.Errorf("the log does not give the fix:\n%s", joined)
	}
	if strings.Contains(joined, "run: obelisk -signin") {
		t.Errorf("the log offers a command that cannot work for this run:\n%s", joined)
	}
	// The folder is fixed, but this run never wrote its file.
	var out, errOut bytes.Buffer
	if code := runSignIn("127.0.0.1:7821", true, &out, &errOut); code == 0 {
		t.Fatal("-signin succeeded with no sign-in file")
	}
	if !strings.Contains(errOut.String(), "stop and restart") {
		t.Errorf("-signin does not say to restart a running Obelisk: %s", errOut.String())
	}
}

func TestSigninParentRefusal_GivesTheFixThatApplies(t *testing.T) {
	for _, c := range []struct {
		name    string
		plain   bool
		uid     int
		perm    os.FileMode
		want    string
		notWant string
	}{
		{"open", true, 1000, 0o755, "chmod 700", "chown"},
		{"link", false, 1000, 0o700, "real folder", "chmod"},
		{"other owner", true, 1001, 0o700, "chown", "chmod"},
	} {
		err := signinParentRefusal("/x/Obelisk", c.plain, c.uid, 1000, c.perm)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), c.notWant) {
			t.Errorf("%s: %v", c.name, err)
		}
		if err != nil && !strings.Contains(err.Error(), "stop and restart Obelisk") {
			t.Errorf("%s does not say to restart: %v", c.name, err)
		}
	}
	if err := signinParentRefusal("/x/Obelisk", true, 1000, 1000, 0o700); err != nil {
		t.Errorf("a private folder was refused: %v", err)
	}
}

// A call refused with the old key, just after another tab signed in, retries with the
// new key instead of opening a sign-in box nothing will close.
func TestUI_CallRefusedWithAnOldKeyRetriesWithTheNewOne(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("SKIPPED, NOT PASSED: node is not on PATH, so the retry was not run")
	}
	assertions := `
const d=__el('#dsignin');let shown=0;d.showModal=()=>{shown++;d.open=true};
localStorage.setItem('obelisk_page_key','old-key');
const keys=[];
globalThis.__fetch=async(u,o)=>{const k=o.headers['X-Obelisk-Page-Key'];keys.push(k);
  if(k==='old-key'){localStorage.setItem('obelisk_page_key','new-key');
    return {status:401,ok:false,statusText:'Unauthorized',json:async()=>({error:'sign-in required',sign_in:'owner'})}}
  return {status:200,ok:true,statusText:'OK',json:async()=>({fine:true})}};
(async()=>{
  const r=await api('GET','/api/x');
  if(!r.fine||keys.join()!=='old-key,new-key'||shown!==0){console.error('FAIL keys='+keys+' shown='+shown);process.exit(1)}
  keys.length=0;
  globalThis.__fetch=async(u,o)=>{keys.push(o.headers['X-Obelisk-Page-Key']);
    return {status:401,ok:false,statusText:'Unauthorized',json:async()=>({error:'sign-in required',sign_in:'owner'})}};
  let threw=false;try{await api('GET','/api/x')}catch(e){threw=true}
  if(!threw||keys.length!==1||shown!==1){console.error('FAIL unchanged key: calls='+keys.length+' shown='+shown);process.exit(1)}
  console.log('PASS');process.exit(0);
})();
`
	file := filepath.Join(t.TempDir(), "old_key.js")
	if err := os.WriteFile(file, []byte(uiPrelude+uiScript(t)+assertions), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("retry: %v\n%s", err, out)
	}
}

func TestOwnerOnlyProblem(t *testing.T) {
	for _, c := range []struct {
		name  string
		plain bool
		uid   int
		perm  os.FileMode
		want  string
	}{
		{"private", true, 1000, 0o700, ""},
		{"a link", false, 1000, 0o700, "not a plain"},
		{"another owner", true, 1001, 0o700, "another account"},
		{"group can enter", true, 1000, 0o750, "another account"},
		{"anyone can enter", true, 1000, 0o755, "another account"},
		{"anyone can write", true, 1000, 0o777, "another account"},
	} {
		err := ownerOnlyProblem("/x/Obelisk", c.plain, c.uid, 1000, c.perm)
		if c.want == "" && err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: got %v, want %q", c.name, err, c.want)
		}
	}
}

func TestDocs_StateWhatSignInDoesNotCover(t *testing.T) {
	for _, file := range []string{"README.md", filepath.Join("docs", "KNOWN_ISSUES.md")} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(b)), " ")
		const want = "Sign-in keeps out other accounts on the PC and web pages. It does not keep out programs running under your own account, or an administrator."
		if !strings.Contains(text, want) {
			t.Errorf("%s does not state the limit", file)
		}
	}
}

func TestReenterSeal_RefusesAlteredOrMisdirectedAnswers(t *testing.T) {
	const key, addr = "key", "127.0.0.1:7821"
	nonce := strings.Repeat("ab", 32)
	code := []byte("the-sign-in-code")
	sealed := sealReentryCode(key, addr, nonce, code)
	if got, err := openReentryCode(key, addr, nonce, sealed); err != nil || !bytes.Equal(got, code) {
		t.Fatalf("round trip: %q %v", got, err)
	}
	altered := append([]byte{}, sealed...)
	altered[len(altered)-1] ^= 1
	for name, try := range map[string]func() ([]byte, error){
		"altered":       func() ([]byte, error) { return openReentryCode(key, addr, nonce, altered) },
		"wrong key":     func() ([]byte, error) { return openReentryCode("other", addr, nonce, sealed) },
		"wrong address": func() ([]byte, error) { return openReentryCode(key, "[::1]:7821", nonce, sealed) },
		"wrong request": func() ([]byte, error) { return openReentryCode(key, addr, strings.Repeat("cd", 32), sealed) },
		"too short":     func() ([]byte, error) { return openReentryCode(key, addr, nonce, sealed[:4]) },
	} {
		if _, err := try(); err == nil {
			t.Errorf("%s: opened", name)
		}
	}
}

// With a short repeating mask, two ciphertext bytes one period apart XOR to the same
// value as the two plaintext bytes, which gives away part of the code.
func TestReenterSeal_MaskDoesNotRepeat(t *testing.T) {
	code := []byte(strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789-_ABCDEFG", 3))
	sealed := sealReentryCode("key", "127.0.0.1:7821", strings.Repeat("ab", 32), code)
	if len(sealed) <= len(code) {
		t.Fatalf("sealed is %d bytes for a %d-byte code, so nothing authenticates it", len(sealed), len(code))
	}
	for _, offset := range []int{0, 12} {
		for period := 16; period <= 64; period += 16 {
			matches, total := 0, 0
			for i := 0; i+period < len(code) && offset+i+period < len(sealed); i++ {
				total++
				if sealed[offset+i]^sealed[offset+i+period] == code[i]^code[i+period] {
					matches++
				}
			}
			if total > 0 && matches == total {
				t.Fatalf("the mask repeats every %d bytes", period)
			}
		}
	}
}
