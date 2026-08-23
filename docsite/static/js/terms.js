// terms.js — progressive enhancement for {{ explainable }} glossary links.
//
// A term renders from the server as an ordinary anchor carrying the one-line summary in `title` and
// pointing at the term's own page. That already works: with no JavaScript the browser shows the
// summary on hover and a click lands on the full page. This script upgrades it to an inline popover
// carrying the WHOLE term page, diagram included, so a reader never leaves the paragraph they are in.
//
// The popover's content is the term page itself, fetched and cached, rather than a generated JSON
// blob shipped with every page. One source, the same argument agnirun.go makes for captured output:
// a second copy of a definition is a copy that rots.
(function () {
  "use strict";

  var cache = {};          // href -> Promise<string of HTML>
  var pop = null;          // the single shared popover element
  var current = null;      // the anchor the popover is currently attached to
  var hideTimer = null;

  function popover() {
    if (pop) return pop;
    pop = document.createElement("div");
    pop.className = "xterm-pop";
    pop.setAttribute("role", "dialog");
    pop.hidden = true;
    // Moving onto the popover itself must not dismiss it, or a reader cannot scroll or click a link
    // inside the definition they just opened.
    pop.addEventListener("mouseenter", cancelHide);
    pop.addEventListener("mouseleave", scheduleHide);
    document.body.appendChild(pop);
    return pop;
  }

  // fetchBody pulls the article body out of a term page. The pages render through the same Content
  // template as everything else, so .content-body is the definition and nothing else.
  function fetchBody(href) {
    if (!cache[href]) {
      cache[href] = fetch(href)
        .then(function (r) {
          if (!r.ok) throw new Error(r.status);
          return r.text();
        })
        .then(function (html) {
          var doc = new DOMParser().parseFromString(html, "text/html");
          var body = doc.querySelector(".content-body");
          var head = doc.querySelector(".content-header h1");
          if (!body) throw new Error("no .content-body");
          return (head ? "<h2>" + head.textContent + "</h2>" : "") + body.innerHTML;
        })
        .catch(function () {
          // Fall back to the summary the anchor already carries rather than showing an error box.
          return null;
        });
    }
    return cache[href];
  }

  // place decides above-or-below from the room actually available, and CAPS the panel when there is
  // room for it in neither direction. A term page carrying a diagram runs to about 570px, which for
  // a link near the middle of the viewport fits neither way; the first version fell through to
  // "below" and put half the definition past the bottom of the window, where a reader hovering a
  // link cannot follow it. Capping and letting it scroll internally is the only answer that keeps
  // the whole panel reachable.
  function place(a) {
    var margin = 8;
    var r = a.getBoundingClientRect();
    var p = popover();

    // Measure at natural size. Clearing the inline cap restores the stylesheet's 70vh ceiling, so a
    // previous anchor's cap cannot leak into this measurement.
    p.style.maxHeight = "";
    p.style.left = "0px";
    p.style.top = "0px";
    var pr = p.getBoundingClientRect();

    var roomBelow = window.innerHeight - r.bottom - margin * 2;
    var roomAbove = r.top - margin * 2;
    var top;

    if (pr.height <= roomBelow) {
      top = r.bottom + margin;
    } else if (pr.height <= roomAbove) {
      top = r.top - pr.height - margin;
    } else if (roomBelow >= roomAbove) {
      p.style.maxHeight = roomBelow + "px";
      top = r.bottom + margin;
    } else {
      p.style.maxHeight = roomAbove + "px";
      top = margin;
    }
    pr = p.getBoundingClientRect(); // the cap may have changed the width via the scrollbar

    var left = Math.min(
      Math.max(margin, r.left + r.width / 2 - pr.width / 2),
      Math.max(margin, window.innerWidth - pr.width - margin)
    );
    p.style.left = left + window.scrollX + "px";
    p.style.top = top + window.scrollY + "px";
  }

  function show(a) {
    cancelHide();
    if (current === a && !popover().hidden) return;
    current = a;
    var p = popover();

    fetchBody(a.href).then(function (html) {
      if (current !== a) return; // the reader moved on while it was in flight
      p.innerHTML = html || "<p>" + (a.getAttribute("data-summary") || "") + "</p>";
      var more = document.createElement("a");
      more.className = "xterm-more";
      more.href = a.href;
      more.textContent = "Open full page";
      p.appendChild(more);
      p.hidden = false;
      place(a);
      runMermaid(p);
    });
  }

  // runMermaid renders any diagram that arrived with the fetched body. BasePage.html only loads
  // mermaid when the PAGE itself contains a diagram, so a page whose only diagrams come from a
  // popover has to import it here too. Both paths share the browser's module cache.
  function runMermaid(scope) {
    var nodes = scope.querySelectorAll("pre.mermaid, .mermaid");
    if (!nodes.length) return;
    nodes.forEach(function (n) {
      n.removeAttribute("data-processed");
    });
    import("https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs").then(function (m) {
      var dark = document.documentElement.classList.contains("dark");
      m.default.initialize({ startOnLoad: false, theme: dark ? "dark" : "default" });
      m.default.run({ nodes: nodes }).then(function () {
        if (current) place(current);
      });
    });
  }

  function cancelHide() {
    if (hideTimer) {
      clearTimeout(hideTimer);
      hideTimer = null;
    }
  }

  function scheduleHide() {
    cancelHide();
    // A gap between the link and the popover is unavoidable, so dismissal waits long enough for a
    // pointer to cross it.
    hideTimer = setTimeout(hide, 250);
  }

  function hide() {
    cancelHide();
    current = null;
    if (pop) pop.hidden = true;
  }

  function enhance(a) {
    // The native tooltip and the popover would otherwise both fire. Stash the summary so the
    // no-JS fallback text is still available if the fetch fails.
    var t = a.getAttribute("title");
    if (t !== null) {
      a.setAttribute("data-summary", t);
      a.removeAttribute("title");
    }

    a.addEventListener("mouseenter", function () {
      show(a);
    });
    a.addEventListener("mouseleave", scheduleHide);
    a.addEventListener("focus", function () {
      show(a);
    });
    a.addEventListener("blur", scheduleHide);

    // Touch has no hover. The first tap opens the popover, and "Open full page" inside it is how a
    // reader navigates, so a tap never jumps the page out from under them by surprise.
    a.addEventListener("click", function (e) {
      if (window.matchMedia("(hover: none)").matches) {
        e.preventDefault();
        show(a);
      }
    });
  }

  function run() {
    document.querySelectorAll("a.xterm").forEach(enhance);
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") hide();
    });
    window.addEventListener("scroll", function () {
      if (current && pop && !pop.hidden) place(current);
    }, { passive: true });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", run);
  } else {
    run();
  }
})();
