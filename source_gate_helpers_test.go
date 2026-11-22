package main

// AddFolder puts a source in the catalog for test setup, as an older catalog may
// already hold it (a whole drive, say). It is test-only so that no production path
// can add a source around the gate; tests of the gate call RegisterSource.
func (s *Store) AddFolder(collectionID int, path string) *Folder {
	return s.addFolderRecord(collectionID, path)
}

// addFolderRecord adds a source with no check, as an older catalog would hold it.
func (s *Store) addFolderRecord(collectionID int, path string) *Folder {
	f, _ := s.insertSource(collectionID, path, pathVolumeID(path), nil)
	return f
}
