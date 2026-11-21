# Mooring design

Status: approved by the user on 2026-11-14.

Dial: ENERGY 1 / RHYTHM 1 / MOTION 1. Read as: all-day developer workspace, one user, desktop and phone.
Reasons: ENERGY 1 so agent state is the loudest thing on screen; RHYTHM 1 because panes repeat the same
bar-plus-body pattern; MOTION 1 because nothing should move while someone reads a terminal.

## Identity
- Calm, dense, plain. Tinted neutrals (slightly green grey, a harbour in fog), one teal accent, one amber signal.
- Teal (`--accent`) is for three things only: focus, the primary button, and the active tab line. Used once per view.
  Function names in code are not teal.
- Amber (`--attention`) means "this needs you": permission ask, question, stopped chat. One permission request shows in
  three places: the tab dot, the banner by the composer, and the status bar link on the right with its dot. Project and
  chat rows get the dot only (with an accessible name), no text chip and no tinted row. Never colour alone. Red is only for errors and destructive actions.
- The amber attention motif first appears with the first chat state (agent chats, after phase 3). Do not add decoration before then.
- Mooring nod: amber is the harbour light, a thing left on that you notice when it matters. No logo yet:
  the wordmark is the name in bold text (placeholder, ask before drawing a mark). No nautical imagery.
- `--bg` is the terminal background. Panels, bars and sidebar use `--surface`; menus, inputs, dialogs `--raised`.
- Separation by surface steps first, 1px lines second. No shadows except menus and dialogs. No gradients, glass or glow.

## Colour tokens (ratio is text on `--bg` unless noted)
The work area is the lightest surface in light mode and the darkest in dark mode; bars and the sidebar step away from it.
| Token | Light | Dark | Use / ratio L, D |
|---|---|---|---|
| `--bg` | #f9fbf9 | #0f1413 | work area, editor, terminal |
| `--surface` | #ecf0ee | #1a201e | bars, sidebar, panels |
| `--raised` | #ffffff | #222928 | inputs, menus, dialogs |
| `--hover` | #e2e8e5 | #29312f | hover fill |
| `--selected` | #cbe7e2 | #1a3c39 | selected row/tab fill, with a 2px `--accent` bar on the left of rows (an inset 2px shadow, not a gradient) |
| `--fg` | #1d2321 | #e4e9e6 | text 15.4, 15.1 |
| `--muted` | #56605c | #9ba5a0 | secondary text 6.3, 7.3 (5.0, 4.7 on selected) |
| `--faint` | #626c68 | #8f9a95 | hints, hashes, times; only on bg, surface, raised, never on `--hover` or `--selected` (4.4 and 4.2 in light): 4.7 to 5.4 light, 5.1 to 6.4 dark |
| `--border` | #d5dbd8 | #323a38 | hairlines, decorative |
| `--border-strong` | #7d8884 | #707b76 | edge of inputs, selects, checkboxes: 3.5 on bg, 3.2 on surface, 3.7 on raised (light); 4.2, 3.8, 3.4 (dark) |
| `--border-btn` | #b9c1bd | #46514d | edge of buttons only, see Components |
| `--accent` | #1d6b66 | #6cc4bc | links, focus, primary fill 6.0, 9.1 |
| `--accent-hover` | #165a55 | #8dd3cc | primary hover |
| `--on-accent` | #ffffff | #0c1b19 | text on accent 6.3, 8.7 |
| `--attention` | #8a5a00 | #e8b04a | text 5.7, 9.5 |
| `--attention-bg` | #fbf0d5 | #2c2412 | tint; attention on it 5.2, 7.9 |
| `--danger` | #b3261e | #ff8a80 | errors 6.3, 8.1 (5.8, 6.7 on attention-bg) |
| `--success` | #2b6a3a | #7fcf92 | done, connected 6.3, 10.0 |
| `--diff-add` | #dcf0df | #17301f | added lines, A badge; fg 13.4, 11.6 |
| `--diff-del` | #fbe3e0 | #3c1f1d | removed lines, D badge; fg 13.1, 12.2 |
| `--scrim` | rgb(0 0 0 / 0.45) | same | behind dialogs |
| `--shadow-dialog`, `--shadow-menu` | | | the only shadows |

Checked with `.claude/skills/antislop-human/contrast-check.py` logic. All text pairs pass 4.5:1 (ANSI 0 on dark is the one stated exception). Input, select and checkbox edges pass 3:1 on their own fill. Light `--border-strong` on `--hover` is 2.95: inputs are not drawn on hover fills. **Button edge exception:** buttons carry a light edge `--border-btn` (1.8:1 on `--raised`, below 3:1) because every button has a visible text label, a fill step on hover and a focus ring; the edge only frames it. Icon buttons and quiet buttons have no edge. Dark `--border-btn` #46514d is the same 1.8:1 step on `--raised`, chosen to match the light value rather than reach 3:1.

## Terminal ANSI (`--ansi-0` to `--ansi-15`, on `--bg`)
| # | Name | Light | Dark |
|---|---|---|---|
| 0 | black | #1d2321 | #6a7772 |
| 1 | red | #b3261e | #e5716a |
| 2 | green | #2b6a3a | #6fc283 |
| 3 | yellow | #8a5a00 | #e8b04a |
| 4 | blue | #2a5fb0 | #7aa7f0 |
| 5 | magenta | #8b3f9c | #c792e0 |
| 6 | cyan | #14707a | #5ec8d4 |
| 7 | white | #5a6460 | #c4cdc9 |
| 8 | bright black | #66716d | #7d8984 |
| 9 | bright red | #c8372d | #ff8a80 |
| 10 | bright green | #357a46 | #9be0ab |
| 11 | bright yellow | #976400 | #f2c86e |
| 12 | bright blue | #386cbf | #9bbff7 |
| 13 | bright magenta | #9f4eb2 | #d9aef0 |
| 14 | bright cyan | #167a85 | #86dce6 |
| 15 | bright white | #3b4541 | #eef2f0 |

Light: every entry is at least 4.5:1 on `#f4f5f3`, so on a light terminal "white" is a dark grey and
"bright white" is darker still (names follow position, not lightness). Dark: all at least 5:1 except
0 black, 3.5:1 (it is mostly used as a background by TUIs, kept visible as text). Cursor `--accent`,
selection `--selected` (the terminal sets `selectionBackground` from that token, not `--border`).
Known limit: white (7) on black (0), as TUI status bars draw it, is 2.6:1 in light and 3.4:1 in dark.
Both stay legible as text on `--bg`, so the pair is not adjusted.

## Type
- UI: `system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif`. Native, zero bytes, no external host (CSP is
  `default-src 'self'`). Mono: `ui-monospace, 'SF Mono', Menlo, Consolas, monospace` for terminal, editor, paths, code.
- Scale: `--fs-xs` 11 (times, counts), `--fs-s` 12 (meta, labels, status bar), `--fs-m` 13 (base), `--fs-l` 16 / 20 (dialog and
  chat titles), `--fs-xl` 20 (sign-in only). Chat text `--fs-read` 15 / 24; code in chat `--fs-code` 13 / 19.5.
- Touch (`pointer: coarse`): s 13, m 16 (inputs must be 16 to stop iOS zoom).
- Text size setting (fine pointers only): small 12/11/14, default 13/12/15, large 15/13/17 for m/s/l.
- Chat column: one `--chat-col` 760px centred column holds messages, the permission banner and the composer. The chat
  header is 40px: title, state, a "…" menu (shorten output, new chat from summary, context meter that turns amber at
  80%, cost). Tool calls are one mono line each; an edit opens its diff, a command shows its last 10 lines. A message
  typed while the agent replies is queued ("Sends when the reply ends"). Markdown is rendered from a safe parser;
  links go only to http(s) and mailto, in a new tab with `rel="noopener noreferrer"`.
- Weights 400 (text), 500 (buttons, tabs, controls), 600 (headings, labels, the wordmark). No 700 or above. Line height 1.5 body, 1.25 titles.
- No uppercase tracking, no large mono headings. A bundled mono (e.g. JetBrains Mono) is possible later; user decides.

## Space, radius, border, density
- Space `--s-1..6` = 4, 8, 12, 16, 24, 32. Gaps inside a bar use s-2/s-3, between groups s-5.
- Radius: `--r-s` 3 (tabs, rows, badges), `--r-m` 6 (buttons, inputs, selects, panels, dialogs), checkbox 4. No pills.
- Border: 1px `--border` between regions; 1px `--border-strong` on inputs, selects and checkboxes; 1px `--border-btn` on buttons.
- Density: desktop control 30px (buttons, inputs; icon buttons 28), sidebar row 28px, bar 36px, list row 28px, status bar 24px. Touch: control 44px, bar 48px.
- Page edges respect `env(safe-area-inset-*)`. No horizontal scroll at 390px; one pane at a time on phones (see Workspace).

## Motion
- Only colour/background/border transitions, `--ease` 120ms, plus a 1px press on primary buttons.
- No entrance animation, no scroll effects. Attention is static (dot + tint), never pulsing.
- `prefers-reduced-motion: reduce` sets all durations to ~0 (global rule in `app.css`).

## States
- Focus: 2px `--accent` outline, 2px offset, on `:focus-visible` only. Never remove it.
- Hover: `--hover` fill (secondary, quiet) or `--accent-hover` (primary); input edge goes to `--fg`. Active: `--selected` fill.
- Disabled: lighter than enabled: `--surface` fill, `--faint` text, no edge, `cursor: not-allowed`. Pointer on enabled controls.
- Error: `--danger` text in a `role="alert"` line plus a `--danger` edge on the field. Message says what to do.
- Attention: `--attention-bg` row, `--attention` dot and label ("Needs permission", "Asked a question", "Stopped").
  Shown on the list row, the tab and the pane header at once; one per thing, never a toast.
- Empty, loading, error exist for every data view. Loading text is a verb ("Signing in…"). Ellipsis is the single character …, never three dots.

## Components
- Buttons (30px, 6px radius, weight 500, `--border-btn` edge unless stated):
  Primary = `--accent` fill, `--on-accent` text, the only filled button on a screen.
  Secondary = `--raised` fill, `--fg` text, light edge.
  Quiet = no edge, `--muted` text, `--hover` fill on hover; toolbars and rows.
  Danger = `--danger` text, light edge; solid `--danger` fill only on the confirm step.
  Icon = 28x28, 16px icon, no edge, `--muted`; hover `--hover` fill and `--fg`. Needs `aria-label`.
  On touch all are 44px high.
- Text input and select: 30px high, `--raised` fill, `--border-strong` edge, 6px radius, visible label above (12px, 600).
  Selects use a custom chevron (`appearance: none`, a rotated box corner in `--muted` on a `.select` wrapper, so it follows the token and uses no gradient). Help text 12px muted.
- Checkbox: 16px, 4px radius, `--border-strong` edge, `--accent` fill with an `--on-accent` tick when checked.
- Change row: 18px letter badge, 3px radius, soft fill: M (and R, T) `--hover` on `--fg` (amber stays for attention only), A `--diff-add` on `--success`,
  D `--diff-del` on `--danger`, ? and others `--hover` on `--muted`; the screen-reader name keeps the full word. The file name
  shows in full, the folder is the part that truncates. Row actions are grey icons; discard turns `--danger` on hover.
  Discard all sits at the far end of the list bar, apart from Stage all.
- Changes list: the section keeps at least 260px (40dvh on phones) and the sidebar scrolls around it, so rows are never squeezed.
- Sidebar rows (footer commands): 28px, no edge, icon 16px plus label, `--muted` text, `--hover` on hover. The footer is
  sticky at the bottom with the `--surface` fill.
- Tab: 36px, flush with the strip, 96 to 180px wide, a 16px kind icon (terminal, chat, file, diff, history) then the name. Active: `--bg` fill and a 2px `--accent` underline, no grey pill. Close control on hover and on the active tab. Attention dot at left of the icon.
- Sidebar footer: a "Search commands" row with the palette key, a theme switch icon button (moon in light, sun in dark; "Switch to dark theme" / "Switch to light theme", also in the palette) and a Settings gear icon button. The switch stores an explicit light or dark choice; Settings brings back "system". Everything else (accounts, tool servers, SSH hosts, language servers, token saving, appearance, sign out) is inside Settings or the palette.
- Settings: one dialog, 640px wide, section list on the left (on top on a phone). Sections that already have their own dialog open it from a button. Appearance holds theme (system, light, dark) and text size (small, default, large), stored per browser and applied before first paint.
- Dialogs: two widths, 480 and 640px, top edge at 10dvh, title row, body, and a footer row with the main action on the right.
- New tab "+" opens a menu (Terminal, Chat, Open file); there is no blank tab. Split opens a terminal. Pane buttons show in the focused pane only.
- Empty states and pickers: centred, at most 560px wide.
- Pane header: 36px bar, `--surface`, bottom `--border`; title 13px 600, actions right-aligned, quiet buttons.
- Status bar: 28px, `--surface`, top `--border`, 12px muted text; attention counts use `--attention`.
- List row: 28px, 3px radius, `--hover` on hover, `--selected` plus a 2px `--accent` bar on its left (inset shadow) when current;
  Selected tabs keep the fill only.
- Icons: 16px, 1.5 stroke, `currentColor`, inline SVG from `web/src/Icon.svelte` (paths in `web/src/lib/icons.ts`); always `aria-hidden` next to a name.
- Panels (login card, dialogs): `--surface` or `--raised`, 1px `--border`, 6px radius, no shadow on login.

## Workspace (phase 2)
- Desktop: project sidebar (default 264px, drag or arrow keys between 220 and 480, width remembered), then panes. Each pane is a 36px tab bar plus a body; dividers are 1px `--border`
  with a wider hit area, `--accent` on hover, keyboard-resizable (arrows, Shift for larger steps).
- Focused pane: its active tab is `--selected` and its bar has a 1px `--accent` top edge; other panes show `--hover` and no edge.
- Old saved blank tabs are still numbered ("Blank 1"); new ones are never created.
- Drag a tab onto a pane body: outer quarter splits on that side (`--selected` fill, `--accent` edge), middle joins the
  pane; onto a tab bar: a 2px `--accent` line shows the slot. Keyboard: Alt+Shift+Left/Right moves a tab to another pane.
- Shortcuts (Alt, read by key position): `\` split right, `-` split down, T open the + menu, W close tab, Left/Right tabs,
  Up/Down panes. Cmd/Ctrl+K opens the palette. Not read while typing in a field or with a dialog open.
- Phone (max 900px): sidebar is a drawer behind a "Projects" button; only the focused pane shows, and its bar gets a
  "Pane 2 of 3" button that cycles panes. Split buttons are hidden there (the palette still has them).
- Connection: status bar says "Connected" in muted text; when not live it switches to `--fg` 600 text, not amber
  (amber is for things that need the user).
- Menus and dialogs are the only shadowed surfaces. Project row actions live in a "…" menu, so reorder and remove work
  without dragging.

## Terminal (phase 3)

- One terminal has one size, set by the last window that typed or resized. Windows that
  did not set it are **letterboxed**, not scaled: the grid keeps its real columns and rows
  at the normal font size, clipped or padded inside the pane. Scaling would blur text and
  break alignment the user is reading. The clipped edge is a dashed `--border-strong` line.
  A fixed 24px strip under the screen says "Sized by another window: 120 × 42. Type here to
  take over the size." The strip is always there, so the note never covers output or changes the fit.
- A hidden pane or browser tab stops drawing and keeps its place; it catches up on return.
- Exited: a bar under the screen says the shell exited (with the code) and offers
  "Start a new terminal". The old screen stays readable above it. Focus moves to that button
  and the screen stops taking keys, so Tab and Enter work.
- Phone width and coarse pointers show a key row under the screen: Ctrl (sticky, one
  key), Esc, Tab and the four arrows. Buttons keep focus in the terminal so the keyboard
  stays open.
- Alt shortcuts and Ctrl/Cmd+K go to the workspace, not the shell.
- **Leaving a live terminal:** Tab, Esc and Enter belong to the shell, so one chord is reserved:
  **Alt+Esc** moves DOM focus to the focused tab in the pane's tab bar (Left and Right then
  switch tabs, Tab reaches the pane buttons). It is shown in the strip under every terminal
  and in the palette. Alt+Up and Alt+Down move to another pane and put DOM focus there (in its
  terminal, or on its tab). Alt+letter also works inside text boxes, but only for letters that are bound; unbound Alt combinations are never touched. Trade-off: on a Mac, ⌥ plus a bound letter runs the shortcut instead of typing a special character (the palette names both platforms plainly). Input method (IME) key events are never shortcuts. Off a Mac, Ctrl+P, Ctrl+Shift+F and Ctrl+comma override the browser's own meaning (print, find in page, settings) outside a terminal; Alt+arrow and Alt+Esc stay with a box. On a Mac Alt shows as ⌥. Cmd/Ctrl+P opens a file, +Shift+P the commands, +Shift+F searches text, +comma opens Settings: on a Mac (Cmd) they work everywhere, including the editor and a focused terminal; elsewhere plain Ctrl+P and Ctrl+comma stay with a focused terminal. Cmd/Ctrl+W and +T are left to the browser. Reserved keys are Alt+letter, Alt+arrow, Alt+Esc and Ctrl/Cmd+K, which
  shells, vim and tmux rarely bind; never take Ctrl+letter, Esc, Tab or plain Alt+Enter.
- Text in the terminal is held at 4.5:1 against the cell behind it (xterm `minimumContrastRatio`), so tools that
  pick their own colours for a dark screen, such as Claude Code's default theme, stay readable in light.
- The terminal background is `--bg` everywhere, including xterm's scroll area, so no black
  strip shows above the phone key row. Selection is `--selected`.
- Key row buttons have a `--raised` fill and a `--border-btn` edge so they read as buttons;
  Ctrl, while armed, is filled with `--accent`.
- Tab titles: the title the shell reports, else "Terminal" (never the project name); two tabs in one pane with the same
  title get " 2", " 3".

## Chat (phase 4)
- Chat is a tab kind. The "+" menu and the pane bar offer "New chat" (Alt+A); the tab starts on
  the agent list. Agents not installed stay listed, disabled, with what to put on the PATH.
- Messages are rows in a virtual list. User text sits on `--surface`; agent text has no box. Tool
  rows are collapsed by default and show a text status, never colour alone.
- Attention is amber (`--attention`, `--attention-bg`) with a dot and the words "Needs permission",
  on the tab, the project row, the sidebar chat and the status bar. One shared `AttentionMark`.
- A permission request docks above the composer so it is reachable at any scroll position and on a
  phone. The primary button is the allow-once option. A request answered on another device shows
  "You allowed" or "You refused" in the transcript row once answered (the banner shows only while it waits). No shadows; the dock has a top border.
- Modes that skip asking need an inline confirm. Enter sends on a keyboard and inserts a newline on
  a touch screen; Shift+Enter always inserts one.
- "Jump to latest" appears when the list is not following new messages.
- Announcements: a visually hidden `role="log"` line says "{agent} replied: ..." once a reply ends
  (never per chunk); the permission dock is `role="alert"`. The dock does not take focus, so typing
  is not interrupted. The mode confirm does take focus, on "Keep asking", and gives it back.
- A persistent amber strip shows when the server could not switch the agent to a mode that asks
  first. Any mode not known to ask first (`asksFirst` in `chatfmt.ts`) needs the confirm.
- Agent text renders paragraphs, code fences and `inline code` only. Headings, lists, bold and links
  stay as typed until a markdown renderer fits the chat chunk limit (20 KB gzip).
- `.sr` in `app.css` is the shared screen-reader-only class.

## Copy voice
- Say what happened and what to do next, in plain words. "That token does not match this server."
- Name the thing (project, chat, agent), not "item" or "entry". Buttons are verbs: "Sign in", "Sign out".
- No exclamation marks, buzzwords, emoji or em dashes. Sentence case everywhere. Numbers only when real.
- Server unreachable is a state, not blame: "Mooring's server is not answering. Check that it is running."
