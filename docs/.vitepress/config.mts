import { defineConfig } from 'vitepress'

const site = 'https://bobcli.dev'
const ogImage = `${site}/og.png`
const description =
  'Bob is a deterministic repository factory for agent-native developer tools. It plans before it writes, proves ownership of every file it touches, and detects drift in CI. Model-free, local, MIT.'

export default defineConfig({
  title: 'Bob',
  titleTemplate: ':title — Bob, the deterministic repository factory',
  description,
  lang: 'en-US',
  appearance: 'dark',
  cleanUrls: true,
  lastUpdated: true,

  sitemap: { hostname: site },

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#0e1116' }],
    ['meta', { name: 'author', content: 'Abdul Hamid Achik' }],
    [
      'meta',
      {
        name: 'keywords',
        content:
          'bob cli, repository factory, deterministic scaffolding, code generation, drift detection, MCP tools, agent-native, go cli generator, bob.yaml',
      },
    ],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'Bob' }],
    ['meta', { property: 'og:title', content: 'Bob — the deterministic repository factory' }],
    ['meta', { property: 'og:description', content: description }],
    ['meta', { property: 'og:url', content: site }],
    ['meta', { property: 'og:image', content: ogImage }],
    ['meta', { property: 'og:image:width', content: '1200' }],
    ['meta', { property: 'og:image:height', content: '630' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:title', content: 'Bob — the deterministic repository factory' }],
    ['meta', { name: 'twitter:description', content: description }],
    ['meta', { name: 'twitter:image', content: ogImage }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    [
      'link',
      {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Archivo+Black&family=Archivo:wght@400;700&family=JetBrains+Mono:wght@400;700&display=swap',
      },
    ],
    [
      'script',
      { type: 'application/ld+json' },
      JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: 'Bob',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'macOS, Linux',
        description,
        url: site,
        downloadUrl: 'https://github.com/abdul-hamid-achik/bob/releases',
        license: 'https://opensource.org/licenses/MIT',
        author: { '@type': 'Person', name: 'Abdul Hamid Achik' },
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
      }),
    ],
  ],

  transformPageData(pageData) {
    const canonical =
      site + '/' + pageData.relativePath.replace(/index\.md$/, '').replace(/\.md$/, '')
    pageData.frontmatter.head ??= []
    pageData.frontmatter.head.push(['link', { rel: 'canonical', href: canonical }])
  },

  themeConfig: {
    logo: '/favicon.svg',
    search: { provider: 'local' },
    nav: [
      { text: 'Get Started', link: '/getting-started' },
      { text: 'For Agents', link: '/agents' },
      { text: 'Studio', link: '/studio' },
      {
        text: 'Guides',
        items: [
          { text: 'Existing Repository', link: '/guides/existing-repository' },
          { text: 'Build any repository', link: '/guides/any-repository' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Manifest', link: '/reference/manifest' },
          { text: 'CLI', link: '/reference/cli' },
          { text: 'Configuration', link: '/configuration' },
        ],
      },
      { text: 'Architecture', link: '/architecture' },
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'Getting Started', link: '/getting-started' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Existing Repository', link: '/guides/existing-repository' },
          { text: 'Build any repository', link: '/guides/any-repository' },
        ],
      },
      {
        text: 'Core workflow',
        items: [
          { text: 'Ownership & Safety', link: '/ownership-and-safety' },
          { text: 'Configuration & Telemetry', link: '/configuration' },
          { text: 'Studio', link: '/studio' },
        ],
      },
      {
        text: 'Agent surface',
        items: [
          { text: 'Bob for Coding Agents', link: '/agents' },
          { text: 'MCPHub & local-agent', link: '/guides/mcphub-local-agent' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Manifest', link: '/reference/manifest' },
          { text: 'CLI', link: '/reference/cli' },
          { text: 'Development', link: '/development' },
        ],
      },
      {
        text: 'Design',
        items: [
          { text: 'Architecture', link: '/architecture' },
          { text: 'Product Direction', link: '/product-direction' },
          { text: 'ADR-0001: Repository factory', link: '/adr/0001-repository-factory' },
          { text: 'ADR-0002: Read-only MCP', link: '/adr/0002-read-only-mcp' },
          { text: 'ADR-0003: Local operator surfaces', link: '/adr/0003-local-operator-surfaces' },
        ],
      },
    ],
    outline: { level: [2, 3], label: 'On this page' },
    editLink: {
      pattern: 'https://github.com/abdul-hamid-achik/bob/edit/main/docs/:path',
      text: 'Improve this page on GitHub',
    },
    socialLinks: [{ icon: 'github', link: 'https://github.com/abdul-hamid-achik/bob' }],
    footer: {
      message: 'Deterministic plans. Explicit authority. Honest integration boundaries.',
      copyright: 'MIT Licensed © 2026 Abdul Hamid Achik',
    },
  },
})
