const app = document.getElementById("app");

// Everything the page tells the app: an action, and the id it applies to.
function send(action, id) {
  window.webkit.messageHandlers.ledge.postMessage(id == null ? { action } : { action, id });
}

// update shows html, rendered by Go from BarContent, with its text escaped. It
// morphs #app into it, matching cards and actions by their ids, so what's
// still there keeps its element: the row keeps its scroll, a hovered card its hover,
// and a clicked action its flash: "clicked" is the page's own class, not in
// html, so it's carried over rather than morphed away. A card or action that's
// gone leaves rather than vanishing.
function update(html) {
  Idiomorph.morph(app, html, {
    morphStyle: "innerHTML",
    callbacks: {
      beforeNodeMorphed(from, to) {
        if (from.classList?.contains("clicked")) to.classList.add("clicked");
      },
      beforeNodeRemoved(node) {
        if (!node.matches?.("#cards > li, #cards .actions > .action")) return true;
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
  node.classList.remove("clicked");
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

// A click on anything with data-click posts its id: the innermost, so an
// action's click isn't also its card's. A clicked action has the class
// clicked for a theme to flash it, restarting if it's clicked again quickly.
app.addEventListener("click", event => {
  const target = event.target.closest("[data-click]");
  if (!target) return;
  if (target.classList.contains("action")) {
    target.classList.remove("clicked");
    void target.offsetWidth;
    target.classList.add("clicked");
  }
  send("click", target.dataset.click);
});

// Whether node, or something it's in, scrolls vertically and has more to show, as
// a theme can make a card's description do.
function scrollsVertically(node) {
  for (; node && node !== app; node = node.parentElement) {
    if (node.scrollHeight > node.clientHeight && /auto|scroll/.test(getComputedStyle(node).overflowY)) return true;
  }
  return false;
}

// A mouse wheel only scrolls vertically; turn that into sideways scrolling of the row,
// or of a card's actions when they don't fit. Over anything that scrolls
// vertically, it's left alone.
app.addEventListener("wheel", event => {
  const actions = event.target.closest?.(".actions");
  const row = actions && actions.scrollWidth > actions.clientWidth ? actions : app.querySelector("ul");
  if (!row || scrollsVertically(event.target)
      || Math.abs(event.deltaX) > Math.abs(event.deltaY)) return;
  row.scrollLeft += event.deltaY;
  event.preventDefault();
}, { passive: false });

// tick counts down every timer on the bar: an element with data-start and data-end,
// in milliseconds, shows the time left, rounded up to the second, as Go's timeLeft
// does, and sets --progress on its card, from 0 at its start to 1 at its end, and
// data-done once it's up. It runs a few times a second, and after every update, as
// a render puts back the time left as of then.
function tick() {
  const now = Date.now();
  for (const timer of app.querySelectorAll("[data-end]")) {
    const start = +timer.dataset.start, end = +timer.dataset.end;
    const seconds = Math.ceil(Math.max(0, end - now) / 1000);
    const pad = n => String(n).padStart(2, "0");
    timer.textContent = seconds >= 3600
      ? `${Math.floor(seconds / 3600)}:${pad(Math.floor(seconds / 60) % 60)}:${pad(seconds % 60)}`
      : `${Math.floor(seconds / 60)}:${pad(seconds % 60)}`;
    const card = timer.closest("li");
    if (!card) continue;
    card.style.setProperty("--progress", end > start ? Math.min(1, Math.max(0, (now - start) / (end - start))) : 1);
    card.toggleAttribute("data-done", seconds === 0);
  }
}
setInterval(tick, 250);

send("ready");
