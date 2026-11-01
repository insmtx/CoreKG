#!/usr/bin/env bash
set -euo pipefail

STAGES=(health login create_forest preupload upload callback parse search chat keapi_search mcp)

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TEST_DOMAIN="${TEST_DOMAIN:-}"
TEST_USERNAME="${TEST_USERNAME:-}"
TEST_PASSWORD_MD5="${TEST_PASSWORD_MD5:-}"
MODEL_ID="${MODEL_ID:-}"
KEAPI_API_KEY="${KEAPI_API_KEY:-}"
EXPECT_TEXT="${EXPECT_TEXT:-golden-path-$(date +%s)}"
CLEANUP="${CLEANUP:-1}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)-$$}"
PARSE_TIMEOUT="${PARSE_TIMEOUT:-300}"
PARSE_INTERVAL="${PARSE_INTERVAL:-5}"
HTTP_TIMEOUT="${HTTP_TIMEOUT:-120}"
CHAT_TIMEOUT="${CHAT_TIMEOUT:-300}"

MODE=run
HEALTH_ONLY=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) MODE=dry-run ;;
    --self-test) MODE=self-test ;;
    --health-only) HEALTH_ONLY=1 ;;
    --no-cleanup) CLEANUP=0 ;;
    -h|--help) MODE=help ;;
    *) printf '[golden-path] unknown argument: %s\n' "$arg" >&2; exit 2 ;;
  esac
done

usage() {
  cat <<'EOF'
Usage: scripts/golden-path.sh [options]

Options:
  --dry-run       print the 11 stage names and env vars, no network, exit 0
  --self-test     offline checks (resp parses .Response/.response, 11 stages), exit 0
  --health-only   run only stage 1 (GET /healthz)
  --no-cleanup    keep the created forest on exit (default: forest.DeleteForest)

Env vars:
  BASE_URL             default http://127.0.0.1:8080
  TEST_DOMAIN          login domain name (optional)
  TEST_USERNAME        login username (required for full run)
  TEST_PASSWORD_MD5    md5 hex of the password (required for full run)
  MODEL_ID             numeric chat model id (required for full run)
  KEAPI_API_KEY        keapi API key, also used for MCP (required for full run)
  EXPECT_TEXT          unique token written into the test file,
                       default golden-path-$(date +%s)
EOF
}

log() { printf '[golden-path] %s\n' "$*"; }

resp() { jq -c '(.response // .Response)' <<<"$1"; }

assert_eq() {
  if [[ "$1" != "$2" ]]; then
    printf '[golden-path] assert_eq FAILED: got=%s want=%s\n' "$1" "$2" >&2
    exit 1
  fi
}

fail() {
  local stage="$1" msg="$2"
  log "stage=$stage FAILED: $msg"
  if [[ -n "${HTTP_STATUS:-}" ]]; then
    log "stage=$stage http_status=$HTTP_STATUS"
  fi
  if [[ -n "${HTTP_BODY:-}" ]]; then
    log "stage=$stage response_body: $HTTP_BODY"
  fi
  exit 1
}

if [[ "$MODE" == help ]]; then
  usage
  exit 0
fi

if [[ "$MODE" == self-test ]]; then
  assert_eq "$(resp '{"Response":{"x":1}}')" '{"x":1}'
  assert_eq "$(resp '{"response":{"x":2}}')" '{"x":2}'
  if [[ "${#STAGES[@]}" -ne 11 ]]; then
    printf '[golden-path] self-test FAILED: stage count %s != 11\n' "${#STAGES[@]}" >&2
    exit 1
  fi
  log "self-test ok: resp handles .Response and .response, stages=${#STAGES[@]}"
  exit 0
fi

env_state() {
  if [[ -n "${!1}" ]]; then printf 'set'; else printf 'MISSING'; fi
}

if [[ "$MODE" == dry-run ]]; then
  log "dry-run: ${#STAGES[@]} stages, no network requests will be made"
  for s in "${STAGES[@]}"; do
    log "stage: $s"
  done
  log "env vars:"
  log "  BASE_URL             current=${BASE_URL} (default http://127.0.0.1:8080)"
  log "  TEST_DOMAIN          current=${TEST_DOMAIN:-<empty>} (optional)"
  log "  TEST_USERNAME        $(env_state TEST_USERNAME) (required for full run)"
  log "  TEST_PASSWORD_MD5    $(env_state TEST_PASSWORD_MD5) (required for full run)"
  log "  MODEL_ID             $(env_state MODEL_ID) (required for full run, numeric)"
  log "  KEAPI_API_KEY        $(env_state KEAPI_API_KEY) (required for full run)"
  log "  EXPECT_TEXT          current=${EXPECT_TEXT} (default golden-path-\$(date +%s))"
  exit 0
fi

check_deps() {
  local missing=() dep
  for dep in "$@"; do
    command -v "$dep" >/dev/null 2>&1 || missing+=("$dep")
  done
  if (( ${#missing[@]} > 0 )); then
    log "missing required dependencies: ${missing[*]}"
    exit 2
  fi
}

if [[ "$HEALTH_ONLY" == 1 ]]; then
  check_deps curl jq
  RUN_STAGES=(health)
else
  check_deps curl jq sha256sum
  RUN_STAGES=("${STAGES[@]}")
  missing_env=()
  for v in TEST_USERNAME TEST_PASSWORD_MD5 MODEL_ID KEAPI_API_KEY; do
    [[ -n "${!v}" ]] || missing_env+=("$v")
  done
  if (( ${#missing_env[@]} > 0 )); then
    log "missing required env vars: ${missing_env[*]}"
    log "hint: --dry-run lists all vars; --health-only needs none of them"
    exit 2
  fi
  if ! [[ "$MODEL_ID" =~ ^[0-9]+$ ]]; then
    log "MODEL_ID must be numeric, got: $MODEL_ID"
    exit 2
  fi
fi

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/golden-path.XXXXXX")"
TMP_FILE="$WORKDIR/golden.txt"
TOKEN=""
FOREST_ID=""
UPLOAD_ID=""
UPLOAD_URL=""
SHA256=""
SIZE=""
FILE_ID=""
HTTP_STATUS=""
HTTP_BODY=""
CURRENT_STAGE=""
TIMING_LINES=()

cleanup() {
  local rc=$?
  trap - EXIT
  if [[ "$CLEANUP" == 1 && -n "$FOREST_ID" && -n "$TOKEN" ]]; then
    log "cleanup: forest.DeleteForest id=$FOREST_ID"
    local body status
    body="$(jq -n --argjson id "$FOREST_ID" '{cmd:"forest.DeleteForest", request:{id:$id}}' 2>/dev/null || true)"
    if [[ -n "$body" ]]; then
      status="$(curl -sS --max-time 30 -o "$WORKDIR/cleanup.json" -w '%{http_code}' \
        -H 'Content-Type: application/json' \
        -H "Authorization: Bearer $TOKEN" \
        -d "$body" "$BASE_URL/v3/forest.DeleteForest" 2>/dev/null)" || status="curl-error"
      log "cleanup: http_status=$status body=$(cat "$WORKDIR/cleanup.json" 2>/dev/null | head -c 500)"
    else
      log "cleanup: skipped, failed to build DeleteForest body"
    fi
  fi
  rm -rf "$WORKDIR"
  exit "$rc"
}
trap cleanup EXIT

now_ms() {
  local t
  t="$(date +%s%3N)"
  if [[ "$t" == *N* ]]; then
    t="$(( $(date +%s) * 1000 ))"
  fi
  printf '%s' "$t"
}

jqval() { jq -r "$1" <<<"$HTTP_BODY" 2>/dev/null || true; }

post() {
  local action="$1" body="$2" auth="${3-$TOKEN}" timeout="${4:-$HTTP_TIMEOUT}"
  local -a hdr=(-H 'Content-Type: application/json')
  if [[ -n "$auth" ]]; then
    hdr+=(-H "Authorization: Bearer $auth")
  fi
  local out="$WORKDIR/resp.json" err="$WORKDIR/curl.err"
  if ! HTTP_STATUS="$(curl -sS --max-time "$timeout" -o "$out" -w '%{http_code}' \
    "${hdr[@]}" -d "$body" "$BASE_URL/v3/$action" 2>"$err")"; then
    HTTP_BODY="$(cat "$out" 2>/dev/null || true)"
    fail "$CURRENT_STAGE" "curl error for $action: $(cat "$err" 2>/dev/null || true)"
  fi
  HTTP_BODY="$(cat "$out")"
}

assert_status() {
  if [[ "$HTTP_STATUS" != "$1" ]]; then
    fail "$CURRENT_STAGE" "expected http_status=$1 got=$HTTP_STATUS"
  fi
}

assert_code() {
  local code
  code="$(jqval '.code')"
  if [[ "$code" != "0" ]]; then
    fail "$CURRENT_STAGE" "expected .code=0 got=${code:-<missing>}"
  fi
}

assert_nonempty() {
  if [[ -z "$1" || "$1" == "null" ]]; then
    fail "$CURRENT_STAGE" "$2 is empty"
  fi
}

assert_val() {
  if [[ "$1" != "$2" ]]; then
    fail "$CURRENT_STAGE" "expected $3=$2 got=$1"
  fi
}

stage_health() {
  local out="$WORKDIR/resp.json" err="$WORKDIR/curl.err"
  if ! HTTP_STATUS="$(curl -sS --max-time 10 -o "$out" -w '%{http_code}' "$BASE_URL/healthz" 2>"$err")"; then
    HTTP_BODY="$(cat "$out" 2>/dev/null || true)"
    fail health "curl error: $(cat "$err" 2>/dev/null || true)"
  fi
  HTTP_BODY="$(cat "$out")"
  assert_status 200
  assert_val "$(jqval '.status')" "ok" ".status"
}

stage_login() {
  local body
  body="$(jq -n \
    --arg domain "$TEST_DOMAIN" \
    --arg user "$TEST_USERNAME" \
    --arg pass "$TEST_PASSWORD_MD5" \
    '{cmd:"account.LoginByPassword", request:{domain_name:$domain, username:$user, password:$pass}}')"
  post account.LoginByPassword "$body" ""
  assert_status 200
  TOKEN="$(jqval '((.response // .Response).jwt_token) // empty')"
  assert_nonempty "$TOKEN" "jwt_token"
}

stage_create_forest() {
  local body
  body="$(jq -n --arg name "golden-path-$RUN_ID" \
    '{cmd:"forest.CreateForest", request:{name:$name, description:"phase0 golden path", public_scope:"private", forest_type:"file", data_source_type:"standard", data_source_subtype:"standard"}}')"
  post forest.CreateForest "$body"
  assert_status 200
  FOREST_ID="$(jqval '((.Response // .response).forest_id) // empty')"
  assert_nonempty "$FOREST_ID" "forest_id"
}

stage_preupload() {
  printf 'golden path regression file\nexpect_token: %s\nphase0 foundation baseline\n' "$EXPECT_TEXT" >"$TMP_FILE"
  SHA256="$(sha256sum "$TMP_FILE" | awk '{print $1}')"
  SIZE="$(wc -c <"$TMP_FILE" | tr -d '[:space:]')"
  local body multipart
  body="$(jq -n \
    --argjson forest_id "$FOREST_ID" \
    --arg hash "sha256:$SHA256" \
    --argjson size "$SIZE" \
    '{cmd:"forest.PreUploadFile", request:{forest_id:$forest_id, parent_id:0, files:[{filename:"golden.txt", hash:$hash, size:$size, content_type:"text/plain", split_config:{split_mode:"auto", chunk_size:512, split_mark:["\n\n"], split_overlap:0, preprocessing_rules:{remove_empty_line:true, remove_url:false, remove_email:false}}}]}}')"
  post forest.PreUploadFile "$body"
  assert_status 200
  multipart="$(jqval '((.Response // .response).files[0].multipart.enabled) // false')"
  if [[ "$multipart" == "true" ]]; then
    fail preupload "multipart.enabled=true: the test file must be below the multipart threshold"
  fi
  UPLOAD_ID="$(jqval '((.Response // .response).files[0].upload_id) // empty')"
  assert_nonempty "$UPLOAD_ID" "files[0].upload_id"
  UPLOAD_URL="$(jqval '(((.Response // .response).files[0].upload_urls // {}) | to_entries | .[0].value) // empty')"
  assert_nonempty "$UPLOAD_URL" "files[0].upload_urls first value (file may already exist; use a unique EXPECT_TEXT)"
}

stage_upload() {
  local out="$WORKDIR/upload.out"
  if ! curl -fsS --max-time "$HTTP_TIMEOUT" -X PUT --data-binary @"$TMP_FILE" "$UPLOAD_URL" >"$out" 2>&1; then
    HTTP_BODY="$(cat "$out" 2>/dev/null || true)"
    fail upload "PUT to presigned upload_url failed"
  fi
}

stage_callback() {
  local body
  body="$(jq -n \
    --argjson forest_id "$FOREST_ID" \
    --argjson upload_id "$UPLOAD_ID" \
    --arg hash "sha256:$SHA256" \
    '{cmd:"forest.UploadFileCallBack", request:{forest_id:$forest_id, upload_id:$upload_id, filename:"golden.txt", hash:$hash, parts:[]}}')"
  post forest.UploadFileCallBack "$body"
  assert_status 200
  FILE_ID="$(jqval '((.Response // .response).file_id) // empty')"
  assert_nonempty "$FILE_ID" "file_id"
}

stage_parse() {
  local body parse_status knowledge_status deadline
  body="$(jq -n --argjson file_id "$FILE_ID" '{cmd:"forest.GetFileInfo", request:{file_id:$file_id}}')"
  deadline=$(( SECONDS + PARSE_TIMEOUT ))
  while :; do
    post forest.GetFileInfo "$body"
    assert_status 200
    parse_status="$(jqval '((.Response // .response).parse_status) // empty')"
    knowledge_status="$(jqval '((.Response // .response).knowledge_status) // empty')"
    if [[ "$parse_status" == success && "$knowledge_status" == success ]]; then
      return 0
    fi
    if (( SECONDS >= deadline )); then
      fail parse "timeout after ${PARSE_TIMEOUT}s: parse_status=${parse_status:-<empty>} knowledge_status=${knowledge_status:-<empty>}"
    fi
    log "parse: waiting parse_status=${parse_status:-<empty>} knowledge_status=${knowledge_status:-<empty>}"
    sleep "$PARSE_INTERVAL"
  done
}

stage_search() {
  local body n
  body="$(jq -n \
    --argjson forest_id "$FOREST_ID" \
    --arg text "$EXPECT_TEXT" \
    '{cmd:"kesearch.ForestSearch", request:{forest_id:$forest_id, text:$text, is_semantics:false}}')"
  post kesearch.ForestSearch "$body"
  assert_status 200
  assert_code
  n="$(jqval '((.Response // .response).doc_search_result) | length')"
  if ! (( n > 0 )); then
    fail search "doc_search_result length=${n:-0}, expected > 0"
  fi
}

stage_chat() {
  local body answer
  body="$(jq -n \
    --argjson model_id "$MODEL_ID" \
    --argjson forest_id "$FOREST_ID" \
    --arg question "$EXPECT_TEXT 是什么？" \
    '{cmd:"chat.SubmitChatQuestion", request:{session_id:0, model_id:$model_id, base_type:"standard", resource_type:"forest", resource_ids:[$forest_id], resource_names:["golden-path"], question:$question}}')"
  post chat.SubmitChatQuestion "$body" "$TOKEN" "$CHAT_TIMEOUT"
  assert_status 200
  assert_code
  answer="$(jqval '((.Response // .response).answer) // empty')"
  assert_nonempty "$answer" "answer"
}

stage_keapi_search() {
  local body n
  body="$(jq -n \
    --argjson forest_id "$FOREST_ID" \
    --arg query "$EXPECT_TEXT" \
    '{cmd:"keapi.Search", request:{forest_ids:[$forest_id], query:$query}}')"
  post keapi.Search "$body" "$KEAPI_API_KEY"
  assert_status 200
  assert_code
  n="$(jqval '((.response // .Response).doc_search_result) | length')"
  if ! (( n > 0 )); then
    fail keapi_search "doc_search_result length=${n:-0}, expected > 0"
  fi
}

stage_mcp() {
  local hdr_file="$WORKDIR/mcp_headers.txt" out="$WORKDIR/resp.json" err="$WORKDIR/curl.err"
  local init_body tools_body sid
  init_body="$(jq -n '{jsonrpc:"2.0", id:1, method:"initialize", params:{protocolVersion:"2025-03-26", capabilities:{}, clientInfo:{name:"golden-path", version:"1.0"}}}')"
  if ! HTTP_STATUS="$(curl -sS --max-time 60 -D "$hdr_file" -o "$out" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -H "Authorization: Bearer $KEAPI_API_KEY" \
    -d "$init_body" "$BASE_URL/v3/keapi/mcp" 2>"$err")"; then
    HTTP_BODY="$(cat "$out" 2>/dev/null || true)"
    fail mcp "curl error on initialize: $(cat "$err" 2>/dev/null || true)"
  fi
  HTTP_BODY="$(cat "$out")"
  assert_status 200
  if ! grep -q '"result"' <<<"$HTTP_BODY"; then
    fail mcp 'initialize response does not contain "result"'
  fi
  sid="$(grep -i '^mcp-session-id:' "$hdr_file" | head -n1 | sed 's/^[^:]*:[[:space:]]*//; s/\r$//' || true)"
  if [[ -z "$sid" ]]; then
    log "mcp: no Mcp-Session-Id header returned, skipping tools/list"
    return 0
  fi
  log "mcp: initialize ok, sending tools/list with Mcp-Session-Id"
  tools_body="$(jq -n '{jsonrpc:"2.0", id:2, method:"tools/list", params:{}}')"
  if ! HTTP_STATUS="$(curl -sS --max-time 60 -o "$out" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -H "Authorization: Bearer $KEAPI_API_KEY" \
    -H "Mcp-Session-Id: $sid" \
    -d "$tools_body" "$BASE_URL/v3/keapi/mcp" 2>"$err")"; then
    HTTP_BODY="$(cat "$out" 2>/dev/null || true)"
    fail mcp "curl error on tools/list: $(cat "$err" 2>/dev/null || true)"
  fi
  HTTP_BODY="$(cat "$out")"
  assert_status 200
  if ! grep -q '"result"' <<<"$HTTP_BODY"; then
    fail mcp 'tools/list response does not contain "result"'
  fi
}

run_stage() {
  local name="$1" start end
  CURRENT_STAGE="$name"
  HTTP_STATUS=""
  HTTP_BODY=""
  start="$(now_ms)"
  "stage_$name"
  end="$(now_ms)"
  log "stage=$name ok elapsed_ms=$(( end - start ))"
  TIMING_LINES+=("stage=$name elapsed_ms=$(( end - start ))")
}

log "run_id=$RUN_ID base_url=$BASE_URL expect_text=$EXPECT_TEXT cleanup=$CLEANUP health_only=$HEALTH_ONLY"
for s in "${RUN_STAGES[@]}"; do
  run_stage "$s"
done
log "summary:"
for line in "${TIMING_LINES[@]}"; do
  log "  $line"
done
log "all ${#RUN_STAGES[@]} stage(s) passed"
