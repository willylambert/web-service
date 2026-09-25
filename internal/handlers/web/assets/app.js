const rowsEl = document.getElementById("rows");
const statusEl = document.getElementById("status");
const updatedEl = document.getElementById("updated");
const clockEl = document.getElementById("clock");
const refreshBtn = document.getElementById("refresh");
const installBtn = document.getElementById("install");
const tabs = [...document.querySelectorAll(".tab")];

function defaultDirection(date = new Date()) {
  return date.getHours() >= 12 ? "arrivals" : "departures";
}

let direction = defaultDirection();
let timer;
let deferredInstallPrompt = null;

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

function renderTrains(trains) {
  if (!trains.length) {
    rowsEl.innerHTML =
      '<li class="train-list__empty">Aucun train annoncé pour le moment.</li>';
    return;
  }

  rowsEl.innerHTML = trains
    .map((train) => {
      const delay = Number(train.delay_minutes) || 0;
      const late = delay > 0;
      const statusText = late ? delayLabel(delay) : train.status || "à l'heure";
      const showBase =
        late && train.base_time && train.base_time !== train.time;
      const isArrival = direction === "arrivals";
      const angersTime = train.angers_time || train.via_stop_time || "";
      const saintMathurinTime = train.saint_mathurin_time || "";

      if (isArrival) {
        const menitreTime = train.time || "--:--";
        return `
          <li class="train train--arrival${late ? " is-late" : ""}">
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
            <div class="train__status${late ? " is-late" : ""}">
              <span class="train__delay">${escapeHtml(statusText)}</span>
            </div>
          </li>
        `;
      }

      return `
        <li class="train train--departure${late ? " is-late" : ""}">
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
          <div class="train__status${late ? " is-late" : ""}">
            <span class="train__delay">${escapeHtml(statusText)}</span>
          </div>
        </li>
      `;
    })
    .join("");
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
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
