CREATE TABLE IF NOT EXISTS company (
    qb_company_id VARCHAR(50) PRIMARY KEY,
    qb_auth_code VARCHAR(255) NOT NULL,
    qb_bearer_token VARCHAR NOT NULL,
    qb_bearer_token_expiry TIMESTAMP NOT NULL,
    qb_refresh_token VARCHAR NOT NULL,
    qb_refresh_token_expiry TIMESTAMP NOT NULL,
    firebase_id VARCHAR(50) UNIQUE NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
GRANT SELECT, INSERT, UPDATE, DELETE ON company TO PUBLIC;

CREATE TABLE IF NOT EXISTS customer (
    qb_customer_id VARCHAR(50) PRIMARY KEY,
    qb_company_id VARCHAR(50) REFERENCES company(qb_company_id),
    firebase_id VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

GRANT SELECT, INSERT, UPDATE, DELETE ON customer TO PUBLIC;

-- This is for if we just to make the table public so we don't have to give permissions to users or groups
-- In prod we want to use iam groups but right now we can't create because I have to do stupid gcp checklist for iam (billing and prod ready requirements)
-- for now we just set to pubilc
-- https://cloud.google.com/sql/docs/postgres/add-manage-iam-users
