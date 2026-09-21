// Pure recorded-evidence comparison. No filesystem access or host path rules.
import { validateSnapshots, exactDecimal } from './catalog-protocol.mjs';
import { validUnicode } from './catalog-names.mjs';
export const classes = Object.freeze({agreement:'Recorded checksum agreement',difference:'Recorded difference',inconclusive:'Inconclusive recorded pair',referenceOnly:'Only recorded in reference',counterpartOnly:'Only recorded in counterpart'});
export function comparisonFrame(snapshot) {
  const data=snapshot.catalog,frame=data.comparisonFrame;
  if(!frame || frame.convention!=='slash-relative-v1' || !validUnicode(frame.root) || !frame.root || frame.recordCount!==data.files.length || data.collections.length!==1)
    throw Error('Comparison requires one recorded collection/root and a complete slash-relative path frame per input. Choose supported single-root snapshots; Library/Find remain available.');
  const keys=new Map();
  for(const f of data.files) {
    // Slash is the only separator. Backslashes inside a key are literal, never
    // host separators; leading slash/backslash and drive prefixes are refused.
    if(f.sourceFolder!==frame.root || !validUnicode(f.path) || !f.path || f.path.includes('\0') || /^[\\/]|^[A-Za-z]:/.test(f.path) || f.path.split('/').some(p=>!p||p==='.'||p==='..'))
      throw Error('Unsupported relative path or root association. No path normalization or inferred root mapping is performed.');
    if(keys.has(f.path))throw Error('Ambiguous duplicate recorded relative path. Choose snapshots with unique keys; ordinary browsing remains available.');
    keys.set(f.path,f);
  }
  return {root:frame.root,keys};
}
export function classifyPair(a,b) {
  if(!a)return {kind:'counterpartOnly',reason:'No row for this exact key in the complete loaded reference set.'};
  if(!b)return {kind:'referenceOnly',reason:'No row for this exact key in the complete loaded counterpart set.'};
  const sizes=[a,b].every(f=>exactDecimal(f.bytes,'9223372036854775807',true));
  const comparable=[a,b].every(f=>f.algorithm==='sha256'&&/^[a-f0-9]{64}$/i.test(f.hash));
  const sameHash=comparable&&a.hash.toLowerCase()===b.hash.toLowerCase();
  const equalSize=sizes&&BigInt(a.bytes)===BigInt(b.bytes);
  if(sameHash&&sizes&&!equalSize)return {kind:'inconclusive',reason:'Inconsistent recorded evidence: equal full SHA-256 values but unequal exact sizes.'};
  if(sizes&&!equalSize)return {kind:'difference',reason:'Recorded exact byte sizes differ.'};
  if(comparable&&!sameHash)return {kind:'difference',reason:'Recorded full SHA-256 content checksums differ.'};
  if(comparable&&sizes)return {kind:'agreement',reason:'Full recorded SHA-256 content checksums and exact sizes agree; no new verification.'};
  return {kind:'inconclusive',reason:!sizes?'Recorded size is not exactly interpretable.':'Missing, incomplete or incompatible supported content checksums; equal metadata is insufficient.'};
}
export function compareRecorded(snapshots,reference,counterpart,request) {
  validateSnapshots(snapshots);
  if(!/^[a-f0-9-]{36}$/.test(request)||reference===counterpart)throw Error('Invalid comparison identity');
  const a=snapshots.find(s=>s.handle===reference),b=snapshots.find(s=>s.handle===counterpart);
  if(!a||!b)throw Error('Unknown comparison handle');
  const af=comparisonFrame(a),bf=comparisonFrame(b);
  const keys=[...new Set([...af.keys.keys(),...bf.keys.keys()])].sort(); // exact UTF-16 ordering, no locale/case folding
  const counts=Object.fromEntries(Object.keys(classes).map(k=>[k,0]));
  const rows=keys.map(key=>{
    const left=af.keys.get(key),right=bf.keys.get(key),result=classifyPair(left,right);counts[result.kind]++;
    const missing=!left?a:!right?b:null;
    const policy=missing?.catalog.inventoryScope?.policy;
    const scopeNote=missing ? (policy==='ignore-exact-ds-store'&&key.split('/').at(-1)==='.DS_Store'
      ? 'This regular-file key is outside the other snapshot’s declared exact .DS_Store policy. Excluded paths are not individually enumerated.'
      : 'Recorded-set absence only; consult each scope and observation time. Current source absence is not established.') : 'Scope differences qualify coverage; paired evidence is still comparable.';
    return {key,referenceId:left?.id??null,counterpartId:right?.id??null,...result,scopeNote};
  });
  const result={ok:true,reference,counterpart,request,complete:true,roots:[af.root,bf.root],totals:{reference:af.keys.size,counterpart:bf.keys.size,union:keys.length},counts,rows};
  if(new TextEncoder().encode(JSON.stringify(result)).length>8*1024*1024)throw Error('Comparison response exceeds the 8 MiB limit; no complete result presented.');
  return result;
}
export function validateComparison(result,snapshots,reference,counterpart,request) {
  const expected=compareRecorded(snapshots,reference,counterpart,request);
  if(JSON.stringify(result)!==JSON.stringify(expected))throw Error('Incomplete or mismatched comparison response');
  return result;
}
