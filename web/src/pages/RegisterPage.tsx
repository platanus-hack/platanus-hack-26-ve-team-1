import { useState, type SubmitEvent } from 'react'
import { Link } from 'react-router'
import { Button, Input } from '../components/ui'
import { useAuth } from '../hooks/useAuth'
import { useToast } from '../hooks/useToast'
import { focusRing } from '../lib/cn'
import { getErrorMessage } from '../lib/errors'
import {
  MIN_PASSWORD_LENGTH,
  validateEmail,
  validateName,
  validatePassword,
} from '../lib/validation'

type FieldErrors = { nombre?: string; email?: string; password?: string }

export default function RegisterPage() {
  const { register } = useAuth()
  const { toast } = useToast()
  const [nombre, setNombre] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextErrors: FieldErrors = {
      nombre: validateName(nombre),
      email: validateEmail(email),
      password: validatePassword(password, { checkLength: true }),
    }
    setErrors(nextErrors)
    if (nextErrors.nombre || nextErrors.email || nextErrors.password) return

    setSubmitting(true)
    try {
      // AuthLayout redirige según el rol cuando el usuario queda en sesión.
      await register({ nombre: nombre.trim(), email: email.trim(), password })
    } catch (error) {
      toast({
        title: 'No se pudo crear la cuenta',
        description: getErrorMessage(error),
        variant: 'error',
      })
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <h1 className="text-3xl font-bold text-primary-900">Crear cuenta</h1>
      <Input
        label="Nombre"
        autoComplete="name"
        value={nombre}
        onChange={(event) => setNombre(event.target.value)}
        error={errors.nombre}
      />
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
        autoComplete="new-password"
        hint={`Mínimo ${MIN_PASSWORD_LENGTH} caracteres.`}
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        error={errors.password}
      />
      <Button type="submit" loading={submitting}>
        {submitting ? 'Creando cuenta…' : 'Crear cuenta'}
      </Button>
      <p className="text-lg text-neutral-700">
        ¿Ya tienes cuenta?{' '}
        <Link
          to="/login"
          className={`rounded font-semibold text-primary-700 underline ${focusRing}`}
        >
          Iniciar sesión
        </Link>
      </p>
    </form>
  )
}
