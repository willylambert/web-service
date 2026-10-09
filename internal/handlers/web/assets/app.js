const rowsEl = document.getElementById("rows");
const statusEl = document.getElementById("status");
const updatedEl = document.getElementById("updated");
const clockEl = document.getElementById("clock");
const refreshBtn = document.getElementById("refresh");
const installBtn = document.getElementById("install");
const detailDialog = document.getElementById("train-detail");
const detailEyebrow = document.getElementById("detail-eyebrow");
const detailTitle = document.getElementById("detail-title");
const detailStops = document.getElementById("detail-stops");
const tabs = [...document.querySelectorAll(".tab")];

function defaultDirection(date = new Date()) {
  return date.getHours() >= 12 ? "arrivals" : "departures";
}

let direction = defaultDirection();
let timer;
let deferredInstallPrompt = null;
let latestTrains = [];
let journeyRequest = 0;

function formatClock(date = new Date()) {
  return date.toLocaleTimeString("fr-FR", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function tickClock() {
  clockEl.textContent = formatClock();
}

function setStatus(message) {
  if (!message) {
    statusEl.hidden = true;
    statusEl.textContent = "";
    return;
  }
  statusEl.hidden = false;
  statusEl.textContent = message;
}

function delayLabel(minutes) {
  if (!minutes || minutes <= 0) return "à l'heure";
  return `+${minutes} min`;
}

function statusLabel(train) {
  const delay = Number(train.delay_minutes) || 0;
  if (train.status === "supprimé") return "supprimé";
  if (delay > 0) return delayLabel(delay);
  return train.status || "à l'heure";
}

function renderTrains(trains) {
  latestTrains = trains;
  if (!trains.length) {
    rowsEl.innerHTML =
      '<li class="train-list__empty">Aucun train annoncé pour le moment.</li>';
    return;
  }

  rowsEl.innerHTML = trains
    .map((train, index) => {
      const delay = Number(train.delay_minutes) || 0;
      const cancelled = train.status === "supprimé";
      const late = !cancelled && delay > 0;
      const enRoute = train.status === "en route";
      const statusText = statusLabel(train);
      const statusTitle = cancelled
        ? train.status_detail || "Train supprimé"
        : "Voir le détail";
      const showBase =
        late && train.base_time && train.base_time !== train.time;
      const isArrival = direction === "arrivals";
      const angersTime = train.angers_time || train.via_stop_time || "";
      const saintMathurinTime = train.saint_mathurin_time || "";
      const rowClass = [
        isArrival ? "train train--arrival" : "train train--departure",
        late ? "is-late" : "",
        enRoute ? "is-en-route" : "",
        cancelled ? "is-cancelled" : "",
      ]
        .filter(Boolean)
        .join(" ");
      const statusClass = [
        "train__status",
        late ? "is-late" : "",
        enRoute ? "is-en-route" : "",
        cancelled ? "is-cancelled" : "",
      ]
        .filter(Boolean)
        .join(" ");

      if (isArrival) {
        const menitreTime = train.time || "--:--";
        return `
          <li class="${rowClass}" role="button" tabindex="0" data-train-index="${index}" aria-label="Détail du train ${escapeHtml(train.trip_name || "")}">
            <div class="train__time-block">
              <span class="train__time">${escapeHtml(angersTime || "--:--")}</span>
              <span class="train__time-label">Angers</span>
              ${
                showBase
                  ? `<span class="train__base" title="Horaire prévu La Ménitré">${escapeHtml(train.base_time)}</span>`
                  : ""
              }
            </div>
            <div class="train__body">
              ${
                saintMathurinTime
                  ? `<div class="train__secondary"><strong>${escapeHtml(saintMathurinTime)}</strong><span>St Math</span></div>`
                  : ""
              }
              <div class="train__secondary"><strong>${escapeHtml(menitreTime)}</strong><span>La Ménitré</span></div>
            </div>
            <div class="${statusClass}" title="${escapeHtml(statusTitle)}">
              <span class="train__delay">${escapeHtml(statusText)}</span>
            </div>
          </li>
        `;
      }

      return `
        <li class="${rowClass}" role="button" tabindex="0" data-train-index="${index}" aria-label="Détail du train ${escapeHtml(train.trip_name || "")}">
          <div class="train__time-block">
            <span class="train__time">${escapeHtml(train.time || "--:--")}</span>
            <span class="train__time-label">La Ménitré</span>
            ${
              showBase
                ? `<span class="train__base" title="Horaire prévu">${escapeHtml(train.base_time)}</span>`
                : ""
            }
          </div>
          <div class="train__body">
            ${
              saintMathurinTime
                ? `<div class="train__secondary"><strong>${escapeHtml(saintMathurinTime)}</strong><span>St Math</span></div>`
                : ""
            }
            ${
              angersTime
                ? `<div class="train__secondary"><strong>${escapeHtml(angersTime)}</strong><span>Angers</span></div>`
                : ""
            }
          </div>
          <div class="${statusClass}" title="${escapeHtml(statusTitle)}">
            <span class="train__delay">${escapeHtml(statusText)}</span>
          </div>
        </li>
      `;
    })
    .join("");
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function boardingStopPattern() {
  // Point where the user boards for the current board direction.
  return direction === "arrivals"
    ? /angers/i
    : /ménitré|menitre/i;
}

function stopMatches(name, pattern) {
  return pattern.test(name || "");
}

/** Keep origin → boarding stop only (no stops after departure point). */
function stopsUntilBoarding(stops) {
  const boardAt = boardingStopPattern();
  const cut = [];
  for (const stop of stops) {
    cut.push(stop);
    if (stopMatches(stop.name, boardAt)) break;
  }
  return cut;
}

function stopDate(stop) {
  const raw = stop.departure_datetime || stop.arrival_datetime;
  if (!raw) return null;
  // Local wall-clock from API (no Z) — parse as local time.
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? null : d;
}

/**
 * Index of the stop where the train currently is (last stop whose datetime has passed).
 * -1 = not departed yet from the first stop.
 */
function currentStopIndex(stops, now = new Date()) {
  const nowMs = now.getTime();
  let current = -1;
  for (let i = 0; i < stops.length; i += 1) {
    const when = stopDate(stops[i]);
    if (!when) continue;
    if (when.getTime() <= nowMs) current = i;
    else break;
  }
  return current;
}

function stopDisplayTime(stop) {
  const real = stop.departure_time || stop.arrival_time || "--:--";
  const base = stop.base_departure_time || stop.base_arrival_time || "";
  if (base && base !== real) {
    return `<span class="train-detail__stop-time is-realtime">${escapeHtml(real)}</span><span class="train-detail__stop-base">${escapeHtml(base)}</span>`;
  }
  return `<span class="train-detail__stop-time">${escapeHtml(real)}</span>`;
}

function openTrainDetail(train) {
  if (!train || !detailDialog) return;

  const number = train.trip_name || train.headsign || "Train";
  detailEyebrow.textContent = "Approche";
  detailTitle.textContent = number;
  detailStops.innerHTML =
    '<li class="train-detail__loading">Chargement des arrêts…</li>';
  detailDialog.showModal();
  loadJourneyStops(train);
}

async function loadJourneyStops(train) {
  const requestId = ++journeyRequest;
  const vj = train.vehicle_journey_id;
  if (!vj) {
    detailStops.innerHTML =
      '<li class="train-detail__empty">Parcours détaillé indisponible.</li>';
    return;
  }

  try {
    const res = await fetch(
      `/api/v1/trains/journey?vehicle_journey_id=${encodeURIComponent(vj)}`
    );
    const data = await res.json();
    if (requestId !== journeyRequest) return;
    if (!res.ok) {
      throw new Error(data.error || `Erreur ${res.status}`);
    }
    const allStops = data.stops || [];
    if (!allStops.length) {
      detailStops.innerHTML =
        '<li class="train-detail__empty">Aucun arrêt renvoyé par l’API.</li>';
      return;
    }

    const stops = stopsUntilBoarding(allStops);
    const currentIdx = currentStopIndex(stops);
    const boardAt = boardingStopPattern();
    const firstAt = stopDate(stops[0]);
    const notStarted = currentIdx < 0;

    if (notStarted) {
      detailEyebrow.textContent = firstAt
        ? `Pas encore parti · ${firstAt.toLocaleDateString("fr-FR", { weekday: "short", day: "numeric", month: "short" })}`
        : "Pas encore parti";
    } else if (stopMatches(stops[currentIdx]?.name, boardAt)) {
      detailEyebrow.textContent = "À quai / parti";
    } else {
      detailEyebrow.textContent = `Position : ${stops[currentIdx].name}`;
    }

    detailStops.innerHTML = stops
      .map((stop, index) => {
        const isBoard = stopMatches(stop.name, boardAt);
        const isCurrent = !notStarted && index === currentIdx;
        const isPassed = currentIdx >= 0 && index < currentIdx;
        const classes = [
          "train-detail__stop",
          isBoard ? "is-board" : "",
          isCurrent ? "is-current" : "",
          isPassed ? "is-passed" : "",
        ]
          .filter(Boolean)
          .join(" ");
        const marker = isCurrent
          ? `<span class="train-detail__train" title="Position actuelle">Train</span>`
          : "";
        return `
          <li class="${classes}">
            <span class="train-detail__time-block">${stopDisplayTime(stop)}</span>
            <span class="train-detail__stop-dot" aria-hidden="true"></span>
            <span class="train-detail__stop-name">
              ${escapeHtml(stop.name)}
              ${marker}
            </span>
          </li>
        `;
      })
      .join("");
  } catch (err) {
    if (requestId !== journeyRequest) return;
    detailStops.innerHTML = `<li class="train-detail__empty">${escapeHtml(
      err.message || "Impossible de charger le parcours."
    )}</li>`;
  }
}

async function loadBoard() {
  setStatus("");
  try {
    const res = await fetch(`/api/v1/trains?direction=${direction}&count=20`);
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || `Erreur ${res.status}`);
    }
    renderTrains(data.trains || []);
    const stamp = data.updated_at
      ? new Date(data.updated_at).toLocaleTimeString("fr-FR", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })
      : formatClock();
    updatedEl.textContent = `Mis à jour ${stamp}`;
  } catch (err) {
    latestTrains = [];
    rowsEl.innerHTML =
      '<li class="train-list__empty">Impossible de charger les horaires.</li>';
    setStatus(err.message || "Erreur inconnue");
  }
}

function selectTab(next) {
  direction = next;
  tabs.forEach((tab) => {
    const active = tab.dataset.direction === direction;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", active ? "true" : "false");
  });
  loadBoard();
}

rowsEl.addEventListener("click", (event) => {
  const row = event.target.closest("[data-train-index]");
  if (!row) return;
  const index = Number(row.dataset.trainIndex);
  openTrainDetail(latestTrains[index]);
});

rowsEl.addEventListener("keydown", (event) => {
  if (event.key !== "Enter" && event.key !== " ") return;
  const row = event.target.closest("[data-train-index]");
  if (!row) return;
  event.preventDefault();
  const index = Number(row.dataset.trainIndex);
  openTrainDetail(latestTrains[index]);
});

tabs.forEach((tab) => {
  tab.addEventListener("click", () => selectTab(tab.dataset.direction));
});

refreshBtn.addEventListener("click", () => loadBoard());

window.addEventListener("beforeinstallprompt", (event) => {
  event.preventDefault();
  deferredInstallPrompt = event;
  if (installBtn) installBtn.hidden = false;
});

window.addEventListener("appinstalled", () => {
  deferredInstallPrompt = null;
  if (installBtn) installBtn.hidden = true;
});

if (installBtn) {
  installBtn.addEventListener("click", async () => {
    if (!deferredInstallPrompt) return;
    deferredInstallPrompt.prompt();
    await deferredInstallPrompt.userChoice;
    deferredInstallPrompt = null;
    installBtn.hidden = true;
  });
}

tickClock();
setInterval(tickClock, 1000);
selectTab(direction);
timer = setInterval(loadBoard, 60_000);

window.addEventListener("beforeunload", () => clearInterval(timer));

if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {
      /* ignore offline registration errors */
    });
  });
}
