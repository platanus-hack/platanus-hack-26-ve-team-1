import type { Attribute } from '../api/types'

const stroke = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.8,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
} as const

function AgeIcon({ label }: { label: string }) {
  return (
    <>
      <circle cx="12" cy="12" r="9.5" {...stroke} />
      <text
        x="12"
        y="15.2"
        textAnchor="middle"
        fontSize="9"
        fontWeight="700"
        fill="currentColor"
      >
        {label}
      </text>
    </>
  )
}

function Glyph({ attribute }: { attribute: Attribute }) {
  switch (attribute) {
    case 'mayor_de_18':
      return <AgeIcon label="18+" />
    case 'mayor_de_21':
      return <AgeIcon label="21+" />
    case 'identidad':
      return (
        <>
          <rect x="2.5" y="5" width="19" height="14" rx="2.5" {...stroke} />
          <circle cx="8.5" cy="11" r="2" {...stroke} />
          <path d="M5.5 16c.6-1.6 1.7-2.3 3-2.3s2.4.7 3 2.3" {...stroke} />
          <path d="M14.5 10h4M14.5 14h3" {...stroke} />
        </>
      )
    case 'sangre_alergias':
      return <path d="M12 3s6 6.5 6 11a6 6 0 0 1-12 0c0-4.5 6-11 6-11Z" {...stroke} />
    case 'contacto_emergencia':
      return (
        <path
          d="M5 4h4l2 5-2.5 1.5a11 11 0 0 0 5 5L15 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2Z"
          {...stroke}
        />
      )
    case 'donante_licencia':
      return (
        <path d="M12 21s-7-4.5-9-9a5 5 0 0 1 9-3 5 5 0 0 1 9 3c-2 4.5-9 9-9 9Z" {...stroke} />
      )
  }
}

export function AttributeIcon({
  attribute,
  className,
}: {
  attribute: Attribute
  className?: string
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      aria-hidden="true"
      focusable="false"
    >
      <Glyph attribute={attribute} />
    </svg>
  )
}
