#!/bin/sh
# Compile the page's Tailwind classes into www/vendor/tailwind.css.
# Run it after changing a class in www/ (index.html or a script): a class the
# compiled file lacks renders unstyled. The first line of the output carries a
# fingerprint of the sources it was built from; tools/www-check/check.mjs
# recomputes it and says when the file is out of date.
#
# Needs node; npx fetches tailwindcss 3.4 into npm's cache the first time.
set -e
here=$(cd "$(dirname "$0")" && pwd)
cd "$here"
out=../../www/vendor/tailwind.css
npx -y tailwindcss@3.4.17 -c tailwind.config.js -i input.css -o "$out" --minify
fp=$(node -e "const {createHash}=require('node:crypto');const fs=require('node:fs');const p=require('node:path');const www=p.resolve('../../www');const walk=(d)=>fs.readdirSync(d,{withFileTypes:true}).flatMap(e=>e.isDirectory()?walk(p.join(d,e.name)):[p.join(d,e.name)]);const files=[p.join(www,'index.html'),...walk(www).filter(f=>f.endsWith('.js')&&!f.includes('/vendor/'))].sort();const h=createHash('sha256');for(const f of files){h.update(p.relative(www,f)+'\0');h.update(fs.readFileSync(f));}console.log(h.digest('hex').slice(0,16))")
{ printf '/* sources %s */\n' "$fp"; cat "$out"; } > "$out.tmp" && mv "$out.tmp" "$out"
ls -la "$out"
