const titlebar = document.querySelector(".titlebar");
const frame = action => window.webkit?.messageHandlers?.frame?.postMessage(action);
titlebar.addEventListener("mousedown", e => {
  if (e.button === 0 && !e.target.closest("button")) frame("drag");
});
for (const action of ["close", "minimise", "zoom"]) {
  titlebar.querySelector("." + action).addEventListener("click", () => frame(action));
}
// The arrow keys, space and page up and down scroll <main> from the start, not
// only once it's been clicked.
document.querySelector("main").focus({ preventScroll: true });

// unwrap puts a Block's elements in place of the <template> it comes in, each marked
// data-block with its id, so the report's blocks stay the <article>'s children.
function unwrap(template) {
  const nodes = [...template.content.childNodes];
  for (const node of nodes) {
    if (node.nodeType === Node.ELEMENT_NODE) node.dataset.block = template.dataset.block;
  }
  template.replaceWith(...nodes);
}
for (const template of document.querySelectorAll("article > template[data-block]")) unwrap(template);

// replaceBlock replaces the block with id by html, a Block with that id rendered by
// Go, its text escaped. Without such a block, it does nothing.
function replaceBlock(id, html) {
  const old = document.querySelectorAll(`article > [data-block="${CSS.escape(id)}"]`);
  if (old.length === 0) return;
  const parsed = document.createElement("template");
  parsed.innerHTML = html;
  const template = parsed.content.querySelector("template[data-block]");
  old[0].before(template);
  unwrap(template);
  for (const node of old) node.remove();
}

// A click on anything with data-click, such as a Buttons' button, with the main or
// middle button, posts its id and how it was clicked: the modifier keys held, and
// which button. One on what's busy, its Click still running from the last, posts
// nothing (and the app would drop it).
function clicked(event) {
  const target = event.target.closest("[data-click]");
  if (target && !target.hasAttribute("data-busy")) {
    const how = { alt: event.altKey, cmd: event.metaKey, shift: event.shiftKey, ctrl: event.ctrlKey, middle: event.button === 1 };
    window.webkit.messageHandlers.ledge.postMessage({ action: "click", id: target.dataset.click, event: how });
  }
}
const article = document.querySelector("article");
article.addEventListener("click", clicked);
article.addEventListener("auxclick", event => {
  if (event.button === 1) clicked(event);
});

// setBusy marks data-busy what has each of ids in its data-click, while its Click
// runs, and takes it off everything else. The app calls it whenever that changes,
// once the page is ready, and after replacing a block.
function setBusy(ids) {
  const busy = new Set(ids);
  for (const node of document.querySelectorAll("article [data-click]")) {
    node.toggleAttribute("data-busy", busy.has(node.dataset.click));
  }
}

// Once loaded, the page can take replaceBlock and setBusy; until then the app waits.
window.webkit.messageHandlers.ledge.postMessage({ action: "ready" });
