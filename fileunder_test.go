//go:build !guionly

package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// A prefix filter keeps exactly the files below the prefix, on either engine: not
// a sibling that starts with the same letters, not the folder's name as a file,
// and a whole folder when the prefix is empty.
func TestFileQuery_UnderKeepsExactlyTheFilesBelow(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			s := openEngineStore(t, engine, t.TempDir())
			coll := s.AddCollection("A")
			f1 := s.AddFolder(coll.ID, "/srv/one")
			f2 := s.AddFolder(coll.ID, "/srv/one/two")
			for i, p := range []struct {
				folder int
				rel    string
			}{
				{f1.ID, "wide/a"}, {f1.ID, "wide/b/c"}, {f1.ID, "wide2/x"}, {f1.ID, "wide"}, {f1.ID, "Wide/z"},
				{f1.ID, "café/x"}, {f1.ID, "cafe/y"}, {f2.ID, "deep/k"}, {f2.ID, "top"},
			} {
				if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: p.folder, RelPath: p.rel,
					SizeBytes: int64(i), HashAlg: "SHA256", Hash: fakeHash(i)}); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.FlushPending(); err != nil {
				t.Fatal(err)
			}
			got := func(under []folderPrefix) string {
				var out []string
				s.mu.Lock()
				defer s.mu.Unlock()
				if err := s.eachFileLocked(fileQuery{CollectionID: coll.ID, Under: under, ByPath: true}, func(f *File) error {
					out = append(out, fmt.Sprintf("%d:%s", f.FolderID, f.RelPath))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				sort.Strings(out)
				return strings.Join(out, ",")
			}
			for _, c := range []struct {
				under []folderPrefix
				want  string
			}{
				{[]folderPrefix{{f1.ID, "wide/"}}, fmt.Sprintf("%d:wide/a,%d:wide/b/c", f1.ID, f1.ID)},
				{[]folderPrefix{{f1.ID, "café/"}}, fmt.Sprintf("%d:café/x", f1.ID)},
				{[]folderPrefix{{f2.ID, ""}}, fmt.Sprintf("%d:deep/k,%d:top", f2.ID, f2.ID)},
				{[]folderPrefix{{f1.ID, "wide/b/"}, {f2.ID, "deep/"}}, fmt.Sprintf("%d:wide/b/c,%d:deep/k", f1.ID, f2.ID)},
				{[]folderPrefix{}, ""},
			} {
				if g := got(c.under); g != c.want {
					t.Errorf("under %v: %s, want %s", c.under, g, c.want)
				}
			}
		})
	}
}
