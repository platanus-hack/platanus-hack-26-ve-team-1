import { useState, type SubmitEvent } from 'react'
import { Link } from 'react-router'
import { Button, Input } from '../components/ui'
import { useAuth } from '../hooks/useAuth'
import { useToast } from '../hooks/useToast'
import { focusRing } from '../lib/cn'
import { getErrorMessage } from '../lib/errors'
import { validateEmail, validatePassword } from '../lib/validation'

type FieldErrors = { email?: string; password?: string }

export default function LoginPage() {
  const { login } = useAuth()
  const { toast } = useToast()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextErrors: FieldErrors = {
      email: validateEmail(email),
      password: validatePassword(password),
    }
    setErrors(nextErrors)
    if (nextErrors.email || nextErrors.password) return

    setSubmitting(true)
    try {
      // AuthLayout redirige según el rol cuando el usuario queda en sesión.
      await login({ email: email.trim(), password })
    } catch (error) {
      toast({
        title: 'No se pudo iniciar sesión',
        description: getErrorMessage(error),
        variant: 'error',
      })
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <h1 className="text-3xl font-bold text-primary-900">Iniciar sesión</h1>
      <Input
        label="Correo"
        type="email"
        autoComplete="email"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        error={errors.email}
      />
      <Input
        label="Contraseña"
        type="password"
        autoComplete="current-password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        error={errors.password}
      />
      <Button type="submit" loading={submitting}>
        {submitting ? 'Ingresando…' : 'Ingresar'}
      </Button>
      <p className="text-lg text-neutral-700">
        ¿No tienes cuenta?{' '}
        <Link
          to="/registro"
          className={`rounded font-semibold text-primary-700 underline ${focusRing}`}
        >
          Crear cuenta
        </Link>
      </p>
    </form>
  )
}
