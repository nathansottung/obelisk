package main

// AddFolder registers a source for test setup, where nothing is expected to refuse.
// It is test-only so that no production path can add a source around the gate.
func (s *Store) AddFolder(collectionID int, path string) *Folder {
	f, err := s.RegisterSource(collectionID, path)
	if err != nil {
		panic(err)
	}
	return f
}
