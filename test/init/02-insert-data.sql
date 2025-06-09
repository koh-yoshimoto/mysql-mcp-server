-- Insert sample data for testing

-- Insert users
INSERT INTO users (username, email, full_name, is_active) VALUES
    ('john_doe', 'john@example.com', 'John Doe', TRUE),
    ('jane_smith', 'jane@example.com', 'Jane Smith', TRUE),
    ('bob_wilson', 'bob@example.com', 'Bob Wilson', TRUE),
    ('alice_johnson', 'alice@example.com', 'Alice Johnson', FALSE),
    ('charlie_brown', 'charlie@example.com', 'Charlie Brown', TRUE);

-- Insert products
INSERT INTO products (name, description, price, stock_quantity, category) VALUES
    ('Laptop Pro 15"', 'High-performance laptop with 16GB RAM and 512GB SSD', 1299.99, 50, 'Electronics'),
    ('Wireless Mouse', 'Ergonomic wireless mouse with precision tracking', 29.99, 200, 'Electronics'),
    ('USB-C Hub', '7-in-1 USB-C hub with HDMI, USB 3.0, and card reader', 49.99, 150, 'Electronics'),
    ('Standing Desk', 'Adjustable height standing desk, 60" x 30"', 399.99, 30, 'Furniture'),
    ('Ergonomic Chair', 'Mesh back office chair with lumbar support', 249.99, 45, 'Furniture'),
    ('Notebook Set', 'Pack of 5 premium notebooks, lined pages', 19.99, 300, 'Stationery'),
    ('Mechanical Keyboard', 'RGB mechanical keyboard with blue switches', 89.99, 75, 'Electronics'),
    ('Monitor 27"', '4K IPS monitor with USB-C connectivity', 449.99, 40, 'Electronics'),
    ('Desk Lamp', 'LED desk lamp with adjustable brightness', 34.99, 100, 'Furniture'),
    ('Coffee Maker', 'Programmable coffee maker with thermal carafe', 79.99, 60, 'Appliances');

-- Insert orders
INSERT INTO orders (user_id, total_amount, status, shipping_address) VALUES
    (1, 1329.98, 'delivered', '123 Main St, New York, NY 10001'),
    (2, 469.98, 'shipped', '456 Oak Ave, Los Angeles, CA 90001'),
    (3, 119.97, 'processing', '789 Pine Rd, Chicago, IL 60601'),
    (1, 449.99, 'pending', '123 Main St, New York, NY 10001'),
    (4, 649.97, 'cancelled', '321 Elm St, Houston, TX 77001');

-- Insert order items
INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
    (1, 1, 1, 1299.99),
    (1, 2, 1, 29.99),
    (2, 4, 1, 399.99),
    (2, 5, 1, 249.99),
    (3, 2, 2, 29.99),
    (3, 3, 1, 49.99),
    (3, 6, 2, 19.99),
    (4, 8, 1, 449.99),
    (5, 1, 1, 1299.99),
    (5, 7, 1, 89.99);

-- Insert analytics events (sample data)
INSERT INTO analytics_events (event_type, user_id, session_id, event_data) VALUES
    ('page_view', 1, 'sess_001', '{"page": "/products", "duration": 45}'),
    ('click', 1, 'sess_001', '{"element": "add_to_cart", "product_id": 1}'),
    ('purchase', 1, 'sess_001', '{"order_id": 1, "amount": 1329.98}'),
    ('page_view', 2, 'sess_002', '{"page": "/home", "duration": 30}'),
    ('search', 2, 'sess_002', '{"query": "standing desk", "results": 1}'),
    ('click', 2, 'sess_002', '{"element": "product_link", "product_id": 4}'),
    ('page_view', 3, 'sess_003', '{"page": "/categories/electronics", "duration": 60}'),
    ('add_to_cart', 3, 'sess_003', '{"product_id": 2, "quantity": 2}'),
    ('page_view', NULL, 'sess_004', '{"page": "/about", "duration": 15}'),
    ('error', NULL, 'sess_004', '{"code": 404, "message": "Page not found"}');

-- Create a view for order summaries
CREATE VIEW order_summary AS
SELECT 
    o.id AS order_id,
    u.username,
    u.email,
    o.order_date,
    o.total_amount,
    o.status,
    COUNT(oi.id) AS item_count,
    SUM(oi.quantity) AS total_items
FROM orders o
JOIN users u ON o.user_id = u.id
LEFT JOIN order_items oi ON o.id = oi.order_id
GROUP BY o.id;

-- Create a stored procedure for testing
DELIMITER //
CREATE PROCEDURE GetUserOrders(IN user_id INT)
BEGIN
    SELECT 
        o.id,
        o.order_date,
        o.total_amount,
        o.status,
        COUNT(oi.id) as item_count
    FROM orders o
    LEFT JOIN order_items oi ON o.id = oi.order_id
    WHERE o.user_id = user_id
    GROUP BY o.id
    ORDER BY o.order_date DESC;
END //
DELIMITER ;