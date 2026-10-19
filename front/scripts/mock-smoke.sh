#!/usr/bin/env bash
# Main scenario (SPEC §1) through the Nitro proxy: anna publishes → boris applies →
# anna accepts → boris sees "accepted". Run against `NUXT_MOCK_API=true npm run dev`
# (or a real backend). Usage: scripts/mock-smoke.sh [http://localhost:3000]
set -euo pipefail

BASE="${1:-http://localhost:3000}/api"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
JSON=(-H 'content-type: application/json')
field() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

echo '1. anna logs in (cookie is httpOnly, body has no token)'
curl -sf -c "$TMP/anna" "${JSON[@]}" -d '{"username":"anna","password":"demo1234"}' -D "$TMP/h" "$BASE/auth/login" | field '"token" in d, d["user"]["username"]'
grep -i '^set-cookie' "$TMP/h"

echo '2. wrong password → 401'
curl -s -o /dev/null -w '%{http_code}\n' "${JSON[@]}" -d '{"username":"anna","password":"nope1234"}' "$BASE/auth/login"

echo '3. anna publishes a startup'
ID=$(curl -sf -b "$TMP/anna" "${JSON[@]}" -d '{"kind":"LISTING_KIND_STARTUP","title":"Smoke Startup","description":"Стартап для smoke-проверки сценария","lookingFor":"CTO","tags":["Go","CTO"],"stage":"STARTUP_STAGE_IDEA","compensation":""}' "$BASE/v1/listings" | field 'd["id"]')
echo "   listing $ID"

echo '4. boris finds it in the feed'
curl -sf -c "$TMP/boris" "${JSON[@]}" -d '{"username":"boris","password":"demo1234"}' "$BASE/auth/login" >/dev/null
curl -sf -b "$TMP/boris" "$BASE/v1/listings?kind=LISTING_KIND_STARTUP&query=smoke" | field 'd["total"], [i["title"] for i in d["items"]]'

echo '5. boris applies; second attempt → 409'
AID=$(curl -sf -b "$TMP/boris" "${JSON[@]}" -d '{"message":"Хочу в команду"}' "$BASE/v1/listings/$ID/applications" | field 'd["id"]')
curl -s -w ' %{http_code}\n' -b "$TMP/boris" "${JSON[@]}" -d '{"message":"Ещё раз"}' "$BASE/v1/listings/$ID/applications" | tr -d '\n'; echo

echo '6. anna sees it in received and accepts'
curl -sf -b "$TMP/anna" "$BASE/v1/me/applications/received?status=APPLICATION_STATUS_PENDING" | field '[(i["applicant"]["username"], i["listing"]["title"]) for i in d["items"]]'
curl -sf -b "$TMP/anna" -X POST "${JSON[@]}" -d '{}' "$BASE/v1/applications/$AID/accept" | field 'd["status"]'

echo '7. boris sees "accepted"'
curl -sf -b "$TMP/boris" "$BASE/v1/me/applications/sent" | field '[(i["listing"]["title"], i["status"]) for i in d["items"]]'

echo '8. no cookie → 401; /api/v1/auth/login via proxy → 404'
curl -s -o /dev/null -w '%{http_code}\n' "$BASE/v1/me"
curl -s -o /dev/null -w '%{http_code}\n' "${JSON[@]}" -d '{"username":"anna","password":"demo1234"}' "$BASE/v1/auth/login"

echo '9. boris uploads a photo; it is served with the original headers'
PNG=$(python3 -c 'import base64,zlib,struct
def ch(t,d): return struct.pack(">I",len(d))+t+d+struct.pack(">I",zlib.crc32(t+d)&0xffffffff)
raw=b"".join(b"\x00"+b"\x1f\x5e\xff"*64 for _ in range(64))
print(base64.b64encode(b"\x89PNG\r\n\x1a\n"+ch(b"IHDR",struct.pack(">IIBBBBB",64,64,8,2,0,0,0))+ch(b"IDAT",zlib.compress(raw))+ch(b"IEND",b"")).decode())')
URL=$(curl -sf -b "$TMP/boris" "${JSON[@]}" -d "{\"data\":\"$PNG\"}" "$BASE/v1/me/avatar" | field 'd["avatarUrl"]')
echo "   $URL"
curl -s -D - -o /dev/null "$BASE$URL" | grep -iE '^(HTTP|content-type|cache-control)'

echo '10. logout clears the cookie'
curl -s -D - -o /dev/null -b "$TMP/anna" -X POST "$BASE/auth/logout" | grep -i '^set-cookie'
