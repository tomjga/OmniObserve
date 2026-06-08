#!/usr/bin/env bash
# enable-ha.sh — stand up the High-Availability posture for a chosen scenario,
# on AWS, Azure, or both.
#
#   ./enable-ha.sh --scenario ec2+vm --cloud both
#   ./enable-ha.sh --scenario b --cloud aws --apply --auto-approve   # (deferred: needs creds)
#
# HA here = the redundant compute pair for the scenario + health-checked traffic
# routing. DR (cross-region standby, backups, RTO/RPO) is layered by enable-dr.sh.
#
# SAFETY: defaults to --dry-run (plan only). It never runs `apply` unless you pass
# BOTH --apply and --auto-approve, and even then only if an IaC binary + cloud
# credentials are present. This increment ships scaffolding, not live infra.

set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/_tofu.sh"

SCENARIO="" CLOUD="both" DRY_RUN=1 AUTO_APPROVE=""

usage() {
  cat <<EOF
${C_BOLD}enable-ha.sh${C_RESET} — provision the HA pair for a multi-cloud scenario (OpenTofu).

${C_BOLD}Usage:${C_RESET}
  enable-ha.sh --scenario <A|B|C|ec2+vm|ec2+aks|vm+eks> [--cloud aws|azure|both] [options]

${C_BOLD}Scenarios:${C_RESET}
  A  ec2+vm    1 AWS EC2  + 1 Azure VM      cheapest cross-cloud HA   (~\$15–30/mo)
  B  ec2+aks   1 AWS EC2  + Azure AKS       IaaS vs managed-K8s       (~\$90–140/mo)
  C  vm+eks    1 Azure VM + AWS EKS         mirror of B               (~\$90–140/mo)

${C_BOLD}Options:${C_RESET}
  --scenario <id>     required; one of the aliases above
  --cloud <target>    aws | azure | both        (default: both)
  --dry-run           plan only, print, change nothing   (DEFAULT)
  --apply             intend to apply (still needs --auto-approve + creds)
  --auto-approve      skip interactive approval on apply
  -h, --help          this help

Pricing is approximate (smallest burstable types, on-demand). See
infrastructure/docs/MULTICLOUD-HA-DR.md for the full design, pros/cons and RTO/RPO.
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

SCEN_DIR_NAME="$(resolve_scenario "$SCENARIO")" || { err "Unknown scenario '$SCENARIO' (use A|B|C or ec2+vm|ec2+aks|vm+eks)"; exit 2; }
SCEN_DIR="$SCENARIOS_DIR/$SCEN_DIR_NAME"
[[ -d "$SCEN_DIR" ]] || { err "Scenario directory missing: $SCEN_DIR"; exit 1; }
TFVARS="$ENVS_DIR/ha.tfvars"; [[ -f "$TFVARS" ]] || TFVARS=""

hr
info "Posture : ${C_BOLD}HA${C_RESET}"
info "Scenario: ${C_BOLD}$SCEN_DIR_NAME${C_RESET}  (cloud target: ${C_BOLD}$CLOUD${C_RESET})"
info "Mode    : ${C_BOLD}$([[ $DRY_RUN -eq 1 ]] && echo 'DRY-RUN (plan only)' || echo 'APPLY')${C_RESET}"
hr

if ! detect_iac_bin >/dev/null; then
  print_tooling_guidance
fi
check_cloud_clis "$CLOUD" || warn "Cloud CLI(s) missing — dry-run will still print the plan command."

if [[ $DRY_RUN -eq 1 ]]; then
  run_iac plan "$SCEN_DIR" "$TFVARS" ""
  echo
  ok "Dry-run complete. Nothing was provisioned."
  info "To really apply (deferred until creds are wired): add ${C_BOLD}--apply --auto-approve${C_RESET}."
else
  if [[ -z "$AUTO_APPROVE" ]]; then
    err "--apply requires --auto-approve (guards against accidental provisioning)."; exit 2
  fi
  if ! detect_iac_bin >/dev/null || ! check_cloud_clis "$CLOUD"; then
    err "Refusing to apply: IaC binary and/or cloud CLIs are not available + authenticated."
    info "This increment is design + dry-run only. Install tooling, authenticate, then re-run."
    exit 1
  fi
  warn "APPLY path is intentionally inert in this increment (no provider/credentials wired)."
  run_iac apply "$SCEN_DIR" "$TFVARS" "$AUTO_APPROVE"
fi
