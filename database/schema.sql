CREATE TABLE franchise (
    franchise_id SERIAL PRIMARY KEY,
    franchise_name VARCHAR(100) NOT NULL,
    headquarters_address VARCHAR(255),
    phone_number VARCHAR(20),
    admin_account_id VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX admin_account_id_index ON franchise USING HASH (admin_account_id);

CREATE TABLE franchisee (
    franchisee_id SERIAL PRIMARY KEY,
    franchise_id INT REFERENCES franchise(franchise_id),
    franchisee_name VARCHAR(100) NOT NULL, -- Store name or specific identifier for the franchisee
    headquarters_address VARCHAR(255),
    phone_number VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app_user (
    user_id SERIAL PRIMARY KEY,
    account_id VARCHAR UNIQUE NOT NULL,
    franchise_id INT REFERENCES franchise(franchise_id), -- Nullable if the user is not tied to a specific franchise
    franchisee_id INT REFERENCES franchisee(franchisee_id) NULL, -- nullable for franchisers
    -- 0 for franchisee_non_admin, 1 for franchisee_admin, 2 for franchiser_non_admin, 3 for franchiser_admin
    role INT NOT NULL,
    name VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (role IN (0, 1, 2, 3))
);
CREATE INDEX account_id_index ON app_user USING HASH (account_id);

CREATE TABLE product (
    product_id SERIAL PRIMARY KEY,
    franchise_id INT REFERENCES franchise(franchise_id),
    product_name VARCHAR(100) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    product_status INT DEFAULT 0, -- 0 for active, 1 for inactive
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE invoice (
    invoice_id SERIAL PRIMARY KEY,
    franchise_id INT REFERENCES franchise(franchise_id), -- Franchise for which the invoice is created
    franchisee_id INT REFERENCES franchisee(franchisee_id), -- Invoice belongs to a franchisee
    created_by INT REFERENCES app_user(user_id), -- User who created the invoice
    total_amount DECIMAL(15, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- 'pending' or 'paid'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('pending', 'paid'))
);

CREATE TABLE orders (
    order_id SERIAL PRIMARY KEY,
    franchise_id INT REFERENCES franchise(franchise_id),
    franchisee_id INT REFERENCES franchisee(franchisee_id), -- Franchisee placing the order
    created_by INT REFERENCES app_user(user_id), -- User who placed the order
    invoice_id INT REFERENCES invoice(invoice_id) NULL, -- Nullable, linked once invoiced
    status VARCHAR(20) DEFAULT 'ordered', -- Status: ordered, preparing, ready, picked up, invoiced
    total DECIMAL(15, 2) DEFAULT 0 NOT NULL, -- Total cost of all items in the order
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    preparing_at TIMESTAMP NULL, -- Timestamp when status changed to 'preparing'
    ready_at TIMESTAMP NULL, -- Timestamp when status changed to 'ready'
    picked_up_at TIMESTAMP NULL, -- Timestamp when status changed to 'picked up'
    invoiced_at TIMESTAMP NULL, -- Timestamp when status changed to 'invoiced'
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('ordered', 'preparing', 'ready', 'picked up', 'invoiced'))
);
CREATE TABLE order_product (
    order_product_id SERIAL PRIMARY KEY,
    order_id INT REFERENCES orders(order_id) ON DELETE CASCADE,
    product_id INT REFERENCES product(product_id) ON DELETE CASCADE,
    quantity INT NOT NULL, -- Quantity of the product in the order
    price DECIMAL(10, 2) NOT NULL, -- Price per item at the time of order (can differ from base product price)
    discount DECIMAL(10, 2) DEFAULT 0, -- Any discount applied to this product in the order
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);