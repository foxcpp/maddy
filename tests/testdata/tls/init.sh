#!/bin/sh

# Generate CA key and certificate
openssl req -x509 -newkey rsa:4096 -keyout ca.key -out ca.crt -days 36500 -nodes -subj "/CN=maddy.test"

# Generate server certificate.
openssl req -new -nodes -newkey rsa:4096 -keyout server.key -out server.req -batch \
  -subj "/CN=mx.maddy.test" \
  -addext "subjectAltName = DNS:mx.maddy.test" \
  -addext "keyUsage = keyEncipherment" \
  -addext "extendedKeyUsage = serverAuth"
openssl x509 -req -in server.req -CA ca.crt -CAkey ca.key -copy_extensions copy -out server.crt -days 36500

# Generate test client certs.
openssl req -new -nodes -newkey rsa:4096 -keyout client_cn.key -out client_cn.req -batch \
  -subj "/CN=cn@maddy.test" \
  -addext "keyUsage = digitalSignature" \
  -addext "extendedKeyUsage = clientAuth"
openssl x509 -req -in client_cn.req -CA ca.crt -CAkey ca.key -copy_extensions copy -out client_cn.crt -days 36500

openssl req -new -nodes -newkey rsa:4096 -keyout client_san_email.key -out client_san_email.req -batch \
  -subj "/CN=SAN test" \
  -addext "subjectAltName = email:san@maddy.test" \
  -addext "keyUsage = digitalSignature" \
  -addext "extendedKeyUsage = clientAuth"
openssl x509 -req -in client_san_email.req -CA ca.crt -CAkey ca.key -copy_extensions copy -out client_san_email.crt -days 36500

openssl req -new -nodes -newkey rsa:4096 -keyout client_san_email_multi.key -out client_san_email_multi.req -batch \
  -subj "/CN=SAN test" \
  -addext "subjectAltName = email:san1@maddy.test,email:san2@maddy.test" \
  -addext "keyUsage = digitalSignature" \
  -addext "extendedKeyUsage = clientAuth"
openssl x509 -req -in client_san_email_multi.req -CA ca.crt -CAkey ca.key -copy_extensions copy -out client_san_email_multi.crt -days 36500

openssl req -new -nodes -newkey rsa:4096 -keyout client_san_email_no_usage.key -out client_san_email_no_usage.req -batch \
  -subj "/CN=SAN test" \
  -addext "subjectAltName = email:san@maddy.test"
openssl x509 -req -in client_san_email_no_usage.req -CA ca.crt -CAkey ca.key -copy_extensions copy -out client_san_email_no_usage.crt -days 36500

rm *.req
