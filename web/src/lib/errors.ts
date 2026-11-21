// Which field an error from the server belongs to, so the dialog can show it next to that field.
import { CommandError } from './events'

export type Form = 'open' | 'create' | 'clone'
export type Field = 'folder' | 'parent' | 'name' | 'url' | 'form'

const PARENT_CODES = new Set(['NOT_FOUND', 'OUTSIDE_ROOT'])

export function fieldFor(form: Form, code: string, hasName = true): Field {
  if (form === 'open') return 'folder'
  if (PARENT_CODES.has(code)) return 'parent'
  if (code === 'ALREADY_EXISTS') return form === 'clone' && !hasName ? 'url' : 'name'
  if (code === 'BAD_REQUEST') return form === 'create' ? 'name' : 'url'
  if (code === 'CLONE_FAILED') return 'url'
  return 'form'
}

export interface FieldError {
  field: Field
  message: string
}

export function describe(form: Form, err: unknown, hasName = true): FieldError {
  if (!(err instanceof CommandError)) return { field: 'form', message: 'Something went wrong. Try again.' }
  if (err.code === 'CLOSED') return { field: 'form', message: 'The connection was closed. Sign in again.' }
  if (err.code === 'OUTCOME_UNKNOWN') {
    return { field: 'form', message: 'Mooring restarted while this was running. Check the project list before trying again.' }
  }
  return { field: fieldFor(form, err.code, hasName), message: err.message }
}
