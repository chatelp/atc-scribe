#!/usr/bin/env node
// The rendered-text probe (contrast-probe.js) as a bookmarklet, so that it
// replays on the test instance without a copy-paste into the console (step 0
// of docs-fr/design/04-migration.md). Prints a javascript: URL: drag it to the
// bookmarks bar, or paste it in the address bar of the page; the result is
// copied to the clipboard as JSON and shown in an alert, the full object is
// also logged to the console.
//
//   node tools/design/probe-bookmarklet.mjs > /tmp/probe-bookmarklet.txt

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const probe = fs.readFileSync(path.join(here, 'contrast-probe.js'), 'utf8')
    .split('\n').filter((l) => !/^\s*\/\//.test(l)).join('\n');   // comments out, the URL stays short

const wrapped = `(()=>{const r=${probe.trim().replace(/;\s*$/, '')};const s=JSON.stringify(r,null,1);console.log(r);` +
    `if(navigator.clipboard&&navigator.clipboard.writeText){navigator.clipboard.writeText(s).catch(()=>{});}` +
    `alert('Clairance render probe\\n'+s.slice(0,1500)+(s.length>1500?'\\n…(the rest is in the console and the clipboard)':''));})();`;

process.stdout.write('javascript:' + encodeURIComponent(wrapped) + '\n');
