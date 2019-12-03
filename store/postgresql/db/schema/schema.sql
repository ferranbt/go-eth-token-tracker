
CREATE TABLE tokens (
    id TEXT PRIMARY KEY
);

CREATE TABLE transfers (
    token_id        TEXT REFERENCES tokens(id),
    block_hash      TEXT,
    txn_hash        TEXT,
    from_addr       TEXT,
    to_addr         TEXT,
    value           TEXT
);
