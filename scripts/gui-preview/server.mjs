import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { startCatalog } from './catalog-adapter.mjs';
import { createStopParser } from './stop-command.mjs';

// Deliberately outside Go's embedded ui tree. No application imports or proxy.
const assets = new Map([
  ['/', ['index.html', 'text/html']],
  ['/app.mjs', ['app.mjs', 'text/javascript']],
  ['/fixtures.mjs', ['fixtures.mjs', 'text/javascript']],
  ['/style.css', ['style.css', 'text/css']],
  ['/catalog-ui.mjs', ['catalog-ui.mjs', 'text/javascript']],
  ['/catalog-protocol.mjs', ['catalog-protocol.mjs', 'text/javascript']],
  ['/catalog-names.mjs', ['catalog-names.mjs', 'text/javascript']],
]);
export async function startPreview(port = 0, catalogOptions = null) {
  const reader = catalogOptions ? await startCatalog(catalogOptions) : null;
  const contents = new Map(await Promise.all([...assets].map(async ([route, [file, type]]) =>
    [route, { body: await readFile(new URL(file, import.meta.url)), type }])));
  const server = http.createServer(async (req, res) => {
    res.setHeader('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'none'; img-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'");
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Cache-Control', 'no-store');
    if (reader) res.setHeader('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'");
    const expectedHost = `127.0.0.1:${server.address().port}`;
    if (req.headers.host !== expectedHost) { res.writeHead(403).end('Loopback host required'); return; }
    if (req.method !== 'GET' && req.method !== 'HEAD') { res.writeHead(405, { Allow: 'GET, HEAD' }).end(); return; }
    if (reader && (req.url === '/catalog-query' || req.url.startsWith('/catalog-query?'))) {
      if (req.method !== 'GET' || req.url.length > 16384) { res.writeHead(400).end(); return; }
      try { decodeURIComponent(req.url); } catch { res.writeHead(400).end(); return; }
      const params = new URL(req.url, 'http://127.0.0.1').searchParams;
      if ([...params.keys()].some(k => !['text', 'hash', 'exact'].includes(k)) || params.getAll('text').length > 1 || params.getAll('hash').length > 1 || params.getAll('exact').length > 1 || (params.get('text') ?? '').length > 256 || (params.get('hash') ?? '').length > 64 || (!params.has('exact') && req.url.length > 2048) || (params.has('exact') && (params.has('text') || params.has('hash') || Buffer.byteLength(params.get('exact'),'utf8') > 4096))) { res.writeHead(400).end(); return; }
      try { const result = await reader.query(params.get('text') ?? '', params.get('hash') ?? '', params.has('exact') ? params.get('exact') : undefined); res.writeHead(result.ok ? 200 : 422, { 'Content-Type': 'application/json' }).end(JSON.stringify(result)); }
      catch { res.writeHead(503, { 'Content-Type': 'application/json' }).end(JSON.stringify({ ok: false, error: 'Catalog query unavailable; no results presented' })); }
      return;
    }
    const asset = req.url === '/mode.mjs'
      ? { body: `export default ${JSON.stringify(reader?.mode ?? { enabled: false })};`, type: 'text/javascript' }
      : contents.get(req.url);
    if (!asset) { res.writeHead(404).end('Preview asset not found'); return; }
    res.writeHead(200, { 'Content-Type': `${asset.type}; charset=utf-8` });
    res.end(req.method === 'HEAD' ? undefined : asset.body);
  });
  server.catalogStopped = new Promise(resolve => server.once('close', async () => resolve(reader ? await reader.close() : null)));
  try { await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', resolve);
  }); } catch (error) { if (reader) await reader.close(); throw error; }
  return server;
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  if (![0, 1, 5].includes(args.length) || (args[0] && !/^\d+$/.test(args[0])) || (args.length === 5 && (args[1] !== '--catalog' || args[3] !== '--adapter'))) throw new Error('Usage: node scripts/gui-preview/server.mjs [port] [--catalog absolute-file --adapter absolute-exe]');
  const port = Number(args[0] ?? 0);
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error('Port must be 0..65535');
  const server = await startPreview(port, args.length === 5 ? { catalog: args[2], adapter: args[4] } : null);
  console.log(`Preview — synthetic data only: http://127.0.0.1:${server.address().port}/`);
  console.log('Press Ctrl+C or send stop on stdin to stop. Automatic shutdown after 60 minutes.');
  let stopping = false;
  const stop = async reason => {
    if (stopping) return; stopping = true;
    parser.close(); process.stdin.off('data', onData); process.stdin.off('end', onEnd);
    process.off('SIGINT', onInterrupt); process.off('SIGTERM', onTerminate);
    clearTimeout(timer); server.close(); server.closeAllConnections();
    const readerExit = await server.catalogStopped;
    console.log('Preview stopped (' + reason + '); catalog reader waited: ' + JSON.stringify(readerExit));
    process.stdin.destroy();
  };
  const parser = createStopParser(() => stop('stdin stop'));
  const onData = data => parser.write(data);
  // A final unterminated stop line is accepted. Bare EOF keeps the preview alive.
  const onEnd = () => { parser.end(); process.stdin.off('data', onData); process.stdin.off('end', onEnd); };
  const onInterrupt = () => stop('SIGINT'), onTerminate = () => stop('SIGTERM');
  const timer = setTimeout(() => stop('timeout'), 60 * 60 * 1000);
  process.once('SIGINT', onInterrupt);
  process.once('SIGTERM', onTerminate);
  process.stdin.on('data', onData); process.stdin.once('end', onEnd);
}
