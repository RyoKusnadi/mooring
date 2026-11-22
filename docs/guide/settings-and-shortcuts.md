# Settings and shortcuts

Open Settings with the "Settings" button in the sidebar, or Cmd+, (Ctrl+, elsewhere). Click "Done"
to close it. "Sign out" is at the bottom.

## Settings

Four sections open a dialog of their own. They each have an "Open" button.

| Section | What it holds |
| --- | --- |
| Accounts | Sign in to an agent provider, add a second login, see each login's limits and usage. See [Agent chats](agent-chats.md). |
| Tool servers | MCP servers your agents can use: add one from the catalog, check it, and choose who may use it. |
| SSH hosts | Saved hosts for remote terminals and files. See [SSH](ssh.md). |
| Language servers | Programs that give the editor errors, hover text and go to definition. See [Language servers](language-servers.md). |

### Sign-in

Set, change or remove a username and password, and "Sign out other browsers". See [Getting
started](getting-started.md).

### Token saving

These settings cut the number of tokens Mooring spends. They are saved on the server.

| Setting | Default | What it does |
| --- | --- | --- |
| Shorten long command output in new chats | Off | New chats start with long command output shortened. |
| Name chats with a small model | Off | Uses a small model to title chats. |
| Suggest commit messages | Off | Adds a "Suggest message" button to the commit box. |
| Daily token limit for small jobs | 200,000 | Whole number from 1,000 to 100,000,000. Chats are not counted. |
| Small model per agent | None set | Pick a model for each agent. "Find models" asks the agent for its list, and needs an open project because it starts the agent in that folder. A model Mooring suggests is tagged "(suggested)". |

Without a small model for an agent, that agent does no small jobs. A bad daily limit shows "The
daily limit is a whole number from 1,000 to 100,000,000."

### Notifications

- "Allow notifications" turns on browser notifications. The page says whether they are on in this
  browser, blocked, or not supported. The notification names the project only.
- Once they are on, "Send a test notification" shows one straight away. If none appears, check this
  browser in your system's notification settings, and turn off Do Not Disturb or Focus.
- "A terminal bell counts as needing you" is on by default. It is saved in this browser.
- A phone gets no notifications while Mooring is closed.

### Appearance

Saved in this browser only.

| Setting | Choices |
| --- | --- |
| Theme | "Same as the system", "Light", "Dark" |
| Text size | "Small", "Default", "Large" |

On a phone or tablet the Text size setting is ignored. The sidebar also has a theme button, which
reads "Switch to dark theme" or "Switch to light theme".

## The command palette

Press Cmd+K (Ctrl+K elsewhere), or Cmd+Shift+P, or click "Search commands" in the sidebar. On a
phone use the "Commands" button. Type to filter, use Up and Down, and press Enter to run.

It lists these:

- Always: Settings, Sign out, Search chat history, Add project, the theme switch, Tool servers, Accounts and usage,
  SSH hosts and Language servers.
- With a project open: New terminal, New chat, Open file by name, Search text in project, Show
  changes, Show history, Refresh changes, Split right, Split down, Switch branch, New tab menu,
  Close tab, Leave the terminal for its tab, Focus next pane, Move tab to next pane and Move tab
  to a new pane on the right.
- "Format document", when the open file has a formatter. "Find references" and "Rename symbol", when it has a
  running language server.
- Each saved SSH host, each worktree and each project ("Switch to project NAME") add rows.

## Keyboard shortcuts

On a Mac, Alt shows as ⌥, Shift as ⇧ and Ctrl as ⌃. Mooring never reads Cmd+W or Cmd+T, because the
browser keeps them.

### Browser-style keys

Use Cmd on a Mac and Ctrl elsewhere.

| Key | What it does |
| --- | --- |
| Cmd+K | Command palette |
| Cmd+Shift+P | Command palette |
| Cmd+P | Open file by name |
| Cmd+Shift+F | Search text in project |
| Cmd+, | Settings |
| Cmd+S | Save the file in the editor |

Elsewhere, Ctrl+P and Ctrl+, go to a terminal that has focus, because shells use them.

In a terminal, Cmd+F finds in the terminal's output (Ctrl+Shift+F elsewhere). Cmd+Shift+F still searches
the project on a Mac; use Alt+F for that from a terminal elsewhere. Outside a terminal Cmd+F is the browser's.

### Alt keys

| Key | What it does |
| --- | --- |
| Alt+N | New terminal |
| Alt+A | New chat |
| Alt+T | New tab menu |
| Alt+O | Open file by name |
| Alt+F | Search text in project |
| Alt+G | Show changes |
| Alt+H | Show history |
| Alt+S | SSH hosts |
| Alt+M | Tool servers |
| Alt+Y | Search chat history |
| Alt+P | Show problems |
| Alt+W | Close tab |
| Alt+Right, Alt+Left | Next and previous tab |
| Alt+Down, Alt+Up | Next and previous pane |
| Alt+Shift+Right, Alt+Shift+Left | Move the tab to the next or previous pane |
| Alt+\ | Split right |
| Alt+- | Split down |
| Alt+Shift+F | Format document, when the server can |
| Alt+Esc | Leave the terminal for its tab |

In a text box, Alt plus the arrows and Alt+Esc keep their editing meaning.

### File tree

| Key | What it does |
| --- | --- |
| Up, Down, Home, End | Move |
| Right, Left | Expand and collapse a folder |
| Enter | Keep a previewed file open |
| F2 | Rename |
| M | Move |
| Delete | Delete |

### Changes view

| Key | What it does |
| --- | --- |
| Up, Down, Home, End | Move through files and show each diff |
| Space | Stage or unstage the file |
| Delete | Discard |
| Cmd+Enter | Commit, in the message box |

### Chats, search and editor

| Key | What it does |
| --- | --- |
| Enter | Send a chat message (not on a touch screen) |
| Shift+Enter | New line in a chat message |
| Up, Down, Enter, Esc | Move, open and close in the find dialog |
| Alt+R | In the search panel: go to "Replace with" (on a Mac, not from the search box) |
| F12 | Go to definition |
| Shift+F12 | Find references |
| F2 | Rename the name under the cursor |
| Cmd+Shift+Space (Ctrl+Shift+Space elsewhere) | Signature help |
| Cmd+Shift+Up and Down | Previous and next signature |
| Esc, then Tab | Leave the editor with the keyboard |
| Esc | Close the drawer on a phone |

Next: [Troubleshooting](troubleshooting.md).
