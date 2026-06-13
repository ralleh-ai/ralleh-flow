export type OperationalTone = 'default' | 'ok' | 'warn' | 'danger'

export interface OperationalFact {
  label: string
  value: string
  tone?: OperationalTone
  detail?: string
}

export interface OperationalLink {
  label: string
  to: string
}
