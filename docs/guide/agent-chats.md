# Agent chats

An agent chat is a tab where you talk to a coding agent that works in your project folder. Mooring
talks to agents over ACP. Mooring does not ship an agent or store your login to one. You install the
agent, and Mooring starts it.

## Supported agents

| Agent | Mooring runs | Needs on PATH | Sign in |
| --- | --- | --- | --- |
| Claude | `npx -y` with the Claude ACP adapter | `npx` | `claude auth login` |
| Gemini | `gemini --acp` | `gemini` | |
| Codex | `npx -y @agentclientprotocol/codex-acp` | `npx` and `codex` | |
| Grok | `grok agent stdio` | `grok` | `grok login`, or set `XAI_API_KEY` |
| Cursor | `agent acp` | `agent` | |
| Antigravity | `agy acp` | `agy` (Google Antigravity CLI 1.5 or later) | Run `agy` once, or `agy acp --login` with no browser |

Mooring finds each program on the PATH of the Mooring process. If you start Mooring from a
terminal, that is your terminal's PATH. With autostart, `mooring autostart on` keeps the PATH from
the moment you ran it, so run it again after you install a new tool. See
[Troubleshooting](troubleshooting.md) if an agent is not found.

## Start a chat

1. Open a project, then press Alt+A, or click "+" in a pane and choose "Chat".
2. Under "Start a chat", pick an agent. The picker says it starts in the project folder and asks
   first where the agent supports it.
3. Click "Start chat".

![The Start a chat picker: Claude and Gemini can start, Codex, Grok, Cursor and Antigravity say Not installed with Install buttons.](images/agent-chats-picker.png)

If the agent is missing, the picker says "Not installed. Mooring looks for the `X` command and did
not find it. Install NAME, then check again." Click "Install NAME" to open a terminal, install it,
then click "Check again".

Options in the picker:

- "Work in its own worktree" starts the chat in a new git worktree. "Branch name (optional)" left
  empty makes a branch that starts with `mooring/`. See [Git](git.md).
- "Tool servers for this chat" lists your enabled tool servers. Tick the ones this chat may use.
  "Set up tool servers" opens the tool server list. See below.
- If you have chats from before, "Start with X" picks the same agent again.

## Talk to the agent

Type in the box labelled "Message to AGENT" (placeholder "Ask or tell it what to do"). Enter sends.
Shift+Enter starts a new line. On a touch screen Enter makes a new line instead.

- "Send" sends the message.
- While the agent replies, the button reads "Send after reply". Your message waits in a queue
  labelled "Sends when the reply ends". "Take back" removes it.
- "Stop reply" stops the agent mid-reply.

The model and mode selects, when the agent offers them, are above the box. If you pick a mode that
may run commands and edit files without asking, Mooring asks "This mode may let AGENT run commands
and edit files without asking you first. Switch anyway?" Click "Switch mode" or "Keep asking". If
an agent has no mode that asks before it acts, the chat warns that it may run commands without
asking.

The "Chat options" menu has "Shorten long output", "New chat from summary", a context meter
("Context N of M") and "Cost so far". "New chat from summary" opens a draft of the summary, up to
16 KB, that you can edit before starting a new chat with it. The new chat can use another agent: the
picker offers "Start with" the same agent and says "Pick another below to switch".

## Permission prompts and Attention

When the agent wants to run a command or edit a file, the chat shows "Needs permission" with a
button for each option the agent gives. If several are waiting it says "N more waiting".

![A chat with a Needs permission bar over the message box, showing the command npm test with Allow and Reject buttons. The status bar says 1 chat needs permission.](images/agent-chats-permission.png)

A prompt, a question, or a stopped chat counts as Attention. The chat shows a mark in the sidebar,
the status bar says how many chats need you, the browser tab title reads "(n) Mooring" and the page
icon gets an amber dot. To get a browser notification when you are in another tab, open Settings,
Notifications and click "Allow notifications". The notification names the project only.

## Sessions

Every chat is saved as a Session in the "Chats" list of its project. The list shows each chat's
state: Starting, Replying, Not signed in or Stopped.

- Closing a chat tab stops its agent. The saved chat stays in the list. Click it to open it again.
- A chat that is not running says "X is not running. The messages are saved. Resume to keep going."
  Click "Resume chat".
- Click the pencil to rename a chat. Click the cross to delete it: "Delete this chat?" then
  "Delete" or "Keep".
- "Filter chats" appears once a project has more than 5.

## Attach files, lines and output

- In the editor, select lines and click "Ask the agent about these lines".
- In a terminal, select text and click "Send selection to chat".
- In a chat, a command's output can be added with "Send full output". "Show raw output" switches
  the view.

These appear as chips above the box. A message takes at most 8 attachments. Attached lines and
terminal text are limited to 64 KB. With several chats open, Mooring asks "Send to which chat?"

## Accounts and Providers

An Account is one login of a Provider (an agent), kept in its own settings folder. Mooring never
reads or stores the login. Only Claude supports separate accounts today.

To add an account:

1. Open Settings and click "Open Accounts", or choose "Accounts and usage" in the command palette.
2. Under the agent, click "Add Claude account". Type a Name such as "Work".
3. Choose "Mooring makes a folder" or "Use a folder I already have" and pick the folder, which
   must be inside your home folder.
4. Click "Add account". A new account starts signed out. Click "Sign in" on its row.

Each row has Sign in, Rename, Make default and Remove. Tags show "Default for new projects", "The
agent's own login", "Folder managed by Mooring" and "Your folder". Usage shows "N% used" per
window, "Limit reached, ..." or "Usage: unknown." Usage comes from what the agent reports while you
chat.

Each project can use a different account. Pick one under "X account for this project". Changing it
restarts that agent's open chats in the project.

When a chat hits a limit, it says "The usage limit of X was reached." with "Switch to Y" or "Open
accounts". When an account is not signed in it says "X is not signed in." with "Sign in to X" and
"Resume chat".

## Tool servers

A tool server is an MCP server that gives agents extra tools. Nothing runs until you allow it.

To add one from the catalog:

1. Open Settings and click "Open Tool servers", press Alt+M, or use the command palette.
2. Under "Find a tool server", search by name or description. Click "Check for a newer catalog" to
   refresh it, or "Show more" for more results.
3. Click "Add" beside the way you want to run it. Ways are npx, uvx, docker, Remote (http) and
   Local command.
4. On the "Allow this tool server?" screen, read the name, type, command and arguments (or address). Fill the value
   fields. They go to the secret store, not Mooring's database. Click "Allow and enable". "Back"
   goes back.

To add your own, use "Add your own" and fill "How it runs" (a command on this computer, a remote
address over http, or a remote address over sse), Name, Command with "Arguments (optional)", or
Address. "Allow a private address" lets a remote address on your own network through. Click
"Review", then allow it as above.

![The Allow this tool server screen for a local command named Notes, with Back and Allow and enable buttons.](images/agent-chats-tool-consent.png)

To check a server, click "Check". Mooring runs it and lists its tools. The first run of an npx,
uvx or docker server downloads it and can take up to 2 minutes. Later checks stop after 30
seconds. A remote check also stops after 30 seconds. Click "Check again" to retry. The row says one
of "Not allowed yet", "Enabled", "Check failed", "Needs a check", "Checked, not enabled", "Needs
sign-in", "Sign-in failed" or "Signed in".

Other row buttons: "Review and allow", Enable or Disable, Edit, and Remove. Remove deletes its saved
values and sign-in. Chats already running keep the server until they stop.

To sign in to a remote server, click "Sign in". Click "Open the sign-in page", which opens a new
tab. On the same computer it finishes by itself. If your browser is on another computer, paste the
address of the page it ended on into "Address of the page it ended on" and click "Finish sign-in".
"Sign out" removes the sign-in.

A chat gets the servers you tick under "Tool servers for this chat" when you start it.

Next: [Git](git.md).
