# Mooring design

Status: Redesign 2026-11-22. Approved by the owner 2026-11-22. Replaces the 2026-11-14 spec. The CSS that goes with it
is in `web/src`: `tokens.css`, `app.css`, `base.css`, `forms.css`, `controls.css`, `menu.css`, `dialog.css`. Token names
stayed; values changed.

Dial: ENERGY 1 / RHYTHM 1 / MOTION 1. Read as: all-day developer workspace, one user, desktop and phone.
Reasons: ENERGY 1 so agent state is the loudest thing on screen; RHYTHM 1 because panes repeat the same
bar-plus-body pattern; MOTION 1 because nothing should move while someone reads a terminal.

## Identity
- Calm, dense, plain. Neutral greys with no tint, one teal accent, one amber signal. Geist for text, Geist Mono for code.
- Teal (`--accent`) is for focus, the primary button, and the active tab line (pane tabs and the sidebar tabs). Two small extras: the handle between Files and Chats while hovered, dragged or focused, and the underline on a filter match. Used once per view.
  Function names in code are not teal.
- Amber (`--attention`) means "this needs you": permission ask, question, stopped chat. One permission request shows in
  three places: the tab dot, the banner by the composer, and the status bar link on the right with its dot. Project and
  chat rows get the dot only (with an accessible name), no text chip and no tinted row. Never colour alone. Red is only for errors and destructive actions.
- Mooring nod: amber is the harbour light, a thing left on that you notice when it matters. No logo yet:
  the wordmark is the name in bold text (placeholder, ask before drawing a mark). No nautical imagery.
- `--bg` is the terminal background. Panels, bars and sidebar use `--surface`; menus, inputs, dialogs `--raised`.
- Separation by surface steps first, 1px lines second. Shadows only on menus and dialogs. No gradients, glass, blur or glow.
  No card has a border on one side only. Dividers between regions (bar, pane, sidebar) are 1px lines, and those are fine.

## Where this follows the design skills and where it does not
Adopted: the type scale, the 2/4/8/12/16/24/32 spacing set, radius 4/6/8/12 with the nested rule, two-layer colour tokens,
neutral greys, motion on `cubic-bezier(0.22, 1, 0.36, 1)` with transform and opacity only, skeleton loading, flat fills.
Two deliberate exceptions, both because this is a dense tool and not a page:
- **Controls are 32px, not 8px + 20px line + 8px = 36 to 40.** Two panes, a sidebar and a status bar must fit on a laptop; 32px keeps 44px for touch only.
- **Motion is 150ms, not 200 to 500ms.** Hover and press feedback that lasts half a second feels slow when you click
  forty times an hour. Nothing animates position or size.
Also kept from before: no italic in UI text (only markdown emphasis, code, and the name on a preview tab, where it is the only sign of that state, may), weights 400/500/600 only, our own SVG icons.

## Colour tokens
Two layers. `tokens.css` holds primitives (`--grey-0..1000`, `--teal-*`, `--amber-*`, `--red-*`, `--green-*`): values with
no meaning. `app.css` gives roles (`--bg`, `--fg`, `--accent` ...) per theme, each one `var(--grey-…)` or similar.
Components use roles only, never a primitive. The work area is the lightest surface in light mode and the darkest in dark
mode; bars and the sidebar step away from it. Dark surfaces come from the approved neutral list (#181818, #1F1F1F, #272727, #313131).
| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg` | #fafafa | #181818 | work area, editor, terminal |
| `--surface` | #f1f1f1 | #1f1f1f | bars, sidebar, panels, user chat text |
| `--raised` | #ffffff | #272727 | inputs, menus, dialogs, secondary buttons |
| `--hover` | #e7e7e7 | #313131 | hover fill |
| `--pressed` | #dcdcdc | #3a3a3a | pressed fill |
| `--selected` | #d3eae7 | #18403c | selected row and tab: a filled background, no accent bar |
| `--fg` | #181818 | #e7e7e7 | text |
| `--muted` | #525252 | #adadad | secondary text |
| `--faint` | #636363 | #9a9a9a | hints, hashes, times. Not on `--selected` in dark (4.1) |
| `--disabled-fg`, `--disabled-bg` | #858585, #e7e7e7 | #7a7a7a, #2c2c2c | disabled controls |
| `--border` | #dcdcdc | #2c2c2c | region lines, decorative only |
| `--border-strong` | #767676 | #858585 | edge of inputs, selects, checkboxes, buttons |
| `--border-btn` | #767676 | #858585 | same as `--border-strong`; the name stays so call sites do not change |
| `--accent`, `-hover`, `-pressed` | #0b6b63, #085650, #06453f | #5cc8bd, #7dd8ce, #45b0a5 | links, focus ring (`--focus-ring`), primary fill |
| `--on-accent` | #ffffff | #06201d | text on accent |
| `--attention`, `--attention-bg` | #8a5a00, #fbf0d5 | #e8b04a, #2b2314 | Attention text and tint |
| `--danger`, `--danger-bg` | #b3261e, #fbe6e3 | #ff8a80, #3a2020 | errors, danger hover tint |
| `--danger-fill`, `-hover`, `--on-danger` | #b3261e, #8f1d17, #fff | #ff8a80, #ffa8a0, #2a0a08 | the confirm step of a destructive action |
| `--success` | #2b6a3a | #7fcf92 | done, connected |
| `--diff-add`, `--diff-del` | #dcf0df, #fbe3e0 | #17301f, #3c1f1d | added and removed lines, A and D badges |
| `--scrim` | rgb(0 0 0 / .4) | rgb(0 0 0 / .6) | behind dialogs |
| `--shadow-dialog`, `--shadow-menu` | | | the only shadows |

### Contrast (computed with the `antislop-human` checker formula, from the token files)
Light / dark. Every text role is at least 4.5:1 on every surface it is used on.
| Pair | on `--bg` | on `--surface` | on `--raised` | on `--hover` | on `--selected` |
|---|---|---|---|---|---|
| `--fg` | 17.0 / 14.4 | 15.7 / 13.3 | 17.8 / 12.1 | 14.4 / 10.5 | 14.1 / 9.2 |
| `--muted` | 7.5 / 7.9 | 6.9 / 7.3 | 7.8 / 6.7 | 6.3 / 5.8 | 6.2 / 5.1 |
| `--faint` | 5.8 / 6.3 | 5.3 / 5.9 | 6.0 / 5.3 | 4.9 / 4.6 | 4.8 / 4.1 (dark: not allowed) |
| `--accent` text | 6.1 / 8.8 | 5.6 / 8.2 | 6.4 / 7.4 | 5.2 / 6.5 | 5.1 / 5.7 |
| `--attention` text | 5.7 / 9.1 | 5.2 / 8.4 | 5.9 / 7.6 | 4.8 / 6.7 | 4.7 / 5.8 |
| `--danger` text | 6.3 / 7.8 | 5.8 / 7.2 | 6.5 / 6.5 | 5.3 / 5.7 | 5.2 / 5.0 |
| `--success` text | 6.2 / 9.5 | 5.8 / 8.8 | 6.5 / 8.0 | 5.3 / 7.0 | 5.2 / 6.1 |
| `--border-strong` edge (3:1) | 4.4 / 4.8 | 4.0 / 4.5 | 4.5 / 4.0 | 3.7 / 3.5 | pressed 3.3 / 3.1 |

Other pairs, light / dark: `--on-accent` on `--accent` 6.4 / 8.5, on hover 8.5 / 10.2, on pressed 10.9 / 6.5.
`--on-danger` on `--danger-fill` 6.5 / 8.0. `--attention` on `--attention-bg` 5.2 / 7.9; `--danger` on `--danger-bg` 5.5 / 6.5.
`--fg` on `--diff-add` 14.9 / 11.5, on `--diff-del` 14.5 / 12.1; `--success` on `--diff-add` 5.4 / 7.6; `--danger` on `--diff-del` 5.3 / 6.5.
Focus ring (`--accent`, 2px) and the danger and attention button edges are at least 5.1 on bg, surface and raised, so above 3:1.
`--disabled-fg` on `--disabled-bg` is 3.0 / 3.3: disabled text is exempt from 4.5, kept at 3 so it can still be read.
`--border` is 1.3 / 1.3: decorative region lines only, never the edge of something you act on.
No exceptions remain: the old 1.8:1 button edge is gone, buttons use `--border-btn` at 4.4 / 4.8 on bg.

## Terminal ANSI (`--ansi-0` to `--ansi-15`, on `--bg`)
| # | Name | Light | Dark |
|---|---|---|---|
| 0 | black | #181818 | #858585 |
| 1 | red | #b3261e | #ef7a72 |
| 2 | green | #2b6a3a | #6fc283 |
| 3 | yellow | #8a5a00 | #e8b04a |
| 4 | blue | #2a5fb0 | #7aa7f0 |
| 5 | magenta | #8b3f9c | #c792e0 |
| 6 | cyan | #0f6a74 | #5ec8d4 |
| 7 | white | #5a5a5a | #c4c4c4 |
| 8 | bright black | #666666 | #9a9a9a |
| 9 | bright red | #c0302a | #ff8a80 |
| 10 | bright green | #357a46 | #9be0ab |
| 11 | bright yellow | #976400 | #f2c86e |
| 12 | bright blue | #386cbf | #9bbff7 |
| 13 | bright magenta | #9f4eb2 | #d9aef0 |
| 14 | bright cyan | #167a85 | #86dce6 |
| 15 | bright white | #3b3b3b | #f1f1f1 |

Every entry is at least 4.8:1 on `--bg` in both themes (lowest: light 13 and 14, dark 0). On a light terminal "white" is a
dark grey and "bright white" is darker still (names follow position, not lightness). Cursor `--accent`, selection `--selected`
(the terminal sets `selectionBackground` from that token). Known limit: white (7) on black (0), as TUI status bars draw it,
is low in both themes; both stay legible as text on `--bg`, so the pair is not adjusted.

## Type
- UI: Geist, variable woff2, self-hosted (served by the binary, so the CSP's `default-src 'self'` covers it; no external host).
  Why Geist: it is drawn for screens at small sizes, its mono twin shares the metrics so paths and code sit level with
  UI text, and it is OFL, so it ships inside the binary. Code, terminal, paths: Geist Mono. Fallbacks:
  `system-ui` and `ui-monospace`. `font-display: swap`. About 68 KB and 70 KB; loaded after the app shell, not counted in the 150 KB JS limit.
  Numbers in the status bar, times and counts use tabular figures.
- Steps (size / line): 12/16, 14/20, 16/24, 18/28, 20/28, 24/32. Nothing else. Code is 13/20, the one value off the scale.
| Token | Default | Use |
|---|---|---|
| `--fs-xs`, `--lh-xs` | 12/16 | times, counts, badges, kbd. Same step as `--fs-s`; the name stays for call sites |
| `--fs-s`, `--lh-s` | 12/16 | meta, field labels (600), help, status bar |
| `--fs-m`, `--lh-m` | 14/20 | base: rows, buttons, inputs, tabs, pane titles (600) |
| `--fs-l`, `--lh-l` | 16/24 | dialog and chat titles (600) |
| `--fs-xl`, `--lh-xl` | 20/28 | sign-in and empty-state titles (600) |
| `--fs-2xl`, `--lh-2xl` (new) | 24/32 | page-level title, used once at most |
| `--fs-read`, `--lh-read` | 16/24 | chat reading text |
| `--fs-code`, `--lh-code` | 13/20 | code in chat, editor, diff |
- Text size setting (fine pointers only) moves the same tokens along the steps. Small: m 12/16, l 14/20, xl 18/28, read 14/20, code 12/16.
  Default as above. Large: s 14/20, m 16/24, l 18/28, xl 24/32, read 18/28, code 14/20.
- Touch (`pointer: coarse`): s 14/20, m 16/24 (inputs must be 16 to stop iOS zoom), l 18/28. Read stays 16/24.
- Weights 400 (text), 500 (buttons, tabs, controls), 600 (headings, labels, the wordmark). No 700 or above.
  Token names `--fw-regular`, `--fw-medium`, `--fw-semibold`. No italics in UI text; markdown emphasis in chat and the name on a preview tab may be italic.
- No uppercase tracking, no large mono headings. `text-wrap: pretty` on chat paragraphs, `balance` on dialog and empty-state titles.
- Chat column: one `--chat-col` 760px centred column holds messages, the permission banner and the composer. The chat
  header is 40px: title, state, a "…" menu (shorten output, new chat from summary, context meter that turns amber at
  80%, cost). Tool calls are one mono line each; an edit opens its diff, a command shows its last 10 lines. A message
  typed while the agent replies is queued ("Sends when the reply ends"). Markdown is rendered from a safe parser;
  links go only to http(s) and mailto, in a new tab with `rel="noopener noreferrer"`.

## Space, radius, border, density
- Space `--s-0..6` = 2, 4, 8, 12, 16, 24, 32. Gaps inside a bar use s-2/s-3, between groups s-5. `--s-0` is for hairline gaps (menu items, list rows).
- Radius: `--r-s` 4 (tabs, rows, badges, menu items, checkbox), `--r-m` 6 (buttons, inputs, selects), `--r-l` 8 (menus, panels, cards, code blocks), `--r-xl` 12 (dialogs). No pills.
  Nested shapes: inner = outer minus the gap when the gap is under 32px and the result is above 2 (menu 8, padding 4, item 4). Otherwise use the inner shape's own token.
- Border: 1px `--border` between regions; 1px `--border-strong` on inputs, selects, checkboxes and buttons.
- Sizes (tokens): `--control-h` 32 (buttons, inputs), `--control-h-s` 28 (small buttons, icon buttons), `--row-h` 28 (sidebar and list rows),
  `--bar-h` 36 (tab strip, pane header, sidebar header), `--bar-h-l` 40 (chat header), `--status-h` 28 (status bar; the old text said 24 and 28, code uses 28).
  Touch: control, small control, icon button and row 44, bars 48, status bar 44.
- Icon buttons are 28, not 32: inside a 36px bar a 32px button leaves 2px of air and its hover fill touches the bar lines. Touch makes them 44.
- Page edges respect `env(safe-area-inset-*)`. No horizontal scroll at 390px; one pane at a time on phones (see Workspace).

## Motion
- `--ease` is `150ms cubic-bezier(0.22, 1, 0.36, 1)` (`--dur`, `--curve`). Never `ease`, `linear` or `ease-in-out`.
- Only colour, background, border-colour, opacity and transform change. Press: `translateY(1px)` on buttons, `scale(.96)` on icon buttons.
- No entrance animation, no scroll effects. Attention is static (dot + tint), never pulsing. Skeletons fade between 50% and 100% opacity only.
- `prefers-reduced-motion: reduce` sets transitions to ~0, stops skeletons and removes the press movement (global rule).

## States
- Focus: 2px `--focus-ring` (`--accent`) outline, 2px offset, on `:focus-visible` only. Never remove it. At least 5.1:1 on every surface.
- Hover: `--hover` fill (secondary, tertiary) or `--accent-hover` (primary); input edge goes to `--fg`.
- Pressed: `--pressed` fill (primary: `--accent-pressed`) and a 1px move down.
- Disabled: `--disabled-bg` fill, `--disabled-fg` text, no edge, `cursor: not-allowed`. Pointer on enabled controls.
- Error: `--danger` text in a `role="alert"` line plus a `--danger` edge on the field. Message says what to do.
- Attention: `--attention-bg` tint, `--attention` dot and label ("Needs permission", "Asked a question", "Stopped").
  Shown on the list row dot, the tab and the pane header at once; one per thing, never a toast.
- Empty, loading, error exist for every data view. Lists and data load as skeleton blocks (`.skel`, `--hover` fill, `--r-s`), shaped like the
  rows they replace, not a spinner. Buttons keep a verb ("Signing in…"). Ellipsis is the single character …, never three dots.

## Components
- Buttons (32px, 6px radius, weight 500, 12px side padding, `--border-btn` edge unless stated):
  Primary = `--accent` fill, `--on-accent` text, the only filled button on a screen.
  Secondary = `--raised` fill, `--fg` text, `--border-btn` edge.
  Tertiary (`.ghost`) = no edge, `--muted` text, `--hover` fill on hover; toolbars and rows.
  Danger = `--danger` text and edge, `--danger-bg` on hover; a solid `--danger-fill` only on the confirm step.
  Warning = `--attention` text and edge, `--attention-bg` on hover; for actions that skip a safeguard ("Skip asking").
  Small = 28px, 12px text. Icon = 28x28, 16px icon, no edge, `--muted`; hover `--hover` fill and `--fg`. Needs `aria-label`.
  On touch all are 44px high.
- Text input and select: 32px high, `--raised` fill, `--border-strong` edge, 6px radius, visible label above (12px, 600).
  Selects use a custom chevron (`appearance: none`, a rotated box corner in `--muted` on a `.select` wrapper). Help text 12px muted.
- Checkbox: 16px, 4px radius, `--border-strong` edge, `--accent` fill with an `--on-accent` tick when checked.
- Menu: `--raised`, 1px `--border`, 8px radius, `--shadow-menu`, 4px padding, items 28px with 4px radius, `--hover` on hover. Separators are 1px `--border`.
- Change row: 18px letter badge, 4px radius, soft fill: M (and R, T) `--hover` on `--fg` (amber stays for attention only), A `--diff-add` on `--success`,
  D `--diff-del` on `--danger`, ? and others `--hover` on `--muted`; the screen-reader name keeps the full word. The file name
  shows in full, the folder is the part that truncates. Row actions are grey icons; discard turns `--danger` on hover.
  Discard all sits at the far end of the list bar, apart from Stage all.
- Changes list: the section keeps at least 260px (40dvh on phones) and the sidebar scrolls around it, so rows are never squeezed.
- Sidebar rows (footer commands): 28px, no edge, icon 16px plus label, `--muted` text, `--hover` on hover. The footer is
  sticky at the bottom with the `--surface` fill.
- List row: 28px, 4px radius, `--hover` on hover, `--selected` fill when current (no accent bar). Chats under a project are indented 24px.
- Tab: 36px, flush with the strip, 96 to 180px wide, a 16px kind icon (terminal, chat, file, diff, history) then the name. Active: `--bg` fill and a 2px `--accent` underline, no grey pill. Close control on hover and on the active tab. Attention dot at left of the icon.
- Sidebar footer: a "Search commands" row with the palette key, a theme switch icon button (moon in light, sun in dark; "Switch to dark theme" / "Switch to light theme", also in the palette) and a Settings gear icon button. The switch stores an explicit light or dark choice; Settings brings back "system". Everything else (accounts, tool servers, SSH hosts, language servers, token saving, appearance, sign out) is inside Settings or the palette.
- Sidebar tabs (Files, Changes, Board): text only, `--muted`, the chosen one `--fg` with a 2px `--accent` underline and no fill. Counts are `--muted`.
- File tree git marks: a 16px letter after the name (A, ?, D, U, M, R, T) with its word as the accessible name and a `title`. New is `--success`, deleted and conflict `--danger`, modified, renamed and type change are `--fg` (amber is for attention only). A folder with changes inside shows a 6px `--muted` dot named "Contains changes". Ignored and hidden names are `--faint`.
- Tree fade: a 16px fade into `--surface` at the bottom of the list, drawn only while more rows wait below (`data-more`). At the end of the list it is gone, so the last row is whole and a short list never scrolls.
- Files/Chats handle: a focusable separator with its own 8px of height between the sections, so it never covers a row. On touch it is 44px high with a short grip. Up and Down move it (Shift for 64px), Home resets it.
- Filter: its result count ("3 matches", "No files match") is read out in a polite live region. The sidebar commit box says why Commit is off ("Stage a file to commit", "Write a message to commit") once one of the two is done.
- Settings: one dialog, 640px wide, section list on the left (on top on a phone). Sections that already have their own dialog open it from a button. Appearance holds theme (system, light, dark) and text size (small, default, large), stored per browser and applied before first paint.
- Dialogs: two widths, 480 and 640px, 12px radius, top edge at 10dvh, title row (16/24, 600), body, and a footer row with the main action on the right. `--shadow-dialog`, `--scrim` behind.
- New tab "+" opens a menu (Terminal, Chat, Open file); there is no blank tab. Split opens a terminal. Pane buttons show in the focused pane only.
- Empty states and pickers: centred, at most 560px wide.
- Pane header: 36px bar, `--surface`, bottom `--border`; title 14px 600, actions right-aligned, tertiary buttons.
- Status bar: 28px, `--surface`, top `--border`, 12px muted text; attention counts use `--attention`.
- Code block (chat): `--surface`, 1px `--border`, 8px radius, a 28px head with the language and a small tertiary Copy button; code 13/20 Geist Mono.
- Icons: 16px, 1.5 stroke, `currentColor`, round caps, inline SVG from `web/src/Icon.svelte` (paths in `web/src/lib/icons.ts`); always `aria-hidden` next to a name. No icon library.
- Panels (login card, dialogs): `--surface` or `--raised`, 1px `--border` on all four sides, 8px radius (dialogs 12), no shadow on login.

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
- Images in a chat. An attached image is a chip 264px wide: a 40px thumbnail, the name, the size and
  an x (full width in a box narrower than 520px, a container query). Dragging files over the box
  shows a dashed `--accent` edge on `--selected` with "Drop images to attach". A problem shows as a
  red edge and one line under the box (`role="alert"`) and goes when the chip is removed or sent.
  A sent image is a 96px thumbnail with its name; a missing one shows "Image not found" on `--hover`.
  The viewer is a `.dlg.wide` dialog with the name and size, Close and Esc. Thumbnails load from the
  same origin because the policy allows no blob: images.

## Copy voice
- Say what happened and what to do next, in plain words. "That token does not match this server."
- Name the thing (project, chat, agent), not "item" or "entry". Buttons are verbs: "Sign in", "Sign out".
- No exclamation marks, buzzwords, emoji or em dashes. Sentence case everywhere. Numbers only when real.
- Server unreachable is a state, not blame: "Mooring's server is not answering. Check that it is running."
