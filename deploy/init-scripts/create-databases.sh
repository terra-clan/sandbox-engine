#!/bin/bash
set -e

# Create additional databases for proxy-api and assessment-service
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE proxy_api;
    CREATE DATABASE assessment_service;
    GRANT ALL PRIVILEGES ON DATABASE proxy_api TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE assessment_service TO $POSTGRES_USER;
EOSQL
