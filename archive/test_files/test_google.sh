#!/bin/bash
# Test script for Google provider

echo "Testing Google provider..."
echo "/provider 4" | ./bin/devorch &
DEVORCH_PID=$!

sleep 2
echo "test message" | ./bin/devorch
echo "/exit" | ./bin/devorch

kill $DEVORCH_PID 2>/dev/null