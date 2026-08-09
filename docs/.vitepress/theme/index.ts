import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'

// LLMs plugin
import CopyOrDownloadAsMarkdownButtons from 'vitepress-plugin-llms/vitepress-components/CopyOrDownloadAsMarkdownButtons.vue'

// Tabs plugin
import { enhanceAppWithTabs } from 'vitepress-plugin-tabs/client'

// PlantUML plugin
import { enhanceAppWithPlantuml } from 'vitepress-plugin-plantuml/client'

// Video plugin
import { enhanceAppWithVideo } from 'vitepress-plugin-video/client'

// PDF plugin
import { enhanceAppWithPDF } from 'vitepress-plugin-pdf/client'

// QRCode plugin
import { enhanceAppWithQrcode } from 'vitepress-plugin-qrcode/client'

// Steps plugin
import 'vitepress-plugin-steps/style.css'

// Collapse plugin
import { enhanceAppWithCollapse } from 'vitepress-plugin-collapse/client'

// Mark plugin
import { enhanceAppWithMark } from 'vitepress-plugin-mark/client'

export default {
  extends: DefaultTheme,
  enhanceApp(ctx) {
    const { app } = ctx

    // Register LLMs copy/download markdown buttons
    app.component('CopyOrDownloadAsMarkdownButtons', CopyOrDownloadAsMarkdownButtons)

    // Register Tabs plugin
    enhanceAppWithTabs(app)

    // Register PlantUML plugin
    enhanceAppWithPlantuml(ctx)

    // Register Video plugin
    enhanceAppWithVideo(ctx)

    // Register PDF plugin
    enhanceAppWithPDF(ctx)

    // Register QRCode plugin
    enhanceAppWithQrcode(ctx)

    // Register Collapse plugin
    enhanceAppWithCollapse(ctx)

    // Register Mark plugin
    enhanceAppWithMark(ctx)
  }
} satisfies Theme
