#!/usr/bin/env bash
# Smoke test of the running stack (make up): the main scenario of SPEC §1 through the
# public API, access/refresh tokens (rotation, logout), the photo upload through the
# real MinIO, and a login through the front (Nitro proxy + httpOnly cookies).
#
#   API_URL    backend public API     (default http://localhost:8080)
#   FRONT_URL  front; empty = skip    (default http://localhost:3000)
#   PASSWORD   demo password          (default demo1234, SPEC §6)
#
# Requires bash, curl, jq. Exits with a non-zero code and a message on the first mismatch.
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
API_URL="${API_URL%/}"
FRONT_URL="${FRONT_URL-http://localhost:3000}"
FRONT_URL="${FRONT_URL%/}"
PASSWORD="${PASSWORD:-demo1234}"

# 64×64 PNG (the backend rejects photos smaller than 64×64, SPEC §2.4).
PHOTO_PNG_BASE64='iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAIAAAAlC+aJAAAATklEQVR42u3PQQkAAAgEsKtqJ/tqBN/CYAWW6nktAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgKXBX7WQdKWfnBCAAAAAElFTkSuQmCC'

for bin in curl jq; do
  command -v "$bin" >/dev/null || { echo "smoke: не найден $bin — установите его и повторите" >&2; exit 2; }
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

step=0
STATUS=""
BODY=""

if [[ -t 1 ]]; then OK=$'\e[32mOK\e[0m'; FAIL=$'\e[31mОШИБКА\e[0m'; else OK="OK"; FAIL="ОШИБКА"; fi

fail() {
  echo "  $FAIL: $1" >&2
  if [[ -n "${2:-}" ]]; then echo "  ответ: $(printf '%s' "$2" | head -c 600)" >&2; fi
  exit 1
}

begin() {
  step=$((step + 1))
  printf '%2d. %s ... ' "$step" "$1"
}

ok() { echo "$OK${1:+ ($1)}"; }

# api METHOD PATH [TOKEN] [JSON] — sets STATUS and BODY. Connection errors are fatal.
api() {
  local method="$1" path="$2" token="${3:-}" body="${4:-}"
  local args=(-sS -o "$tmp/body" -w '%{http_code}' -X "$method" --max-time 30)
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$body" ]] && args+=(-H 'Content-Type: application/json' --data-binary @-)
  if ! STATUS="$(printf '%s' "$body" | curl "${args[@]}" "$API_URL$path" 2>"$tmp/err")"; then
    fail "API $API_URL не отвечает на $method $path: $(cat "$tmp/err"). Стек поднят (make up)? Другой адрес — API_URL=..."
  fi
  BODY="$(cat "$tmp/body")"
}

# expect CODE — the last response must have this HTTP status.
expect() {
  [[ "$STATUS" == "$1" ]] || fail "ожидался HTTP $1, получен $STATUS ($LAST)" "$BODY"
}

# call CODE METHOD PATH [TOKEN] [JSON]
call() {
  local code="$1"; shift
  LAST="$1 $2"
  api "$@"
  expect "$code"
}

# field JQ — a value from BODY; jq errors (not JSON) are reported as a mismatch.
field() {
  printf '%s' "$BODY" | jq -er "$1" 2>/dev/null || fail "в ответе $LAST нет ожидаемого значения: $1" "$BODY"
}

# check JQ_CONDITION MESSAGE
check() {
  printf '%s' "$BODY" | jq -e "$1" >/dev/null 2>&1 || fail "$2" "$BODY"
}

# login USERNAME — sets TOKEN (access) and REFRESH (no subshell: BODY stays available
# to the caller).
login() {
  call 200 POST /v1/auth/login "" "{\"username\":\"$1\",\"password\":\"$PASSWORD\"}"
  TOKEN="$(field .accessToken)"
  REFRESH="$(field .refreshToken)"
}

echo "Smoke-тест: API $API_URL${FRONT_URL:+, фронт $FRONT_URL}"

begin "API отвечает (GET /v1/ping, /readyz)"
LAST="GET /v1/ping"
api GET /v1/ping
[[ "$STATUS" == 200 ]] || fail "API недоступен по $API_URL (HTTP ${STATUS:-нет ответа}). Стек поднят (make up)? Другой адрес — API_URL=..." "$BODY"
LAST="GET /readyz"
api GET /readyz
expect 200
check '.db == "ok" and .blob == "ok"' "readyz: база или хранилище не готовы"
ok

begin "вход Анны (anna)"
login anna
ANNA="$TOKEN"
check '.user.username == "anna" and .accessExpiresIn > 0' "в ответе логина не тот пользователь или нет accessExpiresIn"
ok

begin "refresh: новая пара токенов, новый access работает"
call 200 POST /v1/auth/refresh "" "$(jq -nc --arg r "$REFRESH" '{refreshToken: $r}')"
NEW_ACCESS="$(field .accessToken)"
NEW_REFRESH="$(field .refreshToken)"
[[ "$NEW_REFRESH" != "$REFRESH" ]] || fail "refresh-токен не ротирован" "$BODY"
call 200 GET /v1/me "$NEW_ACCESS"
check '.username == "anna"' "новый access-токен принадлежит не тому пользователю"
ANNA="$NEW_ACCESS"
ok

begin "logout: 200, повторный refresh → 401"
call 200 POST /v1/auth/logout "" "$(jq -nc --arg r "$NEW_REFRESH" '{refreshToken: $r}')"
call 401 POST /v1/auth/refresh "" "$(jq -nc --arg r "$NEW_REFRESH" '{refreshToken: $r}')"
check '.message == "Сессия истекла"' "неожиданный текст ошибки refresh после logout"
ok

suffix="$(date +%s)-$RANDOM"
title="Smoke $suffix — стартап для проверки"
begin "Анна создаёт стартап"
call 200 POST /v1/listings "$ANNA" "$(jq -nc --arg t "$title" '{
  kind: "LISTING_KIND_STARTUP", title: $t,
  description: "Стартап, созданный smoke-тестом для проверки сквозного сценария.",
  lookingFor: "Сооснователь для smoke-теста", tags: ["Smoke", "Go"], stage: "STARTUP_STAGE_MVP"
}')"
LISTING="$(field .id)"
check '.status == "LISTING_STATUS_ACTIVE" and .isMine == true and .stats.total == 0' \
  "новая публикация должна быть активной, своей и с нулевой статистикой"
ok "id $LISTING"

begin "вход Бориса (boris)"
login boris
BORIS="$TOKEN"
ok

begin "лента Бориса содержит стартап"
call 200 GET "/v1/listings?kind=LISTING_KIND_STARTUP&query=$(jq -rn --arg q "$suffix" '$q|@uri')" "$BORIS"
printf '%s' "$BODY" | jq -e --arg id "$LISTING" \
  '.items | map(select(.id == $id)) | length == 1 and .[0].isMine == false and .[0].myApplication == null and .[0].stats == null' \
  >/dev/null || fail "в ленте Бориса нет новой публикации (или у неё неверные isMine/myApplication/stats)" "$BODY"
ok

begin "Борис откликается"
call 200 POST "/v1/listings/$LISTING/applications" "$BORIS" '{"message":"Привет! Это отклик smoke-теста."}'
APP="$(field .id)"
check '.status == "APPLICATION_STATUS_PENDING"' "новый отклик должен быть в статусе PENDING"
ok "id $APP"

begin "повторный отклик → 409"
call 409 POST "/v1/listings/$LISTING/applications" "$BORIS" '{"message":"Второй раз"}'
check '.message == "Вы уже откликались на эту публикацию"' "неожиданный текст ошибки повторного отклика"
ok

begin "Анна видит отклик во входящих"
call 200 GET /v1/me/applications/received "$ANNA"
printf '%s' "$BODY" | jq -e --arg id "$APP" \
  '.items | map(select(.id == $id)) | length == 1 and .[0].status == "APPLICATION_STATUS_PENDING" and .[0].applicant.username == "boris"' \
  >/dev/null || fail "во входящих Анны нет отклика Бориса в статусе PENDING" "$BODY"
ok

begin "Анна принимает отклик"
call 200 POST "/v1/applications/$APP/accept" "$ANNA" '{}'
check '.status == "APPLICATION_STATUS_ACCEPTED" and .decidedAt != null' "после accept статус должен быть ACCEPTED"
ok

begin "Борис видит ACCEPTED в отправленных"
call 200 GET /v1/me/applications/sent "$BORIS"
printf '%s' "$BODY" | jq -e --arg id "$APP" \
  '.items | map(select(.id == $id)) | length == 1 and .[0].status == "APPLICATION_STATUS_ACCEPTED"' \
  >/dev/null || fail "в отправленных Бориса отклик не в статусе ACCEPTED" "$BODY"
ok

begin "публикация у Бориса: myApplication.status = ACCEPTED"
call 200 GET "/v1/listings/$LISTING" "$BORIS"
check '.myApplication.status == "APPLICATION_STATUS_ACCEPTED"' "в GET /v1/listings/{id} нет myApplication со статусом ACCEPTED"
ok

begin "Борис загружает фото"
call 200 POST /v1/me/avatar "$BORIS" "{\"data\":\"$PHOTO_PNG_BASE64\"}"
AVATAR="$(field .avatarUrl)"
[[ "$AVATAR" == /v1/avatars/*.jpg ]] || fail "avatarUrl пустой или неожиданного вида: '$AVATAR'" "$BODY"
ok "$AVATAR"

begin "фото отдаётся из хранилища (image/jpeg)"
LAST="GET $AVATAR"
headers="$(curl -sS -o "$tmp/photo" -D - --max-time 30 "$API_URL$AVATAR" 2>"$tmp/err")" \
  || fail "запрос $AVATAR не выполнен: $(cat "$tmp/err")"
code="$(printf '%s' "$headers" | awk 'NR==1{print $2}')"
ctype="$(printf '%s' "$headers" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2}')"
[[ "$code" == 200 ]] || fail "ожидался HTTP 200 для фото, получен $code"
[[ "$ctype" == image/jpeg ]] || fail "ожидался Content-Type image/jpeg, получен '$ctype'"
# JPEG starts with FF D8.
[[ "$(head -c 2 "$tmp/photo" | od -An -tx1 | tr -d ' \n')" == ffd8 ]] || fail "тело ответа — не JPEG"
ok "$(wc -c <"$tmp/photo" | tr -d ' ') байт"

begin "уборка: Борис удаляет фото, старый URL → 404"
call 200 DELETE /v1/me/avatar "$BORIS"
check '.avatarUrl == ""' "после удаления avatarUrl должен быть пустым"
call 404 GET "$AVATAR"
ok

begin "уборка: Анна закрывает публикацию"
call 200 POST "/v1/listings/$LISTING/close" "$ANNA" '{}'
check '.status == "LISTING_STATUS_CLOSED"' "публикация не закрылась"
ok

if [[ -n "$FRONT_URL" ]]; then
  begin "фронт: вход через Nitro, httpOnly-cookie hbc_access/hbc_refresh, прокси /api/v1/me"
  jar="$tmp/cookies"
  code="$(curl -sS -o "$tmp/front" -w '%{http_code}' -c "$jar" -D "$tmp/front-headers" --max-time 30 \
    -H 'Content-Type: application/json' --data "{\"username\":\"boris\",\"password\":\"$PASSWORD\"}" \
    "$FRONT_URL/api/auth/login" 2>"$tmp/err")" \
    || fail "фронт недоступен по $FRONT_URL: $(cat "$tmp/err"). Другой адрес — FRONT_URL=..., пропустить — FRONT_URL="
  [[ "$code" == 200 ]] || fail "вход через фронт: ожидался HTTP 200, получен $code" "$(cat "$tmp/front")"
  for cookie in hbc_access hbc_refresh; do
    grep -qi "^set-cookie: $cookie=.*httponly" "$tmp/front-headers" || fail "фронт не выставил httpOnly-cookie $cookie"
  done
  jq -e '.token == null and .accessToken == null and .refreshToken == null' "$tmp/front" >/dev/null 2>&1 \
    || fail "фронт вернул токен в теле ответа (должен быть только в cookie)" "$(cat "$tmp/front")"
  code="$(curl -sS -o "$tmp/front" -w '%{http_code}' -b "$jar" --max-time 30 "$FRONT_URL/api/v1/me" 2>"$tmp/err")" \
    || fail "запрос /api/v1/me не выполнен: $(cat "$tmp/err")"
  [[ "$code" == 200 ]] || fail "GET /api/v1/me с cookie: ожидался HTTP 200, получен $code" "$(cat "$tmp/front")"
  jq -e '.username == "boris"' "$tmp/front" >/dev/null 2>&1 || fail "/api/v1/me вернул не того пользователя" "$(cat "$tmp/front")"
  ok
fi

echo "Smoke-тест пройден: $step проверок."
