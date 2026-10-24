import { useId, useRef, useState, type SubmitEvent } from 'react'
import { createTerminal, updateTerminal } from '../../api/terminals'
import type { Attribute, Terminal, TerminalRole } from '../../api/types'
import { useToast } from '../../hooks/useToast'
import { ATTRIBUTE_INFO } from '../../lib/attributes'
import { getErrorMessage } from '../../lib/errors'
import {
  TERMINAL_ROLE_INFO,
  TERMINAL_ROLES,
  defaultAtributos,
} from '../../lib/terminals'
import { validateTerminalName } from '../../lib/validation'
import { Button, Input, Modal, Select } from '../ui'

type TerminalFormProps = {
  /** Terminal que se edita; sin él, el formulario crea uno nuevo. */
  terminal?: Terminal
  onClose: () => void
  /** Al crear, `saved` incluye el token en claro: quien lo recibe debe mostrarlo una vez. */
  onSaved: (saved: Terminal, mode: 'created' | 'updated') => void
}

// Se monta solo mientras se crea o edita un terminal: cerrar lo desmonta.
export function TerminalForm({ terminal, onClose, onSaved }: TerminalFormProps) {
  const { toast } = useToast()
  const formId = useId()
  const editing = terminal !== undefined
  const [nombre, setNombre] = useState(terminal?.nombre ?? '')
  const [rol, setRol] = useState<TerminalRole>(terminal?.rol ?? 'comercio')
  const [atributos, setAtributos] = useState<Attribute[]>(
    terminal?.atributos ?? defaultAtributos('comercio'),
  )
  const [nameError, setNameError] = useState<string>()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string>()
  const submittingRef = useRef(false)
  const info = TERMINAL_ROLE_INFO[rol]

  function handleRoleChange(next: TerminalRole) {
    setRol(next)
    setAtributos(defaultAtributos(next))
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submittingRef.current) return
    const error = validateTerminalName(nombre)
    setNameError(error)
    if (error) return

    submittingRef.current = true
    setSaving(true)
    setSaveError(undefined)
    const input = { nombre: nombre.trim(), rol, atributos }
    try {
      const saved = editing
        ? await updateTerminal(terminal.id, input)
        : await createTerminal(input)
      onSaved(saved, editing ? 'updated' : 'created')
      toast({
        title: editing ? 'Terminal actualizado' : 'Terminal creado',
        description: `«${input.nombre}» quedó guardado.`,
        variant: 'ok',
      })
      onClose()
    } catch (failure) {
      // El modal sigue abierto y no se pierde lo escrito.
      const message = getErrorMessage(failure)
      setSaveError(message)
      toast({
        title: editing ? 'No se pudo actualizar el terminal' : 'No se pudo crear el terminal',
        description: message,
        variant: 'error',
      })
      submittingRef.current = false
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={editing ? 'Editar terminal' : 'Nuevo terminal'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancelar
          </Button>
          <Button type="submit" form={formId} loading={saving}>
            {saving ? 'Guardando…' : editing ? 'Guardar cambios' : 'Crear terminal'}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
        <Input
          label="Nombre del terminal"
          placeholder="Por ejemplo: Caja principal"
          autoComplete="off"
          value={nombre}
          onChange={(event) => setNombre(event.target.value)}
          error={nameError}
        />

        <Select
          label="Tipo de terminal"
          hint={info.description}
          value={rol}
          onChange={(event) => handleRoleChange(event.target.value as TerminalRole)}
        >
          {TERMINAL_ROLES.map((role) => (
            <option key={role} value={role}>
              {TERMINAL_ROLE_INFO[role].label}
            </option>
          ))}
        </Select>

        {info.choice ? (
          <Select
            label="Dato que pide el terminal"
            value={atributos[0]}
            onChange={(event) => setAtributos([event.target.value as Attribute])}
          >
            {info.atributos.map((attribute) => (
              <option key={attribute} value={attribute}>
                {ATTRIBUTE_INFO[attribute].nombre}
              </option>
            ))}
          </Select>
        ) : (
          <div className="flex flex-col gap-2">
            <p className="text-lg font-semibold text-neutral-900">Datos que pide el terminal</p>
            <ul className="list-disc pl-6 text-lg text-neutral-800">
              {info.atributos.map((attribute) => (
                <li key={attribute}>{ATTRIBUTE_INFO[attribute].nombre}</li>
              ))}
            </ul>
          </div>
        )}

        {info.warning && (
          <p
            role="note"
            className="rounded-xl border-2 border-amber-600 bg-amber-50 p-4 text-base font-semibold text-amber-900"
          >
            {info.warning}
          </p>
        )}

        {saveError && (
          <p role="alert" className="font-semibold text-denied-700">
            No se pudo guardar: {saveError}
          </p>
        )}
      </form>
    </Modal>
  )
}
