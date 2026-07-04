import starlight from '@astrojs/starlight';
import { defineConfig } from 'astro/config';
import starlightThemeRapide from 'starlight-theme-rapide';

// GitHub Pages: served at https://michal-bartak.github.io/PgEvidence/
// (base matches the repo name). Update both if the repo is renamed.
export default defineConfig({
  site: 'https://michal-bartak.github.io',
  base: '/PgEvidence',
  integrations: [
    starlight({
      title: 'PgEvidence',
      description: 'Screenshot, record, and checksum PostgreSQL query results for auditors',
      plugins: [starlightThemeRapide()],
      favicon: '/favicon.ico',
      logo: {
        src: './src/assets/appicon.png',
        alt: 'PgEvidence',
        replacesTitle: false,
      },
      social: { github: 'https://github.com/michal-bartak/PgEvidence' },
      customCss: ['./src/styles/custom.css'],
      // Click a screenshot in the docs body to view it full-size in a lightbox.
      // Runs on first load and after every Starlight client-side navigation.
      head: [
        {
          tag: 'script',
          content: `
            (function () {
              function overlay() {
                var el = document.getElementById('img-lightbox');
                if (el) return el;
                el = document.createElement('div');
                el.id = 'img-lightbox';
                el.className = 'img-lightbox';
                el.innerHTML = '<img alt="">';
                el.addEventListener('click', function () { el.classList.remove('open'); });
                document.addEventListener('keydown', function (e) {
                  if (e.key === 'Escape') el.classList.remove('open');
                });
                document.body.appendChild(el);
                return el;
              }
              function wire() {
                var imgs = document.querySelectorAll('.sl-markdown-content img');
                for (var i = 0; i < imgs.length; i++) {
                  (function (img) {
                    if (img.dataset.lightbox) return;
                    img.dataset.lightbox = '1';
                    img.addEventListener('click', function () {
                      var el = overlay();
                      var big = el.querySelector('img');
                      big.src = img.currentSrc || img.src;
                      big.alt = img.alt || '';
                      el.classList.add('open');
                    });
                  })(imgs[i]);
                }
              }
              document.addEventListener('DOMContentLoaded', wire);
              document.addEventListener('astro:page-load', wire);
            })();
          `,
        },
      ],
      sidebar: [
        { label: 'Home', link: '/' },
        { label: 'Installation', link: '/installation/' },
        { label: 'Usage', link: '/usage/' },
        { label: 'Troubleshooting', link: '/troubleshooting/' },
        { label: 'Building from source', link: '/building/' },
        { label: 'Credits', link: '/credits/' },
      ],
    }),
  ],
});
