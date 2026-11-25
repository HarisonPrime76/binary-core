CREATE TABLE aml_alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    transaction_id UUID REFERENCES transactions(id),
    rule_triggered VARCHAR(100) NOT NULL, -- STRUCTURING_24H, HIGH_VALUE_TX, RAPID_VELOCITY
    severity VARCHAR(20) NOT NULL DEFAULT 'medium', -- low, medium, high, critical
    details JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) NOT NULL DEFAULT 'open', -- open, investigating, dismissed, confirmed_fraud
    reviewed_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aml_alerts_status ON aml_alerts(status);
CREATE INDEX idx_aml_alerts_user_id ON aml_alerts(user_id);

GRANT SELECT, INSERT, UPDATE ON aml_alerts TO binary_app;
