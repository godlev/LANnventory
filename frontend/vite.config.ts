import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import solid from 'vite-plugin-solid'

const themeAssetPrefix = '/assets/themes/'
const bootswatchDist = fileURLToPath(new URL('./node_modules/aceberg-bootswatch-fork/dist/', import.meta.url))

function getThemeNames() {
  return readdirSync(bootswatchDist, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
}

function getThemeFromPath(pathname: string) {
  if (!pathname.startsWith(themeAssetPrefix) || !pathname.endsWith('/bootstrap.min.css')) {
    return ''
  }

  const theme = pathname.slice(themeAssetPrefix.length, -'/bootstrap.min.css'.length)
  return /^[a-z0-9-]+$/.test(theme) ? theme : ''
}

function getThemeCss(theme: string) {
  const cssPath = join(bootswatchDist, theme, 'bootstrap.min.css')
  return readFileSync(cssPath, 'utf8')
    .replace(/@import\s+url\((['"]?)https:\/\/fonts\.googleapis\.com[^)'"]+\1\)\s*;?/g, '')
    .replace(/@import\s+['"]https:\/\/fonts\.googleapis\.com[^'"]+['"]\s*;?/g, '')
    .replace(/\/\*# sourceMappingURL=bootstrap\.min\.css\.map\s*\*\//g, '')
}

function localBootswatchThemes(): Plugin {
  return {
    name: 'local-bootswatch-themes',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const pathname = (req.url ?? '').split('?')[0]
        const theme = getThemeFromPath(pathname)
        if (!theme) {
          next()
          return
        }

        const cssPath = join(bootswatchDist, theme, 'bootstrap.min.css')
        if (!existsSync(cssPath)) {
          next()
          return
        }

        res.statusCode = 200
        res.setHeader('Content-Type', 'text/css; charset=utf-8')
        res.end(getThemeCss(theme))
      })
    },
    generateBundle() {
      for (const theme of getThemeNames()) {
        this.emitFile({
          type: 'asset',
          fileName: `assets/themes/${theme}/bootstrap.min.css`,
          source: getThemeCss(theme),
        })
      }
    },
  }
}

export default defineConfig({
  plugins: [solid(), localBootswatchThemes()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8840',
      '/fs': 'http://127.0.0.1:8840',
    },
  },
  build: {
    rollupOptions: {
      output: {
        entryFileNames: `assets/[name].js`,
        chunkFileNames: `assets/[name].js`,
        assetFileNames: `assets/[name].[ext]`
      }
    }
  }
})
