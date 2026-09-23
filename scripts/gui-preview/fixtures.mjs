// All identifiers, paths, media and dates are invented. This module has no I/O.
export const scope = 'Synthetic sample observed 2026-09-18; only listed occurrences are known. No filesystem scan.';
export const media = [
  { id: 'SIM-DISK-A', name: 'Sample workspace disk', online: true, location: 'Simulated desk', observed: '2026-09-18', scope: 'Selected sample project paths only' },
  { id: 'SIM-TAPE-B', name: 'Sample tape B', online: false, location: 'Simulated shelf 2', observed: '2026-06-01', scope: 'Older package inventory; other contents unknown' },
  { id: 'SIM-OPTICAL-C', name: 'Sample optical C', online: false, location: 'Location unknown', observed: null, scope: 'Inventory and verification scope unknown' },
];
export const items = [
  { id: 'SIM-CONTENT-001', name: 'Field notes.txt', project: 'Sample expedition', version: 'v2', hash: 'SYNTHETIC-HASH-NOTES-V2', occurrences: [
    { id: 'SIM-OCC-01', medium: 'SIM-DISK-A', path: '/synthetic/expedition/Field notes.txt', verification: 'Contents matched', verified: '2026-09-18', scope: 'This one occurrence only' },
    { id: 'SIM-OCC-02', medium: 'SIM-TAPE-B', path: '/synthetic/package-01/Field notes.txt', verification: 'Not verified', verified: null, scope: 'Copy recorded; content verification unknown' },
  ] },
  { id: 'SIM-CONTENT-002', name: 'Field notes.txt', project: 'Sample expedition', version: 'v1', hash: 'SYNTHETIC-HASH-NOTES-V1', occurrences: [
    { id: 'SIM-OCC-03', medium: 'SIM-TAPE-B', path: '/synthetic/package-00/Field notes.txt', verification: 'Contents matched', verified: '2026-06-01', scope: 'This occurrence at the evidence date; current condition unknown' },
  ] },
  { id: 'SIM-CONTENT-003', name: 'Scene <draft> & "雪".png', project: 'Sample studio', version: 'v1', hash: 'SYNTHETIC-HASH-SCENE-V1', occurrences: [
    { id: 'SIM-OCC-04', medium: 'SIM-OPTICAL-C', path: '/synthetic/studio/<draft> & "雪".png', verification: 'Unknown', verified: null, scope: 'Occurrence known; inventory completeness unknown' },
  ] },
];
export const activity = [
  { name: 'Sample recurring backup', operation: 'PENDING', recording: 'Recorded', verification: 'Not started', history: 'None' },
  { name: 'Sample incremental copy', operation: 'RUNNING', recording: 'Recorded', verification: 'Not started', history: 'None' },
  { name: 'Sample archive failure', operation: 'FAILED', recording: 'Unrecorded — recording also failed', verification: 'Not verified', history: 'Synthetic recording error' },
  { name: 'Sample completed copy', operation: 'COMPLETED', recording: 'Unrecorded — current result not saved', verification: 'Not verified', history: 'Synthetic recording error' },
  { name: 'Sample recovered recording', operation: 'COMPLETED', recording: 'Recorded — current result saved', verification: 'Not verified', history: 'Earlier recording error; historical only' },
];
export function searchItems({ query = '', medium = 'all', project = 'all', availability = 'all', scenario = 'ready' } = {}) {
  if (['loading', 'error', 'empty'].includes(scenario)) return [];
  const q = query.trim().toLocaleLowerCase('en');
  return items.filter(item => {
    const matches = item.occurrences.filter(o => {
      const m = media.find(m => m.id === o.medium);
      return (medium === 'all' || medium === m.id) && (availability === 'all' || m.online === (availability === 'online'));
    });
    return (project === 'all' || item.project === project) && matches.length &&
      [item.name, item.id, item.project, item.version, item.hash, ...matches.flatMap(o => [o.id, o.path, o.medium, media.find(m => m.id === o.medium).name])]
        .some(value => value.toLocaleLowerCase('en').includes(q));
  });
}
export function operationLabel(row) { return `${row.operation} · ${row.recording} · Verification: ${row.verification}`; }
