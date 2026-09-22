// Generic, small Windows-compatible fixtures. No existing user data is read.
export const tutorialFiles = Object.freeze({
  'café notes.txt': 'Synthetic café observation\n',
  "nested/O'Brien & +%# note.txt": 'A distinct synthetic record\n',
  'same-one.txt': 'Equal synthetic bytes\n',
  'nested/same-two.txt': 'Equal synthetic bytes\n',
  'empty.txt': '',
  '.DS_Store': 'Synthetic metadata control, not a Finder file\n',
  'nested/.DS_Store': 'Second synthetic metadata control\n',
  '.DS_Store.bak': 'Include this near name\n',
  'directory/.DS_Store/keep.txt': 'Traverse a same-named directory\n',
  'nested/photo.xmp': 'Synthetic sidecar must remain\n',
});

// Versioned generated-source pair, not prepared catalog observations. The same
// native producer records each new tree; timestamps/roots/checksums are not edited.
export const comparisonTutorialFiles = Object.freeze({
  ALPHA: Object.freeze({ ...tutorialFiles, 'ALPHA-only.txt': 'Only generated in ALPHA\n' }),
  BETA: Object.freeze({ ...tutorialFiles,
    "nested/O'Brien & +%# note.txt": 'Deliberately different BETA content\n',
    'BETA-only.txt': 'Only generated in BETA\n',
  }),
});
