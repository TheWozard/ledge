const app = document.getElementById("app");

// Everything the page tells the app: an action, and the id it applies to.
function send(action, id) {
  window.webkit.messageHandlers.ledge.postMessage(id == null ? { action } : { action, id });
}

// update shows html, rendered by Go from BarContent, with its text escaped. It
// morphs #app into it, matching cards by their ids, so what's still there keeps its
// element: the row keeps its scroll, and a hovered card its hover. A card that's gone
// leaves rather than vanishing.
function update(html) {
  Idiomorph.morph(app, html, {
    morphStyle: "innerHTML",
    callbacks: {
      beforeNodeRemoved(node) {
        if (!node.matches?.("#cards > li")) return true;
        if (!node.hasAttribute("data-leaving")) leave(node);
        return false;
      },
    },
  });
  tick();
}

// leave marks node data-leaving, with the room it takes, and removes it once its
// theme's animations end; or after a few seconds regardless, as a hidden window's
// may not run. If it comes back first, a render morphs it back into place, taking
// data-leaving off, and it stays.
function leave(node) {
  node.removeAttribute("data-new");
  const gap = parseFloat(getComputedStyle(node.parentElement).columnGap) || 0;
  node.style.setProperty("--collapse-x", -(node.offsetWidth + gap) + "px");
  node.setAttribute("data-leaving", "");
  const leaving = node.leaving = {};
  const gone = () => {
    if (node.leaving === leaving && node.hasAttribute("data-leaving")) node.remove();
  };
  const ending = node.getAnimations().filter(a => a.effect?.getComputedTiming().endTime !== Infinity);
  Promise.all(ending.map(a => a.finished)).then(gone, gone);
  setTimeout(gone, 3000);
}

// A click on a card's ⋯ asks for its menu. A click on anything else with
// data-click posts its id: the innermost, so a click on the × or a part of the
// card's body, such as a check, isn't also its card's. One on what's busy, its
// Click still running from the last, posts nothing (and the app would drop it).
app.addEventListener("click", event => {
  const menu = event.target.closest("button[data-menu]");
  if (menu) {
    send("menu", menu.dataset.menu);
    return;
  }
  const target = event.target.closest("[data-click]");
  if (target && !target.hasAttribute("data-busy")) send("click", target.dataset.click);
});

// setBusy marks data-busy what has each of ids, and takes it off everything else:
// something clicked, by its data-click, while its Click runs, and a card's ⋯, by
// "menu-" and its data-menu, while one of the card's actions runs. The app calls it
// whenever that changes, and with every update, as a morph takes data-busy off.
function setBusy(ids) {
  const busy = new Set(ids);
  for (const node of app.querySelectorAll("[data-click], button[data-menu]")) {
    const id = "click" in node.dataset ? node.dataset.click : "menu-" + node.dataset.menu;
    node.toggleAttribute("data-busy", busy.has(id));
  }
}

// A right-click on a card with actions asks for its menu, which the app shows.
// WebKit's own menu never shows, anywhere on the page.
document.addEventListener("contextmenu", event => {
  event.preventDefault();
  const card = event.target.closest?.("[data-menu]");
  if (card) send("menu", card.dataset.menu);
});

// Whether node, or something it's in, scrolls vertically and has more to show, as
// a theme can make a card's description do.
function scrollsVertically(node) {
  for (; node && node !== app; node = node.parentElement) {
    if (node.scrollHeight > node.clientHeight && /auto|scroll/.test(getComputedStyle(node).overflowY)) return true;
  }
  return false;
}

// A mouse wheel only scrolls vertically; turn that into sideways scrolling of the
// row. Over anything that scrolls vertically, it's left alone.
app.addEventListener("wheel", event => {
  const row = app.querySelector("ul");
  if (!row || scrollsVertically(event.target)
      || Math.abs(event.deltaX) > Math.abs(event.deltaY)) return;
  row.scrollLeft += event.deltaY;
  event.preventDefault();
}, { passive: false });

// clock is seconds as a timer shows them, as Go's clock does: "4:05", or "1:02:03"
// from an hour.
function clock(seconds) {
  const pad = n => String(n).padStart(2, "0");
  return seconds >= 3600
    ? `${Math.floor(seconds / 3600)}:${pad(Math.floor(seconds / 60) % 60)}:${pad(seconds % 60)}`
    : `${Math.floor(seconds / 60)}:${pad(seconds % 60)}`;
}

// tick keeps the time of every timer on the bar, an element with data-start or
// data-end, in milliseconds, as Go's timeLeft and timeSince do. With data-end, it
// shows the time left, rounded up to the second, and sets data-done on its card once
// it's up; with data-start too, it sets --progress on its card, from 0 at its start
// to 1 at its end. With only data-start, it shows the time since, rounded down. It
// runs a few times a second, and after every update, as a render puts back the time
// as of then.
function tick() {
  const now = Date.now();
  for (const timer of app.querySelectorAll("[data-start], [data-end]")) {
    if (!("end" in timer.dataset)) {
      timer.textContent = clock(Math.floor(Math.max(0, now - +timer.dataset.start) / 1000));
      continue;
    }
    const end = +timer.dataset.end;
    const seconds = Math.ceil(Math.max(0, end - now) / 1000);
    timer.textContent = clock(seconds);
    const card = timer.closest("li");
    if (!card) continue;
    if ("start" in timer.dataset) {
      const start = +timer.dataset.start;
      card.style.setProperty("--progress", end > start ? Math.min(1, Math.max(0, (now - start) / (end - start))) : 1);
    }
    card.toggleAttribute("data-done", seconds === 0);
  }
}
setInterval(tick, 250);

send("ready");
