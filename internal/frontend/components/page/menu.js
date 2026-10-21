const items = [...document.querySelectorAll(".menu-item")];

// Everything the page tells the app: an action, and the id it applies to.
function send(action, id) {
  window.webkit.messageHandlers.ledge.postMessage(id == null ? { action } : { action, id });
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

// choose posts item i's index, if it can be chosen.
function choose(i) {
  if (enabled(i)) send("choose", String(i));
}

items.forEach((item, i) => {
  item.addEventListener("mouseenter", () => activate(enabled(i) ? i : -1));
  item.addEventListener("click", () => choose(i));
});
document.addEventListener("mouseleave", () => activate(-1));

// The arrow keys move, Return or space chooses, and Escape closes the menu.
document.addEventListener("keydown", event => {
  switch (event.key) {
    case "ArrowDown": move(1); break;
    case "ArrowUp": move(-1); break;
    case "Enter": case " ": choose(active); break;
    case "Escape": send("close"); break;
    default: return;
  }
  event.preventDefault();
});
document.addEventListener("contextmenu", event => event.preventDefault());

send("ready");
