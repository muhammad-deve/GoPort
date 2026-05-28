#!/bin/bash

# Get SSL certificate for goport.uz and *.goport.uz using the existing
# Barbershop certbot container.
#
# Wildcard certificates require DNS validation. Certbot will print a TXT record
# that you must add in your DNS panel before continuing.

DOMAIN="goport.uz"
EMAIL="admin@goport.uz"  # Change this to your email

echo ">>> Requesting wildcard SSL certificate for $DOMAIN..."
docker exec -it barbershop-certbot certbot certonly \
  --manual \
  --preferred-challenges dns \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN \
  -d "*.$DOMAIN"

echo ">>> Reloading nginx..."
docker exec barbershop-nginx nginx -s reload

echo ">>> Done! Copy nginx/goport.conf into your Barbershop nginx config and reload nginx again."
