import mode from './mode.mjs';
import { validateCatalog, validateIDs } from './catalog-protocol.mjs';
import { parseExactName, displayName, validUnicode } from './catalog-names.mjs';
export const catalogMode = mode.enabled;
const notice = 'Disposable catalog mode — reading a synthetic catalog snapshot. No source/media files are opened. The selected catalog is not modified.';
const node = (tag, text, attrs = {}) => { const e = document.createElement(tag); if (text !== undefined) e.textContent = text; for (const [k,v] of Object.entries(attrs)) e.setAttribute(k,v); return e; };
const unavailable = value => value === null || value === undefined || value === '' || value === '0001-01-01T00:00:00Z' ? 'Unavailable' : String(value);
let query = '', hash = '', selected = null, epoch = 0, timer, failure = '';
let exactName = false, exactEntry = '""';
if (catalogMode && mode.ok) {
  try { validateCatalog(mode.catalog); } catch { failure = 'Invalid catalog response'; }
}

export function renderCatalog() {
  const content = document.querySelector('#content');
  const find = location.hash === '#find';
  const title = find ? 'Find' : location.hash === '#library' || !location.hash ? 'Library' : 'Catalog view unavailable';
  document.querySelector('h1').textContent = title;
  document.querySelector('#purpose').hidden = false;
  document.querySelector('#purpose').textContent = 'Recorded catalog evidence only. Availability, capacity, parity and verification performed now: unavailable.';
  document.querySelector('#scope').hidden = false;
  document.querySelector('#scope').textContent = notice;
  document.querySelector('footer span').textContent = notice;
  document.querySelector('footer strong').textContent = 'Read-only snapshot';
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
  const data = mode.catalog;
  content.append(node('p', `Catalog SHA-256: ${data.digest} · schema ${data.schema} · Session load time: ${data.loadedAt}. Loading is not scanning or verification.`, {class:'notice',id:'catalog-identity'}));
  const scope = data.inventoryScope;
  content.append(node('p', scope
    ? `Recorded inventory scope v${scope.version}: .DS_Store exclusion ${scope.policy==='include-all'?'OFF':'ON'}. ${scope.entries} entries visited; ${scope.regularFiles} regular files observed; ${scope.includedFiles} included; ${scope.excludedFiles} excluded by exact regular-basename policy; ${scope.readBytes} bytes read. Completed within selected scope. ${scope.entries===0?'Truly empty source.':scope.includedFiles===0&&scope.excludedFiles>0?'All observed regular files deliberately excluded.':'Unlisted source content and current state are not verified.'}`
    : 'Historical inventory scope: policy and excluded counts UNKNOWN. Existing records are shown unchanged.', {class:'notice',id:'inventory-scope'}));
  if (title === 'Catalog view unavailable') { content.append(node('p','This workspace has no supported catalog projection. Use Library or Find.')); return; }
  if (!find) {
    const projects = node('section',undefined,{class:'panel'}); projects.append(node('h2','Recorded collections'));
    for (const c of data.collections) projects.append(node('p',`${c.name} · ID ${c.id}${c.retired ? ' · Retired (excluded from default Find)' : ''}`));
    if (!data.collections.length) projects.append(node('p','No collections recorded.'));
    const storage = node('section',undefined,{class:'panel'}); storage.append(node('h2','Recorded storage'));
    for (const v of data.volumes) storage.append(node('p',`${unavailable(v.label)} · ${unavailable(v.kind)} · Recorded location: ${unavailable(v.location)} · Current accessibility: unavailable`));
    if (!data.volumes.length) storage.append(node('p','No storage recorded; this is not a live inventory.'));
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
    input.addEventListener('input',()=>{if(key==='text'){if(exactName)exactEntry=input.value;else query=input.value;}else hash=input.value; clearTimeout(timer); timer=setTimeout(refresh,120);});
    input.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();clearTimeout(timer);refresh();}});
    wrapper.append(input); controls.append(wrapper);
  }
  controls.append(entryError);
  const status=node('p','Loading recorded results…',{id:'catalog-results-status',role:'status'});
  const split=node('div',undefined,{class:'split'}); const results=node('section',undefined,{'aria-label':'Catalog results'}); const inspector=node('section',undefined,{class:'panel inspector','aria-label':'Evidence Inspector'});
  split.append(results,inspector); content.append(controls,status,split);
  function detail(file) {
    inspector.replaceChildren(node('h2','Evidence Inspector'));
    if (!file) {inspector.append(node('p','Select a recorded file.'));return;}
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
    let params;
    try {
      if(exactName)params=new URLSearchParams({exact:parseExactName(exactEntry)});
      else {if(!validUnicode(query)||!validUnicode(hash))throw Error('Search must contain valid Unicode.');params=new URLSearchParams({text:query,hash});}
      entryError.textContent='';entryError.removeAttribute('role');
    } catch(error) {entryError.setAttribute('role','alert');entryError.textContent=error.message;selected=null;results.replaceChildren();detail(null);status.textContent='Invalid name entry; no query sent.';return;}
    try {
      const response=await fetch('/catalog-query?'+params);
      const result=await response.json(); if (request!==epoch || !status.isConnected) return;
      if (!response.ok || !result.ok) throw new Error('query refused');
      validateIDs(result.ids, data);
      const byID = new Map(data.files.map(file => [file.id, file]));
      const rows=result.ids.map(id=>byID.get(id));
      status.textContent=`${rows.length} recorded file record(s). Snapshot only; unlisted inventory is unknown.`;
      results.replaceChildren(); if(!rows.some(f=>f.id===selected)) selected=null;
      for (const f of rows) { const b=node('button',`${displayName(f.path)} · ${f.collection} · Record ${f.id}`,{class:'result','aria-pressed':String(f.id===selected)}); b.addEventListener('click',()=>{selected=f.id;for(const row of results.children)row.setAttribute('aria-pressed',String(row===b));detail(f);});results.append(b); }
      if(!rows.length)results.append(node('p',data.files.length?'No recorded matches.':'Valid empty catalog — no file records.'));
      detail(rows.find(f=>f.id===selected));
    } catch {
      if(request!==epoch||!status.isConnected)return;
      failure='Catalog read unavailable — previous results cleared'; selected=null; epoch++; clearTimeout(timer);
      content.replaceChildren(node('p', failure + '. Relaunch the preview to read a catalog.', {role:'alert',class:'error panel'}));
    }
  }
  refresh();
}
