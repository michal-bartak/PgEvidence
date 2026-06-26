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
      description: 'Tamper-evident PostgreSQL audit evidence',
      plugins: [starlightThemeRapide()],
      favicon: '/appicon.png',
      logo: {
        src: './src/assets/appicon.png',
        alt: 'PgEvidence',
        replacesTitle: false,
      },
      social: { github: 'https://github.com/michal-bartak/PgEvidence' },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        { label: 'Home', link: '/' },
        { label: 'Installation', link: '/installation/' },
        { label: 'Usage', link: '/usage/' },
        { label: 'Troubleshooting', link: '/troubleshooting/' },
        { label: 'Building from source', link: '/building/' },
      ],
    }),
  ],
});
