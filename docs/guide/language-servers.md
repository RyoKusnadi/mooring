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
| Cmd+Shift+Space (Ctrl+Shift+Space elsewhere) | Show signature help |
| Cmd+Shift+Up and Down | Previous and next signature |
| Alt+Shift+F | Format the file, when the server can |

"Format document" in the command palette does the same as Alt+Shift+F. The "Format" button over the
editor does too.

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
