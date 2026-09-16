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

    async function check() {
        try {
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
