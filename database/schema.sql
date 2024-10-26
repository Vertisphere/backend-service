CREATE TABLE company (
    qb_company_id VARCHAR(50) PRIMARY KEY,
    qb_auth_code VARCHAR(255) NOT NULL,
    qb_bearer_token VARCHAR NOT NULL,
    qb_bearer_token_expiry TIMESTAMP NOT NULL,
    qb_refresh_token VARCHAR NOT NULL,
    qb_refresh_token_expiry TIMESTAMP NOT NULL,
    firebase_id VARCHAR(50) UNIQUE NULL
);

CREATE TABLE customer (
    qb_customer_id VARCHAR(50) PRIMARY KEY,
    firebase_id VARCHAR(50) UNIQUE NOT NULL
);