const items = [...document.querySelectorAll(".menu-item")];

// Everything the page tells the app: an action, the id it applies to, and for a
// choice, the modifier keys held as it was made.
function send(action, id, event) {
  const message = { action };
  if (id != null) message.id = id;
  if (event != null) message.event = event;
  window.webkit.messageHandlers.ledge.postMessage(message);
}

// active is the item the mouse or the keys are on, or -1 for none.
let active = -1;

function activate(i) {
  items.forEach((item, j) => item.classList.toggle("active", j === i));
  active = i;
}

function enabled(i) {
  return i >= 0 && !items[i].hasAttribute("aria-disabled");
}

// move goes to the next item that can be chosen, step 1 down or -1 up, round the end.
function move(step) {
  let i = active < 0 ? (step > 0 ? -1 : items.length) : active;
  for (let n = 0; n < items.length; n++) {
    i = (i + step + items.length) % items.length;
    if (enabled(i)) return activate(i);
  }
}

// choose posts item i's index, if it can be chosen, with the modifier keys held as
// event, a click or a key, chose it.
function choose(i, event) {
  if (enabled(i)) {
    send("choose", String(i), { alt: event.altKey, cmd: event.metaKey, shift: event.shiftKey, ctrl: event.ctrlKey });
  }
}

items.forEach((item, i) => {
  item.addEventListener("mouseenter", () => activate(enabled(i) ? i : -1));
  item.addEventListener("click", event => choose(i, event));
});
document.addEventListener("mouseleave", () => activate(-1));

// The arrow keys move, Return or space chooses, and Escape closes the menu.
document.addEventListener("keydown", event => {
  switch (event.key) {
    case "ArrowDown": move(1); break;
    case "ArrowUp": move(-1); break;
    case "Enter": case " ": choose(active, event); break;
    case "Escape": send("close"); break;
    default: return;
  }
  event.preventDefault();
});
document.addEventListener("contextmenu", event => event.preventDefault());

send("ready");
