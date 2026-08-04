// codeblocks.js — progressive enhancement for rendered code blocks.
//
// s3gen renders fenced code with chroma on the server (inline Monokai token colors), wrapping
// it as <pre><code>…chroma <pre><code>…</code></pre>…</code></pre>; an untagged block renders as
// a plain <pre><code>. This script runs over the rendered DOM and adds, to each top-level code
// block, a line-number gutter and a floating "Copy" button. Highlighting already happened on the
// server, so nothing is re-tokenized here; mermaid diagrams and inline <code> are left untouched.
(function () {
  "use strict";

  // codeText flattens a block to its source text. The chroma path double-nests the <pre>, but
  // textContent gathers all descendant text with chroma's literal newlines intact; the trailing
  // newline chroma appends is dropped so it does not add a phantom last line.
  function codeText(pre) {
    return pre.textContent.replace(/\n+$/, "");
  }

  function enhance(pre) {
    if (pre.closest(".code-block")) return; // already wrapped
    if (pre.classList.contains("mermaid")) return; // diagrams are not code
    if (pre.parentElement && pre.parentElement.closest("pre")) return; // inner chroma <pre>

    var text = codeText(pre);
    var lineCount = text.length ? text.split("\n").length : 1;

    // Drop chroma's inline background so the wrapper owns the surface uniformly across highlighted
    // and plain blocks; the inline token colors are kept.
    pre.querySelectorAll("pre[style]").forEach(function (inner) {
      inner.style.backgroundColor = "";
    });

    var block = document.createElement("div");
    block.className = "code-block";

    var copy = document.createElement("button");
    copy.type = "button";
    copy.className = "code-copy";
    copy.setAttribute("aria-label", "Copy code to clipboard");
    copy.textContent = "Copy";

    var inner = document.createElement("div");
    inner.className = "code-inner";

    var gutter = document.createElement("span");
    gutter.className = "code-gutter";
    gutter.setAttribute("aria-hidden", "true");
    var nums = [];
    for (var i = 1; i <= lineCount; i++) nums.push(i);
    gutter.textContent = nums.join("\n");

    var content = document.createElement("div");
    content.className = "code-content";

    pre.parentNode.insertBefore(block, pre);
    content.appendChild(pre); // move the original <pre> into the scroll column
    inner.appendChild(gutter);
    inner.appendChild(content);
    block.appendChild(copy);
    block.appendChild(inner);

    copy.addEventListener("click", function () {
      function done() {
        copy.textContent = "Copied";
        copy.classList.add("is-copied");
        setTimeout(function () {
          copy.textContent = "Copy";
          copy.classList.remove("is-copied");
        }, 1500);
      }
      function fallback() {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        try {
          document.execCommand("copy");
          done();
        } catch (e) {
          /* clipboard unavailable; nothing to do */
        }
        document.body.removeChild(ta);
      }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, fallback);
      } else {
        fallback();
      }
    });
  }

  function run() {
    var scope = document.querySelector("main") || document;
    scope.querySelectorAll("pre").forEach(enhance);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", run);
  } else {
    run();
  }
})();
