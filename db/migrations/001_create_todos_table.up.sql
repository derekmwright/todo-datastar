CREATE TABLE IF NOT EXISTS todos(
    id SERIAL PRIMARY KEY,
    name VARCHAR NOT NULL,
    description TEXT,
    done BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now(),
    completed_at TIMESTAMP
)