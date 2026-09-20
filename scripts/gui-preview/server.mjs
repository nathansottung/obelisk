import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

// Deliberately outside Go's embedded ui tree. No application imports or proxy.
const assets = new Map([
  ['/', ['index.html', 'text/html']],
  ['/app.mjs', ['app.mjs', 'text/javascript']],
  ['/fixtures.mjs', ['fixtures.mjs', 'text/javascript']],
  ['/style.css', ['style.css', 'text/css']],
]);
export async function startPreview(port = 0) {
  const contents = new Map(await Promise.all([...assets].map(async ([route, [file, type]]) =>
    [route, { body: await readFile(new URL(file, import.meta.url)), type }])));
  const server = http.createServer((req, res) => {
    res.setHeader('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'none'; img-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'");
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Cache-Control', 'no-store');
    const expectedHost = `127.0.0.1:${server.address().port}`;
    if (req.headers.host !== expectedHost) { res.writeHead(403).end('Loopback host required'); return; }
    if (req.method !== 'GET' && req.method !== 'HEAD') { res.writeHead(405, { Allow: 'GET, HEAD' }).end(); return; }
    const asset = contents.get(req.url);
    if (!asset) { res.writeHead(404).end('Preview asset not found'); return; }
    res.writeHead(200, { 'Content-Type': `${asset.type}; charset=utf-8` });
    res.end(req.method === 'HEAD' ? undefined : asset.body);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', resolve);
  });
  return server;
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  if (args.length > 1 || (args[0] && !/^\d+$/.test(args[0]))) throw new Error('Usage: node scripts/gui-preview/server.mjs [port; default 0]');
  const port = Number(args[0] ?? 0);
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error('Port must be 0..65535');
  const server = await startPreview(port);
  console.log(`Preview — synthetic data only: http://127.0.0.1:${server.address().port}/`);
  console.log('Press Ctrl+C to stop. Automatic shutdown after 60 minutes.');
  const stop = () => { clearTimeout(timer); server.close(); server.closeAllConnections(); };
  const timer = setTimeout(stop, 60 * 60 * 1000);
  process.once('SIGINT', stop);
  process.once('SIGTERM', stop);
}
