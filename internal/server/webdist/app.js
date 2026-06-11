/* unlapse frontend — no framework, no build step, no tracking. */
"use strict";

const CATEGORIES = ["subscription", "warranty", "insurance", "document", "domain", "membership", "other"];
const CAT_COLOR = {
  subscription: "var(--cat-subscription)",
  warranty: "var(--cat-warranty)",
  insurance: "var(--cat-insurance)",
  document: "var(--cat-document)",
  domain: "var(--cat-domain)",
  membership: "var(--cat-membership)",
  other: "var(--cat-other)",
};
const STATUS_LABEL = { overdue: "overdue", "due-soon": "due soon", upcoming: "upcoming", ok: "ok" };

const state = {
  items: [],        // active items (computed views from API)
  allItems: [],     // including archived
  stats: null,
  settings: { currency: "$", theme: "dark", defaultRemindDays: 14 },
  filter: { search: "", category: "all", archived: false },
  editingId: null,
};

const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

/* ---------- api ---------- */

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  if (res.status === 204) return null;
  const isJSON = (res.headers.get("content-type") || "").includes("json");
  const body = isJSON ? await res.json() : await res.text();
  if (!res.ok) throw new Error(isJSON && body.error ? body.error : `request failed (${res.status})`);
  return body;
}

async function refresh() {
  [state.allItems, state.stats, state.settings] = await Promise.all([
    api("api/items?includeArchived=1"),
    api("api/stats"),
    api("api/settings"),
  ]);
  state.items = state.allItems.filter((it) => !it.archived);
  document.body.dataset.theme = state.settings.theme;
  render();
}

/* ---------- helpers ---------- */

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function money(n) {
  const cur = state.settings.currency || "$";
  return cur + Number(n).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtDate(iso) {
  const d = new Date(iso + "T00:00:00");
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

function daysLabel(it) {
  if (it.daysLeft < 0) return `${-it.daysLeft}d overdue`;
  if (it.daysLeft === 0) return "today";
  if (it.daysLeft === 1) return "tomorrow";
  return `in ${it.daysLeft}d`;
}

function cycleLabel(it) {
  if (it.cycle === "none") return "one-time";
  if (it.cycle === "custom") return `every ${it.cycleDays}d`;
  return it.cycle;
}

function costLabel(it) {
  if (!it.cost) return "free";
  if (it.cycle === "none") return money(it.cost);
  return `${money(it.cost)} / ${cycleLabel(it).replace("ly", "").replace("every ", "")}`;
}

let toastTimer;
function toast(msg, isErr = false) {
  const t = $("#toast");
  t.textContent = msg;
  t.classList.toggle("err", isErr);
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.hidden = true), 2600);
}

/* ---------- rendering ---------- */

function render() {
  const view = location.hash.replace("#/", "") || "dashboard";
  const isEmpty = state.allItems.length === 0;

  $("#view-dashboard").hidden = isEmpty || view !== "dashboard";
  $("#view-items").hidden = isEmpty || view !== "items";
  $("#view-empty").hidden = !isEmpty;
  $$(".nav-link[data-view]").forEach((a) => a.classList.toggle("active", a.dataset.view === view));

  if (isEmpty) return;
  if (view === "dashboard") renderDashboard();
  else renderItems();
}

function itemRow(it, { showStatus = true, showActions = false } = {}) {
  const sub = [esc(it.category), cycleLabel(it)];
  if (it.notes) sub.push(esc(it.notes));
  const actions = showActions
    ? `<div class="item-actions">
         <button class="icon-btn" data-act="archive" data-id="${it.id}" title="${it.archived ? "Unarchive" : "Archive"}">
           <svg viewBox="0 0 24 24"><path d="M20.54 5.23 19.15 3.55A1.5 1.5 0 0 0 18 3H6c-.46 0-.87.21-1.15.55L3.46 5.23A2 2 0 0 0 3 6.5V19a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6.5c0-.48-.17-.93-.46-1.27zM12 17.5 6.5 12H10v-2h4v2h3.5L12 17.5zM5.12 5l.81-1h12l.94 1H5.12z"/></svg>
         </button>
         <button class="icon-btn danger" data-act="delete" data-id="${it.id}" title="Delete">
           <svg viewBox="0 0 24 24"><path d="M6 19a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg>
         </button>
       </div>`
    : "";
  const pill = it.archived
    ? `<span class="pill archived">archived</span>`
    : showStatus
      ? `<span class="pill ${it.status}">${STATUS_LABEL[it.status]}</span>`
      : "";
  return `<div class="item-row" data-id="${it.id}">
    <span class="cat-dot" style="background:${CAT_COLOR[it.category]}"></span>
    <div class="item-main">
      <div class="item-name">${esc(it.name)}</div>
      <div class="item-meta">${sub.join(" · ")}</div>
    </div>
    ${pill}
    <div class="item-right">
      <div class="item-date">${fmtDate(it.nextDate)} <span style="color:var(--text-dim)">· ${daysLabel(it)}</span></div>
      <div class="item-cost">${costLabel(it)}</div>
    </div>
    ${actions}
  </div>`;
}

function renderDashboard() {
  const s = state.stats;
  $("#dash-subtitle").textContent = new Date().toLocaleDateString(undefined, {
    weekday: "long", month: "long", day: "numeric", year: "numeric",
  });

  const attention = s.overdue + s.dueSoon;
  $("#stat-grid").innerHTML = `
    <div class="stat-card"><div class="stat-label">Monthly spend</div>
      <div class="stat-value">${money(s.monthlyCost)}<small> /mo</small></div>
      <div class="stat-sub">across recurring items</div></div>
    <div class="stat-card"><div class="stat-label">Yearly spend</div>
      <div class="stat-value">${money(s.yearlyCost)}<small> /yr</small></div>
      <div class="stat-sub">what auto-renewing really costs</div></div>
    <div class="stat-card ${attention ? (s.overdue ? "alert" : "warn") : ""}">
      <div class="stat-label">Needs attention</div>
      <div class="stat-value">${attention}</div>
      <div class="stat-sub">${s.overdue} overdue · ${s.dueSoon} due soon</div></div>
    <div class="stat-card"><div class="stat-label">Tracked</div>
      <div class="stat-value">${s.activeItems}</div>
      <div class="stat-sub">${s.next30Days} due in the next 30 days</div></div>`;

  const urgent = state.items.filter((it) => it.status === "overdue" || it.status === "due-soon");
  $("#attention-list").innerHTML = urgent.length
    ? urgent.map((it) => itemRow(it)).join("")
    : `<div class="list-empty">All clear — nothing needs attention right now. 🎉</div>`;

  const upcoming = state.items.filter((it) => it.status !== "overdue" && it.status !== "due-soon" && it.daysLeft <= 90);
  const buckets = [
    ["This week", (d) => d <= 7],
    ["This month", (d) => d > 7 && d <= 30],
    ["Next 90 days", (d) => d > 30],
  ];
  let html = "";
  for (const [label, match] of buckets) {
    const group = upcoming.filter((it) => match(it.daysLeft));
    if (!group.length) continue;
    html += `<div class="group-label">${label}</div>` + group.map((it) => itemRow(it, { showStatus: false })).join("");
  }
  $("#upcoming-list").innerHTML = html || `<div class="list-empty">Nothing due in the next 90 days.</div>`;

  const costs = Object.entries(s.costByCategory).sort((a, b) => b[1] - a[1]);
  const maxCost = costs.length ? costs[0][1] : 1;
  $("#category-bars").innerHTML = costs.length
    ? costs.map(([cat, amt]) => `
      <div class="bar-row">
        <div class="bar-top"><span>${esc(cat)}</span><span class="amt">${money(amt)}/mo</span></div>
        <div class="bar-track"><div class="bar-fill" style="width:${Math.max(4, (amt / maxCost) * 100)}%;background:${CAT_COLOR[cat]}"></div></div>
      </div>`).join("")
    : `<div class="list-empty">No recurring costs yet.</div>`;

  const counts = Object.entries(s.byCategory).sort((a, b) => b[1] - a[1]);
  $("#category-counts").innerHTML = counts.map(([cat, n]) => `
    <div class="count-row">
      <span class="cat-dot" style="background:${CAT_COLOR[cat]}"></span>
      ${esc(cat)} <span class="n">${n}</span>
    </div>`).join("");
}

function renderItems() {
  const f = state.filter;
  let list = f.archived ? state.allItems : state.items;
  if (f.category !== "all") list = list.filter((it) => it.category === f.category);
  if (f.search) {
    const q = f.search.toLowerCase();
    list = list.filter((it) => (it.name + " " + (it.notes || "") + " " + it.category).toLowerCase().includes(q));
  }

  $("#items-subtitle").textContent = `${list.length} item${list.length === 1 ? "" : "s"}` + (f.archived ? " (including archived)" : "");

  $("#category-chips").innerHTML = ["all", ...CATEGORIES]
    .map((c) => `<button class="chip ${f.category === c ? "active" : ""}" data-cat="${c}">${c}</button>`)
    .join("");

  $("#items-list").innerHTML = list.length
    ? `<div class="items-card">${list.map((it) => itemRow(it, { showActions: true })).join("")}</div>`
    : `<div class="list-empty">No items match.</div>`;
}

/* ---------- item modal ---------- */

function openItemModal(item = null) {
  state.editingId = item ? item.id : null;
  const form = $("#item-form");
  form.reset();
  $("#item-modal-title").textContent = item ? "Edit item" : "Add item";
  $("#f-category").innerHTML = CATEGORIES.map((c) => `<option value="${c}">${c}</option>`).join("");
  $("#f-currency").textContent = `(${state.settings.currency})`;

  if (item) {
    form.name.value = item.name;
    form.category.value = item.category;
    form.date.value = item.date;
    form.cycle.value = item.cycle;
    form.cycleDays.value = item.cycleDays || 30;
    form.cost.value = item.cost;
    form.remindDays.value = item.remindDays;
    form.url.value = item.url || "";
    form.notes.value = item.notes || "";
  } else {
    form.date.value = new Date().toISOString().slice(0, 10);
    form.remindDays.value = state.settings.defaultRemindDays;
  }
  $("#f-cycledays-wrap").hidden = form.cycle.value !== "custom";
  $("#item-modal").showModal();
}

async function saveItem(form) {
  const body = {
    name: form.name.value,
    category: form.category.value,
    date: form.date.value,
    cycle: form.cycle.value,
    cycleDays: form.cycle.value === "custom" ? Number(form.cycleDays.value) : 0,
    cost: Number(form.cost.value) || 0,
    remindDays: Number(form.remindDays.value) || 0,
    url: form.url.value.trim(),
    notes: form.notes.value.trim(),
    archived: state.editingId ? state.allItems.find((i) => i.id === state.editingId)?.archived ?? false : false,
  };
  const opts = { method: state.editingId ? "PUT" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
  await api(state.editingId ? `api/items/${state.editingId}` : "api/items", opts);
  toast(state.editingId ? "Saved" : "Added");
  await refresh();
}

/* ---------- events ---------- */

window.addEventListener("hashchange", render);

document.addEventListener("click", async (e) => {
  const t = e.target.closest("button, .item-row, a");
  if (!t) return;

  if (t.dataset.action === "add") return openItemModal();
  if (t.id === "btn-settings") return openSettings();
  if (t.id === "btn-calendar") return openCalendar();
  if (t.id === "btn-sample") {
    try {
      await api("api/sample", { method: "POST" });
      toast("Sample data loaded — explore, then delete what you don't need");
      await refresh();
    } catch (err) { toast(err.message, true); }
    return;
  }
  if (t.dataset.close !== undefined) return t.closest("dialog").close();
  if (t.dataset.cat) {
    state.filter.category = t.dataset.cat;
    return renderItems();
  }

  const act = t.dataset.act;
  if (act) {
    e.stopPropagation();
    const id = t.dataset.id;
    const item = state.allItems.find((i) => i.id === id);
    if (!item) return;
    try {
      if (act === "delete") {
        if (!confirm(`Delete “${item.name}”? This cannot be undone.`)) return;
        await api(`api/items/${id}`, { method: "DELETE" });
        toast("Deleted");
      } else if (act === "archive") {
        const upd = { ...item, archived: !item.archived };
        delete upd.nextDate; delete upd.daysLeft; delete upd.status; delete upd.monthlyCost; delete upd.yearlyCost;
        delete upd.id; delete upd.createdAt; delete upd.updatedAt;
        await api(`api/items/${id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(upd) });
        toast(item.archived ? "Unarchived" : "Archived");
      }
      await refresh();
    } catch (err) { toast(err.message, true); }
    return;
  }

  const row = e.target.closest(".item-row");
  if (row && !e.target.closest(".item-actions")) {
    const item = state.allItems.find((i) => i.id === row.dataset.id);
    if (item) openItemModal(item);
  }
});

$("#item-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  try {
    await saveItem(e.target);
    $("#item-modal").close();
  } catch (err) { toast(err.message, true); }
});

$("#f-cycle").addEventListener("change", (e) => {
  $("#f-cycledays-wrap").hidden = e.target.value !== "custom";
});

$("#search").addEventListener("input", (e) => {
  state.filter.search = e.target.value;
  renderItems();
});

$("#show-archived").addEventListener("change", (e) => {
  state.filter.archived = e.target.checked;
  renderItems();
});

/* ---------- settings ---------- */

function openSettings() {
  const form = $("#settings-form");
  form.currency.value = state.settings.currency;
  form.defaultRemindDays.value = state.settings.defaultRemindDays;
  form.theme.value = state.settings.theme;
  $("#settings-modal").showModal();
}

$("#settings-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  try {
    await api("api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        currency: form.currency.value,
        defaultRemindDays: Number(form.defaultRemindDays.value),
        theme: form.theme.value,
      }),
    });
    $("#settings-modal").close();
    toast("Settings saved");
    await refresh();
  } catch (err) { toast(err.message, true); }
});

$("#btn-import").addEventListener("click", () => $("#import-file").click());

$("#import-file").addEventListener("change", async (e) => {
  const file = e.target.files[0];
  if (!file) return;
  try {
    const res = await api("api/import", { method: "POST", body: await file.text() });
    toast(`Imported ${res.added} item${res.added === 1 ? "" : "s"}`);
    $("#settings-modal").close();
    await refresh();
  } catch (err) { toast(err.message, true); }
  e.target.value = "";
});

/* ---------- calendar ---------- */

function openCalendar() {
  $("#ics-url").value = new URL("calendar.ics", location.href).href;
  $("#calendar-modal").showModal();
}

$("#btn-copy-ics").addEventListener("click", async () => {
  const url = $("#ics-url").value;
  try {
    await navigator.clipboard.writeText(url);
    toast("Copied");
  } catch {
    $("#ics-url").select();
    document.execCommand("copy");
    toast("Copied");
  }
});

/* ---------- boot ---------- */

refresh().catch((err) => toast("Could not reach the unlapse server: " + err.message, true));
