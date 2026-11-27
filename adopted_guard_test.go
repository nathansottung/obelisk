//go:build !guionly

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adoptedPackage records an adopted backup as adopt.go does: as found, with
// one verified copy where it was found.
func adoptedPackage(t *testing.T, app *App, status string, encrypted bool) (*Chunk, string) {
	t.Helper()
	where := filepath.Join(t.TempDir(), "FOUND")
	if err := os.MkdirAll(where, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(where, "FOUND.tar"), []byte("as found"), 0o644); err != nil {
		t.Fatal(err)
	}
	vol := app.Store.AddVolume(Volume{Label: "FOUND-VOL", Kind: "HDD"})
	c := app.Store.AddChunk(Chunk{Name: "FOUND", Status: status, Encrypted: encrypted, Adopted: true,
		WrittenDest: where, HashAlg: "SHA256", MediaKind: "HDD"})
	app.Store.RecordAdoptedCopy(c, vol.ID, where, "")
	return app.Store.Chunk(c.ID), where
}

// No code path encrypts, rebuilds, re-writes or writes over an adopted backup,
// whatever state it reaches; an encrypted copy needs a new package.
func TestAdopted_NeverEncryptedOrRewritten(t *testing.T) {
	app, _ := newTestApp(t, nil)
	for _, status := range []string{StatusAdoptedVerified, "PLANNED", "FAILED", "WRITTEN", "STAGED"} {
		c, where := adoptedPackage(t, app, status, false)
		before, _ := os.ReadFile(filepath.Join(where, "FOUND.tar"))
		if err := app.BuildChunk(c.ID, noProg); err == nil || !strings.Contains(err.Error(), "adopted as found") {
			t.Fatalf("%s: an adopted package was built: %v", status, err)
		}
		if code, _, raw := keysAPI(t, app, "POST", "/api/chunks/"+itoaForTest(c.ID)+"/build", nil); code == 200 {
			t.Fatalf("%s: the build route started a build of an adopted package: %s", status, raw)
		}
		if got := app.Store.Chunk(c.ID); got.Encrypted || got.KeyRef != "" || got.StagedDir != "" {
			t.Fatalf("%s: an adopted package was changed: %+v", status, got)
		}
		if _, err := app.RewriteCopy(c.ID, c.Copies[0].VolumeID, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "adopted as found") {
			t.Fatalf("%s: an adopted copy was re-written: %v", status, err)
		}
		if _, err := app.WriteChunkOver(c.ID, filepath.Dir(where), 0, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "adopted as found") {
			t.Fatalf("%s: an adopted copy was written over: %v", status, err)
		}
		for op, params := range map[string]map[string]any{
			"rewrite-copy": {"id": c.ID, "volume_id": c.Copies[0].VolumeID},
			"write-over":   {"id": c.ID, "dest_dir": filepath.Dir(where)},
		} {
			if _, err := app.PrepareDestructive(testSession, op, params); err == nil || !strings.Contains(err.Error(), "adopted as found") {
				t.Fatalf("%s: %s of an adopted copy was offered: %v", status, op, err)
			}
		}
		if _, err := app.CreateKey("", keyPurposePackage, c.ID); err == nil {
			t.Fatalf("%s: a key was made to encrypt an adopted package", status)
		}
		if after, _ := os.ReadFile(filepath.Join(where, "FOUND.tar")); string(after) != string(before) {
			t.Fatalf("%s: the adopted backup's bytes changed", status)
		}
	}
}

// gpg encryption is called only from BuildChunk, which refuses an adopted
// package before anything else: no other code path can encrypt one.
func TestAdopted_OnlyTheBuildEncrypts(t *testing.T) {
	fset := token.NewFileSet()
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "gpgEncrypt" && fd.Name.Name != "BuildChunk" {
					t.Errorf("%s: %s calls gpgEncrypt; only BuildChunk may, behind its adopted-package refusal", fset.Position(call.Pos()), fd.Name.Name)
				}
				return true
			})
		}
	}
}

const uiAdoptedAssertions = `
const __fail=[];const ck=(c,m)=>{if(!c)__fail.push(m)};
const row=s=>{chunksRowProbe=s;return s};
const plain={id:7,name:'FOUND',status:'ADOPTED-VERIFIED',encrypted:false,adopted:true,written_dest:'E:\\FOUND',copies:[]};
const a=actions(plain);
ck(!a.includes('doBuild'),'an adopted package must offer no Build');
ck(!a.includes('separateKeyDlg'),'an adopted package must offer no key, so it cannot be encrypted');
ck(a.includes(ADOPTED_NOTICE),'an adopted package must say why: '+a);
const failed=actions({...plain,status:'FAILED'});
ck(!failed.includes('doBuild'),'an adopted package that reached FAILED must still offer no Build');
const own=actions({id:8,name:'NEW',status:'PLANNED',encrypted:true,adopted:false,copies:[]});
ck(own.includes('doBuild')&&own.includes('separateKeyDlg'),'a planned encrypted package offers Build and the separate key option');
if(__fail.length){console.error('FAIL\n - '+__fail.join('\n - '));process.exit(1)}
console.log('PASS');process.exit(0);
`

func TestUI_AdoptedPackageShowsAdoptedAsFoundAndNoEncrypt(t *testing.T) {
	runUIHarness(t, "ui_adopted_harness.js", "let chunksRowProbe;"+uiAdoptedAssertions,
		"node is not installed: the adopted-package controls are not checked")
}
