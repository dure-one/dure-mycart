#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Set defaults if not provided
DOMAIN=${DOMAIN:-yourdomain.com}
ADMIN_EMAIL=${ADMIN_EMAIL:-admin@example.com}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-YourSecurePass}

# Run mycart install command
docker-compose run --rm mycart install \
  --email "$ADMIN_EMAIL" \
  --password "$ADMIN_PASSWORD" \
  --domain "$DOMAIN"
