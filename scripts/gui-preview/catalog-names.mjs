// Preview-only name representation. Ordinary search never interprets escapes.
export const validUnicode = value => typeof value === 'string' && value.isWellFormed();
export const decodeCatalogResponse = raw => JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(raw));
export function parseExactName(entry) {
  if (!validUnicode(entry) || entry.length > 24578) throw Error('Enter a bounded JSON string containing the exact name.');
  let value;
  try { value = JSON.parse(entry); } catch { throw Error('Enter one JSON string, including quotes; use \\n, \\r, \\t and \\\\ for controls and backslashes.'); }
  // JSON.parse preserves isolated UTF-16 units. Refuse before URL/TextEncoder
  // conversion can replace them; a legitimately encoded U+FFFD stays valid.
  if (!validUnicode(value) || new TextEncoder().encode(value).length > 4096) throw Error('Exact name must be valid Unicode, at most 4096 UTF-8 bytes.');
  return value;
}
export function displayName(value) {
  return /[\u0000-\u001f\u007f]|^\s|\s$/.test(value) ? 'JSON name: ' + JSON.stringify(value) : value;
}
