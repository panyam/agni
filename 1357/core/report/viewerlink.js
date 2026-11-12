// A report's links move an agni viewer that is already open rather than loading it again (agni issue
// 1156). Every link targets the window named agni-viewer, so with scripts off a click still reuses one
// viewer tab, or a frame of that name beside the report. With scripts on, a click finds that window
// and, when it already shows a viewer, asks it to apply the link over postMessage (ApplyLocation with
// the link alone, which the viewer takes from any origin, a report opened from disk included). A
// viewer that does not answer in time, or answers that it could not, gets the link loaded instead.
// A viewer posts an accepted note the moment it takes the request, and once that arrives the script
// waits for the answer, since resolving a verdict link can take longer than the first wait. The
// script makes no request of its own.
(function () {
  var NAME = "agni-viewer";
  var ENVELOPE = "agni-viewer/v1";
  var ACCEPTED = "agni-viewer/v1/accepted";
  var WAIT_MS = 2000;
  var WORKING_MS = 60000; // how long an accepted request may take before the link is loaded anyway
  var seq = 0;
  document.addEventListener("click", function (ev) {
    if (ev.defaultPrevented || ev.button !== 0 || ev.metaKey || ev.ctrlKey || ev.shiftKey || ev.altKey) return;
    var a = ev.target && ev.target.closest ? ev.target.closest('a[target="' + NAME + '"]') : null;
    if (!a) return;
    var w = window.open("", NAME);
    if (!w) return; // a blocked popup: the link opens as it would with no script
    ev.preventDefault();
    var blank = false;
    try {
      blank = w.location.href === "about:blank"; // a window this click just made, so nothing to ask
    } catch (e) {
      blank = false; // another origin's page, which is the viewer this is for
    }
    if (blank) {
      w.location.href = a.href;
      return;
    }
    var id = "report-" + ++seq;
    var settled = false;
    var timer = null;
    var load = function () {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      window.removeEventListener("message", onReply);
      w.location.href = a.href;
    };
    var onReply = function (m) {
      if (m.source !== w || !m.data || m.data.id !== id) return;
      if (m.data.agni === ACCEPTED) {
        clearTimeout(timer);
        timer = setTimeout(load, WORKING_MS);
        return;
      }
      if (m.data.agni !== ENVELOPE) return;
      if (!m.data.ok) return load();
      settled = true;
      clearTimeout(timer);
      window.removeEventListener("message", onReply);
    };
    window.addEventListener("message", onReply);
    w.postMessage({ agni: ENVELOPE, id: id, method: "ApplyLocation", body: { url: a.href } }, "*");
    timer = setTimeout(load, WAIT_MS);
    try {
      w.focus();
    } catch (e) {}
  });
})();
