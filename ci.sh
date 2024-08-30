#!/bin/bash

# Discover APIs

pwd
ls
cd services/api_discovery

echo "Discovering endpoints....."

go run . --apiDocs=/home/akshat/fg/dummyapi2/swagger_output.json

# ZAP Scan

echo "Scanning for vulnerabilities....."
cd ../zap_scan
go run . --baseurl=http://localhost:16000 --apidocs=/home/akshat/fg/dummyapi2/swagger_output.json

echo "Inventory updated!"