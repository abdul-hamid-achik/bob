/** The brick-built lowercase b from docs/public/favicon.svg, as inline SVG. */
export function BrandMark({ size = 22, className }: { size?: number; className?: string }): JSX.Element {
  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      className={className}
      role="img"
      aria-label="Bob"
      focusable="false"
    >
      <rect width="64" height="64" rx="14" fill="#0e1116" />
      <rect x="1" y="1" width="62" height="62" rx="13" fill="none" stroke="#ffffff" strokeOpacity="0.08" strokeWidth="1" />
      <rect x="16" y="10" width="10" height="44" rx="2" fill="#f25c1a" />
      <rect x="30" y="28" width="18" height="10" rx="2" fill="#6ca8ff" />
      <rect x="30" y="44" width="18" height="10" rx="2" fill="#6ca8ff" />
      <rect x="42" y="32" width="6" height="18" rx="2" fill="#6ca8ff" />
    </svg>
  )
}
