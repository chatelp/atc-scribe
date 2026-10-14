// The server's configuration for the bench: a copy of
// configs/config.toml.example with what the bench needs changed, nothing else.
// No sign-in, the loopback only, the made-up receiver of the replay bench
// (45.0 / 4.67, never elsewhere), the generator as a tar1090 source, no
// frequency (so no transcription and no audio), no weather leaving the machine, the data
// and the log in the bench's own directory.
import fs from 'node:fs';

export function benchConfig(examplePath, { port, sourceUrl, dataDir, wwwDir }) {
    let toml = fs.readFileSync(examplePath, 'utf8');
    // Every uncommented [[frequencies.sources]] block goes, up to the next table.
    toml = toml.replace(/^\[\[frequencies\.sources\]\][\s\S]*?(?=^#?\s*\[)/gm, '');
    const set = (table, key, value) => {
        const re = new RegExp(`(^\\[${table.replace('.', '\\.')}\\][\\s\\S]*?^)#?\\s*${key}\\s*=.*$`, 'm');
        if (re.test(toml)) {
            toml = toml.replace(re, `$1${key} = ${value}`);
        } else {
            toml = toml.replace(new RegExp(`^\\[${table.replace('.', '\\.')}\\]\\s*$`, 'm'), `[${table}]\n${key} = ${value}`);
        }
    };
    set('server', 'port', port);
    set('server', 'host', '"127.0.0.1"');
    set('server', 'additional_ports', '[]');
    set('server', 'static_dir', JSON.stringify(wwwDir));
    set('auth', 'enabled', 'false');
    set('station', 'latitude', '45.0');
    set('station', 'longitude', '4.67');
    set('station', 'airport_code', '"LFLU"');
    set('adsb', 'source_type', '"tar1090"');
    set('adsb', 'tar1090_base_url', JSON.stringify(sourceUrl));
    set('transcription', 'log_dir', '""');
    // One weather type must stay on (config validation): it is fetched from
    // a dead loopback port, so nothing leaves the machine.
    set('wx', 'api_base_url', '"http://127.0.0.1:9/airports"');
    set('wx', 'fetch_taf', 'false');
    set('wx', 'fetch_notams', 'false');
    set('storage', 'sqlite_base_path', JSON.stringify(dataDir));
    set('logging', 'file', JSON.stringify(`${dataDir}/co-atc.log`));
    return toml;
}
