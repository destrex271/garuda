#!/bin/bash

# docker install
docker run --detach --name zap -u zap -p 8080:8080 -i ghcr.io/zaproxy/zaproxy:stable zap.sh -daemon -host 0.0.0.0 -port 8080 -config api.disablekey=true -config api.addrs.addr.name=.\* -config api.addrs.addr.regex=true 
