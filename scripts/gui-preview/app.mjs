import { scope, media, items, activity, searchItems, operationLabel } from './fixtures.mjs';

// All content is assigned as text, including synthetic hostile-looking filenames.
const el = (tag, text, attrs = {}) => {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  for (const [name, value] of Object.entries(attrs)) node.setAttribute(name, value);
  return node;
};
const routes = ['Library', 'Back Up', 'Archive', 'Find', 'Activity', 'Devices & locations', 'Settings'];
const purposes = [
  'Inspect known content and where each intentional copy was observed.',
  'Explore a recurring, incremental backup workflow. Operations are unavailable.',
  'Explore packaging, queueing and verification. Operations are unavailable.',
  'Find known content and inspect possible recovery sources within this sample.',
  'Simulated operation, recording and verification states are separate.',
  'Sample media observations, including offline and unknown information.',
  'Only display disclosure is available in this preview.',
];
const state = { query: '', medium: 'all', project: 'all', availability: 'all', scenario: 'ready', selected: null, disclosure: 'Studio', storage: 'Photography NAS', expanded: true };
const content = document.querySelector('#content');
const nav = document.querySelector('nav');
const slug = name => name.toLowerCase().replaceAll(' ', '-');
for (const [i, name] of routes.entries()) { const link = el('a', undefined, { href: `#${slug(name)}` }); link.append(icon(['disk','backup','archive','search','clock','devices','settings'][i]), el('span', name)); nav.append(link); }
document.querySelector('#scope').textContent = scope;
document.querySelector('#disclosure').addEventListener('change', event => {
  state.disclosure = event.target.value;
  render();
});
for (const button of document.querySelectorAll('[data-mode]')) button.addEventListener('click', () => {
  document.querySelector('#disclosure').value = button.dataset.mode;
  state.disclosure = button.dataset.mode;
  render();
  demo('Disclosure only; permissions and preservation guarantees never change. Unshown mode layouts are not specified by the export.');
});
function field(label, values, value, change) {
  const wrapper = el('label', label);
  const select = el('select');
  for (const [v, text] of values) select.append(el('option', text, { value: v }));
  select.value = value;
  select.addEventListener('change', () => change(select.value));
  wrapper.append(select);
  return wrapper;
}
function pair(target, label, value) { target.append(el('dt', label), el('dd', value)); }
function detail(item, target, find) {
  target.replaceChildren(el('h2', item ? 'Content details' : 'Select content'));
  if (!item) { target.append(el('p', 'Choose a result to inspect its known occurrences.')); return; }
  const info = el('dl');
  pair(info, 'Name / version', `${item.name} · ${item.version}`);
  pair(info, 'Project', item.project);
  pair(info, 'Content identity', item.id);
  if (state.disclosure !== 'Guided') pair(info, 'Synthetic hash (not a real checksum)', item.hash);
  target.append(info, el('p', 'All known sample occurrences below, including those outside the active filter. Multiple copies are intentional; offline does not mean lost.'));
  for (const o of item.occurrences) {
    const medium = media.find(m => m.id === o.medium);
    const card = el('section', undefined, { class: 'occurrence', 'aria-label': o.id });
    card.append(el('h3', medium.name), el('span', medium.online ? 'Online · simulated' : 'Offline · not automatically lost', { class: 'tag' }));
    const list = el('dl');
    pair(list, 'Occurrence / path', `${o.id} · ${o.path}`);
    pair(list, 'Last observation', medium.observed ?? 'Unknown');
    pair(list, 'Known location', medium.location);
    pair(list, 'Verification', `${o.verification} · Evidence date: ${o.verified ?? 'Unknown'}`);
    pair(list, 'Evidence scope', o.scope);
    if (state.disclosure !== 'Guided') pair(list, 'Inventory scope', medium.scope);
    if (state.disclosure === 'Expert') pair(list, 'Synthetic medium identifier', medium.id);
    card.append(list); target.append(card);
  }
  if (find) target.append(el('h3', 'Recovery planning · simulated'), el('p', 'Compare the version, occurrence, observation age and verification scope above. An offline medium would need to be located and its current condition checked. No recovery has been attempted.'), el('button', 'Restore unavailable in preview', { disabled: '' }));
}
function collection(find) {
  const controls = el('div', undefined, { class: 'panel controls' });
  const search = el('label', find ? 'Find by name, path, hash, version, occurrence or medium' : 'Filter known sample content');
  const input = el('input', undefined, { type: 'search', placeholder: 'Try v1, SIM-OCC-02 or Field notes', 'aria-label': 'Search sample content' });
  input.value = state.query;
  search.append(input); controls.append(search);
  const update = (key, value) => { state[key] = value; refresh(); };
  controls.append(field('Medium', [['all', 'All sample media'], ...media.map(m => [m.id, m.name])], state.medium, v => update('medium', v)));
  controls.append(field('Project', [['all', 'All sample projects'], ...[...new Set(items.map(i => i.project))].map(p => [p, p])], state.project, v => update('project', v)));
  controls.append(field('Availability', [['all', 'Online and offline'], ['online', 'Online'], ['offline', 'Offline']], state.availability, v => update('availability', v)));
  controls.append(field('Fixture presentation', [['ready', 'Ready'], ['partial', 'Partial information'], ['empty', 'Empty inventory'], ['loading', 'Loading (held simulation)'], ['error', 'Error']], state.scenario, v => update('scenario', v)));
  const reset = el('button', 'Reset filters');
  reset.addEventListener('click', () => { Object.assign(state, { query: '', medium: 'all', project: 'all', availability: 'all', scenario: 'ready', selected: null }); render(); const disclosure=document.querySelector('.sample-browser'); if(disclosure) disclosure.open=true; document.querySelector('.controls input').focus(); });
  controls.append(reset);
  const status = el('p', '', { id: 'results-status', role: 'status', 'aria-live': 'polite' });
  const split = el('div', undefined, { class: 'split' });
  const results = el('section', undefined, { 'aria-label': 'Search results' });
  const details = el('section', undefined, { class: 'panel', 'aria-label': 'Content details' });
  split.append(results, details); content.append(controls, status, split);
  input.addEventListener('input', () => update('query', input.value));
  function refresh() {
    results.replaceChildren();
    const matches = searchItems(state);
    const messages = { loading: 'Loading sample inventory — held simulation. Choose Ready to continue.', error: 'Sample inventory unavailable — simulated read error. No totals or stale results are shown. Choose Ready to retry.', empty: 'Empty simulated inventory. No occurrences in this scenario; this says nothing about real storage.' };
    status.textContent = messages[state.scenario] ?? `${matches.length} of ${items.length} sample content identities match. ${state.scenario === 'partial' ? 'Partial information: inventory completeness and current offline condition are unknown.' : 'Totals cover this sample only; unlisted contents are unknown.'}`;
    if (!matches.length && !messages[state.scenario]) results.append(el('p', 'No results in this sample. Change the search or reset filters.'));
    if (!matches.some(i => i.id === state.selected)) state.selected = null;
    for (const item of matches) {
      const row = el('button', undefined, { class: 'result', 'aria-pressed': String(state.selected === item.id), 'aria-label': `Inspect ${item.name} ${item.version}` });
      row.append(el('strong', `${item.name} · ${item.version}`), el('span', `${item.project} · ${item.occurrences.length} known sample occurrence(s)`));
      row.addEventListener('click', () => {
        state.selected = item.id;
        for (const button of results.children) button.setAttribute('aria-pressed', String(button === row));
        detail(item, details, find);
        status.textContent = `Selected ${item.name} ${item.version}. ${item.occurrences.length} known sample occurrence(s); details follow the results.`;
      });
      results.append(row);
    }
    detail(matches.find(i => i.id === state.selected), details, find);
  }
  refresh();
}
function render() {
  const projectView = location.hash === '#smith-wedding';
  const index = routes.findIndex(name => `#${slug(name)}` === location.hash);
  const route = index < 0 ? 0 : index;
  for (const link of nav.children) {
    if (link.textContent === routes[route]) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  }
  document.querySelector('h1').textContent = routes[route];
  document.querySelector('#purpose').textContent = purposes[route];
  content.replaceChildren();
  document.querySelector('#breadcrumb').replaceChildren();
  document.querySelector('#header-actions').replaceChildren();
  document.querySelector('#demo-message').textContent = '';
  document.querySelector('#scope').hidden = route === 0;
  document.querySelector('#purpose').hidden = route === 0 && !projectView;
  document.querySelector('.modes').hidden = projectView;
  document.querySelector('main').classList.toggle('project-view', projectView);
  for (const b of document.querySelectorAll('[data-mode]')) b.setAttribute('aria-pressed', String(b.dataset.mode === state.disclosure));
  if (projectView) { projectPage(); }
  else if (route === 0) { libraryPage(); }
  else if (route === 3) {
    document.querySelector('#purpose').textContent += ' Visual alignment is not specified by this export.';
    content.append(el('p', 'Inventory/enrollment describes observations. It does not copy, move or reorganize originals. Completed copying does not establish verification.', { class: 'notice' }));
    collection(route === 3);
  } else if (route === 1 || route === 2) {
    const steps = route === 1 ? ['1 · Choose a recurring plan', '2 · Inspect incremental changes', '3 · Copy, record, then verify'] : ['1 · Prepare a package', '2 · Queue the package', '3 · Write, record, then verify'];
    const grid = el('div', undefined, { class: 'steps' });
    for (const step of steps) { const panel = el('section', undefined, { class: 'panel' }); panel.append(el('h2', step), el('p', 'Workflow placeholder. No plan, package or operation is created.')); grid.append(panel); }
    content.append(grid, el('p', 'Schedules, retention, encryption, device selection and execution are deferred. No preservation guarantee is implied.'), el('button', route === 1 ? 'Run backup unavailable' : 'Write archive unavailable', { disabled: '' }));
  } else if (route === 4) {
    const panel = el('section', undefined, { class: 'panel' });
    panel.append(el('h2', 'Sample activity'), el('p', 'FAILED stays primary when recording also fails. Completed is not verified. These examples are static, not live progress.'));
    for (const row of activity) { const card = el('article', undefined, { class: `row${row.operation === 'FAILED' ? ' error' : ''}` }); card.append(el('h3', row.name), el('strong', operationLabel(row)), el('p', `Recording history: ${row.history}`)); panel.append(card); }
    content.append(panel);
  } else if (route === 5) {
    for (const m of media) { const panel = el('section', undefined, { class: 'panel' }); panel.append(el('h2', m.name), el('p', `${m.id} · ${m.online ? 'Online (simulated)' : 'Offline; not automatically lost'}`), el('p', `${m.location} · Last observed: ${m.observed ?? 'Unknown'}`), el('p', m.scope)); content.append(panel); }
    content.append(el('p', 'Unknown is not zero. No devices have been discovered or enrolled.'), el('button', 'Discover / enroll unavailable', { disabled: '' }));
  } else {
    content.append(el('section', 'Use “How much to show” to reveal inventory scope in Studio and synthetic medium identifiers in Expert. Core offline, recording and verification warnings remain visible in every mode. Display selection lasts only for this page session. Retention, encryption, keys, permissions and operational settings are unavailable.', { class: 'panel' }));
  }
}
// Skipping navigation moves focus within this workspace, without routing or rendering.
document.querySelector('.skip').addEventListener('click', event => {
  event.preventDefault();
  document.querySelector('main').focus();
});
addEventListener('hashchange', () => { render(); document.querySelector('h1').focus(); });
render();

// PDF-reference alignment. Static design values are synthetic fixtures, not live state.
function icon(name) {
  const paths = {
    disk: 'M4 6C4 2 20 2 20 6S4 10 4 6M4 6V18C4 22 20 22 20 18V6M4 12C4 16 20 16 20 12',
    backup: 'M5 15A7 7 0 1 1 18 17M5 15V8M5 15H12M9 18A4 4 0 1 0 15 12',
    archive: 'M4 4H20V20H4ZM8 8L16 16M16 8L8 16', search: 'M16 16L21 21M18 10A8 8 0 1 1 2 10A8 8 0 1 1 18 10',
    clock: 'M12 3A9 9 0 1 0 12 21A9 9 0 1 0 12 3M12 6V12L16 14',
    devices: 'M4 3V20H21M7 16L11 10L16 13L21 5M7 16H8M11 10H12M16 13H17',
    settings: 'M5 3V9M5 14V21M12 3V14M12 18V21M19 3V6M19 11V21M2 10H8M9 16H15M16 8H22',
    folder: 'M3 5H9L12 8H21V20H3Z', alert: 'M12 3L22 21H2ZM12 9V14M12 17V18',
    check: 'M5 12L10 17L20 6', dot: 'M12 7A5 5 0 1 0 12 17A5 5 0 1 0 12 7', plus: 'M12 4V20M4 12H20', arrow: 'M4 12H20M14 6L20 12L14 18',
    compare: 'M4 3H20V21H4ZM10 8L7 12L10 16M14 8L17 12L14 16',
    info: 'M12 3A9 9 0 1 0 12 21A9 9 0 1 0 12 3M12 10V17M12 6V7',
  };
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 24 24'); svg.setAttribute('aria-hidden', 'true'); svg.setAttribute('class', 'icon');
  const path = document.createElementNS(svg.namespaceURI, 'path'); path.setAttribute('d', paths[name] ?? paths.disk); svg.append(path); return svg;
}
function demo(action) { document.querySelector('#demo-message').textContent = `Demo only — ${action} No files are read or written; no operation or registration is performed.`; }
function action(text, type = 'link', symbol, callback) {
  const b = el('button', undefined, { class: type, title: 'Demo only — no files are read or written', 'data-demo': '' });
  if (symbol) b.append(icon(symbol)); b.append(el('span', text));
  b.addEventListener('click', callback ?? (() => demo(`${text}: this operational control is unavailable in the preview.`))); return b;
}
function heading(title, actionText, symbol) {
  const row = el('div', undefined, { class: 'section-heading' }); const h = el('h2'); if(symbol) h.append(icon(symbol)); h.append(el('span',title)); row.append(h);
  if(actionText) row.append(action(actionText)); return row;
}
function badge(text, tone = 'neutral', symbol) { const b=el('span', undefined, {class:`badge ${tone}`}); if(symbol) b.append(icon(symbol)); b.append(el('span',text)); return b; }
function table(headers, label) { const t=el('table', undefined, {'aria-label':label}); const head=el('thead'); const tr=el('tr'); for(const h of headers) tr.append(el('th',h,{scope:'col'})); head.append(tr); t.append(head,el('tbody')); return t; }
function cell(row, value, cls) { const td=el('td',undefined,cls?{class:cls}:{}); td.append(typeof value==='string'?document.createTextNode(value):value); row.append(td); return td; }
function headerSearch(placeholder, label, onInput) { const wrap=el('label',undefined,{class:'header-search'}); const input=el('input',undefined,{type:'search',placeholder,'aria-label':label}); wrap.append(icon('search'),input); input.addEventListener('input',()=>onInput(input.value)); document.querySelector('#header-actions').append(wrap); }
function libraryPage() {
  const top=el('div',undefined,{class:'library-grid'}); const storage=el('section',undefined,{class:'panel storage-panel'});
  storage.append(heading('Configured Storage Locations','Manage mountpoints'));
  const t=table(['Name','Status','Capacity','Last inventory','Action'],'Configured Storage Locations');
  const rows=[['Photography NAS','Online','12.4 TB','Sep 12, 2025','Explore'],['HDD-017','Connected','2 TB','Sep 15, 2025','Scan'],['HDD-023','Offline · Closet / Box 3','2 TB','Aug 3, 2025','Locate'],['This PC · Documents','Online','450 GB','Sep 17, 2025','Scan']];
  for(const [name,status,capacity,date,act] of rows) {
    const tr=el('tr',undefined,{'data-search':name.toLowerCase(),class:`${state.storage===name?'selected ':''}${name==='HDD-023'?'offline':''}`});
    const select=action(name,'storage-choice','disk',()=>{state.storage=name; for(const row of t.tBodies[0].rows) {row.classList.toggle('selected',row===tr); row.querySelector('.storage-choice').setAttribute('aria-pressed',String(row===tr));} demo(`${name} selected; inventory dates are historical sample observations.`);}); select.removeAttribute('data-demo'); select.setAttribute('aria-pressed',String(state.storage===name));
    cell(tr,select); cell(tr,badge(status,status==='Online'?'good':status==='Connected'?'teal':'neutral',status==='Online'?'dot':undefined)); cell(tr,capacity); cell(tr,date,'muted'); cell(tr,action(act)); t.tBodies[0].append(tr);
  }
  storage.append(t); const attention=el('section',undefined,{class:'panel attention'}); attention.append(heading('Needs Attention',null,'alert'));
  attention.append(badge('6 unresolved comparisons','warning','alert'),badge('1 incomplete inventory','neutral','check'),el('p','Obelisk verifies file parity across all active sources. Review these anomalies to resolve sync state differences.'),action('Launch parity wizard','link','arrow'));
  top.append(storage,attention); content.append(top);
  const projects=el('section',undefined,{class:'panel projects-panel'}); projects.append(heading('Active Projects & Catalogs','Define new project boundaries','folder'));
  const pt=table(['Name','Sources','Status','Action'],'Active Projects & Catalogs');
  for(const [name,sources,count] of [['Smith Wedding',3,6],['Family Photos 2019–2024',2,0],['Tax Documents 2020–2024',1,1]]) {
    const tr=el('tr',undefined,{'data-search':name.toLowerCase()}); const label=el('div',undefined,{class:'project-name'});
    if(name==='Smith Wedding') {label.append(el('a',name,{href:'#smith-wedding'}),el('span','— October 2021',{class:'muted'}));} else label.append(action(name,'project-choice',null,()=>demo(`${name} selected. This project's detail is not specified by the supplied export.`)));
    cell(tr,label); const s=el('span',undefined,{class:'with-icon'}); s.append(icon('disk'),document.createTextNode(`${sources} source${sources>1?'s':''}`)); cell(tr,s);
    const status=el('div',undefined,{class:'project-status'}); status.append(badge(count?'Needs attention':'Up to date',count?'warning':'good',count?'alert':'check'),el('span',count?`${count} item${count>1?'s':''} need${count===1?'s':''} attention`:'Up to date',{class:count?'warn-text':'good-text'})); cell(tr,status);
    if(name==='Smith Wedding') cell(tr,el('a','Fix issues',{href:'#smith-wedding',class:'link'})); else cell(tr,action(count?'Fix issues':'Verify')); pt.tBodies[0].append(tr);
  }
  projects.append(pt); content.append(projects);
  headerSearch('Search inventory...','Search inventory',q=>{for(const row of content.querySelectorAll('[data-search]')) row.hidden=!row.dataset.search.includes(q.trim().toLowerCase()); document.querySelector('#demo-message').textContent=q?'Filtering this synthetic inventory only. Unlisted contents are unknown.':'';});
  document.querySelector('#header-actions').append(action('Compare copies','secondary','compare',()=>{location.hash='#smith-wedding';}),action('Add storage','primary','plus'));
  // Keep prior Library content inspection reachable without crowding the supplied frame.
  const extra=el('details',undefined,{class:'sample-browser'}); extra.append(el('summary','Inspect synthetic content and occurrences (additional preview controls)'));
  const prior=el('div'); const oldChildren=[...content.children]; collection(false); for(const node of [...content.children]) if(!oldChildren.includes(node)) prior.append(node); extra.append(prior); content.append(extra);
}
function projectPage() {
  document.querySelector('h1').textContent='Smith Wedding — October 2021';
  const breadcrumb=document.querySelector('#breadcrumb'); breadcrumb.append(el('a','Library',{href:'#library'}),el('span','›'),el('strong','Smith Wedding'));
  const purpose=document.querySelector('#purpose'); purpose.replaceChildren(document.createTextNode('Comparing against: '),el('strong','Photography NAS'),document.createTextNode(' (1,000 expected paths) '),el('span','· USER-SELECTED COMPARISON REFERENCE',{class:'reference-note'}));
  headerSearch('Search files...','Search project files',q=>{const found='img_4587.cr2'.includes(q.trim().toLowerCase()); document.querySelector('.inspector-body').hidden=!found; document.querySelector('.file-no-results').hidden=found;});
  document.querySelector('#header-actions').append(action('View comparison logs','secondary','archive'));
  const grid=el('div',undefined,{class:'project-grid'}); const left=el('div'); const panel=el('section',undefined,{class:'panel matrix-panel'}); panel.append(heading('Storage Targets Sync State','Parity Matrix Settings'));
  const t=table(['Source','Status','Matching','Absent','Differing','Unresolved'],'Storage Targets Sync State');
  const ref=el('tr'); const refName=el('strong',undefined,{class:'with-icon'}); refName.append(icon('disk'),document.createTextNode('Photography NAS')); cell(ref,refName); cell(ref,badge('Reference','blue')); cell(ref,'1,000','number'); for(let i=0;i<3;i++) cell(ref,'–','number muted'); t.tBodies[0].append(ref);
  const drive=el('tr',undefined,{class:'selected'}); const toggle=action('HDD-017','storage-choice','disk',()=>{state.expanded=!state.expanded; details.hidden=!state.expanded; toggle.setAttribute('aria-expanded',String(state.expanded));}); toggle.removeAttribute('data-demo'); toggle.setAttribute('aria-expanded',String(state.expanded)); toggle.setAttribute('aria-controls','hdd017-details');
  cell(drive,toggle); cell(drive,badge('Connected','teal')); cell(drive,'✓ 994','number good-text'); cell(drive,'4','number warn-text'); cell(drive,'2','number warn-text'); cell(drive,'0','number muted'); t.tBodies[0].append(drive);
  const details=el('tr',undefined,{class:'selected detail-row',id:'hdd017-details'}); details.hidden=!state.expanded; const d=cell(details,el('div')); d.colSpan=6;
  d.firstChild.append(el('p','4 absent paths — expected files missing from the destination drive.'),el('p','2 differing files — identical relative paths but file sizes or checksums do not match.'),el('p','12 destination-only files (not counted in the 1,000 reference denominator).',{class:'teal-text'})); t.tBodies[0].append(details);
  const off=el('tr',undefined,{class:'offline-drive'}); cell(off,'HDD-023'); cell(off,badge('Offline')); cell(off,'✓ 800','number muted'); cell(off,'–','number muted'); cell(off,'–','number muted'); cell(off,'200','number warn-text'); t.tBodies[0].append(off);
  const note=el('tr',undefined,{class:'offline-note'}); const n=cell(note,'Inventory incomplete. 200 paths unresolved — may be present, absent, or different. Last scan: Aug 3, 2025.'); n.colSpan=6; t.tBodies[0].append(note); panel.append(t); left.append(panel);
  const actions=el('div',undefined,{class:'registration'}); actions.append(action('Register 994 matching copies on HDD-017','primary','check'),action('Review 6 unresolved items','secondary','arrow',()=>{demo('Source wording retained: 4 absent + 2 differing; the matrix Unresolved category is 0. No category changes made.'); document.querySelector('.inspector h2').focus();})); left.append(actions,el('p','Records existing locations. Does not copy, move, delete, or reorganize.',{class:'registration-note'}));
  const inspector=el('section',undefined,{class:'panel inspector','aria-label':'Evidence Inspector'}); const h=heading('Evidence Inspector',null); h.querySelector('h2').tabIndex=-1; h.append(icon('info')); inspector.append(h);
  const body=el('div',undefined,{class:'inspector-body'}); body.append(el('p','SELECTED FILE',{class:'eyebrow'}),el('strong','IMG_4587.CR2',{class:'filename'}));
  body.append(badge('Different content — same filename and path, different bytes.','warning','alert'));
  const observations=el('section',undefined,{class:'evidence-block'});
  for(const [label,path,size,date] of [['ON PHOTOGRAPHY NAS','/SmithWedding/RAW/IMG_4587.CR2','28.4 MB','Sep 12, 2025'],['ON HDD-017','/WeddingArchive/October2021/IMG_4587.CR2','28.1 MB','Sep 15, 2025']]) {observations.append(el('p',label,{class:'eyebrow'}),el('code',path)); const line=el('p',undefined,{class:'observed'}); line.append(el('strong',size),el('span',` · Last observed: ${date}`,{class:'muted'})); observations.append(line);}
  body.append(observations); const hashes=el('section',undefined,{class:'evidence-block checksums'}); for(const [name,hash] of [['NAS CHECKSUM','8a3f9d4b … 2e51fc89'],['HDD-017 CHECKSUM','cf90a182 … 98547b6a']]) {hashes.append(el('p',name,{class:'eyebrow'}),el('code',hash));} body.append(hashes);
  const buttons=el('div',undefined,{class:'inspector-actions'}); buttons.append(action('Compare versions','primary'),action('Keep both files','secondary'),action('Dismiss anomaly','plain')); body.append(buttons,el('p','Obelisk does not automatically choose the newest file or delete either copy.',{class:'inspector-note'})); inspector.append(body,el('p','No matching file in this synthetic project.',{class:'file-no-results',hidden:''})); grid.append(left,inspector); content.append(grid);
}
