import { BrowserRouter, Route, Routes } from 'react-router'
import { AuthProvider } from './components/AuthProvider'
import { ProtectedRoute } from './components/ProtectedRoute'
import { ToastProvider } from './components/ui'
import { ROLES } from './lib/roles'
import AppLayout from './layouts/AppLayout'
import AuthLayout from './layouts/AuthLayout'
import HomePage from './pages/HomePage'
import IssuerPage from './pages/IssuerPage'
import LoginPage from './pages/LoginPage'
import MerchantPage from './pages/MerchantPage'
import NotFoundPage from './pages/NotFoundPage'
import RegisterPage from './pages/RegisterPage'
import UiKitPage from './pages/UiKitPage'
import VerifyPage from './pages/VerifyPage'
import WalletPage from './pages/WalletPage'

function App() {
  return (
    <ToastProvider>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route element={<ProtectedRoute />}>
              <Route element={<AppLayout />}>
                <Route element={<ProtectedRoute roles={[ROLES.CIUDADANO]} />}>
                  <Route index element={<HomePage />} />
                  <Route path="billetera" element={<WalletPage />} />
                </Route>
                <Route element={<ProtectedRoute roles={[ROLES.EMISOR]} />}>
                  <Route path="emisor" element={<IssuerPage />} />
                </Route>
                <Route element={<ProtectedRoute roles={[ROLES.COMERCIO]} />}>
                  <Route path="comercio" element={<MerchantPage />} />
                </Route>
                <Route path="verificar" element={<VerifyPage />} />
                <Route path="*" element={<NotFoundPage />} />
              </Route>
            </Route>
            <Route element={<AuthLayout />}>
              <Route path="login" element={<LoginPage />} />
              <Route path="registro" element={<RegisterPage />} />
            </Route>
            <Route path="/ui-kit" element={<UiKitPage />} />
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </ToastProvider>
  )
}

export default App
