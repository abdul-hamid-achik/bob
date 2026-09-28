import clsx from 'clsx'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes
} from 'react'
import { Check, ChevronDown, ChevronRight, Copy, Loader2 } from 'lucide-react'

export const cx = clsx

export type Tone = 'neutral' | 'positive' | 'caution' | 'danger' | 'info' | 'accent'

const TONE_TEXT: Record<Tone, string> = {
  neutral: 'text-ink-muted',
  positive: 'text-jade',
  caution: 'text-brass',
  danger: 'text-clay',
  info: 'text-steel',
  accent: 'text-copper'
}

const TONE_BG: Record<Tone, string> = {
  neutral: 'bg-stone/12',
  positive: 'bg-jade/14',
  caution: 'bg-brass/14',
  danger: 'bg-clay/14',
  info: 'bg-steel/14',
  accent: 'bg-copper/14'
}

/* ------------------------------------------------------------------ text --- */

export function Label({ children, className }: { children: ReactNode; className?: string }): JSX.Element {
  return (
    <span className={cx('text-[10.5px] font-medium uppercase text-ink-dim', className)} style={{ letterSpacing: '0.05em' }}>
      {children}
    </span>
  )
}

export function Mono({ children, className, title }: { children: ReactNode; className?: string; title?: string }): JSX.Element {
  return (
    <span className={cx('font-mono text-[11.5px]', className)} title={title}>
      {children}
    </span>
  )
}

export function SectionTitle({ title, hint, actions }: { title: string; hint?: string; actions?: ReactNode }): JSX.Element {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-line pb-2">
      <div className="min-w-0">
        <h2 className="text-[13px] font-semibold text-ink">{title}</h2>
        {hint ? <p className="mt-0.5 text-[11.5px] leading-snug text-ink-muted">{hint}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  )
}

export function Section({
  title,
  hint,
  actions,
  children,
  className
}: {
  title?: string
  hint?: string
  actions?: ReactNode
  children: ReactNode
  className?: string
}): JSX.Element {
  return (
    <section className={cx('flex flex-col gap-3', className)}>
      {title ? <SectionTitle title={title} hint={hint} actions={actions} /> : null}
      {children}
    </section>
  )
}

/* --------------------------------------------------------------- controls --- */

type ButtonVariant = 'primary' | 'default' | 'ghost' | 'danger' | 'quiet'

const BUTTON_VARIANT: Record<ButtonVariant, string> = {
  primary: 'bg-copper text-copper-ink border border-copper hover:brightness-110 active:brightness-95 font-medium',
  default: 'bg-raise text-ink border border-line-strong hover:border-ink-dim hover:bg-panel',
  ghost: 'bg-transparent text-ink-muted border border-transparent hover:bg-raise hover:text-ink',
  danger: 'bg-clay/15 text-clay border border-clay/45 hover:bg-clay/25',
  quiet: 'bg-transparent text-ink-muted border border-line hover:text-ink hover:border-line-strong'
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: 'sm' | 'md'
  loading?: boolean
  icon?: ReactNode
}

export function Button({ variant = 'default', size = 'md', loading, icon, children, className, disabled, ...rest }: ButtonProps): JSX.Element {
  return (
    <button
      type="button"
      disabled={disabled || loading}
      className={cx(
        'inline-flex items-center justify-center gap-1.5 rounded-[6px] whitespace-nowrap transition-[background-color,border-color,color,filter] duration-100',
        'disabled:cursor-not-allowed disabled:opacity-45',
        size === 'sm' ? 'h-6.5 px-2 text-[11.5px]' : 'h-8 px-3 text-[12.5px]',
        BUTTON_VARIANT[variant],
        className
      )}
      {...rest}
    >
      {loading ? <Loader2 className="size-3.5 animate-spin" aria-hidden /> : icon}
      {children}
    </button>
  )
}

export function IconButton({
  label,
  icon,
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string; icon: ReactNode }): JSX.Element {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={cx(
        'inline-flex size-7 items-center justify-center rounded-[6px] border border-transparent text-ink-muted',
        'hover:border-line hover:bg-raise hover:text-ink disabled:cursor-not-allowed disabled:opacity-40',
        className
      )}
      {...rest}
    >
      {icon}
    </button>
  )
}

export function CopyButton({ value, label = 'Copy' }: { value: string; label?: string }): JSX.Element {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1400)
    return () => clearTimeout(timer)
  }, [copied])
  return (
    <Button
      size="sm"
      variant="quiet"
      icon={copied ? <Check className="size-3.5 text-jade" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
      onClick={() => {
        void navigator.clipboard.writeText(value).then(() => setCopied(true))
      }}
    >
      {copied ? 'Copied' : label}
    </Button>
  )
}

export function Toggle({
  checked,
  onChange,
  label,
  hint,
  disabled,
  tone = 'accent'
}: {
  checked: boolean
  onChange: (next: boolean) => void
  label: string
  hint?: string
  disabled?: boolean
  tone?: 'accent' | 'danger'
}): JSX.Element {
  const id = useId()
  return (
    <div className="flex items-start gap-2.5">
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cx(
          'relative mt-0.5 h-4.5 w-8 shrink-0 rounded-full border transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-40',
          checked
            ? tone === 'danger'
              ? 'border-clay/60 bg-clay/35'
              : 'border-copper/60 bg-copper/30'
            : 'border-line-strong bg-sunken'
        )}
      >
        <span
          className={cx(
            'absolute top-[3px] size-3 rounded-full transition-[left,background-color] duration-150',
            checked ? (tone === 'danger' ? 'bg-clay' : 'bg-copper') : 'bg-ink-dim'
          )}
          style={{ left: checked ? 17 : 3 }}
        />
      </button>
      <div className="min-w-0">
        <label htmlFor={id} className="block cursor-pointer text-[12.5px] text-ink">
          {label}
        </label>
        {hint ? <p className="mt-0.5 text-[11.5px] leading-snug text-ink-muted">{hint}</p> : null}
      </div>
    </div>
  )
}

const FIELD_BASE =
  'w-full rounded-[6px] border border-line-strong bg-sunken px-2.5 py-1.5 text-[12.5px] text-ink placeholder:text-ink-dim ' +
  'focus:border-copper focus:outline-none disabled:cursor-not-allowed disabled:opacity-50'

export function TextInput({ className, ...rest }: InputHTMLAttributes<HTMLInputElement>): JSX.Element {
  return <input className={cx(FIELD_BASE, className)} {...rest} />
}

export function TextArea({ className, ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>): JSX.Element {
  return <textarea className={cx(FIELD_BASE, 'resize-y leading-relaxed', className)} {...rest} />
}

export function Select({ className, children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>): JSX.Element {
  return (
    <div className="relative inline-flex w-full">
      <select className={cx(FIELD_BASE, 'appearance-none pr-7', className)} {...rest}>
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute top-1/2 right-2 size-3.5 -translate-y-1/2 text-ink-dim" aria-hidden />
    </div>
  )
}

export function Field({
  label,
  hint,
  children,
  className
}: {
  label: string
  hint?: string
  children: ReactNode
  className?: string
}): JSX.Element {
  const id = useId()
  return (
    <div className={cx('flex flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-[11px] font-medium text-ink-muted">
        {label}
      </label>
      <div id={id} className="flex flex-col gap-1">
        {children}
      </div>
      {hint ? <p className="text-[11px] leading-snug text-ink-dim">{hint}</p> : null}
    </div>
  )
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  ariaLabel
}: {
  value: T
  options: { value: T; label: string; count?: number }[]
  onChange: (next: T) => void
  ariaLabel: string
}): JSX.Element {
  return (
    <div role="tablist" aria-label={ariaLabel} className="inline-flex rounded-[6px] border border-line bg-sunken p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={value === option.value}
          onClick={() => onChange(option.value)}
          className={cx(
            'inline-flex items-center gap-1.5 rounded-[4px] px-2.5 py-1 text-[11.5px] transition-colors duration-100',
            value === option.value ? 'bg-raise text-ink shadow-[0_1px_0_rgba(0,0,0,0.25)]' : 'text-ink-muted hover:text-ink'
          )}
        >
          {option.label}
          {option.count !== undefined ? (
            <span className={cx('font-mono text-[10.5px]', value === option.value ? 'text-copper' : 'text-ink-dim')}>
              {option.count}
            </span>
          ) : null}
        </button>
      ))}
    </div>
  )
}

export function Badge({ tone = 'neutral', children, className }: { tone?: Tone; children: ReactNode; className?: string }): JSX.Element {
  return (
    <span
      className={cx(
        'inline-flex items-center gap-1 rounded-[4px] px-1.5 py-0.5 text-[10.5px] font-medium whitespace-nowrap',
        TONE_BG[tone],
        TONE_TEXT[tone],
        className
      )}
    >
      {children}
    </span>
  )
}

/* ---------------------------------------------------------------- surfaces --- */

export function Panel({ children, className }: { children: ReactNode; className?: string }): JSX.Element {
  return <div className={cx('rounded-[10px] border border-line bg-panel', className)}>{children}</div>
}

export function Callout({
  tone = 'info',
  title,
  children,
  actions
}: {
  tone?: Tone
  title?: string
  children?: ReactNode
  actions?: ReactNode
}): JSX.Element {
  return (
    <div className={cx('flex items-start gap-3 rounded-[8px] border px-3 py-2.5', TONE_BG[tone], 'border-current/25')}>
      <div className="min-w-0 flex-1">
        {title ? <p className={cx('text-[12.5px] font-medium', TONE_TEXT[tone])}>{title}</p> : null}
        {children ? <div className="mt-0.5 text-[12px] leading-relaxed text-ink-muted">{children}</div> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  )
}

export function EmptyState({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }): JSX.Element {
  return (
    <div className="flex flex-col items-start gap-2 rounded-[8px] border border-dashed border-line px-4 py-6">
      <p className="text-[13px] font-medium text-ink">{title}</p>
      {hint ? <p className="max-w-prose text-[12px] leading-relaxed text-ink-muted">{hint}</p> : null}
      {action ? <div className="mt-1">{action}</div> : null}
    </div>
  )
}

export function ErrorState({ title, detail, action }: { title: string; detail?: string; action?: ReactNode }): JSX.Element {
  return (
    <div className="flex flex-col items-start gap-2 rounded-[8px] border border-clay/40 bg-clay/10 px-4 py-3">
      <p className="text-[13px] font-medium text-clay">{title}</p>
      {detail ? <pre className="argv max-w-full text-clay/85 whitespace-pre-wrap">{detail}</pre> : null}
      {action ? <div className="mt-1">{action}</div> : null}
    </div>
  )
}

export function Skeleton({ className }: { className?: string }): JSX.Element {
  return <div className={cx('animate-pulse rounded bg-raise', className)} aria-hidden />
}

/* ------------------------------------------------------------------ tables --- */

export interface Column<T> {
  key: string
  header: string
  width?: string
  align?: 'left' | 'right'
  render: (row: T) => ReactNode
  sortValue?: (row: T) => string | number
}

export function DataTable<T>({
  rows,
  columns,
  keyFor,
  onRowClick,
  selectedKey,
  empty,
  dense,
  className
}: {
  rows: T[]
  columns: Column<T>[]
  keyFor: (row: T) => string
  onRowClick?: (row: T) => void
  selectedKey?: string | null
  empty?: ReactNode
  dense?: boolean
  className?: string
}): JSX.Element {
  const [sort, setSort] = useState<{ key: string; dir: 'asc' | 'desc' } | null>(null)
  const sorted = useMemo(() => {
    if (!sort) return rows
    const column = columns.find((entry) => entry.key === sort.key)
    if (!column?.sortValue) return rows
    const copy = [...rows]
    copy.sort((a, b) => {
      const left = column.sortValue?.(a) ?? ''
      const right = column.sortValue?.(b) ?? ''
      const result = typeof left === 'number' && typeof right === 'number' ? left - right : String(left).localeCompare(String(right))
      return sort.dir === 'asc' ? result : -result
    })
    return copy
  }, [rows, columns, sort])

  if (rows.length === 0 && empty) return <>{empty}</>

  return (
    <div className={cx('overflow-hidden rounded-[8px] border border-line', className)}>
      <div className="max-h-full overflow-auto">
        <table className="w-full border-collapse text-[12px]">
          <thead className="sticky top-0 z-10 bg-raise">
            <tr>
              {columns.map((column) => {
                const sortable = Boolean(column.sortValue)
                const active = sort?.key === column.key
                return (
                  <th
                    key={column.key}
                    scope="col"
                    style={column.width ? { width: column.width } : undefined}
                    className={cx(
                      'border-b border-line-strong px-2.5 py-1.5 text-left text-[10.5px] font-medium text-ink-dim uppercase',
                      column.align === 'right' && 'text-right',
                      sortable && 'cursor-pointer select-none hover:text-ink-muted'
                    )}
                    onClick={
                      sortable
                        ? () => setSort((prev) => (prev?.key === column.key ? { key: column.key, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { key: column.key, dir: 'asc' }))
                        : undefined
                    }
                    aria-sort={active ? (sort?.dir === 'asc' ? 'ascending' : 'descending') : undefined}
                  >
                    <span className="inline-flex items-center gap-1">
                      {column.header}
                      {active ? (sort?.dir === 'asc' ? '↑' : '↓') : null}
                    </span>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.map((row) => {
              const key = keyFor(row)
              return (
                <tr
                  key={key}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  tabIndex={onRowClick ? 0 : undefined}
                  onKeyDown={
                    onRowClick
                      ? (event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault()
                            onRowClick(row)
                          }
                        }
                      : undefined
                  }
                  className={cx(
                    'border-b border-line/70 last:border-b-0',
                    onRowClick && 'cursor-pointer',
                    selectedKey === key ? 'bg-copper/10' : onRowClick && 'hover:bg-raise/70'
                  )}
                >
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={cx('px-2.5 align-top', dense ? 'py-1' : 'py-1.5', column.align === 'right' && 'text-right')}
                    >
                      {column.render(row)}
                    </td>
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export function KeyValue({ entries, className }: { entries: [string, ReactNode][]; className?: string }): JSX.Element {
  return (
    <dl className={cx('grid grid-cols-[minmax(9rem,auto)_1fr] gap-x-4 gap-y-1.5', className)}>
      {entries.map(([key, value]) => (
        <div key={key} className="col-span-2 grid grid-cols-subgrid">
          <dt className="text-[11.5px] text-ink-dim">{key}</dt>
          <dd className="min-w-0 text-[12px] text-ink">{value}</dd>
        </div>
      ))}
    </dl>
  )
}

export function CodeBlock({
  children,
  className,
  maxHeight = '24rem',
  copy
}: {
  children: string
  className?: string
  maxHeight?: string
  copy?: boolean
}): JSX.Element {
  return (
    <div className="relative">
      <pre
        className={cx('argv overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2.5 text-ink-muted', className)}
        style={{ maxHeight }}
      >
        <code>{children}</code>
      </pre>
      {copy ? (
        <div className="absolute top-2 right-2">
          <CopyButton value={children} label="" />
        </div>
      ) : null}
    </div>
  )
}

/* -------------------------------------------------------------------- json --- */

function JsonNode({ name, value, depth, defaultOpen }: { name?: string; value: unknown; depth: number; defaultOpen: number }): JSX.Element {
  const [open, setOpen] = useState(depth < defaultOpen)
  const isObject = value !== null && typeof value === 'object'
  const entries = isObject ? Object.entries(value as Record<string, unknown>) : []

  if (!isObject) {
    const text = typeof value === 'string' ? value : JSON.stringify(value)
    const tone =
      typeof value === 'string' ? 'text-jade' : typeof value === 'number' ? 'text-brass' : typeof value === 'boolean' ? 'text-steel' : 'text-ink-dim'
    return (
      <div className="flex gap-1.5 py-px">
        {name !== undefined ? <span className="shrink-0 text-ink-dim">{name}:</span> : null}
        <span className={cx('break-all', tone)}>{typeof value === 'string' ? `"${text}"` : text}</span>
      </div>
    )
  }

  return (
    <div className="py-px">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="inline-flex items-center gap-1 rounded text-left text-ink-muted hover:text-ink"
        aria-expanded={open}
      >
        {open ? <ChevronDown className="size-3" aria-hidden /> : <ChevronRight className="size-3" aria-hidden />}
        {name !== undefined ? <span className="text-ink-dim">{name}:</span> : null}
        <span className="font-mono text-[11px] text-ink-dim">
          {Array.isArray(value) ? `[${entries.length}]` : `{${entries.length}}`}
        </span>
      </button>
      {open ? (
        <div className="ml-3 border-l border-line pl-2.5">
          {entries.map(([key, child]) => (
            <JsonNode key={key} name={Array.isArray(value) ? undefined : key} value={child} depth={depth + 1} defaultOpen={defaultOpen} />
          ))}
        </div>
      ) : null}
    </div>
  )
}

export function JsonView({ value, defaultOpen = 2, className }: { value: unknown; defaultOpen?: number; className?: string }): JSX.Element {
  return (
    <div className={cx('overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2 font-mono text-[11.5px]', className)}>
      <JsonNode value={value} depth={0} defaultOpen={defaultOpen} />
    </div>
  )
}

/* ------------------------------------------------------------------- tabs --- */

export function Tabs<T extends string>({
  value,
  onChange,
  items,
  ariaLabel
}: {
  value: T
  onChange: (next: T) => void
  items: { value: T; label: string; badge?: ReactNode }[]
  ariaLabel: string
}): JSX.Element {
  return (
    <div role="tablist" aria-label={ariaLabel} className="flex gap-0.5 border-b border-line">
      {items.map((item) => (
        <button
          key={item.value}
          type="button"
          role="tab"
          aria-selected={value === item.value}
          onClick={() => onChange(item.value)}
          className={cx(
            '-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-1.5 text-[12.5px] transition-colors duration-100',
            value === item.value
              ? 'border-copper text-ink'
              : 'border-transparent text-ink-muted hover:border-line-strong hover:text-ink'
          )}
        >
          {item.label}
          {item.badge}
        </button>
      ))}
    </div>
  )
}

/* --------------------------------------------------------------- overlays --- */

export function Modal({
  open,
  title,
  description,
  children,
  footer,
  onClose
}: {
  open: boolean
  title: string
  description?: string
  children?: ReactNode
  footer?: ReactNode
  onClose: () => void
}): JSX.Element {
  const titleId = useId()
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    ref.current?.focus()
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return <></>
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/55 p-6" role="presentation" onClick={onClose}>
      <div
        ref={ref}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="max-h-full w-full max-w-lg overflow-auto rounded-[10px] border border-line-strong bg-panel shadow-2xl outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="border-b border-line px-4 py-3">
          <h2 id={titleId} className="text-[14px] font-semibold text-ink">
            {title}
          </h2>
          {description ? <p className="mt-1 text-[12px] leading-relaxed text-ink-muted">{description}</p> : null}
        </div>
        {children ? <div className="px-4 py-3">{children}</div> : null}
        {footer ? <div className="flex items-center justify-end gap-2 border-t border-line px-4 py-3">{footer}</div> : null}
      </div>
    </div>
  )
}

/* ------------------------------------------------------------- disclosure --- */

export function Disclosure({ label, children, defaultOpen = false }: { label: ReactNode; children: ReactNode; defaultOpen?: boolean }): JSX.Element {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className="rounded-[8px] border border-line bg-panel">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-expanded={open}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-[12.5px] text-ink hover:bg-raise/60"
      >
        {open ? <ChevronDown className="size-3.5 text-ink-dim" aria-hidden /> : <ChevronRight className="size-3.5 text-ink-dim" aria-hidden />}
        <span className="flex-1">{label}</span>
      </button>
      {open ? <div className="border-t border-line px-3 py-2.5">{children}</div> : null}
    </div>
  )
}

/* --------------------------------------------------------------- shortcuts -- */

export function useCopy(): [boolean, (value: string) => void] {
  const [copied, setCopied] = useState(false)
  const copy = useCallback((value: string) => {
    void navigator.clipboard.writeText(value).then(() => setCopied(true))
  }, [])
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1400)
    return () => clearTimeout(timer)
  }, [copied])
  return [copied, copy]
}

export const ToastContext = createContext<(message: string, tone?: Tone) => void>(() => undefined)

export function useToast(): (message: string, tone?: Tone) => void {
  return useContext(ToastContext)
}
