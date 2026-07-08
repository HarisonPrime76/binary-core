-- 1. Table des Comptes (Wallets)
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    account_type VARCHAR(20) NOT NULL, -- 'customer', 'merchant', 'system_fees', 'escrow'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Table des Transactions (Le conteneur logique)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL, -- Sécurité anti-double clic
    status VARCHAR(20) NOT NULL, -- 'pending', 'completed', 'failed'
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Table des Écritures Comptables (Les mouvements d'argent réels)
CREATE TABLE postings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT,
    account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT,
    amount NUMERIC(18, 4) NOT NULL, -- Négatif pour un Débit, Positif pour un Crédit
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

--4 . Table des Utilisateurs (Users)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'rejected'
    kyc_reference VARCHAR(255), -- ID de transaction chez Sumsub/Onfido
    public_key_biometric TEXT, -- Pour la connexion FaceID/TouchID
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);