#!/usr/bin/env bash
# Reference helper for statusline integration — monthly spend.
# Uses corrected_cost_usd when a spend override is active, falls back to cost_usd.
claude-otlp status monthly 2>/dev/null | jq -r '(.corrected_cost_usd // .cost_usd | tostring | .[0:6]) + " /mo"'
