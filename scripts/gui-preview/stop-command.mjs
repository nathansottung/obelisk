// LF-delimited UTF-8 commands, optional CR, maximum 4096 bytes per line.
// Reject the WHOLE overlong line, including any suffix that resembles a command.
export function createStopParser(onStop) {
  const buffer = Buffer.alloc(4096);
  let length = 0, rejected = false, closed = false;
  const line = () => {
    const stop = !rejected && buffer.subarray(0, length).toString('utf8').trim() === 'stop';
    length = 0; rejected = false;
    if (stop && !closed) { closed = true; onStop(); }
  };
  return {
    write(chunk) {
      if (closed) return;
      for (const byte of Buffer.from(chunk)) {
        if (closed) break;
        if (byte === 10) line();
        else if (!rejected) {
          if (length === buffer.length) { rejected = true; length = 0; }
          else buffer[length++] = byte;
        }
      }
    },
    end() { if (!closed && (length || rejected)) line(); closed = true; length = 0; },
    close() { closed = true; length = 0; },
  };
}
