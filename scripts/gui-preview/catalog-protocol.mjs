// Validate only the preview response contract, never the persisted native schema.
import { validUnicode } from './catalog-names.mjs';
const int64Max = '9223372036854775807';
const requireValue = condition => { if (!condition) throw new Error('Invalid catalog response'); };
export function exactDecimal(value, max, zero = false) {
  return typeof value === 'string' && (zero ? /^(0|[1-9][0-9]*)$/ : /^[1-9][0-9]*$/).test(value)
    && (value.length < max.length || (value.length === max.length && value <= max));
}
const text = value => validUnicode(value) && new TextEncoder().encode(value).length <= 4096;
const date = value => typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value));
export function validateCatalog(data) {
  requireValue(data && data.schema === 8 && ['2147483647', int64Max].includes(data.idMax));
  requireValue(typeof data.digest === 'string' && /^[a-f0-9]{64}$/.test(data.digest) && date(data.loadedAt));
  requireValue(data.recordedAt === undefined || data.recordedAt === null || date(data.recordedAt));
  const scope = data.inventoryScope;
  if (scope !== undefined && scope !== null) {
    requireValue(scope && Object.keys(scope).sort().join(',') === 'complete,entries,excludedFiles,includedFiles,policy,readBytes,regularFiles,version');
    requireValue(scope.version === 1 && scope.complete === true && ['include-all','ignore-exact-ds-store'].includes(scope.policy));
    for (const [key,max] of [['entries',128],['regularFiles',64],['includedFiles',64],['excludedFiles',64],['readBytes',32*1024*1024]]) requireValue(Number.isSafeInteger(scope[key]) && scope[key]>=0 && scope[key]<=max);
    requireValue(scope.regularFiles <= scope.entries && scope.includedFiles + scope.excludedFiles === scope.regularFiles && scope.includedFiles === data.files?.length && (scope.policy !== 'include-all' || scope.excludedFiles === 0));
    requireValue(Array.isArray(data.files) && data.files.every(f=>typeof f.bytes==='string' && /^(0|[1-9][0-9]*)$/.test(f.bytes) && BigInt(f.bytes)<=8388608n) && data.files.reduce((n,f)=>n+BigInt(f.bytes),0n)===BigInt(scope.readBytes));
  }
  for (const [key, limit] of [['collections',100], ['volumes',100], ['files',1000]]) {
    requireValue(Array.isArray(data[key]) && data[key].length <= limit);
    const ids = new Set();
    for (const row of data[key]) {
      requireValue(row && exactDecimal(row.id, data.idMax) && !ids.has(row.id)); ids.add(row.id);
    }
  }
  for (const c of data.collections) requireValue(text(c.name) && typeof c.retired === 'boolean');
  for (const v of data.volumes) requireValue(['label','kind','location'].every(k => text(v[k])));
  let copies = 0;
  for (const f of data.files) {
    requireValue(['collection','path','sourceFolder','hash','algorithm'].every(k => text(f[k])));
    requireValue(exactDecimal(f.bytes, int64Max, true) && date(f.firstSeen) && Array.isArray(f.copies));
    copies += f.copies.length; requireValue(copies <= 1000);
    const ids = new Set();
    for (const c of f.copies) {
      requireValue(c && ['id','chunk','path','volume','location'].every(k => text(c[k])));
      const match = /^chunk-([1-9][0-9]*)-copy-(0|[1-9][0-9]*)$/.exec(c.id);
      requireValue(match && exactDecimal(match[1], data.idMax) && exactDecimal(match[2], '999', true) && !ids.has(c.id)); ids.add(c.id);
      requireValue(typeof c.superseded === 'boolean' && (c.recordedVerifyOK === null || typeof c.recordedVerifyOK === 'boolean'));
      requireValue(c.recordedVerifiedAt === null || date(c.recordedVerifiedAt));
    }
  }
  return data;
}
export function validateIDs(ids, data) {
  requireValue(Array.isArray(ids) && ids.length <= 1000);
  const known = new Set(data.files.map(f => f.id)), seen = new Set();
  for (const id of ids) {
    requireValue(exactDecimal(id, data.idMax) && known.has(id) && !seen.has(id)); seen.add(id);
  }
  return ids;
}

// Session handles are process-local authority, not medium or durable identities.
export function validateSnapshots(snapshots) {
  requireValue(Array.isArray(snapshots) && snapshots.length === 2);
  const handles = new Set(), digests = new Set();
  let files = 0, copies = 0;
  for (const s of snapshots) {
    requireValue(s && typeof s.handle === 'string' && /^[a-f0-9]{32}$/.test(s.handle) && !handles.has(s.handle) && text(s.label));
    validateCatalog(s.catalog);
    requireValue(!digests.has(s.catalog.digest));
    handles.add(s.handle); digests.add(s.catalog.digest);
    files += s.catalog.files.length;
    copies += s.catalog.files.reduce((n, f) => n + f.copies.length, 0);
  }
  requireValue(files <= 1000 && copies <= 1000);
  return snapshots;
}

export function validateMatches(result, snapshots, filter) {
  validateSnapshots(snapshots);
  const requested = filter === 'all' ? snapshots : snapshots.filter(s => s.handle === filter);
  requireValue(requested.length > 0 && result?.ok === true && Array.isArray(result.groups) && result.groups.length === requested.length);
  const seen = new Set();
  return result.groups.flatMap(group => {
    const snapshot = requested.find(s => s.handle === group?.snapshot);
    requireValue(snapshot && !seen.has(group.snapshot)); seen.add(group.snapshot);
    return validateIDs(group.ids, snapshot.catalog).map(id => ({ snapshot: snapshot.handle, id }));
  });
}
