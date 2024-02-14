#!/bin/bash

# Usage: ./call_endpoint.sh <client_id>

# Check if the client ID is provided
if [ $# -eq 0 ]; then
    echo "Error: Client ID not provided."
    echo "Usage: ./call_endpoint.sh <client_id>"
    exit 1
fi

# Assign the client ID to a variable
client_id="$1"

# Execute the curl command with the provided client ID
curl -X POST \
     -H "Content-Type: application/json" \
     -H "X-GV-CLIENTID: $client_id" \
     -d '{"serviceName":"https://fakerapi.it","serviceEndpoint":"/api/v1/texts","httpMethod":"GET","payload": ""}' \
     http://localhost:8080/tunnel

