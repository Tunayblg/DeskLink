// Sends script errors (and a few milestones) to the PC's log, so problems that
// only happen on the phone can be seen. Loaded before everything else.
(function () {
  "use strict";
  function send(message, where) {
    try {
      var x = new XMLHttpRequest();
      x.open("POST", "/api/report", true);
      x.setRequestHeader("Content-Type", "application/json");
      x.send(JSON.stringify({ message: String(message).slice(0, 300), where: String(where || "").slice(0, 200) }));
    } catch (e) {}
  }
  window.onerror = function (msg, src, line, col) {
    send(msg, String(src || "").replace(location.origin, "") + ":" + line + ":" + col);
  };
  window.addEventListener("unhandledrejection", function (e) {
    send("unhandled rejection: " + (e.reason && e.reason.message ? e.reason.message : e.reason));
  });
  window.DeskReport = send;
})();
