import mode from './mode.mjs';
import { validateCatalog, validateIDs } from './catalog-protocol.mjs';
export const catalogMode = mode.enabled;
const notice = 'Disposable catalog mode — reading a synthetic catalog snapshot. No source/media files are opened. The selected catalog is not modified.';
const node = (tag, text, attrs = {}) => { const e = document.createElement(tag); if (text !== undefined) e.textContent = text; for (const [k,v] of Object.entries(attrs)) e.setAttribute(k,v); return e; };
const unavailable = value => value === null || value === undefined || value === '' || value === '0001-01-01T00:00:00Z' ? 'Unavailable' : String(value);
let query = '', hash = '', selected = null, epoch = 0, timer, failure = '';
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
  for (const [label,value,key] of [['Recorded path contains',query,'text'],['Recorded SHA-256 prefix',hash,'hash']]) {
    const wrapper=node('label',label); const input=node('input',undefined,{type:'search','aria-label':label,maxlength:key==='text'?'256':'64'}); input.value=value;
    input.addEventListener('input',()=>{if(key==='text') query=input.value; else hash=input.value; clearTimeout(timer); timer=setTimeout(refresh,120);}); wrapper.append(input); controls.append(wrapper);
  }
  const status=node('p','Loading recorded results…',{id:'catalog-results-status',role:'status'});
  const split=node('div',undefined,{class:'split'}); const results=node('section',undefined,{'aria-label':'Catalog results'}); const inspector=node('section',undefined,{class:'panel inspector','aria-label':'Evidence Inspector'});
  split.append(results,inspector); content.append(controls,status,split);
  function detail(file) {
    inspector.replaceChildren(node('h2','Evidence Inspector'));
    if (!file) {inspector.append(node('p','Select a recorded file.'));return;}
    inspector.append(node('p',`File record ${file.id} · ${file.collection}`),node('strong',file.path,{class:'filename'}),node('p','Native file record identity is distinct from content hash and physical copies. No deduplication or authoritative original is inferred.'));
    const dl=node('dl');
    for (const [label,value] of [['Source folder (text only)',file.sourceFolder],['Recorded relative path',file.path],['Recorded bytes',file.bytes],['Recorded hash algorithm',file.algorithm],['Recorded SHA-256',file.hash],['Recorded first-seen time',file.firstSeen]]) dl.append(node('dt',label),node('dd',unavailable(value)));
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
    try {
      const response=await fetch('/catalog-query?'+new URLSearchParams({text:query,hash}));
      const result=await response.json(); if (request!==epoch || !status.isConnected) return;
      if (!response.ok || !result.ok) throw new Error('query refused');
      validateIDs(result.ids, data);
      const byID = new Map(data.files.map(file => [file.id, file]));
      const rows=result.ids.map(id=>byID.get(id));
      status.textContent=`${rows.length} recorded file record(s). Snapshot only; unlisted inventory is unknown.`;
      results.replaceChildren(); if(!rows.some(f=>f.id===selected)) selected=null;
      for (const f of rows) { const b=node('button',`${f.path} · ${f.collection} · Record ${f.id}`,{class:'result','aria-pressed':String(f.id===selected)}); b.addEventListener('click',()=>{selected=f.id;for(const row of results.children)row.setAttribute('aria-pressed',String(row===b));detail(f);});results.append(b); }
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
