#!/bin/bash

# Usage: ./send_request.sh <agentID>

# Check if the agent ID is provided
if [ $# -eq 0 ]; then
    echo "Error: Agent ID not provided."
    echo "Usage: ./call_endpoint.sh <agentID>"
    exit 1
fi

# Assign the agent ID to a variable
agentID="$1"

# Execute the curl command with the provided agent ID
curl -X POST \
     -H "Content-Type: application/json" \
     -H "AGENT-ID: $agentID" \
     -d '{"serviceName":"https://fakerapi.it","serviceEndpoint":"/api/v1/texts","httpMethod":"GET","payload": ""}' \
     http://localhost:8080/gateway

