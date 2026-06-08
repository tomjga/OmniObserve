package main

import "strings"

type AutonomyMode string

const (
	AutonomyObserve        AutonomyMode = "observe"
	AutonomySuggest        AutonomyMode = "suggest"
	AutonomyApproval       AutonomyMode = "approval"
	AutonomyAuto           AutonomyMode = "auto"
	AutonomyAutoWithVerify AutonomyMode = "auto-with-verify"
)

var autonomyMode = AutonomyAutoWithVerify

func parseAutonomyMode(raw string) (AutonomyMode, bool) {
	switch AutonomyMode(strings.ToLower(strings.TrimSpace(raw))) {
	case AutonomyObserve:
		return AutonomyObserve, true
	case AutonomySuggest:
		return AutonomySuggest, true
	case AutonomyApproval:
		return AutonomyApproval, true
	case AutonomyAuto:
		return AutonomyAuto, true
	case AutonomyAutoWithVerify:
		return AutonomyAutoWithVerify, true
	default:
		return AutonomyAutoWithVerify, false
	}
}

func effectiveAutonomy(global AutonomyMode, policy FaultPolicy) AutonomyMode {
	policyMode, ok := parseAutonomyMode(policy.Safety.Autonomy)
	if !ok || policy.Safety.Autonomy == "" {
		return global
	}
	if autonomyRank(global) < autonomyRank(policyMode) {
		return global
	}
	return policyMode
}

func autonomyRank(mode AutonomyMode) int {
	switch mode {
	case AutonomyObserve:
		return 0
	case AutonomySuggest:
		return 1
	case AutonomyApproval:
		return 2
	case AutonomyAuto:
		return 3
	case AutonomyAutoWithVerify:
		return 4
	default:
		return 0
	}
}
