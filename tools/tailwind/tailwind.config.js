// Tailwind, compiled (audit 4.3) instead of the Play CDN, which shipped a
// 400 KB compiler that scanned the page on every mutation. The theme is the one
// that was inline in index.html; the content list is every file that carries
// class names, Alpine's :class objects included. Build with ./build.sh.
module.exports = {
    darkMode: 'class',
    // Every script under www/ but the vendored libraries: a folder added later
    // (www/store/ on 12/10 went unlisted for a day, and eight classes of the
    // aircraft sheet went uncompiled) is scanned without anyone remembering to.
    content: [
        '../../www/index.html',
        '../../www/**/*.js',
        '!../../www/vendor/**',
    ],
    theme: {
        extend: {
            colors: {
                background: '#0a0a0a',
                panel: '#1a1a1a',
                text: '#e0e0e0',
                highlight: '#4CAF50',
                warning: '#FFC107',
                danger: '#F44336',
                border: '#333',
                hover: '#2a2a2a',
            },
            minWidth: {
                '72': '18rem',
                '96': '24rem',
                '104': '26rem',
                '120': '30rem',
            },
            width: {
                '104': '26rem',
                '120': '30rem',
            },
        },
    },
};
