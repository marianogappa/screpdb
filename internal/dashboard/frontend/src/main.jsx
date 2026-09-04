import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { applyFlagFontFallback } from './lib/flagFont'
import { LocaleProvider } from './lib/i18nContext'
import './styles.css'

applyFlagFontFallback()

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <LocaleProvider>
      <App />
    </LocaleProvider>
  </React.StrictMode>,
)

