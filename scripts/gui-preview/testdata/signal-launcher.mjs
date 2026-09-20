// Pipe/IPC test coordination only; exercises the real handler, not console Ctrl+C.
import { fileURLToPath } from 'node:url';
process.stdin.once('end', () => process.send({ eofObserved: true }));
process.once('message', message => {
  if (message !== 'interrupt') throw Error('Unexpected test coordination');
  process.disconnect(); process.emit('SIGINT');
});
process.argv[1] = fileURLToPath(new URL('../server.mjs', import.meta.url));
await import('../server.mjs');
