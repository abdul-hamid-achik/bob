import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Bob',
  titleTemplate: ':title — Bob',
  description: 'Deterministic repository construction for agent-native developer tools.',
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: true,

  themeConfig: {
    search: { provider: 'local' },
    nav: [
      { text: 'Get Started', link: '/getting-started' },
      { text: 'Studio', link: '/studio' },
      { text: 'Guides', link: '/guides/existing-repository' },
      {
        text: 'Reference',
        items: [
          { text: 'Manifest', link: '/reference/manifest' },
          { text: 'CLI', link: '/reference/cli' },
          { text: 'Configuration', link: '/configuration' },
        ],
      },
      { text: 'Architecture', link: '/architecture' },
      { text: 'GitHub', link: 'https://github.com/abdul-hamid-achik/bob' },
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
        text: 'Core workflow',
        items: [
          { text: 'Existing Repository', link: '/guides/existing-repository' },
          { text: 'Ownership & Safety', link: '/ownership-and-safety' },
          { text: 'Configuration & Telemetry', link: '/configuration' },
          { text: 'Studio', link: '/studio' },
        ],
      },
      {
        text: 'Agent surface',
        items: [
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
    socialLinks: [
      { icon: 'github', link: 'https://github.com/abdul-hamid-achik/bob' },
    ],
    footer: {
      message: 'Deterministic plans. Explicit authority. Honest integration boundaries.',
      copyright: 'MIT Licensed © 2026 Abdul Hamid Achik',
    },
  },
})
