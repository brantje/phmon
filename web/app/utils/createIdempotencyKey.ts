export function createIdempotencyKey() {
  // randomUUID() is unavailable on the operator's explicitly supported HTTP LAN
  // origin. getRandomValues() remains cryptographically strong in insecure contexts.
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16))
  const token = Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('')
  return `web-${token}`
}
