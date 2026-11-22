# Language servers

A language server is a program that gives the editor errors, hover text, completions and go to
definition for one language. Mooring starts one when you open a file of that language.

## What is supported

| Language | Program | File endings |
| --- | --- | --- |
| Go | `gopls serve` | .go |
| TypeScript and JavaScript | `typescript-language-server --stdio` | .ts .tsx .js .jsx .mjs .cjs .mts .cts |
| Python | `pyright-langserver --stdio` | .py |
| Rust | `rust-analyzer` | .rs |
| C and C++ | `clangd` | .c .h .cc .cpp .hpp .cxx |
| Java | `jdtls` | .java |
| Lua | `lua-language-server` | .lua |
| Shell | `bash-language-server start` | .sh .bash |
| HTML | from `vscode-langservers-extracted` | .html .htm |
| CSS, SCSS and Less | from `vscode-langservers-extracted` | .css .scss .less |
| JSON | from `vscode-langservers-extracted` | .json .jsonc |

Mooring looks for the program in `<data folder>/lsp/bin` and `<data folder>/lsp/node_modules/.bin`,
then on your PATH, then in `$GOPATH/bin` and `~/.cargo/bin`.

## Set one up

1. Open a file of that language. If the server is missing, a bar above the editor says "X is not
   installed. Install it for completions, errors and go to definition in .ext files."
2. Click "Install X". Mooring installs it under `<data folder>/lsp`. Click "Not now" to skip.
3. Go, TypeScript and JavaScript, Python, Rust, Shell, HTML, CSS and JSON have an "Install X"
   button. Go needs `go` on your PATH and Rust needs `rustup`; without them the bar says the tool
   "is needed for this install and was not found on the PATH." C and C++, Java and Lua have no
   one-click install. The bar shows a hint instead, for example for clangd: "Install clangd with
   your package manager (for example brew install llvm), then reopen the file."
4. The first time, the bar says "Mooring wants to run COMMAND for .ext files." Click "Show full
   command" to read it. Click "Allow and start" to run it, or "Not now".

After that the editor shows errors in the gutter, hover text, completions and go to definition.
Files over 2 MB get no language server.

## Keys in the editor

| Key | What it does |
| --- | --- |
| F12 | Go to definition |
| Shift+F12 | Find references |
| F2 | Rename the name under the cursor |
| Cmd+Shift+Space (Ctrl+Shift+Space elsewhere) | Show signature help |
| Cmd+Shift+Up and Down | Previous and next signature |
| Alt+Shift+F | Format the file, when the server can |

"Format document" in the command palette does the same as Alt+Shift+F. The "Format" button over the
editor does too.

## Find references

Put the cursor on a name and press Shift+F12, click "References" over the editor, or choose "Find
references" in the command palette. The dialog lists every use the server finds, grouped by file,
with the line and the name marked. Click a row, or move to it and press Enter, to open the file at
that line. If the server finds none, or cannot look them up, the dialog says so.

## Rename

Press F2 on a name, click "Rename" over the editor, or choose "Rename symbol" in the command
palette. Type the new name and click "Preview". The next step reads, for example, "Rename in 3
files, 7 places" and lists each file with its count. Click "Rename" to make the change, or
"Cancel" to leave everything as it is. An empty name, or the same name, is refused.

Files open in an editor change there and stay unsaved, so Cmd+Z (Ctrl+Z elsewhere) undoes the
rename. Every other file is saved, and only if it has not changed on disk since Mooring read it.
When the rename ends, the dialog lists the files it changed and any it could not, each with the
reason. If a file with unsaved changes is open in a tab that is not showing, the whole rename is
refused before anything changes; save that file or show its tab, then try again. A server edit that
creates, moves or deletes files is refused too.

## Problems

When a running server reports errors or warnings, the status bar at the bottom shows a count such
as "2 errors, 5 warnings". Click it, press Alt+P, or choose "Show problems" in the command palette
to open the Problems list for the open project. Errors come first, then warnings, information and
hints. Each row gives the severity, the message, `file:line:column` and the server that reported
it. Click a row to open the file at that line. The list shows the first 500 problems and says how
many more there are. With no running server, or nothing to report, the count is hidden and the
list says so.

## The Language servers dialog

Choose "Language servers" in the command palette or Settings. It lists each server with its state:
Ready, Waiting for your permission, Not installed or Installing. Buttons are "Install" and "Allow".
"Running now" shows memory use and tab count with a "Stop" button. Click "Done" to close.

To add a server that is not in the list, use "Add a server":

1. Type a Name.
2. Type File endings, such as `.rs, .rlib`.
3. Type the Command.
4. Click "Add server". Mooring asks before it runs the command for the first time.

A server you added has a "Remove" button.

## When a server stops

The bar says "X stopped." If it crashed three times in two minutes, it adds "It crashed three
times in two minutes." Click "Start again" or "Hide". "X was stopped." means Mooring stopped it.

Mooring stops a server after it has had no editor open for 10 minutes, and restarts one that uses
more than 2048 MB. Change these with the `-lsp-idle` and `-lsp-memory-mb` flags.

If a server is not found, see [Troubleshooting](troubleshooting.md).

Next: [Settings and shortcuts](settings-and-shortcuts.md).
