# Security

## What Mooring is allowed to do

Mooring runs commands as you, by design. A signed-in browser can open a terminal, start AI coding
agents that run commands and edit files, connect over SSH, start language servers and tool
servers, and read and write any file your user account can. Anyone who signs in to Mooring has the
same power as someone sitting at your shell. Treat the login token and the password like an SSH key.

Reports that amount to "a signed-in user can run a command" are not vulnerabilities. They are the
product.

## How access is limited

- Mooring listens on loopback only (127.0.0.1 or ::1). It refuses an `-addr` that is not a
  loopback address. There is no TLS yet, so it is not meant to be reached over a network. If you
  forward the port yourself (an SSH tunnel, a reverse proxy), the protection is whatever you add.
- Every request checks the Host header, which blocks DNS rebinding. Requests that change
  something check Origin. WebSockets use the same check.
- Sign-in is by a login token kept in the data folder, or by one username and password if you set
  one. `mooring open` prints a link that works once and for 5 minutes. A sign-in cookie is
  HttpOnly and SameSite=Strict, and lasts 30 days. Failed logins are limited to 5 per minute.
- Remembered SSH passwords go to the macOS keychain, `secret-tool` on Linux, or a file encrypted
  with a passphrase you give. Tokens, passwords and cookies are not written to logs or to the
  database.
- Any other process running as you on the same machine can read the data folder and try the
  loopback port. Mooring does not defend against a hostile local user or against malware that
  already runs as you.

## Out of scope

- Running commands, editing files or opening SSH connections as a signed-in user.
- Attacks that need access to your account, your data folder or your browser profile.
- Exposing Mooring beyond loopback with your own tunnel or proxy, and what that exposes.
- Bugs in the agents, tool servers, language servers or SSH hosts that Mooring starts or talks
  to. Report those to their authors.
- Missing macOS signing or notarisation of the release binaries.
- Findings with no working steps, such as scanner output alone.

## Report a vulnerability

Use GitHub's private vulnerability reporting: open the repository's Security tab and choose
"Report a vulnerability". Please do not open a public issue or pull request for it.

Include the version (`mooring version`), your OS, and the steps to reproduce. Mooring is
maintained by one person, so replies can take a few days. A fix ships in the next release, and
the report gets credit if you want it.

## Supported versions

Only the latest release gets fixes.
