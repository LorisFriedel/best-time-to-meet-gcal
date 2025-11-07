#!/bin/bash
# Example script showing how to use the JSON output feature

echo "Example 1: Get the recommended meeting time"
echo "============================================"
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60 \
  --json | jq -r '.recommendation | "Best time: \(.start_time) to \(.end_time) (\(.reason))"'

echo -e "\n\nExample 2: List all perfect slots"
echo "================================="
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60 \
  --json | jq -r '.best_options.perfect_slots[] | "  • \(.start_time | split("T")[1] | split("-")[0]) on \(.start_time | split("T")[0])"'

echo -e "\n\nExample 3: Get daily summary"
echo "============================="
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60 \
  --json | jq -r '.daily_summary[] | "  • \(.date): \(.total_slots) slots, \(.perfect_slots) perfect, avg conflict: \(.average_conflict_percentage)%"'

echo -e "\n\nExample 4: Filter slots by conflict threshold"
echo "=============================================="
./best-time-to-meet \
  --emails "alice@company.com,bob@company.com" \
  --start "2024-01-15" \
  --end "2024-01-19" \
  --duration 60 \
  --max-conflicts 25 \
  --json | jq -r '.detailed_slots[] | select(.conflict_percentage <= 25) | "  • \(.start_time): \(.conflict_percentage)% conflict"'
