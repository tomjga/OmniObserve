const endpointInput = document.querySelector("#endpointInput");
const tokenInput = document.querySelector("#tokenInput");
const prometheusInput = document.querySelector("#prometheusInput");
const controlRoomCard = document.querySelector("#controlRoomCard");
const openControlRoomButton = document.querySelector("#openControlRoomButton");
const closeControlRoomButton = document.querySelector("#closeControlRoomButton");
const quickRefreshButton = document.querySelector("#quickRefreshButton");
const refreshButton = document.querySelector("#refreshButton");
const reloadApprovalsButton = document.querySelector("#reloadApprovalsButton");
const clearLogButton = document.querySelector("#clearLogButton");
const healthStatus = document.querySelector("#healthStatus");
const pendingCount = document.querySelector("#pendingCount");
const lastRefresh = document.querySelector("#lastRefresh");
const approvalList = document.querySelector("#approvalList");
const metricList = document.querySelector("#metricList");
const activityLog = document.querySelector("#activityLog");
const approvalTemplate = document.querySelector("#approvalTemplate");

const metricContracts = [
  {
    name: "Pending human approvals",
    query: "sum(remediator_pending_approvals)",
    expected: "0 during full auto; >0 when approval mode gates a fix"
  },
  {
    name: "Approval decisions",
    query: "sum by (decision) (increase(remediator_approvals_total[1h]))",
    expected: "pending, approved, and denied decisions are counted"
  },
  {
    name: "Action outcomes",
    query: "sum by (outcome) (increase(remediator_actions_total[1h]))",
    expected: "healed, needs_human, failed, cooldown, and dry-run paths"
  },
  {
    name: "Verification results",
    query: "sum by (result) (increase(remediator_action_verifications_total[1h]))",
    expected: "improved, not_improved, no_baseline, or no_after_data"
  },
  {
    name: "RCA drafts",
    query: "sum by (result) (increase(remediator_rca_drafts_total[1h]))",
    expected: "drafted or skipped sink/copilot outcomes"
  },
  {
    name: "Webhook latency",
    query: "histogram_quantile(0.95, sum by (le) (rate(remediator_webhook_duration_seconds_bucket[5m])))",
    expected: "p95 stays low even when RCA work is queued"
  }
];

function loadSettings() {
  endpointInput.value = localStorage.getItem("omniobserve.endpoint") || endpointInput.value;
  tokenInput.value = localStorage.getItem("omniobserve.token") || "";
  prometheusInput.value = localStorage.getItem("omniobserve.prometheus") || prometheusInput.value;
}

function saveSettings() {
  localStorage.setItem("omniobserve.endpoint", endpointInput.value.trim());
  localStorage.setItem("omniobserve.token", tokenInput.value.trim());
  localStorage.setItem("omniobserve.prometheus", prometheusInput.value.trim());
}

function setControlRoomOpen(open) {
  controlRoomCard.hidden = !open;
  openControlRoomButton.textContent = open ? "Control room open" : "Open control room";
  openControlRoomButton.disabled = open;
  if (open) {
    refreshAll();
    controlRoomCard.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

function remediatorURL(path) {
  return endpointInput.value.trim().replace(/\/+$/, "") + path;
}

function prometheusURL(query) {
  const base = prometheusInput.value.trim().replace(/\/+$/, "");
  return `${base}/api/v1/query?query=${encodeURIComponent(query)}`;
}

async function remediatorFetch(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (!headers.has("Content-Type") && options.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = tokenInput.value.trim();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const response = await fetch(remediatorURL(path), { ...options, headers });
  const text = await response.text();
  let body = {};
  try {
    body = text ? JSON.parse(text) : {};
  } catch (error) {
    body = { error: text || response.statusText };
  }
  if (!response.ok) {
    const message = body.error || `${response.status} ${response.statusText}`;
    throw new Error(message);
  }
  return body;
}

function logActivity(message) {
  const item = document.createElement("li");
  item.textContent = `${new Date().toLocaleTimeString()} - ${message}`;
  activityLog.prepend(item);
}

function renderEmptyApproval(message) {
  approvalList.innerHTML = "";
  const empty = document.createElement("div");
  empty.className = "empty";
  empty.textContent = message;
  approvalList.append(empty);
}

async function loadHealth() {
  try {
    const body = await remediatorFetch("/healthz");
    healthStatus.textContent = `${body.status} (${body.version})`;
    healthStatus.className = "ok";
  } catch (error) {
    healthStatus.textContent = error.message;
    healthStatus.className = "error";
    logActivity(`health check failed: ${error.message}`);
  }
}

function renderApprovals(approvals) {
  approvalList.innerHTML = "";
  pendingCount.textContent = approvals.filter((approval) => approval.status === "pending").length;

  if (approvals.length === 0) {
    renderEmptyApproval("No approval requests are currently reported by the remediator.");
    return;
  }

  for (const approval of approvals) {
    const node = approvalTemplate.content.cloneNode(true);
    const card = node.querySelector(".approval-card");
    const status = node.querySelector(".status");
    node.querySelector(".alertname").textContent = approval.alertname || "Alert";
    node.querySelector(".summary").textContent = approval.summary || `${approval.action} ${approval.flag}`;
    status.textContent = approval.status;
    status.classList.add(approval.status);
    node.querySelector(".service").textContent = approval.service || "unknown";
    node.querySelector(".flag").textContent = approval.flag || "none";
    node.querySelector(".incident").textContent = approval.incident_key || approval.id;
    node.querySelector(".outcome").textContent = approval.outcome || "pending";

    const approve = node.querySelector(".approve");
    const deny = node.querySelector(".deny");
    approve.disabled = !approval.can_approve;
    deny.disabled = !approval.can_approve;
    approve.addEventListener("click", () => decideApproval(approval.id, "approve", [approve, deny]));
    deny.addEventListener("click", () => decideApproval(approval.id, "deny", [approve, deny]));
    card.dataset.approvalId = approval.id;
    approvalList.append(node);
  }
}

async function loadApprovals() {
  try {
    const body = await remediatorFetch("/approvals");
    renderApprovals(body.approvals || []);
    logActivity("approval queue refreshed");
  } catch (error) {
    renderEmptyApproval(`Approval queue unavailable: ${error.message}`);
    pendingCount.textContent = "n/a";
    logActivity(`approval refresh failed: ${error.message}`);
  }
}

async function decideApproval(id, decision, controls = []) {
  const actor = "visual-control-room";
  const note = decision === "approve" ? "Approved from visual control room" : "Denied from visual control room";
  const verb = decision === "approve" ? "approved" : "denied";
  controls.forEach((control) => {
    control.disabled = true;
  });
  try {
    const body = await remediatorFetch(`/approvals/${id}/${decision}`, {
      method: "POST",
      body: JSON.stringify({ actor, note })
    });
    const approval = body.approval || {};
    logActivity(`${verb} ${approval.flag || id}: ${approval.outcome || approval.status}`);
    await refreshAll();
  } catch (error) {
    logActivity(`${decision} failed for ${id}: ${error.message}`);
    controls.forEach((control) => {
      control.disabled = false;
    });
  }
}

async function queryPrometheus(query) {
  const response = await fetch(prometheusURL(query));
  let body;
  try {
    body = await response.json();
  } catch (error) {
    throw new Error(response.statusText || "Prometheus returned non-JSON response");
  }
  if (!response.ok || body.status !== "success") {
    throw new Error(body.error || `${response.status} ${response.statusText}`);
  }
  const result = body.data.result || [];
  if (result.length === 0) {
    return "no data";
  }
  return result
    .slice(0, 3)
    .map((series) => series.value?.[1] ?? "n/a")
    .join(", ");
}

async function renderMetrics() {
  metricList.innerHTML = "";
  for (const contract of metricContracts) {
    const card = document.createElement("article");
    card.className = "metric-card";
    const value = document.createElement("span");
    value.className = "metric-value";
    value.textContent = "checking";
    card.innerHTML = `
      <div class="metric-topline">
        <strong>${contract.name}</strong>
      </div>
      <code>${contract.query}</code>
      <p class="eyebrow">${contract.expected}</p>
    `;
    card.querySelector(".metric-topline").append(value);
    metricList.append(card);

    try {
      value.textContent = await queryPrometheus(contract.query);
      value.classList.remove("error");
    } catch (error) {
      value.textContent = "unavailable";
      value.classList.add("error");
    }
  }
}

async function refreshAll() {
  saveSettings();
  await Promise.all([loadHealth(), loadApprovals(), renderMetrics()]);
  lastRefresh.textContent = new Date().toLocaleTimeString();
}

refreshButton.addEventListener("click", refreshAll);
quickRefreshButton.addEventListener("click", refreshAll);
openControlRoomButton.addEventListener("click", () => setControlRoomOpen(true));
closeControlRoomButton.addEventListener("click", () => setControlRoomOpen(false));
reloadApprovalsButton.addEventListener("click", loadApprovals);
clearLogButton.addEventListener("click", () => {
  activityLog.innerHTML = "";
});

loadSettings();
refreshAll();
