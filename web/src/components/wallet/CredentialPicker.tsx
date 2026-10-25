import { ISSUER_NAME } from '../../lib/slj/tables.ts'
import type { WalletCredential } from '../../lib/slj/wallet.ts'
import { cn, focusRing } from '../../lib/cn'

type CredentialPickerProps = {
  creds: WalletCredential[]
  selected: number
  onSelect: (index: number) => void
}

/** El ciudadano toca la credencial que quiere mostrar; solo se presenta esa. */
export function CredentialPicker({ creds, selected, onSelect }: CredentialPickerProps) {
  return (
    <ul className="flex flex-col gap-3" aria-label="Credenciales de esta billetera">
      {creds.map((cred, index) => {
        const active = index === selected
        return (
          <li key={`${cred.issuer}-${cred.attr}`}>
            <button
              type="button"
              aria-pressed={active}
              onClick={() => onSelect(index)}
              className={cn(
                'flex min-h-14 w-full items-center justify-between gap-4 rounded-xl border-2 px-5 py-3 text-left transition-colors',
                active
                  ? 'border-primary-700 bg-primary-50'
                  : 'border-neutral-300 bg-white hover:border-primary-400',
                focusRing,
              )}
            >
              <span className="flex flex-col">
                <span className="text-lg font-semibold text-neutral-900">{cred.label}</span>
                <span className="text-base text-neutral-600">
                  {ISSUER_NAME[cred.issuer] ?? `Emisor ${cred.issuer}`}
                </span>
              </span>
              <span className="text-base font-semibold text-primary-800">
                {active ? 'Mostrando' : 'Mostrar'}
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}
