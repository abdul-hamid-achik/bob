import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import { inject } from '@vercel/analytics'
import BobHome from './components/BobHome.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('BobHome', BobHome)
    if (typeof window !== 'undefined') {
      inject()
    }
  },
} satisfies Theme
