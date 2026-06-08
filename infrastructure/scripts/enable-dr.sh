#!/usr/bin/env bash
# enable-dr.sh — layer the Disaster-Recovery posture on top of a scenario's HA pair.
#
#   ./enable-dr.sh --scenario ec2+vm --cloud both
#
# DR adds, on top of HA: a cross-region / cross-cloud standby, state & data backup
# replication, health-checked failover routing (dns-failover module), and the
# documented RTO/RPO targets. It also wires the DR drill (scheduled destroy+apply /
# failover test) referenced in the design doc.
#
# SAFETY: identical guardrails to enable-ha.sh — defaults to --dry-run, never
# applies without --apply + --auto-approve + present-and-authenticated tooling.

set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_tofu.sh"

SCENARIO="" CLOUD="both" DRY_RUN=1 AUTO_APPROVE=""

usage() {
  cat <<EOF
${C_BOLD}enable-dr.sh${C_RESET} — add the Disaster-Recovery layer to a scenario (OpenTofu).

${C_BOLD}Usage:${C_RESET}
  enable-dr.sh --scenario <A|B|C|ec2+vm|ec2+aks|vm+eks> [--cloud aws|azure|both] [options]

${C_BOLD}Adds:${C_RESET} cross-region/cross-cloud standby · backup replication ·
       health-checked failover (dns-failover) · RTO/RPO targets · DR drill.

${C_BOLD}Options:${C_RESET}
  --scenario <id>     required (A|B|C or ec2+vm|ec2+aks|vm+eks)
  --cloud <target>    aws | azure | both        (default: both)
  --dry-run           plan only, change nothing  (DEFAULT)
  --apply             intend to apply (needs --auto-approve + creds)
  --auto-approve      skip interactive approval on apply
  -h, --help          this help

DR raises cost (standby + replication). See infrastructure/docs/MULTICLOUD-HA-DR.md
for RTO/RPO targets, failover design and the DR-drill schedule.
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
case "$CLOUD" in aws|azure|both) ;; *) err "--cloud must be aws|azure|both (got '$CLOUD')"; exit 2 ;; esac

SCEN_DIR_NAME="$(resolve_scenario "$SCENARIO")" || { err "Unknown scenario '$SCENARIO'"; exit 2; }
SCEN_DIR="$SCENARIOS_DIR/$SCEN_DIR_NAME"
[[ -d "$SCEN_DIR" ]] || { err "Scenario directory missing: $SCEN_DIR"; exit 1; }
TFVARS="$ENVS_DIR/dr.tfvars"; [[ -f "$TFVARS" ]] || TFVARS=""

hr
info "Posture : ${C_BOLD}DR (HA + recovery layer)${C_RESET}"
info "Scenario: ${C_BOLD}$SCEN_DIR_NAME${C_RESET}  (cloud target: ${C_BOLD}$CLOUD${C_RESET})"
info "Mode    : ${C_BOLD}$([[ $DRY_RUN -eq 1 ]] && echo 'DRY-RUN (plan only)' || echo 'APPLY')${C_RESET}"
warn "Reminder: run enable-ha.sh for this scenario first — DR builds on the HA pair."
hr

if ! detect_iac_bin >/dev/null; then
  print_tooling_guidance
fi
check_cloud_clis "$CLOUD" || warn "Cloud CLI(s) missing — dry-run will still print the plan command."

if [[ $DRY_RUN -eq 1 ]]; then
  run_iac plan "$SCEN_DIR" "$TFVARS" ""
  echo
  ok "Dry-run complete. Nothing was provisioned."
  info "RTO/RPO targets and the DR drill are documented in MULTICLOUD-HA-DR.md."
else
  if [[ -z "$AUTO_APPROVE" ]]; then
    err "--apply requires --auto-approve (guards against accidental provisioning)."; exit 2
  fi
  if ! detect_iac_bin >/dev/null || ! check_cloud_clis "$CLOUD"; then
    err "Refusing to apply: IaC binary and/or cloud CLIs are not available + authenticated."
    exit 1
  fi
  warn "APPLY path is intentionally inert in this increment (no provider/credentials wired)."
  run_iac apply "$SCEN_DIR" "$TFVARS" "$AUTO_APPROVE"
fi
