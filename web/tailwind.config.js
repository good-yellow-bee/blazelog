/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./internal/web/templates/**/*.templ",
    "./web/static/js/**/*.js",
  ],
  theme: {
    extend: {
      colors: {
        primary: '#0ea5a4',
        danger: '#f43f5e',
        success: '#10b981',
        warning: '#f59e0b',
      },
      fontFamily: {
        sans: ['IBM Plex Sans', 'system-ui', 'sans-serif'],
        display: ['Space Grotesk', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
}
