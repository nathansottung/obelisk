import test from 'node:test';
import assert from 'node:assert/strict';
import { createStopParser } from './stop-command.mjs';

test('Stop parser accepts complete commands across controlled chunk boundaries exactly once', () => {
  for (const chunks of [['stop\n'],['st','op\n'],['s','t','o','p','\n'],['stop\r','\n'],['\nunknown\nstop\nstop\n'],[' \tstop \r\n']]) {
    let calls=0; const parser=createStopParser(()=>calls++);
    for(const chunk of chunks) parser.write(Buffer.from(chunk));
    parser.end(); parser.write('stop\n'); assert.equal(calls,1,JSON.stringify(chunks));
  }
});
test('Incomplete prefixes, substrings and unknown/case-mismatched lines cannot stop', () => {
  let calls=0; const parser=createStopParser(()=>calls++);
  parser.write('st'); assert.equal(calls,0);
  parser.write('\nnotstop\nstoplater\nSTOP\n\n');assert.equal(calls,0);
  parser.write('stop\n');assert.equal(calls,1);
});
test('EOF accepts only a complete normalized final stop and never creates a bare-EOF stop', () => {
  for(const [input,expected] of [['',0],['st',0],['stoplater',0],['stop',1],[' stop\r',1]]) {
    let calls=0; const parser=createStopParser(()=>calls++);parser.write(input);parser.end();parser.end();assert.equal(calls,expected,input);
  }
});
test('Overlong commands discard the entire line and resume at a real delimiter', () => {
  let calls=0;const parser=createStopParser(()=>calls++);
  for(let i=0;i<20;i++)parser.write('x'.repeat(4096));
  parser.write('stop\n');assert.equal(calls,0);parser.write('stop\n');assert.equal(calls,1);
  calls=0;const boundary=createStopParser(()=>calls++);boundary.write(' '.repeat(4092)+'stop\n');assert.equal(calls,1);
});
test('Closing a parser releases pending input and ignores subsequent data/EOF', () => {
  let calls=0;const parser=createStopParser(()=>calls++);parser.write('st');parser.close();parser.write('op\n');parser.end();assert.equal(calls,0);
});
