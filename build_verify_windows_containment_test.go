package main

// build_verify_windows_containment_test.go — OBX-006 containment.
//
// On Windows the external tar this build path uses can misread the NUL-delimited
// member list in the host ANSI code page and archive a DIFFERENT, similarly-named
// file while exiting 0 ("café.txt" requested, "cafÃ©.txt" archived). verifyTarContents
// is the only thing that compares the archive against the catalog, and the "none" tier
// switches it off — so at that tier a wrongly named package could be staged, written
// and read back with every one of those checks passing.
//
// Until the native writer lands, such a build is refused. These tests prove the refusal
// is driven by the EFFECTIVE archive configuration, that it happens before tar runs, a
// key is generated or the package becomes eligible as staged, and that it does not
// disturb Contents/Full builds.
//
// The tests drive real production paths (BuildChunk) with disposable synthetic
// fixtures. They set buildUsesWindowsExternalTarHook so the containment can be
// exercised on any platform; production leaves it nil and the real GOOS decides.
//
// This is containment, NOT a Unicode compatibility fix. tar_unicode_names_test.go
// still fails on this path with verification enabled, and is meant to.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// onWindowsTarPath forces the containment predicate for one test and restores it.
func onWindowsTarPath(t *testing.T, windows bool) {
	t.Helper()
	prev := buildUsesWindowsExternalTarHook
	buildUsesWindowsExternalTarHook = func() bool { return windows }
	t.Cleanup(func() { buildUsesWindowsExternalTarHook = prev })
}

// tarInvocationWatch reports whether the build reached the external tar. It reuses the
// existing buildAfterTarHook seam, which fires on the freshly written tar between the
// tar call and stage verification — so it can only fire if tar actually ran and
// produced an archive. TestContainment_WatchDetectsTarInvocation is the positive
// control proving this watch is not vacuous.
func tarInvocationWatch(t *testing.T) *bool {
	t.Helper()
	var ran bool
	prev := buildAfterTarHook
	buildAfterTarHook = func(string) { ran = true }
	t.Cleanup(func() { buildAfterTarHook = prev })
	return &ran
}

// containmentSource is a disposable ASCII fixture — ordinary, supported content, so a
// failure here is about the containment and never about Unicode handling.
func containmentSource(t *testing.T) (string, []ChunkFileRef) {
	t.Helper()
	src := t.TempDir()
	return src, makeUnicodeSource(t, src, []string{"ordinary.txt", "sub/plain.txt"})
}

func containmentChunk(app *App, src string, refs []ChunkFileRef, encrypted bool) *Chunk {
	return app.Store.AddChunk(Chunk{
		Name: "CONTAIN", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: encrypted,
		Files: append([]ChunkFileRef{}, refs...),
	})
}

// assertRefused checks the error is the containment refusal and that it tells the
// operator the three things the decision requires it to say.
func assertRefused(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("build must be REFUSED when content verification is disabled on the Windows external-tar path")
	}
	msg := err.Error()
	for _, want := range []string{
		"refusing to build without content verification on Windows", // what happened
		"Contents or Full", // the actionable remedy
		"misread",          // why: the helper can misinterpret filenames
		"does NOT fix",     // enabling verification is not a Unicode fix
		"Unicode filename", //  …spelled out
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal must mention %q; got: %s", want, msg)
		}
	}
}

// TestContainment_RefusesWindowsBuildWhenVerificationDisabled covers both routes to the
// "none" tier: the global FAST preset, and a per-archive override on top of a verifying
// global. The second is the one a preset-label check would miss.
func TestContainment_RefusesWindowsBuildWhenVerificationDisabled(t *testing.T) {
	tools := nativeTools(t)

	t.Run("global FAST preset", func(t *testing.T) {
		onWindowsTarPath(t, true)
		app, staging := newTestApp(t, tools)
		if _, err := app.SaveConfig(map[string]any{"build_verify": "none"}); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
		src, refs := containmentSource(t)
		ranTar := tarInvocationWatch(t)
		c := containmentChunk(app, src, refs, false)

		assertRefused(t, app.BuildChunk(c.ID, noProg))

		if *ranTar {
			t.Error("tar was invoked before the refusal — the guard must run first")
		}
		got := app.Store.Chunk(c.ID)
		if got.Status != "PLANNED" {
			t.Errorf("a refused build must not make the package eligible as staged; status = %q", got.Status)
		}
		if _, err := os.Stat(filepath.Join(staging, c.Name)); err == nil {
			t.Error("a refused build must not create the package's staging directory")
		}
		// The operator's saved settings must survive a refusal untouched.
		if bv := app.LoadConfig().BuildVerify; normBuildVerify(bv) != BuildVerifyNone {
			t.Errorf("refusal silently rewrote the saved global setting to %q", bv)
		}
	})

	t.Run("archive override on a verifying global", func(t *testing.T) {
		onWindowsTarPath(t, true)
		app, _ := newTestApp(t, tools)
		if _, err := app.SaveConfig(map[string]any{"build_verify": "full"}); err != nil {
			t.Fatalf("SaveConfig: %v", err)
		}
		coll := app.Store.AddCollection("Recreatable")
		if _, err := app.applyArchiveIntegrity(coll.ID, map[string]any{"preset": "FAST"}); err != nil {
			t.Fatalf("apply FAST override: %v", err)
		}
		// Precondition: the GLOBAL label still says a verifying tier. Only the effective
		// configuration says otherwise, so a guard reading the global would let this pass.
		if g := normBuildVerify(app.LoadConfig().BuildVerify); g != BuildVerifyFull {
			t.Fatalf("fixture: global should still be full, got %q", g)
		}
		if eff := effectiveTier(app, coll.ID); eff != BuildVerifyNone {
			t.Fatalf("fixture: effective tier should be none, got %q", eff)
		}

		src, refs := containmentSource(t)
		ranTar := tarInvocationWatch(t)
		c := containmentChunk(app, src, refs, false)
		c.CollectionID = coll.ID
		if err := app.Store.UpdateChunkErr(c); err != nil {
			t.Fatal(err)
		}

		assertRefused(t, app.BuildChunk(c.ID, noProg))

		if *ranTar {
			t.Error("tar was invoked before the refusal — the guard must run first")
		}
		if got := app.Store.Chunk(c.ID); got.Status != "PLANNED" {
			t.Errorf("status = %q, want PLANNED", got.Status)
		}
	})
}

// effectiveTier is a tiny readability helper: the normalised effective build-verify tier.
func effectiveTier(app *App, collectionID int) string {
	return normBuildVerify(app.effectiveIntegrity(collectionID).BuildVerify)
}

// TestContainment_RefusesBeforeKeyGeneration proves the refusal precedes encryption key
// creation: an encrypted package at the "none" tier is refused with no key recorded and
// no key appended to either keystore.
func TestContainment_RefusesBeforeKeyGeneration(t *testing.T) {
	onWindowsTarPath(t, true)
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	if _, err := app.SaveConfig(map[string]any{"build_verify": "none"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	before := keystoreKeyCounts(t, app)
	src, refs := containmentSource(t)
	c := containmentChunk(app, src, refs, true)

	assertRefused(t, app.BuildChunk(c.ID, noProg))

	if got := app.Store.Chunk(c.ID); got.KeyRef != "" {
		t.Errorf("a refused build recorded a key reference %q", got.KeyRef)
	}
	after := keystoreKeyCounts(t, app)
	for i := range before {
		if i < len(after) && after[i] != before[i] {
			t.Errorf("keystore %d gained %d key(s) during a refused build", i, after[i]-before[i])
		}
	}
}

func keystoreKeyCounts(t *testing.T, app *App) []int {
	t.Helper()
	var out []int
	for _, p := range app.LoadConfig().KeystorePaths {
		ks, err := readStore(p)
		if err != nil {
			t.Fatalf("read keystore %s: %v", p, err)
		}
		out = append(out, len(ks.Keys))
	}
	return out
}

// TestContainment_WatchDetectsTarInvocation is the positive control for the two tests
// above: with the SAME watch installed and a verifying tier, the build runs and the
// watch fires. Without this, "tar was not invoked" would prove nothing.
func TestContainment_WatchDetectsTarInvocation(t *testing.T) {
	onWindowsTarPath(t, true)
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	if _, err := app.SaveConfig(map[string]any{"build_verify": "contents"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	src, refs := containmentSource(t)
	ranTar := tarInvocationWatch(t)
	c := containmentChunk(app, src, refs, false)

	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("a Contents build of supported ASCII content must still succeed: %v", err)
	}
	if !*ranTar {
		t.Fatal("the tar-invocation watch did not fire on a build that DID invoke tar — " +
			"the observation mechanism is vacuous and the refusal tests prove nothing")
	}
}

// TestContainment_VerifyingTiersStillBuild proves the containment does not disturb the
// tiers it is meant to require: ordinary supported ASCII content still builds, and still
// records the proofs, at Contents and at Full.
func TestContainment_VerifyingTiersStillBuild(t *testing.T) {
	onWindowsTarPath(t, true)
	tools := nativeTools(t)
	for _, tier := range []string{BuildVerifyContents, BuildVerifyFull} {
		t.Run(tier, func(t *testing.T) {
			onWindowsTarPath(t, true)
			app, staging := newTestApp(t, tools)
			if _, err := app.SaveConfig(map[string]any{"build_verify": tier}); err != nil {
				t.Fatalf("SaveConfig: %v", err)
			}
			src, refs := containmentSource(t)
			c := containmentChunk(app, src, refs, false)
			if err := app.BuildChunk(c.ID, noProg); err != nil {
				t.Fatalf("BuildChunk at %s: %v", tier, err)
			}
			got := app.Store.Chunk(c.ID)
			if got.Status != "STAGED" {
				t.Fatalf("status = %s (%s), want STAGED", got.Status, got.Error)
			}
			if got.BuildVerified == nil || !got.BuildVerified.Contents {
				t.Errorf("%s build must attest contents proven: %+v", tier, got.BuildVerified)
			}
			assertExactPayload(t, stagedTar(staging, c), refs)
		})
	}
}

// TestContainment_NonWindowsPathUnaffected pins the deliberate narrowness: the refusal
// applies to the Windows external-tar path only. Off it, the "none" tier keeps its
// existing documented behaviour — no verification, amber warning, build succeeds.
func TestContainment_NonWindowsPathUnaffected(t *testing.T) {
	onWindowsTarPath(t, false)
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	if _, err := app.SaveConfig(map[string]any{"build_verify": "none"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	src, refs := containmentSource(t)
	c := containmentChunk(app, src, refs, false)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("off the Windows tar path, a none-tier build must still succeed: %v", err)
	}
	got := app.Store.Chunk(c.ID)
	if got.BuildVerified == nil || got.BuildVerified.Mode != BuildVerifyNone ||
		got.BuildVerified.Contents || got.BuildVerified.Warning == "" {
		t.Errorf("none-tier attestation changed off the Windows path: %+v", got.BuildVerified)
	}
}

// TestContainment_WrongFileCannotCompleteUnverified is the point of the whole exercise.
// "café.txt" as UTF-8, read back in Windows-1252, spells "cafÃ©.txt" exactly. With both
// files on disk and only café.txt selected, the unverified path is where the wrong file
// could reach a medium unnoticed. After containment that route is closed: the build is
// refused, nothing is staged, and no package exists to be written.
//
// This does not make the Unicode build work — see TestBuildChunk_WrongFileSelection_
// LookalikeNeighbour, which still fails at the Contents tier. It only removes the route
// where the mismatch would go UNDETECTED.
func TestContainment_WrongFileCannotCompleteUnverified(t *testing.T) {
	onWindowsTarPath(t, true)
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	if _, err := app.SaveConfig(map[string]any{"build_verify": "none"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	src := t.TempDir()
	const wanted = "café.txt"
	const decoy = "cafÃ©.txt" // what café.txt's UTF-8 bytes spell when read as Windows-1252
	all := makeUnicodeSource(t, src, []string{wanted, decoy})
	if all[0].Hash == all[1].Hash {
		t.Fatal("fixture error: the two files must have distinct bytes")
	}
	refs := []ChunkFileRef{all[0]} // select ONLY café.txt

	ranTar := tarInvocationWatch(t)
	c := containmentChunk(app, src, refs, false)

	assertRefused(t, app.BuildChunk(c.ID, noProg))

	if *ranTar {
		t.Error("tar ran: the unverified wrong-file route is still reachable")
	}
	if got := app.Store.Chunk(c.ID); got.Status == "STAGED" {
		t.Error("an unverified package was staged despite containment")
	}
	if _, err := os.Stat(stagedTar(staging, c)); err == nil {
		t.Error("an unverified tar was produced despite containment")
	}
}

// TestContainment_VerifierStillRejectsMismatchedArchive proves containment did not
// replace the check it depends on: at a verifying tier, an archive whose members do not
// match the expected set is still rejected, with the offending member named. Driven
// through the real build path with the existing corruption seam.
func TestContainment_VerifierStillRejectsMismatchedArchive(t *testing.T) {
	onWindowsTarPath(t, true)
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	if _, err := app.SaveConfig(map[string]any{"build_verify": "contents"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	src, refs := containmentSource(t)
	buildAfterTarHook = func(tarPath string) { corruptFirstTarMember(t, tarPath) }
	defer func() { buildAfterTarHook = nil }()

	c := containmentChunk(app, src, refs, false)
	err := app.BuildChunk(c.ID, noProg)
	if err == nil {
		t.Fatal("content verification must still reject an archive whose member content does not match")
	}
	if !strings.Contains(err.Error(), "stage verification") {
		t.Errorf("expected a stage-verification failure, got: %v", err)
	}
	if got := app.Store.Chunk(c.ID); got.Status != "FAILED" {
		t.Errorf("a package failing content verification must be FAILED, got %q", got.Status)
	}
}

// TestContainment_ProductionPredicateFollowsGOOS pins the shipped default: with no test
// hook installed, the predicate is the real platform. A binary has no other value.
func TestContainment_ProductionPredicateFollowsGOOS(t *testing.T) {
	if buildUsesWindowsExternalTarHook != nil {
		t.Fatal("a test leaked buildUsesWindowsExternalTarHook; production must see nil")
	}
	if got, want := buildUsesWindowsExternalTar(), runtime.GOOS == "windows"; got != want {
		t.Errorf("unhooked predicate = %v, want %v for GOOS %q", got, want, runtime.GOOS)
	}
	// And the guard itself: none is refused on Windows, allowed elsewhere; verifying
	// tiers are allowed everywhere.
	none := Integrity{BuildVerify: BuildVerifyNone}.normalize()
	if err := assertWindowsTarBuildVerifiable(none); (err != nil) != (runtime.GOOS == "windows") {
		t.Errorf("none tier on GOOS %q: err = %v", runtime.GOOS, err)
	}
	for _, tier := range []string{BuildVerifyContents, BuildVerifyFull} {
		iv := Integrity{BuildVerify: tier}.normalize()
		if err := assertWindowsTarBuildVerifiable(iv); err != nil {
			t.Errorf("%s tier must never be refused: %v", tier, err)
		}
	}
	// Legacy "fast" normalises to none and must be refused on the same terms — the
	// guard reads the normalised tier, not the raw string it was configured with.
	legacy := Integrity{BuildVerify: "fast"}.normalize()
	if err := assertWindowsTarBuildVerifiable(legacy); (err != nil) != (runtime.GOOS == "windows") {
		t.Errorf("legacy \"fast\" on GOOS %q: err = %v", runtime.GOOS, err)
	}
}
