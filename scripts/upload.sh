#!/bin/bash
set -euo pipefail

BASE_URL="${1:-http://localhost}"
PART_SIZE=$((5 * 1024 * 1024)) # 5MB

# ── Helpers ───────────────────────────────────────────────────────────────────

divider() { echo "────────────────────────────────────────────────────────"; }

# Uploads one part to S3 and reports it to the service.
# Prints nothing — returns only the ETag via stdout.
upload_part() {
  local file=$1 part_num=$2 offset=$3 bytes=$4 url=$5 job_id=$6

  local s3_response etag
  s3_response=$(dd if="$file" bs=1 skip="$offset" count="$bytes" 2>/dev/null \
    | curl -s -X PUT "$url" \
        -H "Content-Type: text/csv" \
        --data-binary @- \
        -D -)
  etag=$(echo "$s3_response" | grep -i "^etag:" | tr -d '\r' | awk '{print $2}' | tr -d '"')

  if [ -z "$etag" ]; then
    return 1
  fi

  curl -sf -X PATCH "$BASE_URL/v1/uploads/multipart/$job_id/part" \
    -H "Content-Type: application/json" \
    -d "{\"part_number\":$part_num,\"etag\":\"$etag\"}" > /dev/null

  echo "$etag"
}

print_summary() {
  local total_parts=$1
  local file_size=$2
  shift 2
  local etags=("$@")

  local threshold=10
  echo ""
  divider

  if [ "$total_parts" -le "$threshold" ]; then
    printf "  %-6s  %-14s  %s\n" "Part" "Bytes" "ETag"
    divider
    for i in "${!etags[@]}"; do
      local part_num=$((i + 1))
      local offset=$((i * PART_SIZE))
      local bytes

      if [ "$part_num" -eq "$total_parts" ]; then
        bytes=$(( file_size - offset ))
      else
        bytes=$PART_SIZE
      fi

      printf "  %-6s  %-14s  %s\n" "$part_num" "$bytes" "${etags[$i]}"
    done
  else
    printf "  %-6s  %-14s  %s\n" "Part" "Bytes" "ETag"
    divider
    # First 3
    for i in 0 1 2; do
      local part_num=$((i + 1))
      printf "  %-6s  %-14s  %s\n" "$part_num" "$PART_SIZE" "${etags[$i]}"
    done
    printf "  %-6s  %-14s  %s\n" "..." "..." "..."
    # Last part
    local last_i=$((total_parts - 1))
    local last_bytes=$(( file_size - last_i * PART_SIZE ))
    printf "  %-6s  %-14s  %s\n" "$total_parts" "$last_bytes" "${etags[$last_i]}"
    divider
    printf "  Total parts:  %d\n" "$total_parts"
    printf "  Total bytes:  %d\n" "$file_size"
  fi

  divider
}

complete_upload() {
  local job_id=$1
  shift
  local etags=("$@")

  local parts_json=""
  for i in "${!etags[@]}"; do
    local part_num=$((i + 1))
    [ -n "$parts_json" ] && parts_json="$parts_json,"
    parts_json="${parts_json}{\"part_number\":${part_num},\"etag\":\"${etags[$i]}\"}"
  done

  echo ""
  echo "Completing upload..."
  curl -s -X POST "$BASE_URL/v1/uploads/multipart/$job_id/complete" \
    -H "Content-Type: application/json" \
    -d "{\"parts\":[$parts_json]}" | jq .
}

# ── Main Upload Flow ─────────────────────────────────────────────────────────

do_upload() {
  local file=$1
  local filename file_size total_parts

  filename=$(basename "$file")
  file_size=$(wc -c < "$file")
  total_parts=$(( (file_size + PART_SIZE - 1) / PART_SIZE ))

  local state_file="/tmp/upload_$(echo "$filename" | tr ' ' '_').jobid"
  local job_id=""
  local etags=()

  if [ -f "$state_file" ]; then
    job_id=$(cat "$state_file")
    echo "Resuming upload for '$filename' (Job ID: $job_id)"
    echo ""

    local status_response
    status_response=$(curl -sf "$BASE_URL/v1/uploads/$job_id/status")
    local job_status
    job_status=$(echo "$status_response" | jq -r '.data.Status')

    if [ "$job_status" = "completed" ]; then
      echo "Upload already completed. Nothing to do."
      rm -f "$state_file"
      exit 0
    fi

    # Pre-fill etags for completed parts (use index assignment to keep positions stable)
    for i in $(seq 0 $((total_parts - 1))); do
      local part_num=$((i + 1))
      local existing_etag
      existing_etag=$(echo "$status_response" | jq -r ".data.Parts[] | select(.PartNumber == $part_num and .Status == \"completed\") | .ETag")
      etags[$i]="${existing_etag:-}"
    done

    local pending_parts
    pending_parts=$(echo "$status_response" | jq -r '.data.Parts[] | select(.Status != "completed") | .PartNumber')

    if [ -z "$pending_parts" ]; then
      echo "All parts already uploaded. Completing..."
      complete_upload "$job_id" "${etags[@]}"
      rm -f "$state_file"
      return
    fi

    local parts_param
    parts_param=$(echo "$pending_parts" | tr '\n' ',' | sed 's/,$//')
    local completed_count=$(( total_parts - $(echo "$pending_parts" | wc -w | tr -d ' ') ))

    divider
    echo "  File:        $filename"
    echo "  Total Parts: $total_parts  |  Completed: $completed_count  |  Pending: $(echo "$pending_parts" | tr '\n' ' ')"
    divider
    echo ""
    echo "Fetching fresh presigned URLs for pending parts..."

    local presign_response
    presign_response=$(curl -sf "$BASE_URL/v1/uploads/multipart/$job_id/presign?parts=$parts_param")
    echo ""

    local presign_index=0
    for part_num in $pending_parts; do
      local i=$((part_num - 1))
      local offset=$((i * PART_SIZE))
      local bytes

      if [ "$part_num" -eq "$total_parts" ]; then
        bytes=$((file_size - offset))
      else
        bytes=$PART_SIZE
      fi

      local url
      url=$(echo "$presign_response" | jq -r ".data.parts[$presign_index].url")

      printf "\r  Uploading: %d/%d  " "$part_num" "$total_parts"

      local etag
      if ! etag=$(upload_part "$file" "$part_num" "$offset" "$bytes" "$url" "$job_id"); then
        printf "\n"
        echo "  Error: No ETag for part $part_num. Aborting."
        exit 1
      fi

      etags[$i]="$etag"
      presign_index=$((presign_index + 1))
    done

    printf "\r  Uploading: %d/%d ✓\n" "$total_parts" "$total_parts"

  else
    divider
    echo "  File:        $filename"
    echo "  Size:        $file_size bytes"
    echo "  Total Parts: $total_parts"
    divider
    echo ""

    echo "Initializing multipart upload..."
    local init_response
    init_response=$(curl -s -X POST "$BASE_URL/v1/uploads/multipart/init" \
      -H "Content-Type: application/json" \
      -d "{\"filename\":\"$filename\",\"content_type\":\"text/csv\",\"total_size\":$file_size}")

    if [ "$(echo "$init_response" | jq -r '.status')" != "success" ]; then
      echo "Error: init failed — $(echo "$init_response" | jq -r '.error // .message // "unknown error"')"
      echo "Response: $init_response"
      exit 1
    fi

    job_id=$(echo "$init_response" | jq -r '.data.job_id')
    echo "$job_id" > "$state_file"
    echo "Job ID: $job_id"
    echo "State saved to $state_file — re-run this script to resume if interrupted."
    echo ""

    for i in $(seq 0 $((total_parts - 1))); do
      local part_num=$((i + 1))
      local offset=$((i * PART_SIZE))
      local bytes

      if [ "$part_num" -eq "$total_parts" ]; then
        bytes=$((file_size - offset))
      else
        bytes=$PART_SIZE
      fi

      local url
      url=$(echo "$init_response" | jq -r ".data.parts[$i].url")

      printf "\r  Uploading: %d/%d  " "$part_num" "$total_parts"

      local etag
      if ! etag=$(upload_part "$file" "$part_num" "$offset" "$bytes" "$url" "$job_id"); then
        printf "\n"
        echo "  Error: No ETag for part $part_num. Aborting."
        exit 1
      fi

      etags+=("$etag")
    done

    printf "\r  Uploading: %d/%d ✓\n" "$total_parts" "$total_parts"
  fi

  print_summary "$total_parts" "$file_size" "${etags[@]}"
  complete_upload "$job_id" "${etags[@]}"
  rm -f "$state_file"
}

# ── Main ──────────────────────────────────────────────────────────────────────

echo ""
divider
echo "  CSV Multipart Upload (Resumeable)"
divider
echo ""

read -rp "CSV Path: " FILE
echo ""

if [ ! -f "$FILE" ]; then
  echo "Error: file '$FILE' not found"
  exit 1
fi

do_upload "$FILE"
