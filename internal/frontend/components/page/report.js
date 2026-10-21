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
