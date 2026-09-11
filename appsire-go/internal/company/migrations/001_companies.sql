CREATE TABLE IF NOT EXISTS companies (
    id INTEGER PRIMARY KEY,
    ruc TEXT NOT NULL UNIQUE,
    business_name TEXT NOT NULL,
    sol_username TEXT NOT NULL,
    sol_password BLOB NOT NULL,
    client_id TEXT NOT NULL,
    client_secret BLOB NOT NULL,
    is_selected INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_companies_selected ON companies(is_selected);
