package main

// tarpkg.go — the package tar, written and read by Obelisk itself with Go's
// archive/tar. A package is a POSIX pax archive: ustar headers, and each name
// that ustar cannot hold (non-ASCII, or long) in a pax "path" record as UTF-8.
// Every stock tar reads that format (bsdtar on Windows and macOS, GNU tar on
// Linux), so RESTORE.txt's `tar -xf` still restores it by hand.
//
// No external tar runs during a build or an Obelisk restore. The Windows tar
// reads its file list and arguments in the ANSI code page, so it could miss or
// misname a Unicode file (OBX-006).

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// writePackageTar writes the package tar at tarPath: first the BagIt payload
// manifest from staging, then each catalogued file read from srcRoot, under its
// catalogued name. The source is only read. A file that is not a regular file,
// or whose size changes while it is read, fails the build.
func writePackageTar(tarPath, manifestPath, srcRoot string, files []ChunkFileRef) (err error) {
	f, err := os.OpenFile(tarPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<20)
	tw := tar.NewWriter(bw)
	if err := addTarFile(tw, manifestPath, bagPayloadManifestName); err != nil {
		return err
	}
	for _, cf := range files {
		name, err := tarMemberName(cf.RelPath)
		if err != nil {
			return err
		}
		if err := addTarFile(tw, filepath.Join(srcRoot, filepath.FromSlash(name)), name); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// tarMemberName checks a catalogued path is a plain relative path that stays
// inside the package, and returns it as the member name.
func tarMemberName(rel string) (string, error) {
	name := strings.TrimPrefix(rel, "./")
	if name == "" || !fs.ValidPath(name) {
		return "", fmt.Errorf("%q is not a relative path inside the package", rel)
	}
	return name, nil
}

func addTarFile(tw *tar.Writer, p, name string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", name)
	}
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     st.Size(),
		Mode:     int64(st.Mode().Perm()),
		ModTime:  st.ModTime().Truncate(1e9),
		Format:   tar.FormatPAX,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("adding %s: %w", name, err)
	}
	n, err := io.Copy(tw, io.LimitReader(f, st.Size()))
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	if n != st.Size() {
		return fmt.Errorf("%s changed size while it was read (%d of %d bytes)", name, n, st.Size())
	}
	var one [1]byte
	if m, _ := f.Read(one[:]); m > 0 {
		return fmt.Errorf("%s grew while it was read", name)
	}
	return nil
}

// extractTar unpacks a package tar into the folder into, which must exist.
// With members, only those (a member, or every member under a folder member)
// are unpacked, and a member not in the package is an error. Every name must
// stay inside into: names are opened through an os.Root, and a name with "..",
// an absolute path, or (on Windows) a drive or stream separator is refused.
// Only files and folders are unpacked; a package holds nothing else.
func extractTar(r io.Reader, into string, members []string) error {
	root, err := os.OpenRoot(into)
	if err != nil {
		return err
	}
	defer root.Close()
	want := map[string]bool{}
	for _, m := range members {
		n := path.Clean(strings.TrimPrefix(filepath.ToSlash(m), "./"))
		want[n] = false
	}
	selected := func(name string) bool {
		if len(want) == 0 {
			return true
		}
		hit := false
		for m := range want {
			if name == m || strings.HasPrefix(name, m+"/") {
				want[m], hit = true, true
			}
		}
		return hit
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading the package tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(hdr.Name, "./"), "/")
		if !fs.ValidPath(name) || name == "." || (runtime.GOOS == "windows" && strings.ContainsAny(name, `:\`)) {
			return fmt.Errorf("refusing member %q: it is not a relative path inside the restore", hdr.Name)
		}
		if !selected(name) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := extractTarFile(root, tr, hdr, name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("refusing member %q: type %q is not a file or folder", hdr.Name, hdr.Typeflag)
		}
	}
	var missing []string
	for m, hit := range want {
		if !hit {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("not in the package: %s", strings.Join(missing, ", "))
	}
	return nil
}

func extractTarFile(root *os.Root, tr *tar.Reader, hdr *tar.Header, name string) error {
	if dir := path.Dir(name); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	perm := os.FileMode(hdr.Mode).Perm() | 0o600
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("the package holds %q more than once", name)
		}
		return err
	}
	if _, err := io.Copy(f, tr); err != nil {
		f.Close()
		return fmt.Errorf("unpacking %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if !hdr.ModTime.IsZero() {
		_ = root.Chtimes(name, hdr.ModTime, hdr.ModTime)
	}
	return nil
}
