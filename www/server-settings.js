// serverSettings(), the x-data of the settings page's Server and Account
// sections. Moved from app.js as it was (docs-fr/32, palier 5.1): a global
// function, which index.html calls by name.
// ---------------------------------------------------------------- server panel
//
// The Server section of the settings panel. Everything else in that panel is a
// browser preference kept in localStorage; this one is the server's own state,
// and the two were worth separating -- one follows you between machines, the
// other decides when data is deleted.
//
// Only two settings are writable while the server runs: the log level and the
// database retention. The rest is reported and not editable, because changing it
// means reopening the log file or the audio pipeline. A panel that silently
// fails to apply half of what it shows is worse than one that says which half.
function serverSettings() {
    return {
        state: null,
        draft: { log_level: 'info', db_retention_gb: 20, reference_airport: '', also_airports: [], matching: null },
        newAccount: null, // { username, password } while the first account is being typed
        busy: false,
        error: '',
        note: '',
        noteBad: false,

        async load() {
            this.busy = true;
            this.error = '';
            try {
                const r = await fetch('/api/v1/server', { credentials: 'same-origin' });
                if (r.status === 401) { this.error = 'Sign in to see the server state.'; return; }
                if (!r.ok) { this.error = 'Server state unavailable (' + r.status + ')'; return; }
                this.state = await r.json();
                this.draft = {
                    log_level: this.state.settings.log_level,
                    db_retention_gb: this.state.settings.db_retention_gb,
                    reference_airport: this.state.settings.reference_airport,
                    also_airports: (this.state.settings.also_airports || []).slice(),
                    // A deep copy: the sector sizes are edited in place by the inputs.
                    matching: this.state.settings.matching ? JSON.parse(JSON.stringify(this.state.settings.matching)) : null,
                };
            } catch (e) {
                this.error = 'Server state unreachable: ' + e.message;
            } finally {
                this.busy = false;
            }
        },

        async save() {
            // Retention decides when data is deleted, so the panel says what it
            // is about to do rather than doing it silently.
            const gb = Number(this.draft.db_retention_gb);
            if (!Number.isFinite(gb) || gb < 1 || gb > 1000) {
                this.flash('The size kept must be between 1 and 1000 GB.', true);
                return;
            }
            if (this.state && gb < this.state.settings.db_retention_gb) {
                // What the server will do: newest first, today's kept whatever
                // its size, and from the first day that does not fit, it and
                // every older one deleted.
                const files = this.state.storage.daily_files || [];
                const cap = gb * 1024 * 1024 * 1024;
                let total = files.filter(f => f.active).reduce((s, f) => s + f.bytes, 0);
                let doomed = 0, over = false;
                for (const f of files.filter(f => !f.active)) {
                    if (!over && total + f.bytes <= cap) { total += f.bytes; continue; }
                    over = true;
                    doomed++;
                }
                if (doomed > 0 && !confirm(
                    'Keeping ' + gb + ' GB will delete ' + doomed +
                    ' older database file(s) at the next hourly sweep. Continue?')) {
                    this.draft.db_retention_gb = this.state.settings.db_retention_gb;
                    return;
                }
            }

            this.busy = true;
            try {
                const r = await fetch('/api/v1/server/settings', {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        log_level: this.draft.log_level,
                        db_retention_gb: gb,
                        reference_airport: this.draft.reference_airport,
                        // The reference airport cannot also be a further one.
                        also_airports: this.draft.also_airports.filter(c => c !== this.draft.reference_airport),
                        matching: this.draft.matching,
                    }),
                });
                if (r.status === 401) { this.flash('Sign in to change server settings.', true); return; }
                if (!r.ok) {
                    this.flash((await r.text()).trim() || ('Refused (' + r.status + ')'), true);
                    // A refused airport must not stay selected as if it were in force,
                    // nor a refused matching rule stay ticked.
                    this.draft.reference_airport = this.state.settings.reference_airport;
                    this.draft.also_airports = (this.state.settings.also_airports || []).slice();
                    if (this.state.settings.matching) this.draft.matching = JSON.parse(JSON.stringify(this.state.settings.matching));
                    return;
                }
                const followed = s => [s.reference_airport].concat(s.also_airports || []).join('+');
                const airportChanged = followed(this.state.settings) !== followed(this.draft);
                this.state.settings = await r.json();
                this.draft.also_airports = (this.state.settings.also_airports || []).slice();
                this.flash('Saved.', false);
                if (airportChanged) {
                    // The map draws the reference airport's runway extensions from
                    // /station; redraw them now. Weather is fetched by the server in
                    // the background after the change, so read it a little later.
                    const atc = Alpine.store('atc');
                    if (atc && atc.fetchStationData) atc.fetchStationData();
                    if (atc && atc.fetchWeatherData) setTimeout(() => atc.fetchWeatherData(), 4000);
                }
                // Reread rather than trust: the figures downstream -- retention,
                // days until full -- move with what was just changed.
                await this.load();
            } catch (e) {
                this.flash('Could not save: ' + e.message, true);
            } finally {
                this.busy = false;
            }
        },

        accessLabel() {
            const a = this.state.access;
            if (a.mode === 'account') return 'Sign-in required · ' + a.accounts.join(', ');
            if (a.mode === 'local') return 'This machine only · no sign-in';
            return 'Not chosen yet';
        },

        accessNote() {
            const a = this.state.access;
            if (a.mode === 'account' && !a.local_allowed) {
                return 'This server can be reached from other machines (it is not bound to 127.0.0.1, ' +
                    'or a proxy is declared), so sign-in cannot be turned off.';
            }
            if (a.mode === 'account') return 'Switching keeps the account, unused, for when sign-in is required again.';
            if (a.accounts.length) return 'Kept, unused: ' + a.accounts.join(', ') + '. Requiring sign-in brings it back with its password.';
            return 'Requiring sign-in creates the first account.';
        },

        // Either way, and as often as wanted. Turning sign-in off asks first:
        // it opens the server to anyone who can reach it.
        async switchAccess(mode) {
            const a = this.state.access;
            if (mode === 'local') {
                if (!confirm('Anyone who can reach this server will use it without signing in. ' +
                    'The account ' + a.accounts.join(', ') + ' is kept, unused, and comes back if you ' +
                    'require sign-in again. Continue?')) return;
                return this.putAccess({ mode: 'local' });
            }
            if (!a.accounts.length) {
                this.newAccount = { username: '', password: '' };
                return;
            }
            if (!confirm('Sign-in will be required, with the account ' + a.accounts.join(', ') +
                ' and its existing password. Continue?')) return;
            return this.putAccess({ mode: 'account' });
        },

        async createAccount() {
            const { username, password } = this.newAccount;
            if (!username.trim()) { this.flash('Choose a user name.', true); return; }
            if (password.length < 10) { this.flash('The password must be at least 10 characters.', true); return; }
            return this.putAccess({ mode: 'account', username: username.trim(), password });
        },

        async putAccess(body) {
            this.busy = true;
            try {
                const r = await fetch('/api/v1/access', {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                });
                if (r.status === 401) { this.flash('Sign in to change access.', true); return; }
                if (!r.ok) { this.flash((await r.text()).trim() || ('Refused (' + r.status + ')'), true); return; }
                // What this browser may see has just changed. Reloading lets the
                // page start over: with the sign-in form if sign-in is now required.
                location.reload();
            } catch (e) {
                this.flash('Could not change access: ' + e.message, true);
            } finally {
                this.busy = false;
            }
        },

        // The airports offered as reference, nearest first -- plus the one in
        // force if it is not among them (set by hand farther than the range).
        airportChoices() {
            const ref = this.state && this.state.reference;
            if (!ref) return [];
            const list = (ref.candidates || []).slice();
            if (ref.airport && !list.some(c => c.code === ref.airport)) {
                list.unshift({ code: ref.airport, name: ref.name || '', distance_nm: ref.distance_nm || 0 });
            }
            return list;
        },

        // Airports followed besides the reference one (Q40): each aircraft is
        // judged against the one whose runway axis it flies, else the nearest.
        alsoChoices() {
            return this.airportChoices().filter(c => c.code !== this.draft.reference_airport);
        },
        alsoFollowed(code) {
            return this.draft.also_airports.includes(code);
        },
        toggleAlso(code) {
            const list = this.draft.also_airports.filter(c => c !== code);
            if (list.length === this.draft.also_airports.length) {
                if (list.length >= 3) { this.flash('Three further airports at most.', true); return; }
                list.push(code);
            }
            this.draft.also_airports = list;
            this.save();
        },

        flash(msg, bad) {
            this.note = msg;
            this.noteBad = bad;
            setTimeout(() => { this.note = ''; }, bad ? 8000 : 3000);
        },

        fmtBytes(n) {
            if (!n && n !== 0) return '—';
            const u = ['B', 'kB', 'MB', 'GB', 'TB'];
            let i = 0;
            while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
            return (i === 0 ? n : n.toFixed(1)) + ' ' + u[i];
        },

        fmtUptime(s) {
            if (!s && s !== 0) return '—';
            const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
            if (d) return d + 'd ' + h + 'h';
            if (h) return h + 'h ' + m + 'm';
            return m + 'm';
        },
    };
}
