//go:build !guionly

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The secret leaves the server only in the reply to a deliberate reveal on the
// Keys page. No GET route hands it out: not as text and not inside a QR image.
func TestKeyReady_NoGetRouteServesTheKey(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, map[string]any{"name": "No GET"})
	mux := http.NewServeMux()
	api(mux, app)
	for _, p := range []string{"/api/keys/" + ref + "/qr.png", "/api/keys/" + ref} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code == 200 {
			t.Fatalf("GET %s served %q (%d bytes): a key leaves the server only through a reveal",
				p, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
}

// Two different secrets under one key, one in the backup and one in the
// keystore already here, cannot be merged. The restore finds that before it
// changes anything: the catalog and the keystore stay as they were.
func TestAppRestore_ConflictingKeySecretRefusesBeforeChangingAnything(t *testing.T) {
	src := abTestApp(t)
	abPopulate(t, src)
	ks := mustConfig(t, src).KeystorePaths[0]
	if err := writeStore(ks, &keystoreFile{Marker: 1, Keys: []map[string]any{keyRec("K-SAME", "secret-in-backup")}}); err != nil {
		t.Fatal(err)
	}
	res, err := src.ExportAppBackup(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}

	dst := abTestApp(t)
	dest := filepath.Join(dst.DataDir, "keystores", filepath.Base(ks))
	if err := writeStore(dest, &keystoreFile{Marker: 1, Keys: []map[string]any{keyRec("K-SAME", "another-secret-here")}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(dest)
	_, rerr := dst.RestoreAppBackup(res.TarPath)
	if rerr == nil || !strings.Contains(rerr.Error(), "before changing anything") {
		t.Fatalf("two secrets under one key must stop the restore before it changes anything: %v", rerr)
	}
	if after, _ := os.ReadFile(dest); string(after) != string(before) {
		t.Fatal("the keystore changed")
	}
	if n := len(dst.Store.Chunks(0)); n != 0 {
		t.Fatalf("the refused restore changed the catalog: %d packages", n)
	}
	if n := len(dst.Store.Collections()); n != 0 {
		t.Fatalf("the refused restore changed the catalog: %d archives", n)
	}
}

// An adopted backup is never written over, by its own package or by another one
// whose folder would land on it. A write-over replaces only the files of its own
// package's folder, so writing a new package's folder elsewhere on the drive of
// an adopted backup (one adopted at a drive's root, say) is not refused.
func TestAdopted_AnotherPackageNeverWritesOverAnAdoptedCopy(t *testing.T) {
	app, _ := newTestApp(t, nil)
	adopted, where := adoptedPackage(t, app, StatusAdoptedVerified, false)
	before, _ := os.ReadFile(filepath.Join(where, "FOUND.tar"))
	other := app.Store.AddChunk(Chunk{Name: adopted.Name, Status: "STAGED", HashAlg: "SHA256", MediaKind: "HDD"})
	if _, err := app.WriteChunkOver(other.ID, filepath.Dir(where), 0, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "adopted as found") {
		t.Fatalf("package %s was written over the adopted copy at %s: %v", other.Name, where, err)
	}
	if _, err := app.PrepareDestructive(testSession, "write-over", map[string]any{"id": other.ID, "dest_dir": filepath.Dir(where)}); err == nil || !strings.Contains(err.Error(), "adopted as found") {
		t.Fatalf("a write-over of package %s onto the adopted copy was offered: %v", other.Name, err)
	}
	if after, _ := os.ReadFile(filepath.Join(where, "FOUND.tar")); string(after) != string(before) {
		t.Fatal("the adopted backup's bytes changed")
	}
	// A folder of its own under the adopted copy's folder is not that copy.
	inside := app.Store.AddChunk(Chunk{Name: "INSIDE", Status: "STAGED", HashAlg: "SHA256", MediaKind: "HDD"})
	if err := app.refuseAdoptedRewrite(inside, filepath.Join(where, inside.Name), false); err != nil {
		t.Fatalf("a package's own folder under an adopted copy's folder was refused: %v", err)
	}
}

// What was typed into the proof leaves the page when the dialog closes, whether
// the proof failed or was never sent, so no copy of the key stays in the page.
func TestUI_ProveDialogLeavesNoTypedKeyWhenClosed(t *testing.T) {
	app, _ := newBareTestApp(t, nil)
	ref := createKeyAPI(t, app, map[string]any{"name": "Typed"})
	mux := http.NewServeMux()
	api(mux, app)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	assertions := fmt.Sprintf(`
const REF=%q;
const d=__el('#dlg');const L={};
d.addEventListener=(ev,fn,o)=>{(L[ev]=L[ev]||[]).push({fn,once:!!(o&&o.once)})};
d.close=()=>{const fs=L.close||[];L.close=fs.filter(x=>!x.once);fs.forEach(x=>x.fn())};
// A proof that does not match, then the dialog closes.
proveKeyDlg(REF);
__el('#kpkind').value='printout';
__el('#kptext').value='L01/02 ALMOSTTHEKEY [0000]';
await keyProve(REF);
ck(__el('#kpout').innerHTML.includes('Not proven'),'the wrong copy must not prove: '+__el('#kpout').innerHTML);
d.close();
ck(__el('#kptext').value==='','a failed proof must leave the page when the dialog closes: '+__el('#kptext').value);
// A late close event of the dialog before must not use up the wipe.
proveKeyDlg(REF);
d.close();
proveKeyDlg(REF);
__el('#kptext').value='TYPED AFTER A LATE CLOSE';
d.close();
ck(__el('#kptext').value==='','typed text must leave the page after a late close event');
// Typed, never sent, then cancelled.
proveKeyDlg(REF);
__el('#kptext').value='TYPED BUT NEVER SENT';
d.close();
ck(__el('#kptext').value==='','typed text must leave the page when the dialog is cancelled');
`, ref)
	runUIAgainstServer(t, srv.URL, assertions)
}
