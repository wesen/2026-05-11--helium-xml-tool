#!/bin/bash
# Script: debug-serve-help-binary.sh
# Purpose: Start the actual xml binary with serve-help and check the API
#
# Run:  cd /home/manuel/code/wesen/2026-05-11--helium-xml-tool && bash ttmp/.../scripts/debug-serve-help-binary.sh

set -e
PORT=19885
BINARY="./xml"

echo "Building..."
go build -o "$BINARY" ./cmd/xml/

echo "Starting server on :$PORT..."
"$BINARY" serve-help --address ":$PORT" &
SRV_PID=$!
sleep 2

echo ""
echo "=== /api/packages ==="
curl -s "http://localhost:$PORT/api/packages" | python3 -m json.tool 2>&1

echo ""
echo "=== /api/sections (no filter) ==="
curl -s "http://localhost:$PORT/api/sections" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'total={d[\"total\"]}, first={d[\"sections\"][0][\"slug\"]} pkg={d[\"sections\"][0][\"packageName\"]!r}')" 2>&1

echo ""
echo "=== /api/sections?package=xml ==="
curl -s "http://localhost:$PORT/api/sections?package=xml" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'total={d[\"total\"]}')" 2>&1

echo ""
echo "=== /api/sections?package=default ==="
curl -s "http://localhost:$PORT/api/sections?package=default" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'total={d[\"total\"]}')" 2>&1

echo ""
echo "Stopping server (PID=$SRV_PID)..."
kill $SRV_PID 2>/dev/null
wait $SRV_PID 2>/dev/null
echo "Done."
