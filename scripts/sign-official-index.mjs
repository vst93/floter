#!/usr/bin/env node
// Sign the official extension index for GitHub-hosted distribution.
//
// The index lives in two files under extensions/official-index/:
//   - payload.json  the human-editable index (what you maintain)
//   - index.json    the signed envelope fetched by Floter at runtime
//                   ({ payload: <base64>, signatures: [{ keyId, algorithm, signature }] })
//
// Editing payload.json requires re-signing index.json with the development
// (or production) root private key, otherwise Floter rejects the index as
// tampered. The private key never lives in this repository.
//
// Usage:
//   FLOTER_INDEX_KEY="ed25519:<base64 32-byte secret key>" node scripts/sign-official-index.mjs
//
// Generate a fresh key pair with:
//   node -e "const c=require('crypto');const k=c.generateKeyPairSync('ed25519');\
//     console.log('secret:', 'ed25519:'+k.privateKey.export({type:'pkcs8',format:'der'}).toString('base64'));\
//     console.log('public:', 'ed25519:'+k.publicKey.export({type:'spki',format:'der'}).subarray(12).toString('base64'))"
//
// The public key must be pinned in src-tauri/src/extensions/official_index.rs
// (DEVELOPMENT_ROOT_PUBLIC_KEY) before clients will trust the index.

import { readFileSync, writeFileSync } from "node:fs";
import { createPrivateKey, sign as cryptoSign } from "node:crypto";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const payloadPath = join(root, "extensions", "official-index", "payload.json");
const envelopePath = join(root, "extensions", "official-index", "index.json");

const keyToken = process.env.FLOTER_INDEX_KEY;
if (!keyToken) {
  console.error("FLOTER_INDEX_KEY is required (ed25519:<base64>).");
  process.exit(1);
}
if (!keyToken.startsWith("ed25519:")) {
  console.error("FLOTER_INDEX_KEY must use the ed25519: prefix.");
  process.exit(1);
}

const payload = readFileSync(payloadPath, "utf8");
const key = createPrivateKey({
  key: Buffer.from(keyToken.slice("ed25519:".length), "base64"),
  format: "der",
  type: "pkcs8",
});
const signature = cryptoSign(null, Buffer.from(payload, "utf8"), key);

const envelope = {
  payload: Buffer.from(payload, "utf8").toString("base64"),
  signatures: [
    {
      keyId: "development-root-1",
      algorithm: "ed25519",
      signature: signature.toString("base64"),
    },
  ],
};

writeFileSync(envelopePath, JSON.stringify(envelope, null, 2) + "\n");
console.log("signed " + envelopePath);