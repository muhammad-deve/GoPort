#!/bin/bash

# Initial SSL certificate setup for goport.uz
# Run this script ONCE on your server before starting docker-compose

DOMAIN="goport.uz"
EMAIL="admin@goport.uz"  # Change this to your email

echo ">>> Creating required directories..."
mkdir -p ./certbot/conf
mkdir -p ./certbot/www

echo ">>> Stopping barbershop-nginx to free port 80/443..."
docker stop barbershop-nginx

echo ">>> Starting nginx with init config for ACME challenge..."
# Temporarily use init config
cp ./nginx/nginx-init.conf ./nginx/nginx.conf.bak
cp ./nginx/nginx-init.conf ./nginx/nginx.conf

docker compose up -d nginx

echo ">>> Requesting SSL certificate from Let's Encrypt..."
docker compose run --rm certbot certonly \
  --webroot \
  --webroot-path=/var/www/certbot \
  --email $EMAIL \
  --agree-tos \
  --no-eff-email \
  -d $DOMAIN \
  -d www.$DOMAIN \
  -d back.$DOMAIN

echo ">>> Restoring full nginx config..."
cp ./nginx/nginx.conf.bak ./nginx/nginx.conf
rm ./nginx/nginx.conf.bak

echo ">>> Restarting all services..."
docker compose down
docker compose up -d

echo ">>> Done! SSL certificate installed for $DOMAIN and back.$DOMAIN"
echo ">>> Note: barbershop-nginx is stopped. Start it again if needed on different ports."
