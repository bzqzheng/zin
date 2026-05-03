import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { DaemonConnectionProvider } from './daemon'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <DaemonConnectionProvider>
      <App />
    </DaemonConnectionProvider>
  </StrictMode>,
)
