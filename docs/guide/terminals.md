# Terminals

A terminal runs your login shell (`$SHELL`, or the program you pass with `-shell`) on the computer
Mooring runs on. It opens in the project folder.

## Open a terminal

To open a terminal:

1. Open a project.
2. Press Alt+N, or click "+" in a pane and choose "Terminal", or use "New terminal" from the
   command palette.

The tab is titled "Terminal" until the shell reports a name. The tab shows "Starting the shell…"
for a moment.

To open one in a worktree, use "New terminal in worktree BRANCH" from the command palette. To open
one on another computer, see [SSH](ssh.md).

## Tabs and panes

- Alt+Right and Alt+Left move to the next and previous tab. Alt+W closes the tab.
- Alt+\ splits the pane to the right, Alt+- splits it down. Alt+Down and Alt+Up move between panes.
- Alt+Shift+Right and Alt+Shift+Left move a tab to the next or previous pane.
- Right-click a tab for Close, Close others, Close to the right, Copy path, Copy full path, Reveal
  in files, and Pin or Unpin.

On a Mac the keys read ⌥ for Alt. The full list is in [Settings and
shortcuts](settings-and-shortcuts.md).

Inside a terminal, keys go to the shell. Press Alt+Esc (⌥Esc) to leave the terminal and use the
shortcuts. A strip under the terminal says "Alt+Esc leaves the terminal. ⌘K opens commands."

## Phone keys

On a narrow window or a touch screen, a "Terminal keys" bar shows Ctrl, Esc, Tab and the arrows.
Tap Ctrl first to send a control key.

## Send terminal text to a chat

Select text in the terminal and click "Send selection to chat". The text goes into your agent chat
as an attachment. See [Agent chats](agent-chats.md). Up to 64 KB can go at once.

## Attention

A terminal asks for attention when the program in it rings the bell (the BEL character) or sends a
desktop-notification escape sequence. A terminal rings at most once every 10 seconds. The terminal
tab gets a mark, the status bar says "N terminals need you", the browser tab title reads "(n)
Mooring", and the page icon gets an amber dot. The mark clears when you look at the terminal.

To ignore bells, open Settings, Notifications and turn off "A terminal bell counts as needing you".
A notification escape sequence still counts.

To get browser notifications, open Settings, Notifications and click "Allow notifications". The
notification names the project only.

## When the connection drops

- A terminal keeps running on the server when you close the browser tab or lose the connection.
  Open Mooring again and the terminal is there. Mooring replays the last 256 KiB of output.
- While the page reconnects the terminal says "Reconnecting to the terminal…".
- If another window has the terminal open at a different size, you see "Sized by another window: C
  × R. Type here to take over the size."
- Terminals do not survive a restart of Mooring. After a restart the tab says "This terminal is
  gone. Terminals do not survive a restart of Mooring." Click "Start a new terminal".
- When the shell exits, the tab says "The shell exited." and adds " with code N" when the code is not 0. It offers "Start a new
  terminal".
- If the terminal could not start, you see "The terminal could not start." with a reason and a
  "Close tab" button.
- An SSH terminal that loses its host says "The connection to the host ended" and offers
  "Reconnect".

Next: [Agent chats](agent-chats.md).
