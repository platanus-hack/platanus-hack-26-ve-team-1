import { BrowserRouter, Route, Routes } from 'react-router'
import { ToastProvider } from './components/ui'
import UiKitPage from './pages/UiKitPage'

function App() {
  return (
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/ui-kit" element={<UiKitPage />} />
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  )
}

export default App
