type LogoMarkProps = {
  className?: string
}

// Escudo con tarjeta de identidad y check de aprobación. El escudo usa currentColor.
export function LogoMark({ className }: LogoMarkProps) {
  return (
    <svg
      viewBox="0 0 100 118"
      className={className}
      aria-hidden="true"
      focusable="false"
    >
      <path
        fill="currentColor"
        fillRule="evenodd"
        d="M50 0 96 12v52c0 24-20 42-46 54C24 106 4 88 4 64V12L50 0Zm-34 31a8 8 0 0 0-8 8v34a8 8 0 0 0 8 8h68a8 8 0 0 0 8-8V39a8 8 0 0 0-8-8H16Z"
      />
      <circle cx="29" cy="46" r="8" fill="currentColor" />
      <rect x="19" y="58" width="22" height="11" rx="5.5" fill="currentColor" />
      <rect x="49" y="40" width="34" height="5" rx="2.5" fill="currentColor" />
      <rect x="49" y="50" width="34" height="5" rx="2.5" fill="currentColor" />
      <rect x="49" y="60" width="22" height="5" rx="2.5" fill="currentColor" />
      <path
        d="m50 84 13 13 34-40"
        fill="none"
        stroke="#12a150"
        strokeWidth="10"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
