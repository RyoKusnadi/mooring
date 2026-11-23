# SSH

Mooring can open a terminal and browse files on another computer over SSH. Saved hosts are in
Settings, under "SSH hosts". Press Alt+S to open the list. Keys and passwords stay in the secret
store, not in Mooring's database.

## Add a host

1. Open the host list and click "Add host".
2. Fill in the form.
   - Name: any label, 1 to 100 characters.
   - Address and Port (22 by default), and User.
   - Sign-in method: "Automatic (agent, keys, password)", "SSH agent only", "Key file" or
     "Password".
   - Key file: shows for the "Key file" method. The file must be inside your home folder on the
     computer Mooring runs on, such as `~/.ssh/id_ed25519`.
   - Root folder: the remote folder the file browser starts in and stays inside. Empty means the
     home folder.
   - "Forward my SSH agent": off is safer. With it on, anyone with admin rights on that host can use
     your keys while you are connected. Turn it on only for hosts you trust.
3. Click "Add and test". Mooring saves the host and tests the connection. If the test fails, the
   host stays saved. Fix it later with "Edit", or click "Test again". Click "Done" when it works.

To bring in hosts you already have, click "Import from SSH config". It reads `~/.ssh/config`, shows
the hosts, and you tick the ones to save. The file is not changed. A host that is already saved
says "Already saved. Importing adds a second entry." Click "Import N hosts".

## Connect

Each host row has "Terminal" and "Files". "Terminal" opens a terminal tab on the host. "Files"
opens its files in a tab. The command palette has "Open a terminal on NAME" and "Browse files on
NAME" for each host.

The first time you connect, Mooring shows the host's key type and fingerprint. Compare the
fingerprint with the one the owner of the host gave you, then click "Trust and connect" (or "Trust
and sign in" when it also asks for a password). "Don't connect" cancels. Mooring saves the key and
warns you if it changes later.

Mooring asks for a password, a key passphrase, or an answer to a server question in a prompt when it
needs one.

A host row also shows its state, "Password remembered" when one is saved, and "Agent forwarding
on". When a connection is open it adds "Forwards" and "Disconnect". "Edit" opens the form again.
"Remove" asks "Remove NAME? Open connections stay until closed."

If the connection to the host ends, its terminal says "The connection to the host ended." Click
"Reconnect". If you end the remote shell yourself (`exit`), it says "The shell exited.", or "The
shell exited with code N." when the code is not 0; "Reconnect" opens a new shell.

## Port forwards

On an open connection, click "Forwards".

1. Fill in "Local port", "Remote host" and "Remote port". A Local port of 0 picks any free port.
2. Click "Add forward".

The forward listens on 127.0.0.1 only and ends when the connection ends. "Stop" ends one forward.
"Back to hosts" returns to the list.

## Remembered passwords

When Mooring asks for a password or passphrase, "Remember on this computer" saves it. The box only
shows when a secret store is available. The text under it says "Kept in the secret store of the
computer Mooring runs on. Never in Mooring's database."

- On macOS, the store is the macOS keychain.
- On Linux, it is `secret-tool`.
- Anywhere, you can use a passphrase file. When you give one, Mooring uses it instead of the
  keychain or `secret-tool`. Passwords go into an encrypted file, `secrets.enc` in the data
  folder, locked by a passphrase you keep in a file only you can read.

To use a passphrase file:

1. Put a passphrase in a file, for example `~/.mooring-passphrase`. The whole file is the
   passphrase. A newline at the end is ignored.
2. Make it readable by you only: `chmod 600 ~/.mooring-passphrase`. Mooring refuses a file that
   others can read.
3. Start Mooring with `mooring -secrets-passphrase-file ~/.mooring-passphrase`. For autostart, pass
   the same flag to `mooring autostart on`.

If the file cannot be used, Mooring logs a warning, "remembering SSH passwords is off", and asks
for the password each time. With no keychain, `secret-tool` or passphrase file, the "Remember on
this computer" box does not show.

## Files on a host

The Files tab of a host lists its remote folders. Uploads and edits of remote files are limited to
10 MB per file. Mooring says "NAME is over 10 MB. Uploads are limited to 10 MB per file."

Next: [Language servers](language-servers.md).
