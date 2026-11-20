-- Création du rôle applicatif utilisé par le backend Go
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'binary_app') THEN
        CREATE ROLE binary_app WITH LOGIN PASSWORD 'change_me_in_prod';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE binary_db TO binary_app;
GRANT USAGE ON SCHEMA public TO binary_app;

-- Droits standards sur les tables évolutives
GRANT SELECT, INSERT, UPDATE ON users, sessions, kyc_documents, accounts, transactions TO binary_app;

-- Droits APPEND-ONLY stricts sur le grand livre et l'audit (Zéro UPDATE / Zéro DELETE)
GRANT SELECT, INSERT ON ledger_entries, audit_logs TO binary_app;
