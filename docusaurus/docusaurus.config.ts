import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Kootenai Platform',
  tagline: 'Cybersecurity & Network Operations Education',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://toddbartholow.github.io',
  // Served from the repository's own GitHub Pages site, so the path is
  // /<repo>/ -- see .github/workflows/docusaurus.yml, which publishes to the
  // root of gh-pages. A fork under a different name must change this to match.
  baseUrl: '/kootenai/',

  organizationName: 'toddbartholow',
  projectName: 'kootenai',

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/toddbartholow/kootenai/tree/main/docusaurus/',
        },
        blog: false, // Disable blog
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  markdown: {
    mermaid: true,
  },
  themes: ['@docusaurus/theme-mermaid'],

  themeConfig: {
    image: 'img/social-card.svg',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Kootenai',
      logo: {
        alt: 'Kootenai Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'guideSidebar',
          position: 'left',
          label: 'Guides',
        },
        {
          type: 'docSidebar',
          sidebarId: 'apiSidebar',
          position: 'left',
          label: 'API',
        },
        {
          type: 'docSidebar',
          sidebarId: 'adminSidebar',
          position: 'left',
          label: 'Admin',
        },
        {
          href: 'https://github.com/toddbartholow/kootenai',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started',
            },
            {
              label: 'User Guide',
              to: '/docs/guides/overview',
            },
            {
              label: 'API Reference',
              to: '/docs/api/reference',
            },
          ],
        },
        {
          title: 'Project',
          items: [
            {
              label: 'GitHub (archived)',
              href: 'https://github.com/toddbartholow/kootenai',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Kootenai Project. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'go', 'sql', 'typescript', 'json'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
