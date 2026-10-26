// What every page shares, inlined before its own script: how it tells the app things,
// and how a click on something with data-click reaches it.

// send posts to the app an action, the id it applies to, and for a click or a choice
// from a menu, how it was made.
function send(action, id, event) {
  const message = { action };
  if (id != null) message.id = id;
  if (event != null) message.event = event;
  window.webkit.messageHandlers.ledge.postMessage(message);
}

// how is how event, a mouse or key event, clicked: the modifier keys held, and
// whether with the middle button.
function how(event) {
  return { alt: event.altKey, cmd: event.metaKey, shift: event.shiftKey, ctrl: event.ctrlKey, middle: event.button === 1 };
}

// clickable has a click in root, with the main or middle button, on anything with
// data-click post its id and how it was clicked: the innermost, so a click on the
// ×, or a part of a card's body such as a check, isn't also its card's. One on
// what's busy, its Click still running from the last, posts nothing (and the app
// would drop it).
function clickable(root) {
  const clicked = event => {
    const target = event.target.closest("[data-click]");
    if (target && root.contains(target) && !target.hasAttribute("data-busy")) send("click", target.dataset.click, how(event));
  };
  root.addEventListener("click", clicked);
  root.addEventListener("auxclick", event => {
    if (event.button === 1) clicked(event);
  });
  // setBusy marks data-busy what in root has each of ids in its data-click, while its
  // Click runs, and takes it off everything else. The app calls it whenever that
  // changes, and with every update, as an update takes data-busy off.
  window.setBusy = ids => {
    const busy = new Set(ids);
    for (const node of root.querySelectorAll("[data-click]")) {
      node.toggleAttribute("data-busy", busy.has(node.dataset.click));
    }
  };
}
