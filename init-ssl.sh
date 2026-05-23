#!/bin/bash

# Initial SSL certificate setup for goport.uz
# Run this script ONCE on your server before starting docker-compose

DOMAIN="goport.uz"
EMAIL="admin@goport.uz"  # Change this to your email

echo ">>> Creating required directories..."
mkdir -p ./certbot/conf
mkdir -p ./certbot/www

echo ">>> Starting nginx without SSL for ACME challenge..."
docker compose up -d nginx

echo ">>> Requesting SSL certificate from Let's Encrypt..."
docker compose run --rm certbot certonly \
  --webroot \
  --webroot-path=/var/www/certbot \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN \
  -d www.$DOMAIN

echo ">>> Restarting nginx with SSL..."
docker compose restart nginx

echo ">>> Done! SSL certificate installed for $DOMAIN"
