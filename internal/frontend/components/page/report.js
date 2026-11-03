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
// still there keeps its element, and the report its scroll. A run's output that was
// scrolled to the bottom, or is new, is scrolled to the bottom of what it has now.
function update(title, html) {
  titlebar.querySelector("span").textContent = title;
  const scrolledUp = new Set([...article.querySelectorAll(".run pre")].filter(pre => pre.scrollTop + pre.clientHeight < pre.scrollHeight - 4));
  Idiomorph.morph(article, html, { morphStyle: "innerHTML" });
  for (const pre of article.querySelectorAll(".run pre")) {
    if (!scrolledUp.has(pre)) pre.scrollTop = pre.scrollHeight;
  }
}

// What's clicked in the report, such as a Buttons' button, posts its id; the app
// calls setBusy with every update, and once the page is ready.
clickable(article);

// Once loaded, the page can take update and setBusy; until then the app waits.
send("ready");
