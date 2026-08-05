# Deploying BitFinite Blockbook

Three files, no secrets. `deploy/bundle/` is gitignored — it is a staging tree the
release step assembles (the compiled `blockbook` binary plus a copy of `static/`),
not source.

| File | What it is |
|---|---|
| `bitfinite.json` | Blockbook coin config. **Template** — two placeholders, below. |
| `Dockerfile.runtime` | Ubuntu 24.04 + RocksDB/ZMQ shared libs, `COPY blockbook static`. |
| `docker-compose.yml` | The long-lived service definition. |

## Placeholders

`bitfinite.json` is committed as a template. Substitute both values when installing
it on the host — never commit the filled-in copy:

    sed -e "s/__RPC_USER__/$RPC_USER/" -e "s/__RPC_PASS__/$RPC_PASS/" \
        bitfinite.json > "$BLOCKBOOK_HOME/config/bitfinite.json"
    chmod 600 "$BLOCKBOOK_HOME/config/bitfinite.json"

Both must match `rpcuser` / `rpcpassword` in the node's `bitfinite.conf`. The RPC
endpoint is loopback-only (`127.0.0.1:19769`), so the credentials never cross the
network — but the config file holds them in cleartext, hence mode 600.

## Running it

`BLOCKBOOK_HOME` selects the host directory holding `config/` and the RocksDB
index in `data/`; it defaults to `/srv/blockbook`. Set it in `deploy/.env` (also
gitignored) or the environment:

    BLOCKBOOK_HOME=/srv/blockbook docker compose up -d

Two properties are deliberate and worth keeping:

- **`restart: unless-stopped`** — survives crashes and host reboots.
- **`stop_grace_period: 120s`** — RocksDB needs time to flush on stop. A fast
  SIGKILL can corrupt the index, which then means a full resync.

The container is `network_mode: host` and binds `127.0.0.1:9130` (internal) and
`127.0.0.1:9131` (public API). Neither is exposed directly; nginx fronts `:9131`
at `blockbook.bitfinitechain.org`.
