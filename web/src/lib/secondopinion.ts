/** The request a second-opinion chat starts with in its message box. The user edits and sends it. */
export const REVIEW_REQUEST =
  'Review the work of the earlier chat: look for bugs, missing tests and anything that does not match its goal. Do not change files; list what you would change and why.'

/** The agent for a second opinion: the first installed one that is not the earlier chat's agent. */
export function otherAgent<T extends { id: string; installed: boolean }>(agents: T[], earlier: string | undefined): T | undefined {
  return agents.find((a) => a.installed && a.id !== earlier)
}
