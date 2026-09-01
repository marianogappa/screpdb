import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { applyFlagFontFallback } from './lib/flagFont'
import './styles.css'

applyFlagFontFallback()

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)

