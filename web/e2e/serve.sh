#!/bin/sh
# Starts the server for browser tests (bin/mooring-e2e, built with -tags e2e by make build-e2e) with a stand-in agent behind the name "gemini" (and a copy behind "grok").
# The stand-in is the chat package's test binary, which runs as an ACP agent when
# MOORING_FAKE_AGENT=1 (see internal/fakeagent). Called from web/ by playwright.config.ts.
# Language servers are stand-ins too: the lsp package test binary runs as one when
# MOORING_FAKE_LSP=1 (see internal/fakelsp) and is on the PATH as "gopls" and
# "typescript-language-server". The server starts with -lsp-idle 3s and -lsp-memory-mb 150
# so tests can see an idle stop and a memory restart (a document with the text BLOAT
# makes the stand-in grow past 150 MB). Consent is not pre-seeded.
# A stand-in SSH server (the fakessh package test binary, MOORING_FAKE_SSH=1, see internal/fakessh)
# listens on 127.0.0.1:7342 with user "e2e" and password "e2e-pass", and also accepts the key
# $data/fakessh_client_key (public half next to it). Its host key is $data/fakessh_host_key, so the
# fingerprint is the same on every run. It runs a real sh on a PTY, with home $data/fakessh-home.
# Mooring starts with -secrets-passphrase-file $data/secrets-pass, so "remember" never touches the
# real keychain. A stand-in tool server (the fakemcp package test binary, MOORING_FAKE_MCP=1, see
# internal/fakemcp) is on the PATH as "fake-mcp" (stdio: tools echo and add) and also listens over
# HTTP with sign-in at http://127.0.0.1:7343/mcp (needs allowPrivate). All stand-ins stop with this script.
base=${MOORING_E2E_PORT:-7341}
sshport=$((base + 1))
mcpport=$((base + 2))
set -e
data=${MOORING_E2E_DATA:-.e2e-data}
mkdir -p "$data/fakebin" "$data/fakestate"
(cd .. && go test -c -o "web/$data/fakebin/fakeagent" ./internal/chat)
cat > "$data/fakebin/gemini" <<WRAP
#!/bin/sh
export MOORING_FAKE_AGENT=1 FAKE_RESUME=1 FAKE_STREAM_TOOL=1 FAKE_CAPS=additionalDirectories,delete FAKE_STATE="$PWD/$data/fakestate"
exec "$PWD/$data/fakebin/fakeagent" "\$@"
WRAP
chmod +x "$data/fakebin/gemini"
# A second stand-in agent behind the name "grok", for tests that need two agents (second opinion).
# It also lists and reopens its own sessions (the import-a-chat test).
cat > "$data/fakebin/grok" <<WRAP
#!/bin/sh
export MOORING_FAKE_AGENT=1 FAKE_RESUME=1 FAKE_STREAM_TOOL=1 FAKE_CAPS=list,additionalDirectories FAKE_LOAD=1 FAKE_STATE="$PWD/$data/fakestate"
exec "$PWD/$data/fakebin/fakeagent" "\$@"
WRAP
chmod +x "$data/fakebin/grok"
(cd .. && go test -c -o "web/$data/fakebin/fakelsp" ./internal/lsp)
for name in gopls typescript-language-server; do
  cat > "$data/fakebin/$name" <<WRAP
#!/bin/sh
export MOORING_FAKE_LSP=1
exec "$PWD/$data/fakebin/fakelsp" "\$@"
WRAP
  chmod +x "$data/fakebin/$name"
done
(cd .. && go test -c -o "web/$data/fakebin/fakessh" ./internal/fakessh)
mkdir -p "$data/fakessh-home"
[ -f "$data/secrets-pass" ] || { printf 'e2e-secrets-passphrase\n' > "$data/secrets-pass"; chmod 600 "$data/secrets-pass"; }
MOORING_FAKE_SSH=1 FAKE_SSH_ADDR=127.0.0.1:$sshport FAKE_SSH_HOSTKEY="$PWD/$data/fakessh_host_key" \
  FAKE_SSH_CLIENTKEY="$PWD/$data/fakessh_client_key" FAKE_SSH_USER=e2e FAKE_SSH_PASSWORD=e2e-pass \
  FAKE_SSH_HOME="$PWD/$data/fakessh-home" FAKE_SSH_SHELL=sh "$PWD/$data/fakebin/fakessh" &
fake=$!
# A leftover stand-in on the port would make the tests talk to the wrong one: stop here instead.
sleep 0.5
kill -0 $fake 2>/dev/null || { echo "serve.sh: the stand-in SSH server did not start (is 127.0.0.1:$sshport taken?)" >&2; exit 1; }
(cd .. && go test -c -o "web/$data/fakebin/fakemcp" ./internal/fakemcp)
cat > "$data/fakebin/fake-mcp" <<WRAP
#!/bin/sh
export MOORING_FAKE_MCP=1
exec "$PWD/$data/fakebin/fakemcp" "\$@"
WRAP
chmod +x "$data/fakebin/fake-mcp"
MOORING_FAKE_MCP=1 FAKE_MCP_HTTP_ADDR=127.0.0.1:$mcpport "$PWD/$data/fakebin/fakemcp" &
fakemcp=$!
sleep 0.5
kill -0 $fakemcp 2>/dev/null || { echo "serve.sh: the stand-in tool server did not start (is 127.0.0.1:$mcpport taken?)" >&2; exit 1; }
# The stand-in agent has accounts: FAKE_CONFIG is its own login's folder (signed in); managed
# accounts get their own folder and start signed out.
mkdir -p "$PWD/$data/fake-login" && touch "$PWD/$data/fake-login/signed-in"
[ -z "$MOORING_E2E_HOME" ] || export HOME="$MOORING_E2E_HOME"
# The test folders sit inside this repo: stop git there, so checkpoints and status never see it.
GIT_CEILING_DIRECTORIES="$PWD/$data" FAKE_CONFIG="$PWD/$data/fake-login" PATH="$PWD/$data/fakebin:$PATH" ../bin/mooring-e2e -fake-account-agent gemini -addr 127.0.0.1:$base -data "$PWD/$data" -shell "$PWD/e2e/shell.sh" -lsp-idle 3s -lsp-memory-mb 150 -secrets-passphrase-file "$data/secrets-pass" &
app=$!
trap 'kill $fake $fakemcp $app 2>/dev/null' EXIT INT TERM
wait $app
