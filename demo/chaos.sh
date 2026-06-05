#!/usr/bin/env bash
set -euo pipefail
# demo/chaos.sh — drive the whole self-heal loop with one command, unattended.
#
# Flips the productCatalogFailure feature flag ON (the fault), then watches the
# autonomous loop run: product-catalog starts erroring -> the SLO burn alert fires
# -> Alertmanager calls the remediator -> the remediator disables the flag (heal)
# -> the RCA copilot drafts a grounded analysis. The script reports each transition
# with timing, so a recording shows the payoff without any manual steps.
#
#   ./demo/chaos.sh            run the full loop (inject -> watch heal -> report RCA)
#   ./demo/chaos.sh --reset    force the flag back off (cleanup after an aborted run)
#   ./demo/chaos.sh --status   show the current flag state + recent remediator activity
#
# The existing otel-demo load generator already drives traffic to product-catalog,
# so no extra load is needed. Requires: kubectl context on the local cluster, jq.

FLAG="productCatalogFailure"
NS_DEMO="otel-demo"
NS_MON="monitoring"
CM="flagd-config"
KEY="demo.flagd.json"
DEPLOY="remediator"
TIMEOUT="${TIMEOUT:-240}"   # seconds to wait for the auto-heal before giving up

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
dim()  { printf '\033[2m%s\033[0m\n' "$1"; }

# kubectl jsonpath needs the dots in "demo.flagd.json" escaped, else it reads them as path separators.
KEY_JP="${KEY//./\\.}"

flag_variant() {
  kubectl -n "$NS_DEMO" get cm "$CM" -o jsonpath="{.data.$KEY_JP}" \
    | jq -r ".flags[\"$FLAG\"].defaultVariant"
}

set_variant() {  # $1 = on|off  — read-modify-write only the one nested key, preserving the rest
  local cur new
  cur="$(kubectl -n "$NS_DEMO" get cm "$CM" -o jsonpath="{.data.$KEY_JP}")"
  new="$(printf '%s' "$cur" | jq -c ".flags[\"$FLAG\"].defaultVariant=\"$1\"")"
  kubectl -n "$NS_DEMO" patch cm "$CM" --type merge \
    -p "$(jq -n --arg v "$new" "{data:{\"$KEY\":\$v}}")" >/dev/null
}

copilot_state() {  # prints e.g.  enabled:true model:gemini-2.5-flash corpus_size:7
  kubectl -n "$NS_MON" logs "deploy/$DEPLOY" 2>/dev/null \
    | grep -m1 '"msg":"rca copilot"' \
    | jq -r '" enabled:\(.enabled) model:\(.model) corpus_size:\(.corpus_size)"' 2>/dev/null || true
}

require() {
  command -v jq >/dev/null || { echo "jq is required"; exit 1; }
  kubectl -n "$NS_DEMO" get cm "$CM" >/dev/null 2>&1 || {
    echo "cannot reach the cluster / $CM not found — is the local cluster up and bootstrapped?"; exit 1; }
}

case "${1:-run}" in
  --reset)
    require
    set_variant off
    bold "flag $FLAG forced back to: $(flag_variant)"
    exit 0 ;;
  --status)
    require
    bold "flag $FLAG defaultVariant: $(flag_variant)"
    echo "copilot:$(copilot_state)"
    dim "recent remediator activity:"
    kubectl -n "$NS_MON" logs "deploy/$DEPLOY" --tail=400 2>/dev/null \
      | grep -E "remediation|rca (drafted|published)" | tail -n 8 || echo "  (none yet)"
    exit 0 ;;
  run|"") : ;;
  *) echo "usage: $0 [--reset|--status]"; exit 2 ;;
esac

require

if [ "$(flag_variant)" = "on" ]; then
  echo "flag is already ON — resetting first so we observe a clean heal"
  set_variant off; sleep 2
fi

bold "RCA copilot:$(copilot_state)"
echo

# Mark the log cursor BEFORE injecting, so we only read this run's lines.
since="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
start="$(date +%s)"

bold "==> injecting fault: $FLAG -> on"
set_variant on
echo "    product-catalog will now throw on the target product; the otel-demo"
echo "    load generator is already exercising it."
echo
dim "    expected timeline:  alert fires ~60s  ·  remediator heals ~75s"
echo

healed=0
while :; do
  now="$(date +%s)"; elapsed=$(( now - start ))
  [ "$elapsed" -ge "$TIMEOUT" ] && break
  if [ "$(flag_variant)" = "off" ]; then healed=1; break; fi
  printf '\r    waiting for autonomous heal… %3ds  (flag still ON)' "$elapsed"
  sleep 5
done
printf '\r%*s\r' 60 ''   # clear the progress line

if [ "$healed" -ne 1 ]; then
  bold "✗ no heal within ${TIMEOUT}s — the flag is still ON"
  dim "diagnostics:"
  echo "  • alert firing?   kubectl -n $NS_MON get prometheusrule -A | grep ProductCatalog"
  echo "  • webhook calls?  kubectl -n $NS_MON logs deploy/$DEPLOY | grep 'alert received'"
  echo "  • load present?   kubectl -n $NS_DEMO get deploy load-generator"
  echo
  echo "  leave it to inspect, or clean up with:  ./demo/chaos.sh --reset"
  exit 1
fi

bold "✓ healed in ${elapsed}s — remediator disabled $FLAG (flag now: $(flag_variant))"
echo

# Pull this run's remediator log lines and surface the action + RCA outcome.
logs="$(kubectl -n "$NS_MON" logs "deploy/$DEPLOY" --since-time="$since" 2>/dev/null || true)"

dim "remediation:"
echo "$logs" | grep -E "\"remediation\"|outcome" | grep -i "$FLAG" | tail -n 2 | sed 's/^/  /' || true
echo

# The RCA runs async after the heal; publishing to sinks follows the draft by a second or
# two, so once we see "drafted" we allow a short grace window to also catch "published".
echo "    waiting for the RCA copilot to draft…"
grace=0
for _ in $(seq 1 24); do
  logs="$(kubectl -n "$NS_MON" logs "deploy/$DEPLOY" --since-time="$since" 2>/dev/null || true)"
  echo "$logs" | grep -qE "rca published|rca draft failed" && break
  if echo "$logs" | grep -q "rca drafted"; then
    grace=$((grace + 1)); [ "$grace" -ge 2 ] && break   # drafted but no publish after ~6s → no sinks
  fi
  sleep 3
done

if echo "$logs" | grep -q "rca drafted"; then
  bold "✓ RCA drafted (grounded in the incident corpus):"
  echo "$logs" | grep -E "rca drafted" | tail -n 1 | sed 's/^/  /'
  if echo "$logs" | grep -q "rca published"; then
    echo "$logs" | grep -E "rca published" | sed 's/^/  /'
  else
    dim "  (no sinks configured — the RCA was drafted but not published anywhere"
    dim "   visible yet. Enable the Grafana annotation sink to see the full text"
    dim "   on the incident timeline.)"
  fi
elif echo "$logs" | grep -q "rca draft failed"; then
  bold "✗ RCA draft failed:"
  echo "$logs" | grep "rca draft failed" | tail -n 1 | sed 's/^/  /'
else
  dim "RCA copilot is disabled (no LLM key) — the loop healed action-only."
  dim "Set rca.llm.* + the LLM_API_KEY secret to enable grounded RCAs."
fi

echo
bold "done — full loop ran unattended: fault -> alert -> heal -> RCA"
