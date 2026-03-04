-- 001_initial.sql: SkyHigh Core initial schema

CREATE TABLE IF NOT EXISTS flights (
    id SERIAL PRIMARY KEY,
    flight_number VARCHAR(20) UNIQUE NOT NULL,
    origin VARCHAR(10) NOT NULL,
    destination VARCHAR(10) NOT NULL,
    departure_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS passengers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS seats (
    id SERIAL PRIMARY KEY,
    flight_id INTEGER NOT NULL REFERENCES flights(id),
    seat_number VARCHAR(10) NOT NULL,
    class VARCHAR(20) DEFAULT 'ECONOMY',
    state VARCHAR(20) DEFAULT 'AVAILABLE',
    passenger_id INTEGER REFERENCES passengers(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(flight_id, seat_number)
);
CREATE INDEX IF NOT EXISTS idx_seats_flight_id ON seats(flight_id);
CREATE INDEX IF NOT EXISTS idx_seats_passenger_id ON seats(passenger_id);

CREATE TABLE IF NOT EXISTS check_ins (
    id SERIAL PRIMARY KEY,
    passenger_id INTEGER NOT NULL REFERENCES passengers(id),
    flight_id INTEGER NOT NULL REFERENCES flights(id),
    seat_id INTEGER REFERENCES seats(id),
    status VARCHAR(30) DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_check_ins_passenger_id ON check_ins(passenger_id);
CREATE INDEX IF NOT EXISTS idx_check_ins_flight_id ON check_ins(flight_id);
CREATE INDEX IF NOT EXISTS idx_check_ins_seat_id ON check_ins(seat_id);

CREATE TABLE IF NOT EXISTS baggage (
    id SERIAL PRIMARY KEY,
    check_in_id INTEGER NOT NULL REFERENCES check_ins(id),
    weight_kg DECIMAL(5,2) NOT NULL,
    excess_fee DECIMAL(10,2) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_baggage_check_in_id ON baggage(check_in_id);

CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    check_in_id INTEGER NOT NULL REFERENCES check_ins(id),
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payments_check_in_id ON payments(check_in_id);

CREATE TABLE IF NOT EXISTS waitlist (
    id SERIAL PRIMARY KEY,
    flight_id INTEGER NOT NULL REFERENCES flights(id),
    passenger_id INTEGER NOT NULL REFERENCES passengers(id),
    position INTEGER NOT NULL,
    status VARCHAR(20) DEFAULT 'WAITING',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_waitlist_flight_id ON waitlist(flight_id);
CREATE INDEX IF NOT EXISTS idx_waitlist_passenger_id ON waitlist(passenger_id);
