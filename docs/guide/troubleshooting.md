# Troubleshooting

## I cannot sign in

Find the message on the login page.

| Message | What to do |
| --- | --- |
| "That token does not match this server." | Run `mooring token` again and paste the new one. Running `mooring token -new` makes older tokens stop working. |
| "Too many wrong tokens. Try again in N seconds." | Wait, then try the right token. |
| "Wrong username or password." | Check both. If you forgot the password, run `mooring password` in a terminal on this computer. |
| "Too many wrong attempts. Try again in N seconds." | Wait. |
| "The password file could not be read. Sign in with mooring open and set the password again." | Run `mooring open`, then set the password in Settings. Or run `mooring password -remove`. |
| "That sign-in link was already used or has expired. Run mooring open for a new one." | A link works once and for 5 minutes. Run `mooring open`. |
| "That sign-in link is not valid. Run mooring open for a new one." | Run `mooring open`. |
| "Too many sign-in attempts. Wait a minute and try again." | Wait a minute. |
| "Too many sign-in links in the last 5 minutes. Wait a few minutes and run mooring open again." | Wait a few minutes. |
| "Mooring's server is not answering. Check that it is running." | Start Mooring with `mooring`. |

`mooring open` says "Mooring is not running at ADDR. Start it with `mooring`, or turn on autostart
with `mooring autostart on`." when it cannot find a server. If you run Mooring on another address,
pass the same `-addr` and `-data` to `mooring open`. If no browser opens, it prints a link. Open it
within 5 minutes.

## The browser does not fill in my saved password

A browser offers a password only if it saw you type it into the login form and submit it. Set the
password in Settings, sign out, then type it on the login page and accept the browser's offer to
save it. Setting a password in Settings, or with `mooring password`, does not give it to the
browser.

http://localhost:7340 and http://127.0.0.1:7340 are different sites to a browser. A password saved
for one is not offered on the other. Use the same address each time.

## The port is in use

Mooring serves on 127.0.0.1:7340. If another program has that port, the server will not start.
`mooring autostart on` says "ADDR is in use. Stop the Mooring that is running, then try again."

To find what has it, run `lsof -i :7340`. Stop that program, or start Mooring on another port with
`mooring -addr 127.0.0.1:7341`. Then pass the same `-addr` to `mooring open`, and to `mooring
autostart on` if you use it. A non-local address is refused unless you give `-tls-cert` and
`-tls-key` and have set a password. See [Open Mooring from another
device](getting-started.md#open-mooring-from-another-device).

## Another device gets 403 or a certificate warning

A 403 means the name in the address bar is not in the certificate. Open Mooring by a DNS name or IP
the certificate lists, such as the https address Mooring prints at start.

A browser warning about the certificate usually means the certificate came from `mkcert` and that
device does not have mkcert's root CA. Install the root CA on that device. A certificate from
`tailscale cert` needs no extra step.

## macOS will not open mooring

A tarball downloaded in a browser is quarantined, and macOS refuses to open `mooring`. Run this once
in the folder that has the binary:

```sh
xattr -d com.apple.quarantine mooring
```

Downloading with `curl` does not set the flag.

## An agent is not found

The chat picker says "Not installed. Mooring looks for the `X` command and did not find it."

1. Check that the command is installed. See the table in [Agent chats](agent-chats.md) for the
   command each agent needs.
2. Mooring looks on the PATH of the Mooring process. A terminal inside Mooring may have a different
   PATH than the one Mooring started with. Run `which X` in the terminal you started Mooring from.
3. If Mooring runs at login, `mooring autostart on` stores the PATH and SHELL from the moment you
   ran it. After you install an agent or change your PATH, run `mooring autostart on` again from a
   terminal where the command works.
4. Click "Check again" in the chat picker.

If the agent starts but says "X is not signed in.", sign in with the agent's own command, such as
`claude auth login` or `grok login`.

## A language server is not found

The bar over the editor says "X is not installed." Click "Install X" if it is offered, or install
it yourself and reopen the file. Mooring searches `<data folder>/lsp/bin`,
`<data folder>/lsp/node_modules/.bin`, your PATH, `$GOPATH/bin` and `~/.cargo/bin`. With
autostart on, the PATH problem is the same as for agents above. See [Language
servers](language-servers.md).

## Where the logs are

- `<data folder>/logs/mooring.log` is the server log, as JSON. It rotates at 10 MB and keeps the 3
  older files as `mooring.log.1` to `mooring.log.3`.
- `<data folder>/logs/autostart.log` has the output of Mooring when autostart starts it.

The data folder is `~/Library/Application Support/mooring` on macOS and `~/.config/mooring` on Linux,
unless you passed `-data`. Passwords, tokens and keys are never written to the log. `-quiet` hides
everything but warnings and errors on the terminal, and does not change the log file.

## The connection dropped

The status bar says "Reconnecting to the server… Changes are kept and sent when it is back." Wait for
"Connected". Terminals keep running on the server. If Mooring itself restarted, open terminals are
gone and the tab says so. See [Terminals](terminals.md).

## Remembered SSH passwords are off

Mooring logs "remembering SSH passwords is off" and asks each time. On Linux, install `secret-tool`
or use `-secrets-passphrase-file`. See [SSH](ssh.md).
