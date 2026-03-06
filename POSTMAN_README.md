# SkyHigh API Postman Collection

This directory contains Postman collection and environment files for testing the SkyHigh Core Digital Check-In API.

## Files

- `SkyHigh_API_Collection.postman_collection.json` - Complete API collection with all endpoints
- `SkyHigh_Local_Environment.postman_environment.json` - Environment variables for local development

## Setup Instructions

### 1. Import the Collection and Environment

1. Open Postman
2. Click "Import" in the top left
3. Import both files:
   - `SkyHigh_API_Collection.postman_collection.json`
   - `SkyHigh_Local_Environment.postman_environment.json`

### 2. Configure Environment Variables

Select the "SkyHigh Local Development" environment from the environment dropdown.

Update the following variables as needed:
- `jwt_token`: Replace with your actual JWT token for authenticated requests
- Other variables are pre-configured for the demo data

### 3. Start the Application

Make sure the SkyHigh application is running:

```bash
docker-compose up --build
```

The API will be available at `http://localhost:8080`.

## API Endpoints Overview

### Flights
- **Create Flight** (POST) - Admin endpoint to create new flights
- **Get Flight Details** (GET) - Public endpoint to get flight information
- **Get Seat Map** (GET) - Public endpoint to view available seats (cached, rate-limited)

### Seats
- **Add Seats to Flight** (POST) - Admin endpoint to add seats to flights
- **Hold Seat** (POST) - Hold a seat for 120 seconds
- **Confirm Seat** (POST) - Confirm a held seat

### Check-ins
- **Start Check-in** (POST) - Begin the check-in process
- **Get Check-in Status** (GET) - View check-in progress
- **Cancel Check-in** (DELETE) - Cancel and release seat

### Baggage
- **Add Baggage** (POST) - Add luggage (25kg limit, $15/kg excess fee)

### Payment
- **Process Payment** (POST) - Pay for excess baggage fees

### Waitlist
- **Join Waitlist** (POST) - Join waitlist for a flight
- **Get Waitlist** (GET) - View current waitlist

## Example Workflow

Use the "Complete Check-in Flow" folder for a step-by-step example:

1. **Get Seat Map** - View available seats
2. **Hold Seat** - Reserve a seat temporarily
3. **Start Check-in** - Begin the check-in process
4. **Add Baggage** - Add luggage within limits
5. **Add Excess Baggage** - Trigger payment requirement
6. **Process Payment** - Complete the payment

## Authentication

Most endpoints require JWT authentication. Include the Authorization header:

```
Authorization: Bearer {{jwt_token}}
```

Replace `{{jwt_token}}` with your actual JWT token.

## Demo Data

The application seeds demo data on startup:
- Flight: SH001 (JFK → LAX)
- Passenger: demo@skyhigh.io
- 30 seats available

## Rate Limiting

The seat map endpoint is rate-limited:
- 50 requests per 2 seconds per IP
- Blocked for 60 seconds after exceeding limit

## Notes

- All monetary values are in USD
- Baggage weight limit is 25kg
- Excess baggage fee is $15 per kg over limit
- Seat holds expire after 120 seconds
- Seat maps are cached for 30 seconds