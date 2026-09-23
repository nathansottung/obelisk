import mode from './mode.mjs';
import { validateCatalog, validateIDs, validateSnapshots, validateMatches } from './catalog-protocol.mjs';
import { parseExactName, displayName, validUnicode } from './catalog-names.mjs';
import { renderComparison, cancelComparison } from './comparison-ui.mjs';
export const catalogMode = mode.enabled;
const notice = mode.multi ? 'Disposable catalog mode — reading two synthetic catalog snapshots. No source/media files are opened. The selected catalogs are not modified.' : 'Disposable catalog mode — reading a synthetic catalog snapshot. No source/media files are opened. The selected catalog is not modified.';
const node = (tag, text, attrs = {}) => { const e = document.createElement(tag); if (text !== undefined) e.textContent = text; for (const [k,v] of Object.entries(attrs)) e.setAttribute(k,v); return e; };
const unavailable = value => value === null || value === undefined || value === '' || value === '0001-01-01T00:00:00Z' ? 'Unavailable' : String(value);
let query = '', hash = '', selected = null, epoch = 0, timer, failure = '';
let exactName = false, exactEntry = '""';
let snapshotFilter = 'all';
const snapshots = mode.multi ? mode.snapshots : mode.ok ? [{handle:'single',label:'Selected snapshot',catalog:mode.catalog}] : [];
// Presentation is fixed once per startup handle; never derive it from filtered results.
const sourceLabels = new Map((snapshots ?? []).map((s, i) => [s.handle, mode.multi ? `Snapshot ${i === 0 ? 'A' : 'B'} — ${displayName(s.label)}` : s.label]));
const sourceLabel = snapshot => sourceLabels.get(snapshot.handle);
const sameRecord = (a,b) => Boolean(a && b && a.snapshot === b.snapshot && a.id === b.id);
if (catalogMode && mode.ok) {
  try { if(mode.multi)validateSnapshots(snapshots);else validateCatalog(mode.catalog); } catch { failure = 'Invalid catalog response'; }
}

export function renderCatalog() {
  cancelComparison();
  const content = document.querySelector('#content');
  content.classList.toggle('multi-snapshot', Boolean(mode.multi));
  const find = location.hash === '#find';
  const comparing=mode.multi&&location.hash==='#compare';
  const title = comparing ? 'Compare recorded snapshots' : find ? 'Find' : location.hash === '#library' || !location.hash ? 'Library' : 'Catalog view unavailable';
  document.querySelector('h1').textContent = title;
  document.querySelector('#purpose').hidden = false;
  document.querySelector('#purpose').textContent = 'Recorded catalog evidence only. Availability, capacity, parity and verification performed now: unavailable.';
  document.querySelector('#scope').hidden = false;
  document.querySelector('#scope').textContent = notice;
  document.querySelector('footer span').textContent = notice;
  document.querySelector('footer strong').textContent = mode.readerVersion ? 'Read-only snapshot · reader ' + mode.readerVersion : 'Read-only snapshot';
  document.querySelector('#header-actions').replaceChildren();
  document.querySelector('#breadcrumb').replaceChildren();
  document.querySelector('#demo-message').textContent = '';
  document.querySelector('main').classList.remove('project-view');
  document.querySelector('.modes').hidden = false;
  for (const button of document.querySelectorAll('[data-mode]')) button.setAttribute('aria-pressed', String(button.dataset.mode === document.querySelector('#disclosure').value));
  // Disclosure modes retain their existing shell behavior; operational demos are absent.
  for (const link of document.querySelectorAll('nav a')) { if (link.textContent === title) link.setAttribute('aria-current','page'); else link.removeAttribute('aria-current'); }
  content.replaceChildren(); epoch++; clearTimeout(timer);
  if (!mode.ok || failure) { content.append(node('p', 'Catalog load refused — ' + (failure || mode.error) + '. Relaunch the preview to read a catalog.', {role:'alert',class:'error panel'})); return; }
  if(mode.multi)content.append(node('a','Compare recorded snapshots',{href:'#compare'}),node('p','Comparison aligns recorded relative roots only after you choose a reference; it does not establish independent copies.'));
  const shown = snapshotFilter === 'all' ? snapshots : snapshots.filter(s=>s.handle===snapshotFilter);
  const data = shown[0].catalog;
  if(mode.multi) {
    const label=node('label','Snapshot view');
    const select=node('select',undefined,{'aria-label':'Snapshot view',id:'snapshot-filter'});
    select.append(node('option','All loaded snapshots',{value:'all'}));
    for(const s of snapshots)select.append(node('option',sourceLabel(s),{value:s.handle}));
    select.value=snapshotFilter;
    select.addEventListener('change',()=>{snapshotFilter=select.value;selected=null;renderCatalog();document.querySelector('#snapshot-filter').focus();});
    label.append(select);content.append(label,node('p','Selection filters this fixed session only; it does not import, merge, compare or verify snapshots. Same content in different snapshots remains separate recorded entries.'));
  }
  for(const s of (mode.multi ? snapshots : shown)) {
  const data=s.catalog;
  const card=node(mode.multi&&find?'details':'section',undefined,{class:'panel snapshot-card','data-snapshot':s.handle});
  if(mode.multi&&find)card.append(node('summary',`${sourceLabel(s)} · ${data.files.length} loaded entries · .DS_Store ${data.inventoryScope?(data.inventoryScope.policy==='include-all'?'OFF':'ON'):'UNKNOWN'} · Recorded time: ${unavailable(data.recordedAt)} · Scope and artifact details`));
  else card.append(node('h2',sourceLabel(s)));
  if(mode.multi) {
    card.append(node('p',`Session identifier: ${s.handle}. Artifact digest identifies catalog bytes, not an independent physical copy.`));
    if(!find) {const choose=node('button',`View ${sourceLabel(s)}`,{'aria-pressed':String(snapshotFilter===s.handle)});choose.addEventListener('click',()=>{snapshotFilter=s.handle;selected=null;renderCatalog();document.querySelector('#snapshot-filter').focus();});card.append(choose);}
  }
  card.append(node('p', `Catalog SHA-256: ${data.digest} · schema ${data.schema} · Session load time: ${data.loadedAt}. Loading is not scanning or verification.`, {class:'notice',...(s===snapshots[0]?{id:'catalog-identity'}:{})}));
  card.append(node('p',`Recorded inventory event time: ${unavailable(data.recordedAt)}. ${data.recordedAt?'Recorded observation only; age must be judged from this timestamp, not load time.':'Historical inventory time is unknown; individual first-seen times remain record evidence.'}`));
  const scope = data.inventoryScope;
  card.append(node('p', scope
    ? `Recorded inventory scope v${scope.version}: .DS_Store exclusion ${scope.policy==='include-all'?'OFF':'ON'}. ${scope.entries} entries visited; ${scope.regularFiles} regular files observed; ${scope.includedFiles} included; ${scope.excludedFiles} excluded by exact regular-basename policy; ${scope.readBytes} bytes read. Completed within selected scope. ${scope.entries===0?'Truly empty source.':scope.includedFiles===0&&scope.excludedFiles>0?'All observed regular files deliberately excluded.':'Unlisted source content and current state are not verified.'}`
    : 'Historical inventory scope: policy and excluded counts UNKNOWN. Existing records are shown unchanged.', {class:'notice',...(s===snapshots[0]?{id:'inventory-scope'}:{})}));
  card.append(node('p',`${data.files.length} loaded recorded entries in this snapshot; not unique content or verified copies.`));
  content.append(card);
  }
  if(comparing){renderComparison(content,snapshots,sourceLabel);return;}
  if (title === 'Catalog view unavailable') { content.append(node('p','This workspace has no supported catalog projection. Use Library or Find.')); return; }
  if (!find) {
    const projects = node('section',undefined,{class:'panel'}); projects.append(node('h2','Recorded collections'));
    for (const s of shown) for (const c of s.catalog.collections) projects.append(node('p',`${mode.multi?sourceLabel(s)+' · ':''}${c.name} · ID ${c.id}${c.retired ? ' · Retired (excluded from default Find)' : ''}`));
    if (!shown.some(s=>s.catalog.collections.length)) projects.append(node('p','No collections recorded.'));
    const storage = node('section',undefined,{class:'panel'}); storage.append(node('h2','Recorded storage'));
    for (const s of shown) for (const v of s.catalog.volumes) storage.append(node('p',`${mode.multi?sourceLabel(s)+' · ':''}${unavailable(v.label)} · ${unavailable(v.kind)} · Recorded location: ${unavailable(v.location)} · Current accessibility: unavailable`));
    if (!shown.some(s=>s.catalog.volumes.length)) storage.append(node('p','No storage recorded; this is not a live inventory.'));
    content.append(projects,storage);
  }
  const controls=node('div',undefined,{class:'panel controls'});
  const toggleLabel=node('label','Exact name (JSON string)',{class:'exact-name-toggle'});
  const toggle=node('input',undefined,{type:'checkbox','aria-label':'Exact name (JSON string)'}); toggle.checked=exactName;
  toggle.addEventListener('change',()=>{exactName=toggle.checked;if(exactName)exactEntry=JSON.stringify(query);renderCatalog();document.querySelector('[aria-label="'+(exactName?'Exact recorded name (JSON string)':'Recorded path contains')+'"]').focus();});
  toggleLabel.prepend(toggle);controls.append(toggleLabel,node('p','Ordinary search is literal; backslashes are not escapes. For control characters or surrounding whitespace, enable Exact name and enter a JSON string including quotes (for example "line\\nname.txt"). Use \\r, \\n, \\t and \\\\ explicitly. Exact matching preserves case and whitespace; SHA-256 filtering is separate.',{id:'name-entry-help'}));
  const entryError=node('p','',{'aria-live':'polite',id:'name-entry-error'});
  for (const [label,value,key] of [[exactName?'Exact recorded name (JSON string)':'Recorded path contains',exactName?exactEntry:query,'text'],['Recorded SHA-256 prefix',hash,'hash']]) {
    const wrapper=node('label',label); const input=node('input',undefined,{type:'search','aria-label':label,'aria-describedby':'name-entry-help',maxlength:key==='text'?(exactName?'24578':'256'):'64'}); input.value=value;
    if(key==='hash')input.disabled=exactName;
    input.addEventListener('input',()=>{if(key==='text'){if(exactName)exactEntry=input.value;else query=input.value;}else hash=input.value; ++epoch;selected=null;results.replaceChildren();detail(null);status.textContent='Query changed; awaiting recorded results…';clearTimeout(timer); timer=setTimeout(refresh,120);});
    input.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();clearTimeout(timer);refresh();}});
    wrapper.append(input); controls.append(wrapper);
  }
  controls.append(entryError);
  const status=node('p','Loading recorded results…',{id:'catalog-results-status',role:'status'});
  const split=node('div',undefined,{class:'split'}); const results=node('section',undefined,{'aria-label':'Catalog results'}); const inspector=node('section',undefined,{class:'panel inspector','aria-label':'Evidence Inspector'});
  split.append(results,inspector); content.append(controls,status,split);
  function detail(record) {
    inspector.replaceChildren(node('h2','Evidence Inspector'));
    if (!record) {inspector.append(node('p','Select a recorded file.'));return;}
    const snapshot=snapshots.find(s=>s.handle===record.snapshot);
    const file=snapshot?.catalog.files.find(f=>f.id===record.id);
    if(!file){inspector.append(node('p','Record unavailable in the requested snapshot.'));return;}
    if(mode.multi)inspector.append(node('p',`Snapshot: ${sourceLabel(snapshot)} · ${snapshot.catalog.digest} · Session ${snapshot.handle}`,{class:'inspector-snapshot'}));
    inspector.append(node('p',`File record ${file.id} · ${file.collection}`),node('strong',displayName(file.path),{class:'filename'}),node('p','Native file record identity is distinct from content hash and physical copies. No deduplication or authoritative original is inferred.'));
    const useName=node('button','Use exact name');useName.addEventListener('click',()=>{exactName=true;exactEntry=JSON.stringify(file.path);renderCatalog();document.querySelector('[aria-label="Exact recorded name (JSON string)"]').focus();});inspector.append(useName);
    const dl=node('dl');
    for (const [label,value] of [['Source folder (text only)',displayName(file.sourceFolder)],['Recorded relative path',displayName(file.path)],['Recorded bytes',file.bytes],['Recorded hash algorithm',file.algorithm],['Recorded SHA-256',file.hash],['Recorded first-seen time',file.firstSeen]]) dl.append(node('dt',label),node('dd',unavailable(value)));
    inspector.append(dl,node('p','Stored checksums and verification flags are historical evidence, not a newly executed verification. No source path is followed.'));
    for (const copy of file.copies) {
      const disclosure=node('details',undefined,{class:'occurrence'}); disclosure.append(node('summary',`${copy.id} · ${copy.volume}`));
      disclosure.append(node('p',`Package: ${copy.chunk}`),node('code',copy.path),node('p',`Recorded location: ${unavailable(copy.location)}`),node('p',`Recorded verification result: ${unavailable(copy.recordedVerifyOK)} · Recorded time: ${unavailable(copy.recordedVerifiedAt)} · Superseded: ${copy.superseded}. Current accessibility unknown; offline is not lost.`)); inspector.append(disclosure);
    }
    if (!file.copies.length) inspector.append(node('p','No matching current-content chunk-copy records. This does not prove no physical copies exist.'));
    const disabled=node('button','Operations unavailable — read-only snapshot',{disabled:''}); inspector.append(disabled);
  }
  async function refresh() {
    const request=++epoch;
    const filter=snapshotFilter;
    let params;
    try {
      if(exactName)params=new URLSearchParams({exact:parseExactName(exactEntry)});
      else {if(!validUnicode(query)||!validUnicode(hash))throw Error('Search must contain valid Unicode.');params=new URLSearchParams({text:query,hash});}
      entryError.textContent='';entryError.removeAttribute('role');
      if(mode.multi)params.set('snapshot',filter);
    } catch(error) {entryError.setAttribute('role','alert');entryError.textContent=error.message;selected=null;results.replaceChildren();detail(null);status.textContent='Invalid name entry; no query sent.';return;}
    try {
      const response=await fetch('/catalog-query?'+params);
      const result=await response.json(); if (request!==epoch || !status.isConnected) return;
      if (!response.ok || !result.ok) throw new Error('query refused');
      const rows=mode.multi?validateMatches(result,snapshots,filter):validateIDs(result.ids,data).map(id=>({snapshot:'single',id}));
      status.textContent=`${rows.length} recorded file record(s). ${mode.multi?'Returned entries across '+shown.length+' selected snapshot(s), not unique content, independent copies or combined coverage. Per-snapshot results: '+result.groups.map(g=>sourceLabel(snapshots.find(s=>s.handle===g.snapshot))+': '+g.ids.length).join('; ')+'. ':''}Snapshot only; unlisted inventory is unknown.`;
      results.replaceChildren(); if(!rows.some(r=>sameRecord(r,selected))) selected=null;
      for (const r of rows) { const s=snapshots.find(s=>s.handle===r.snapshot),f=s.catalog.files.find(f=>f.id===r.id); const b=node('button',`${mode.multi?sourceLabel(s)+' · ':''}${displayName(f.path)} · ${f.collection} · Record ${f.id}`,{class:'result','aria-pressed':String(sameRecord(r,selected)),'data-snapshot':r.snapshot,'data-record':r.id}); b.addEventListener('click',()=>{selected=r;for(const row of results.children)row.setAttribute('aria-pressed',String(row===b));detail(r);});results.append(b); }
      if(!rows.length)results.append(node('p',mode.multi?'No returned records in the selected snapshots. Consult each scope; this is not a combined completeness claim.':data.files.length?'No recorded matches.':'Valid empty catalog — no file records.'));
      detail(rows.find(r=>sameRecord(r,selected)));
    } catch {
      if(request!==epoch||!status.isConnected)return;
      failure='Catalog read unavailable — previous results cleared'; selected=null; epoch++; clearTimeout(timer);
      content.replaceChildren(node('p', failure + '. Relaunch the preview to read a catalog.', {role:'alert',class:'error panel'}));
    }
  }
  refresh();
}
