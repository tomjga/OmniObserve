#!/usr/bin/env bash
# Shared helpers for the enable-ha / enable-dr / destroy wrappers.
#
# Design note: we standardize on OpenTofu but stay tool-agnostic at the call site —
# auto-detect `tofu`, fall back to `terraform`. The HCL is identical for both, so a
# contributor with only Terraform installed can still drive the scenarios.
#
# This file is meant to be `source`d, not executed.

set -euo pipefail

# Repo-relative roots (this file lives in infrastructure/scripts/).
INFRA_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCENARIOS_DIR="$INFRA_DIR/scenarios"
ENVS_DIR="$INFRA_DIR/envs"

# --- pretty output -----------------------------------------------------------
if [[ -t 1 ]]; then
  C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_DIM=$'\033[2m'
  C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YEL=$'\033[33m'; C_BLU=$'\033[34m'
else
  C_RESET=""; C_BOLD=""; C_DIM=""; C_RED=""; C_GRN=""; C_YEL=""; C_BLU=""
fi
info()  { printf '%s\n' "${C_BLU}${C_BOLD}▸${C_RESET} $*"; }
ok()    { printf '%s\n' "${C_GRN}✓${C_RESET} $*"; }
warn()  { printf '%s\n' "${C_YEL}⚠${C_RESET} $*" >&2; }
err()   { printf '%s\n' "${C_RED}✗${C_RESET} $*" >&2; }
hr()    { printf '%s\n' "${C_DIM}────────────────────────────────────────────────────────${C_RESET}"; }

# --- IaC binary detection (tofu preferred, terraform fallback) ---------------
detect_iac_bin() {
  if command -v tofu >/dev/null 2>&1; then
    echo "tofu"; return 0
  fi
  if command -v terraform >/dev/null 2>&1; then
    echo "terraform"; return 0
  fi
  return 1
}

# Map a scenario alias the user types to its directory name.
#   ec2+vm | a   -> ec2-plus-vm
#   ec2+aks | b  -> ec2-plus-aks
#   vm+eks | c   -> vm-plus-eks
resolve_scenario() {
  case "${1,,}" in
    a|ec2+vm|ec2-plus-vm)   echo "ec2-plus-vm" ;;
    b|ec2+aks|ec2-plus-aks) echo "ec2-plus-aks" ;;
    c|vm+eks|vm-plus-eks)   echo "vm-plus-eks" ;;
    *) return 1 ;;
  esac
}

# Print, but never run, the install guidance when tooling is missing.
print_tooling_guidance() {
  hr
  warn "Required tooling not found on this machine."
  cat <<'EOF'

This increment is design + dry-run only, but to run even `plan` you need an IaC
binary, and to actually authenticate you'll need the cloud CLIs:

  OpenTofu (preferred)
    macOS:  brew install opentofu
    docs:   https://opentofu.org/docs/intro/install/
  Terraform (fallback)
    macOS:  brew install hashicorp/tap/terraform

  AWS CLI    brew install awscli   &&  aws configure   # or AWS SSO / OIDC
  Azure CLI  brew install azure-cli &&  az login

No credentials are read or written by this script. Nothing is provisioned.
EOF
  hr
}

# Verify cloud CLIs for the chosen --cloud target. Returns non-zero if any are
# missing, but only *reports* — callers decide whether that's fatal (it isn't in
# dry-run, where we still want to show the plan command we *would* run).
check_cloud_clis() {
  local cloud="$1" missing=0
  if [[ "$cloud" == "aws" || "$cloud" == "both" ]]; then
    command -v aws >/dev/null 2>&1 || { warn "aws CLI not found (needed for --cloud aws/both)"; missing=1; }
  fi
  if [[ "$cloud" == "azure" || "$cloud" == "both" ]]; then
    command -v az  >/dev/null 2>&1 || { warn "az CLI not found (needed for --cloud azure/both)"; missing=1; }
  fi
  return $missing
}

# Echo the tofu/terraform invocation we would run for a scenario, and run it only
# when an IaC binary exists. In --dry-run we cap at `plan`; apply requires
# --auto-approve AND credentials (deferred this increment).
run_iac() {
  local action="$1" scenario_dir="$2" tfvars="$3" auto_approve="$4"
  local bin; bin="$(detect_iac_bin || true)"
  local rel="infrastructure/scenarios/$(basename "$scenario_dir")"

  info "Working dir: ${C_BOLD}$rel${C_RESET}"
  if [[ -n "$tfvars" ]]; then info "Var file:    ${C_BOLD}$(basename "$tfvars")${C_RESET}"; fi

  if [[ -z "$bin" ]]; then
    warn "No tofu/terraform binary — printing the commands that WOULD run:"
    printf '    %s\n' "tofu -chdir=$rel init"
    if [[ "$action" == "plan" ]]; then
      printf '    %s\n' "tofu -chdir=$rel plan${tfvars:+ -var-file=$(basename "$tfvars")}"
    else
      printf '    %s\n' "tofu -chdir=$rel apply${tfvars:+ -var-file=$(basename "$tfvars")}${auto_approve:+ -auto-approve}"
    fi
    return 0
  fi

  ok "Using ${C_BOLD}$bin${C_RESET}"
  info "$bin -chdir=$rel init"
  "$bin" -chdir="$scenario_dir" init -input=false
  if [[ "$action" == "plan" ]]; then
    "$bin" -chdir="$scenario_dir" plan -input=false ${tfvars:+-var-file="$tfvars"}
  else
    "$bin" -chdir="$scenario_dir" apply -input=false ${tfvars:+-var-file="$tfvars"} ${auto_approve:+-auto-approve}
  fi
}
