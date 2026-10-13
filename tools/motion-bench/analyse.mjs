#!/usr/bin/env node
// The metrics of a bench run measured again from its raw poses, without
// running the browser: node tools/motion-bench/analyse.mjs OUT/raw-<label>-<scenario>.json
import fs from 'node:fs';
import path from 'node:path';
import { analyse } from './metrics.mjs';

for (const file of process.argv.slice(2)) {
    const { data, truth, t0, t1 } = JSON.parse(fs.readFileSync(file, 'utf8'));
    const scenario = path.basename(file).replace(/\.json$/, '').split('-').pop();
    const m = analyse(data, truth, { from: t0, to: t1, scenario });
    console.log(path.basename(file), JSON.stringify(m.summary));
    console.log('  by kind', JSON.stringify(m.extra.byKind));
    for (const k of ['afterShown', 'longGap', 'worst']) if (m.extra[k]) console.log(' ', k, JSON.stringify(m.extra[k]));
    if (m.extra.bySecond) console.log('  by second', m.extra.bySecond.slice(0, 15).map((x) => x.join('/')).join(' '));
}
