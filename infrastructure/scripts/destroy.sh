#!/usr/bin/env bash
# destroy.sh — tear down a scenario on a chosen cloud target.
#
#   ./destroy.sh --scenario ec2+vm --cloud both
#
# Destroying is how this stays cheap: spin a demo up, capture it, tear it down.
# Same guardrails as the enable scripts — dry-run by default; real destroy needs
# --apply --auto-approve plus present-and-authenticated tooling.

set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_tofu.sh"

SCENARIO="" CLOUD="both" DRY_RUN=1 AUTO_APPROVE=""

usage() {
  cat <<EOF
${C_BOLD}destroy.sh${C_RESET} — tear down a multi-cloud scenario (OpenTofu).

  destroy.sh --scenario <A|B|C|ec2+vm|ec2+aks|vm+eks> [--cloud aws|azure|both]
             [--dry-run | --apply --auto-approve]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scenario) SCENARIO="${2:-}"; shift 2 ;;
    --cloud)    CLOUD="${2:-}"; shift 2 ;;
    --dry-run)  DRY_RUN=1; shift ;;
    --apply)    DRY_RUN=0; shift ;;
    --auto-approve) AUTO_APPROVE=1; shift ;;
    -h|--help)  usage; exit 0 ;;
    *) err "Unknown argument: $1"; echo; usage; exit 2 ;;
  esac
done

[[ -n "$SCENARIO" ]] || { err "--scenario is required"; echo; usage; exit 2; }
case "$CLOUD" in aws|azure|both) ;; *) err "--cloud must be aws|azure|both"; exit 2 ;; esac
SCEN_DIR_NAME="$(resolve_scenario "$SCENARIO")" || { err "Unknown scenario '$SCENARIO'"; exit 2; }
SCEN_DIR="$SCENARIOS_DIR/$SCEN_DIR_NAME"
[[ -d "$SCEN_DIR" ]] || { err "Scenario directory missing: $SCEN_DIR"; exit 1; }
rel="infrastructure/scenarios/$SCEN_DIR_NAME"

hr
info "Destroy : ${C_BOLD}$SCEN_DIR_NAME${C_RESET}  (cloud target: ${C_BOLD}$CLOUD${C_RESET})"
info "Mode    : ${C_BOLD}$([[ $DRY_RUN -eq 1 ]] && echo 'DRY-RUN' || echo 'APPLY')${C_RESET}"
hr

bin="$(detect_iac_bin || true)"
if [[ -z "$bin" ]]; then
  print_tooling_guidance
  warn "Command that WOULD run:"
  printf '    %s\n' "tofu -chdir=$rel destroy"
  exit 0
fi

if [[ $DRY_RUN -eq 1 ]]; then
  info "$bin -chdir=$rel plan -destroy"
  "$bin" -chdir="$SCEN_DIR" plan -destroy -input=false
  ok "Dry-run complete. Nothing was destroyed."
else
  [[ -n "$AUTO_APPROVE" ]] || { err "--apply requires --auto-approve"; exit 2; }
  warn "DESTROY path is intentionally inert in this increment (no provider/credentials wired)."
  "$bin" -chdir="$SCEN_DIR" destroy -input=false ${AUTO_APPROVE:+-auto-approve}
fi
