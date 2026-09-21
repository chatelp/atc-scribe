/*
 * Sign-in overlay.
 *
 * Kept in a file of its own, loaded before everything else, and written without
 * Alpine on purpose: it has to work before the application boots, and it must not
 * depend on anything the application needs a session to fetch. It also keeps the
 * fork's footprint on index.html to a single script tag.
 *
 * When authentication is disabled -- the default of the published configuration --
 * this does nothing at all.
 */
(function () {
    'use strict';

    const ENDPOINT = '/api/v1/auth';
    const SETUP = '/api/v1/setup';

    function overlay() {
        const el = document.createElement('div');
        el.id = 'coatc-login';
        el.innerHTML = `
          <style>
            #coatc-login {
              position: fixed; inset: 0; z-index: 99999;
              display: flex; align-items: center; justify-content: center;
              background: #0b0f0d; color: #d6e4dc;
              font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
            }
            #coatc-login form {
              width: min(22rem, calc(100vw - 2rem));
              border: 1px solid #1f2d26; border-radius: .5rem;
              padding: 1.5rem; background: #0f1512;
            }
            #coatc-login h1 { font-size: 1rem; margin: 0 0 .25rem; color: #4ade80; }
            #coatc-login p  { font-size: .75rem; margin: 0 0 1.25rem; color: #6b7d74; }
            #coatc-login label { display: block; font-size: .7rem; text-transform: uppercase;
              letter-spacing: .08em; color: #6b7d74; margin-bottom: .35rem; }
            #coatc-login .choice { margin-bottom: 1rem; }
            #coatc-login .choice label {
              display: flex; align-items: flex-start; gap: .5rem;
              text-transform: none; letter-spacing: normal;
              color: #d6e4dc; font-size: .8rem; line-height: 1.45;
              border: 1px solid #1f2d26; border-radius: .25rem;
              padding: .6rem .7rem; margin-bottom: .5rem; cursor: pointer;
            }
            #coatc-login .choice label:has(input:checked) { border-color: #4ade80; }
            #coatc-login .choice b { color: #4ade80; font-weight: 600; }
            #coatc-login .choice span { display: block; color: #6b7d74; font-size: .7rem; margin-top: .25rem; }
            #coatc-login .choice input { width: auto; margin: .2rem 0 0; flex: none; }
            #coatc-login input {
              width: 100%; box-sizing: border-box; margin-bottom: 1rem; padding: .5rem .6rem;
              background: #0b0f0d; border: 1px solid #1f2d26; border-radius: .25rem;
              color: #d6e4dc; font: inherit; font-size: .85rem;
            }
            #coatc-login input:focus { outline: none; border-color: #4ade80; }
            #coatc-login button {
              width: 100%; padding: .55rem; cursor: pointer; font: inherit; font-size: .85rem;
              background: #4ade80; border: 0; border-radius: .25rem; color: #04160c; font-weight: 600;
            }
            #coatc-login button[disabled] { opacity: .5; cursor: default; }
            #coatc-login .err { font-size: .75rem; color: #f87171; min-height: 1.1rem; margin: .5rem 0 0; }
          </style>
          <form autocomplete="on">
            <h1>Co-ATC</h1>
            <p>This receiver requires a sign-in.</p>
            <label for="coatc-user">User</label>
            <input id="coatc-user" name="username" autocomplete="username" autofocus required>
            <label for="coatc-pass">Password</label>
            <input id="coatc-pass" name="password" type="password" autocomplete="current-password" required>
            <button type="submit">Sign in</button>
            <p class="err" role="alert"></p>
          </form>`;
        return el;
    }

    function show() {
        const el = overlay();
        document.body.appendChild(el);
        const form = el.querySelector('form');
        const err = el.querySelector('.err');
        const button = el.querySelector('button');

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            err.textContent = '';
            button.disabled = true;
            try {
                const res = await fetch(`${ENDPOINT}/login`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: el.querySelector('#coatc-user').value,
                        password: el.querySelector('#coatc-pass').value,
                    }),
                });
                if (res.ok) {
                    // Reload rather than tear the overlay down: everything the page
                    // fetched while signed out was a 401, so the cleanest state is a
                    // fresh one.
                    location.reload();
                    return;
                }
                // The server does not say which of the name or the password was
                // wrong, and neither does this.
                err.textContent = res.status === 429
                    ? 'Too many attempts. Wait a few minutes.'
                    : 'Sign-in failed.';
            } catch (e) {
                err.textContent = 'Server unreachable.';
            }
            button.disabled = false;
            el.querySelector('#coatc-pass').select();
        });
    }

    /*
     * First run. Upstream ships with no authentication and a README saying never
     * to expose it; this asks the question instead of leaving it to whoever reads
     * the TOML.
     *
     * No setup token: the server refuses to start unconfigured unless it is bound
     * to loopback, so reaching this page already means being on the machine --
     * the same trust boundary as the terminal where you would run -add-user.
     */
    function setupOverlay(state) {
        const el = overlay();
        el.querySelector('form').innerHTML = `
          <h1>Set up co-atc</h1>
          <p>No account exists yet. Choose how this server should be reached.</p>

          <div class="choice">
            <label><input type="radio" name="coatc-mode" value="local" checked><div>
              <b>This machine only</b>
              <span>No sign-in. The server stays on ${state.host}, reachable from
              nowhere else. This is what upstream does, chosen rather than inherited.</span>
            </div></label>
            <label><input type="radio" name="coatc-mode" value="account"><div>
              <b>Reachable, with an account</b>
              <span>Creates a sign-in. You will still need TLS, or a trusted proxy
              declared in the configuration, before a password is safe to send over
              a network you do not control.</span>
            </div></label>
          </div>

          <div id="coatc-account" hidden>
            <label for="coatc-user">User name</label>
            <input id="coatc-user" autocomplete="username" autocapitalize="none">
            <label for="coatc-pass">Password (10 characters or more)</label>
            <input id="coatc-pass" type="password" autocomplete="new-password">
          </div>

          <p class="err"></p>
          <button type="submit">Continue</button>
        `;
        document.body.appendChild(el);

        const form = el.querySelector('form');
        const err = el.querySelector('.err');
        const button = el.querySelector('button');
        const account = el.querySelector('#coatc-account');
        const modeOf = () => el.querySelector('input[name=coatc-mode]:checked').value;

        el.querySelectorAll('input[name=coatc-mode]').forEach((r) =>
            r.addEventListener('change', () => { account.hidden = modeOf() !== 'account'; }));

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            err.textContent = '';
            button.disabled = true;
            const mode = modeOf();
            try {
                const res = await fetch(SETUP, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        mode,
                        username: el.querySelector('#coatc-user').value,
                        password: el.querySelector('#coatc-pass').value,
                    }),
                });
                if (res.ok) { location.reload(); return; }
                err.textContent = (await res.text()).trim() || 'Setup failed.';
            } catch (e) {
                err.textContent = 'Server unreachable.';
            }
            button.disabled = false;
        });
    }

    async function check() {
        try {
            const setup = await fetch(`${SETUP}/status`);
            if (setup.ok) {
                const st = await setup.json();
                if (st.needed) { setupOverlay(st); return; }
            }
            const res = await fetch(`${ENDPOINT}/status`);
            if (!res.ok) return;
            const s = await res.json();
            if (s.enabled && !s.authenticated) show();
        } catch (e) {
            // Unreachable server: let the application show its own failure rather
            // than covering it with a login form that cannot work either.
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', check);
    } else {
        check();
    }
})();
