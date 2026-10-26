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

const article = document.querySelector("article");

// update shows title in the title bar, and html, rendered by Go from ReportContent,
// with its text escaped, as the report. It morphs the <article> into html, so what's
// still there keeps its element, and the report its scroll.
function update(title, html) {
  titlebar.querySelector("span").textContent = title;
  Idiomorph.morph(article, html, { morphStyle: "innerHTML" });
}

// What's clicked in the report, such as a Buttons' button, posts its id; the app
// calls setBusy with every update, and once the page is ready.
clickable(article);

// Once loaded, the page can take update and setBusy; until then the app waits.
send("ready");
