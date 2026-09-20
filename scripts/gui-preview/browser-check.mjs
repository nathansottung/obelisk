// Optional real-browser smoke check using preinstalled Chromium and Node 24 only.
// Usage: node scripts/gui-preview/browser-check.mjs <chrome.exe> <new-evidence-directory>
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import { startPreview } from './server.mjs';
import { catalogChecks } from './catalog-browser-checks.mjs';

const [browser, evidence, catalog, adapter, expected] = process.argv.slice(2);
if (!browser || !evidence || !path.isAbsolute(evidence)) throw new Error('Provide preinstalled browser path and absolute new evidence directory');
await mkdir(evidence); // Refuse reuse of a profile or existing evidence directory.
const server = await startPreview(0, catalog ? { catalog, adapter } : null);
const base = `http://127.0.0.1:${server.address().port}`;
let child, socket, stopped, browserVersion, completed = false;
const checks = [], requests = [], errors = [];
const deadline = setTimeout(() => { socket?.close(); child?.kill(); server.closeAllConnections(); server.close(); }, 90000);
try {
  child = spawn(browser, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--disable-background-networking', '--disable-component-update', '--disable-sync', '--metrics-recording-only', '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0', `--user-data-dir=${path.join(evidence, 'profile')}`, 'about:blank'], { windowsHide: true, stdio: ['ignore', 'ignore', 'pipe'] });
  stopped = once(child, 'exit');
  let stderr = '';
  const endpoint = await new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', () => reject(new Error('Browser exited before debugging endpoint')));
    child.stderr.on('data', chunk => { stderr += chunk; const m = stderr.match(/DevTools listening on (ws:\/\/[^\s]+)/); if (m) resolve(m[1]); });
  });
  socket = new WebSocket(endpoint);
  await once(socket, 'open');
  let seq = 0;
  const pending = new Map();
  socket.addEventListener('close', () => { for (const p of pending.values()) p.reject(new Error('Browser connection closed')); pending.clear(); });
  socket.addEventListener('message', event => {
    const m = JSON.parse(event.data);
    if (m.id) { const p = pending.get(m.id); pending.delete(m.id); if (m.error) p.reject(new Error(JSON.stringify(m.error))); else p.resolve(m.result); }
    if (m.method === 'Network.requestWillBeSent') requests.push(m.params.request.url);
    if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails);
    if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error') errors.push(m.params);
  });
  const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++seq; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
  browserVersion = await send('Browser.getVersion');
  const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
  const cdp = (method, params) => send(method, params, sessionId);
  await cdp('Page.enable'); await cdp('Runtime.enable'); await cdp('Network.enable');
  await cdp('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1024, deviceScaleFactor: 2, mobile: false });
  const evaluate = async expression => {
    const r = await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (r.exceptionDetails) throw new Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  await cdp('Page.navigate', { url: base });
  await evaluate(`new Promise(resolve => { const timer = setInterval(() => { if(document.querySelector(${JSON.stringify(catalog ? '#catalog-identity, [role=alert]' : '.result')})) { clearInterval(timer); resolve(true); } }, 25); })`);
  const check = async (name, expression) => { assert.equal(await evaluate(expression), true, name); checks.push(name); };
  const screenshot = async name => { const r = await cdp('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await writeFile(path.join(evidence, name + '.png'), Buffer.from(r.data, 'base64')); };
  const route = async slug => { await evaluate(`location.hash=${JSON.stringify('#' + slug)}`); await evaluate('new Promise(resolve => setTimeout(resolve, 60))'); };
  if (catalog) {
    await catalogChecks({ check, evaluate, cdp, screenshot, route, expected, evidence, writeFile, path });
  } else {
  // Real keyboard skip-link regression; setup focuses the first primary-nav link,
  // then Shift+Tab reaches the application's skip link (never direct main focus).
  const skipCases = [];
  const settle = () => evaluate('new Promise(resolve => setTimeout(resolve, 80))');
  const key = async (name, shift = false) => {
    const code = name === 'Tab' ? 9 : 13;
    await cdp('Input.dispatchKeyEvent', { type: 'keyDown', key: name, code: name, windowsVirtualKeyCode: code, modifiers: shift ? 8 : 0, ...(name === 'Enter' ? {text: '\r'} : {}) });
    await cdp('Input.dispatchKeyEvent', { type: 'keyUp', key: name, code: name, windowsVirtualKeyCode: code, modifiers: shift ? 8 : 0 });
    await settle();
  };
  const snapshot = () => evaluate(`({url:location.href, history:history.length, timeOrigin:performance.timeOrigin,
    title:document.querySelector('h1').textContent, content:document.querySelector('#content').textContent,
    inputs:[...document.querySelectorAll('main input,main select')].map(e=>e.value),
    selected:[...document.querySelectorAll('main [aria-pressed=true]')].map(e=>e.textContent),
    expanded:document.querySelector('[aria-controls=hdd017-details]')?.getAttribute('aria-expanded'),
    file:document.querySelector('.filename')?.textContent,
    focus:{tag:document.activeElement.tagName,id:document.activeElement.id,text:document.activeElement.textContent.slice(0,60)}})`);
  for (const name of ['library', 'find', 'smith-wedding', 'find-rerender']) {
    await route(name === 'find-rerender' ? 'find' : name);
    if (name.startsWith('find')) {
      await evaluate(`{document.querySelector('.controls button').click(); const q=document.querySelector('.controls input'); q.value='SIM-OCC-03'; q.dispatchEvent(new Event('input')); document.querySelector('.result').click();}`);
      if (name === 'find-rerender') await evaluate("document.querySelector('[data-mode=Expert]').click()");
    }
    if (name === 'smith-wedding') await evaluate(`{const q=document.querySelector('input');q.value='IMG_4587';q.dispatchEvent(new Event('input')); const b=document.querySelector('[aria-controls=hdd017-details]'); if(b.getAttribute('aria-expanded')==='true') b.click(); b.click();}`);
    await evaluate("document.querySelector('nav a').focus()");
    await key('Tab', true);
    const reached = await evaluate("document.activeElement.matches('.skip') && document.activeElement.matches(':focus-visible') && document.activeElement.getBoundingClientRect().top >= 0");
    await screenshot('skip-link-' + name);
    const before = await snapshot();
    await evaluate("window.skipContentBefore=document.querySelector('#content').firstElementChild");
    await key('Enter');
    const after = await snapshot();
    const focusMain = await evaluate("document.activeElement === document.querySelector('main') && document.activeElement.getBoundingClientRect().height > 0 && document.activeElement.matches(':focus-visible')");
    const sameNode = await evaluate("window.skipContentBefore === document.querySelector('#content').firstElementChild");
    await screenshot('skip-destination-' + name);
    await key('Tab');
    const next = (await snapshot()).focus;
    const tabForward = await evaluate(name === 'smith-wedding' ? "document.activeElement === document.querySelector('#breadcrumb a')" : "document.activeElement === document.querySelector('[data-mode=Guided]')");
    await key('Tab', true);
    const reverse = (await snapshot()).focus;
    const reverseAvailable = await evaluate("document.activeElement !== document.body && !document.activeElement.matches('main')");
    await key('Tab');
    const forwardAgain = await evaluate(name === 'smith-wedding' ? "document.activeElement === document.querySelector('#breadcrumb a')" : "document.activeElement === document.querySelector('[data-mode=Guided]')");
    const {focus: ignoredBefore, ...stateBefore} = before;
    const {focus: ignoredAfter, ...stateAfter} = after;
    const preserved = JSON.stringify(stateBefore) === JSON.stringify(stateAfter);
    const pass = reached && preserved && focusMain && sameNode && tabForward && reverseAvailable && forwardAgain;
    skipCases.push({name, pass, reached, preserved, focusMain, sameNode, tabForward, reverseAvailable, forwardAgain, before, after, next, reverse});
  }
  await writeFile(path.join(evidence, 'skip-link-results.json'), JSON.stringify(skipCases, null, 2));
  for (const result of skipCases) { assert.ok(result.pass, 'Keyboard skip preserves workspace/state/focus/Tab: ' + result.name); checks.push('Keyboard skip preserves workspace/state/focus/Tab: ' + result.name); }
  await evaluate("document.querySelector('nav a[href=\"#library\"]').click()"); await settle();
  await evaluate("document.querySelector('.project-name a').click()"); await settle();
  await evaluate('history.back()'); await settle();
  await check('Back returns to Library after normal project link', "location.hash === '#library' && document.querySelector('h1').textContent === 'Library'");
  await evaluate('history.forward()'); await settle();
  await check('Forward returns to project without a skip history entry', "location.hash === '#smith-wedding' && document.querySelector('.filename').textContent === 'IMG_4587.CR2'");
  await evaluate("document.querySelector('#breadcrumb a').click()"); await settle();
  await check('Breadcrumb still returns to Library after keyboard skip', "location.hash === '#library' && document.activeElement === document.querySelector('h1')");
  await route('find');
  await evaluate("document.querySelector('.controls button').click(); document.querySelector('[data-mode=Studio]').click()");
  await route('library');
  await check('Library reference tables and Studio default', "document.querySelectorAll('.storage-panel tbody tr').length === 4 && document.querySelectorAll('.projects-panel tbody tr').length === 3 && document.querySelector('[data-mode=Studio]').getAttribute('aria-pressed') === 'true'");
  await screenshot('library-reference-aligned');
  await evaluate("document.querySelectorAll('.storage-choice')[1].click()");
  await check('Storage selection is local', "document.querySelector('.storage-choice[aria-pressed=true]').textContent === 'HDD-017' && document.querySelector('#demo-message').textContent.includes('Demo only')");
  await evaluate("{const s=document.querySelector('[aria-label=\"Search inventory\"]');s.value='Smith';s.dispatchEvent(new Event('input'));}");
  await check('Library search filters sample storage and projects', "document.querySelectorAll('.storage-panel tr[data-search]:not([hidden])').length === 0 && document.querySelectorAll('.projects-panel tr[data-search]:not([hidden])').length === 1");
  await evaluate("document.querySelector('.project-name a').focus()");
  await cdp('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', text: '\r', windowsVirtualKeyCode: 13 });
  await cdp('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  await evaluate('new Promise(resolve => setTimeout(resolve, 60))');
  await check('Keyboard Library to Smith Wedding navigation', "location.hash === '#smith-wedding' && document.activeElement === document.querySelector('h1')");
  await evaluate('document.activeElement.blur()');
  await screenshot('project-reference-aligned');
  await check('Project preserves comparison denominator and unknown categories', "document.querySelector('#purpose').textContent.includes('USER-SELECTED COMPARISON REFERENCE') && document.querySelector('#hdd017-details').textContent.includes('12 destination-only files (not counted in the 1,000 reference denominator)') && document.querySelector('.offline-drive').textContent.includes('200') && document.querySelector('.offline-drive').textContent.includes('–')");
  await check('Exact category values and historical evidence dates', "[...document.querySelector('.matrix-panel tbody').rows[1].cells].slice(2).map(c=>c.textContent.replace(/[^0-9]/g,'')).join(',') === '994,4,2,0' && [...document.querySelector('.offline-drive').cells].slice(2).map(c=>c.textContent.trim()).join(',') === '✓ 800,–,–,200' && document.querySelector('.inspector').textContent.includes('Sep 12, 2025') && document.querySelector('.inspector').textContent.includes('Sep 15, 2025') && document.querySelector('.offline-note').textContent.includes('Aug 3, 2025')");
  await check('Offline count stays within its table and outside inspector', "document.querySelector('.offline-drive td:last-child').getBoundingClientRect().right <= document.querySelector('.matrix-panel').getBoundingClientRect().right && document.querySelector('.matrix-panel').getBoundingClientRect().right < document.querySelector('.inspector').getBoundingClientRect().left");
  await evaluate("document.querySelector('[aria-controls=hdd017-details]').click()");
  await check('HDD disclosure collapses', "document.querySelector('#hdd017-details').hidden");
  await evaluate("document.querySelector('[aria-controls=hdd017-details]').click();document.querySelector('.registration button').click()");
  await check('Registration is demo-only', "document.querySelector('#demo-message').textContent.includes('no operation or registration is performed')");
  await evaluate("document.querySelectorAll('.registration button')[1].click()");
  await check('Review source ambiguity explained and inspector focused', "document.querySelector('#demo-message').textContent.includes('4 absent + 2 differing') && document.activeElement === document.querySelector('.inspector h2')");
  await evaluate("{const s=document.querySelector('[aria-label=\"Search project files\"]');s.value='absent';s.dispatchEvent(new Event('input'));}");
  await check('Project file no-results', "document.querySelector('.inspector-body').hidden && !document.querySelector('.file-no-results').hidden");
  await evaluate("{const s=document.querySelector('[aria-label=\"Search project files\"]');s.value='IMG_4587';s.dispatchEvent(new Event('input'));} document.querySelector('.inspector-actions button').click()");
  await check('Inspector actions remain demo-only and do not remove evidence', "!document.querySelector('.inspector-body').hidden && document.querySelector('#demo-message').textContent.includes('Demo only') && document.querySelector('.filename').textContent === 'IMG_4587.CR2'");
  await evaluate("for(const b of document.querySelectorAll('.inspector-actions button')) b.click()");
  await check('All inspector actions leave static evidence intact', "document.querySelectorAll('.inspector .observed').length === 2 && document.querySelector('#demo-message').textContent.includes('Dismiss anomaly')");
  await evaluate("document.querySelector('#breadcrumb a').click()"); await evaluate('new Promise(resolve => setTimeout(resolve, 60))');
  await check('Project return path', "location.hash === '#library' && document.querySelector('h1').textContent === 'Library'");
  await evaluate("document.querySelector('.sample-browser').open=true; document.querySelector('.result').click()");
  await check('Original Library content inspector retained', "document.querySelectorAll('.occurrence').length === 2");
  await route('find');
  await check('Find retains three sample identities and unspecified visual notice', "document.querySelectorAll('.result').length === 3 && document.querySelector('#purpose').textContent.includes('not specified')");
  await evaluate("document.querySelector('.result').focus()");
  await cdp('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', text: '\r', unmodifiedText: '\r', windowsVirtualKeyCode: 13 });
  await cdp('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
  await check('Keyboard activates selection and retains focus', "document.activeElement.matches('.result[aria-pressed=true]') && document.querySelectorAll('.occurrence').length === 2");
  await evaluate("{ const s=document.querySelectorAll('.controls select')[0]; s.value='SIM-TAPE-B'; s.dispatchEvent(new Event('change')); }");
  await check('Rendered medium filter', "document.querySelectorAll('.result').length === 2");
  await evaluate("{ const s=document.querySelectorAll('.controls select')[2]; s.value='online'; s.dispatchEvent(new Event('change')); }");
  await check('Rendered combined filters have no matching online tape', "document.querySelectorAll('.result,.occurrence').length === 0");
  await evaluate("document.querySelector('.controls button').click()");
  await evaluate("document.querySelector('.result').click()");
  await check('Intentional occurrences, offline and verification evidence', "document.querySelectorAll('.occurrence').length === 2 && document.querySelector('#content').textContent.includes('Offline · not automatically lost') && document.querySelector('#content').textContent.includes('Not verified')");
  await screenshot('find-occurrences');
  await route('library');
  await route('find');
  await check('Navigation focuses heading', "document.activeElement === document.querySelector('h1') && document.querySelector('h1').textContent === 'Find'");
  await evaluate("const input=document.querySelector('input'); input.value='SIM-OCC-03'; input.dispatchEvent(new Event('input')); document.querySelector('.result').click()");
  await check('Find occurrence selects v1 with recovery planning', "document.querySelectorAll('.result').length === 1 && document.querySelector('[aria-label=\"Content details\"]').textContent.includes('v1') && document.querySelector('#content').textContent.includes('Recovery planning')");
  await screenshot('find-desktop');
  await evaluate("document.querySelector('input').value='no-such-synthetic-file'; document.querySelector('input').dispatchEvent(new Event('input'))");
  await check('No results clears stale detail', "document.querySelectorAll('.occurrence').length === 0 && document.querySelector('#content').textContent.includes('No results in this sample')");
  await evaluate("document.querySelector('.controls button').click(); document.querySelector('input').value='雪'; document.querySelector('input').dispatchEvent(new Event('input')); document.querySelector('.result').click()");
  await check('Filename markup rendered as text', "document.querySelector('#content').textContent.includes('Scene <draft> & \"雪\".png') && document.querySelectorAll('draft').length === 0");
  for (const scenario of ['empty', 'loading', 'error', 'partial']) {
    await evaluate(`{ const s=document.querySelectorAll('.controls select')[3]; s.value=${JSON.stringify(scenario)}; s.dispatchEvent(new Event('change')); }`);
    await check(scenario + ' presentation', scenario === 'partial' ? "document.querySelector('#results-status').textContent.includes('Partial information')" : "document.querySelectorAll('.result,.occurrence').length === 0");
    if (scenario === 'error') await screenshot('find-error-desktop');
  }
  await evaluate("document.querySelector('.controls button').click(); document.querySelector('.result').click()");
  for (const mode of ['Studio', 'Expert', 'Guided']) {
    await evaluate(`{ const s=document.querySelector('#disclosure'); s.value=${JSON.stringify(mode)}; s.dispatchEvent(new Event('change')); }`);
    await check(mode + ' preserves critical semantics', "document.querySelector('#content').textContent.includes('Offline · not automatically lost') && document.querySelector('#content').textContent.includes('Not verified')");
    await check(mode + ' disclosure', mode === 'Guided' ? "!document.querySelector('#content').textContent.includes('Synthetic hash (not a real checksum)')" : "document.querySelector('#content').textContent.includes('Synthetic hash (not a real checksum)')");
  }
  for (const slug of ['back-up', 'archive', 'activity', 'devices-&-locations', 'settings']) {
    await route(slug);
    await check(slug + ' navigable', "document.querySelector('a[aria-current=page]').textContent === document.querySelector('h1').textContent && document.querySelector('#content').textContent.length > 100");
    await check(slug + ' no enabled operation buttons', "[...document.querySelectorAll('#content button')].every(b=>b.disabled)");
    if (slug === 'activity') {
      await check('FAILED stays primary and historical recording distinct', "document.querySelector('.error strong').textContent.startsWith('FAILED · Unrecorded') && document.querySelector('#content').textContent.includes('historical only')");
      await screenshot('activity-desktop');
    }
  }
  await route('library');
  await cdp('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: false });
  await check('Narrow viewport has no horizontal page overflow', 'document.documentElement.scrollWidth <= innerWidth');
  await screenshot('library-narrow');
  const ax = await cdp('Accessibility.getFullAXTree');
  await writeFile(path.join(evidence, 'accessibility-tree.json'), JSON.stringify(ax, null, 2));
  assert.ok(ax.nodes.some(n => n.role?.value === 'searchbox' && n.name?.value === 'Search inventory'));
  checks.push('Browser accessibility tree exposes named searchbox');
  }
  assert.deepEqual(errors, []);
  assert.ok(requests.every(url => url.startsWith(base + '/') || url === base));
  assert.ok(!requests.some(url => url.includes('/api/')));
  checks.push('No page runtime exceptions or external/API requests observed');
  await writeFile(path.join(evidence, 'browser-stderr.txt'), stderr);
  completed = true;
} finally {
  socket?.close();
  if (child && child.exitCode === null) child.kill();
  if (stopped) await stopped;
  await new Promise(resolve => { server.close(resolve); server.closeAllConnections(); });
  const catalogProcess = await server.catalogStopped;
  await writeFile(path.join(evidence, 'catalog-process.json'), JSON.stringify({ catalog, adapter, expected, catalogProcess }, null, 2));
  clearTimeout(deadline);
  await writeFile(path.join(evidence, 'browser-results.json'), JSON.stringify({ completed, browserVersion, checks, requests, errors, browserPID: child?.pid, browserExit: child?.exitCode, browserSignal: child?.signalCode, browserStoppedAndWaited: Boolean(stopped), serverStopped: !server.listening, viewports: ['1440x1024 scale 2 (PDF comparison assumption)', '390x844 scale 1 (unspecified responsive smoke check)'] }, null, 2));
}
console.log(`${checks.length} browser checks passed; browser stopped/waited; server closed. Evidence: ${evidence}`);
