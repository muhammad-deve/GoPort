#!/bin/bash

# Get SSL certificate for goport.uz using the existing barbershop certbot
# Run this ONCE from the barbershop project directory (where certbot volumes are)

DOMAIN="goport.uz"
EMAIL="admin@goport.uz"  # Change this to your email

echo ">>> Requesting SSL certificate for $DOMAIN..."
docker exec barbershop-certbot certbot certonly \
  --webroot \
  --webroot-path=/var/www/certbot \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN \
  -d www.$DOMAIN \
  -d back.$DOMAIN

echo ">>> Reloading nginx..."
docker exec barbershop-nginx nginx -s reload

echo ">>> Done! Now copy goport.conf into your barbershop nginx config directory and reload again."
