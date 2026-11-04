// Clock screen: two faces on a black screen. Swipe (or use the dots) to switch.
//   tach:   a yellow sports-car rev counter. The scale runs 1 to 12 over 270°
//           (bottom left round to bottom right); a black hour hand and a long
//           red minute needle both sweep it, and the minute needle drops back
//           at the top of the hour like revs after a gear change. Three strips
//           of shift lights (green, amber, red) fill one light every 2 s.
//   simple: a plain round analog clock with the date and a small digital time.
// Only transforms, opacity and SVG fills change, and nothing runs while hidden.
window.DeskClock = (function () {
  "use strict";

  var FACE_KEY = "desklink.clockface";
  var FACES = ["tach", "simple"];
  var $ = function (id) { return document.getElementById(id); };
  var timer = null, built = false, active = false, ignition = false;
  var face = load() === "simple" ? "simple" : "tach";
  var el = {};

  function load() { try { return localStorage.getItem(FACE_KEY); } catch (e) { return null; } }
  function save(v) { try { localStorage.setItem(FACE_KEY, v); } catch (e) {} }
  function pad(n) { return (n < 10 ? "0" : "") + n; }

  // Point at radius r and angle deg (0 = 12 o'clock, clockwise) around (200, 200).
  function pt(r, deg) {
    var a = (deg - 90) * Math.PI / 180;
    return [(200 + r * Math.cos(a)).toFixed(2), (200 + r * Math.sin(a)).toFixed(2)];
  }

  function ticks(n, rOut, majorEvery, majorIn, minorIn, color, majorW, minorW, minorOpacity) {
    var s = "";
    for (var i = 0; i < n; i++) {
      var major = i % majorEvery === 0;
      var p1 = pt(rOut, i * 360 / n), p2 = pt(major ? majorIn : minorIn, i * 360 / n);
      s += '<line x1="' + p1[0] + '" y1="' + p1[1] + '" x2="' + p2[0] + '" y2="' + p2[1] + '" stroke="' + color +
        '" stroke-opacity="' + (major ? 1 : minorOpacity) + '" stroke-width="' + (major ? majorW : minorW) + '" stroke-linecap="round"/>';
    }
    return s;
  }

  // ---- Rev-counter dial ----------------------------------------------------

  // Scale value 0-12 to an angle: a 270° sweep starting at the bottom left.
  // 0 is an unmarked start tick; 1 sits just above it and 12 at the bottom right.
  function tachAngle(v) { return -135 + v * 22.5; }

  // 30 shift lights in three strips of 10 (left, top, right), filled clockwise
  // from the bottom of the left strip: one light every 2 seconds.
  var STRIPS = [
    { from: 236, color: "#22c55e" },
    { from: 330, color: "#facc15" },
    { from: 64, color: "#ef4444" }
  ];
  var PER_STRIP = 10, STRIP_SPAN = 60;

  function tachSVG() {
    var s = '<svg viewBox="-28 -28 456 456" aria-hidden="true"><defs>' +
      '<radialGradient id="tf" cx="50%" cy="42%" r="62%"><stop offset="0" stop-color="#ffe45c"/><stop offset=".6" stop-color="#f6c400"/><stop offset="1" stop-color="#d8a600"/></radialGradient>' +
      '<linearGradient id="tb" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#6b6e76"/><stop offset=".5" stop-color="#22242a"/><stop offset="1" stop-color="#46484f"/></linearGradient>' +
      "</defs>";
    STRIPS.forEach(function (st) {
      for (var i = 0; i < PER_STRIP; i++) {
        var p = pt(214, st.from + i * STRIP_SPAN / (PER_STRIP - 1));
        s += '<circle class="led" cx="' + p[0] + '" cy="' + p[1] + '" r="5.5" data-color="' + st.color + '" fill="#1b1b1f"/>';
      }
    });
    s += '<circle cx="200" cy="200" r="196" fill="url(#tb)"/><circle cx="200" cy="200" r="187" fill="#0d0d0f"/>' +
      '<circle cx="200" cy="200" r="179" fill="url(#tf)"/>';
    // Redline from 10 to 12.
    var a = pt(166, tachAngle(10)), b = pt(166, tachAngle(12));
    s += '<path d="M' + a[0] + " " + a[1] + "A166 166 0 0 1 " + b[0] + " " + b[1] + '" fill="none" stroke="#d1001c" stroke-width="16"/>';
    // Ticks: major at every number, minor at the half and quarter marks.
    for (var q = 0; q <= 48; q++) {
      var v = q / 4, major = q % 4 === 0, half = q % 2 === 0;
      var p1 = pt(175, tachAngle(v)), p2 = pt(major ? 148 : half ? 160 : 166, tachAngle(v));
      s += '<line x1="' + p1[0] + '" y1="' + p1[1] + '" x2="' + p2[0] + '" y2="' + p2[1] + '" stroke="#111" stroke-width="' +
        (major ? 5 : half ? 3 : 2) + '" stroke-linecap="round"/>';
    }
    for (var h = 1; h <= 12; h++) {
      var n = pt(124, tachAngle(h));
      s += '<text x="' + n[0] + '" y="' + n[1] + '" class="num">' + h + "</text>";
    }
    // The hands never reach the bottom of the dial, so the readout sits there.
    s += '<rect x="148" y="276" width="104" height="38" rx="10" fill="#0c0c0e" stroke="#2c2c30" stroke-width="2"/>' +
      '<text x="200" y="296" class="digital"><tspan class="hm"></tspan><tspan class="ss" dx="3"></tspan></text>' +
      // Hands point up (12) and are rotated by CSS around the centre.
      '<g class="hand hour"><path d="M193 216 L197.5 98 Q200 92 202.5 98 L207 216 Z" fill="#141414"/></g>' +
      '<g class="hand min"><path d="M195 222 L198.8 40 Q200 35 201.2 40 L205 222 Z" fill="#e10600"/></g>' +
      '<circle cx="200" cy="200" r="17" fill="#18181b" stroke="#8a8d94" stroke-width="2.5"/><circle cx="200" cy="200" r="5" fill="#3a3c42"/>' +
      "</svg>";
    return s;
  }

  // ---- Simple dial ---------------------------------------------------------

  function simpleSVG() {
    return '<svg viewBox="0 0 400 400" aria-hidden="true">' +
      '<circle cx="200" cy="200" r="192" fill="none" stroke="rgba(255,255,255,.08)" stroke-width="2"/>' +
      ticks(60, 182, 5, 160, 174, "#fff", 5, 2, 0.28) +
      '<text x="200" y="128" class="date"></text>' +
      '<text x="200" y="282" class="digital"></text>' +
      '<g class="hand hour"><line x1="200" y1="214" x2="200" y2="108" stroke="#f2f2f2" stroke-width="11" stroke-linecap="round"/></g>' +
      '<g class="hand min"><line x1="200" y1="218" x2="200" y2="50" stroke="#fff" stroke-width="7" stroke-linecap="round"/></g>' +
      '<g class="hand sec"><line x1="200" y1="238" x2="200" y2="38" stroke="#ff9f0a" stroke-width="2.5" stroke-linecap="round"/>' +
      '<circle cx="200" cy="200" r="7" fill="#ff9f0a"/><circle cx="200" cy="200" r="2.6" fill="#000"/></g>' +
      "</svg>";
  }

  // ---- Running -------------------------------------------------------------

  function build() {
    if (built) return;
    built = true;
    var t = $("face-tach"), s = $("face-simple");
    t.innerHTML = tachSVG();
    s.innerHTML = simpleSVG();
    el.tach = { hour: t.querySelector(".hour"), min: t.querySelector(".min"), hm: t.querySelector(".hm"), ss: t.querySelector(".ss") };
    el.leds = t.querySelectorAll(".led");
    el.simple = { hour: s.querySelector(".hour"), min: s.querySelector(".min"), sec: s.querySelector(".sec"),
      date: s.querySelector(".date"), digital: s.querySelector(".digital") };

    Array.prototype.forEach.call($("face-dots").querySelectorAll("button"), function (b) {
      b.addEventListener("click", function (e) { e.stopPropagation(); setFace(b.dataset.face, true, 0); });
    });

    // Swipe left or right to switch faces.
    var view = $("view-clock"), x0 = null, y0 = 0;
    view.addEventListener("touchstart", function (e) { x0 = e.touches[0].clientX; y0 = e.touches[0].clientY; }, { passive: true });
    view.addEventListener("touchend", function (e) {
      if (x0 === null) return;
      var dx = e.changedTouches[0].clientX - x0, dy = e.changedTouches[0].clientY - y0;
      x0 = null;
      if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) {
        var i = FACES.indexOf(face) + (dx < 0 ? 1 : -1);
        setFace(FACES[(i + FACES.length) % FACES.length], true, dx < 0 ? 1 : -1);
      }
    }, { passive: true });
  }

  // Hand angles keep growing through the day, so a hand never spins backwards at 12.
  function angles(now) {
    var minutesOfDay = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60;
    return { hour: minutesOfDay / 2, min: minutesOfDay * 6 };
  }

  function rotate(node, deg) { node.style.transform = "rotate(" + deg.toFixed(2) + "deg)"; }

  // Rev-counter hands: the hour on the 1-12 scale, the minute needle as
  // minutes/5 (every number is 5 minutes, like a clock face).
  function tachAngles(now) {
    var h = now.getHours() % 12, m = now.getMinutes(), s = now.getSeconds();
    return { hour: tachAngle(h + m / 60), min: tachAngle(m / 5 + s / 300) };
  }

  function tick() {
    var now = new Date(), a = angles(now), h = now.getHours(), m = now.getMinutes(), s = now.getSeconds();
    if (!ignition) {
      var ta = tachAngles(now);
      rotate(el.tach.hour, ta.hour);
      rotate(el.tach.min, ta.min);
      setLeds(Math.floor(s / 2) + 1);
    }
    el.tach.hm.textContent = pad(h) + ":" + pad(m);
    el.tach.ss.textContent = pad(s);

    rotate(el.simple.hour, a.hour);
    rotate(el.simple.min, a.min);
    el.simple.digital.textContent = pad(h) + ":" + pad(m);
    el.simple.date.textContent = now.toLocaleDateString([], { weekday: "short", day: "numeric", month: "short" });
  }

  function setLeds(lit) {
    for (var i = 0; i < el.leds.length; i++) {
      var on = i < lit;
      if (el.leds[i]._on === on) continue;
      el.leds[i]._on = on;
      el.leds[i].setAttribute("fill", on ? el.leds[i].getAttribute("data-color") : "#1b1b1f");
    }
  }

  // Ignition, as when a car starts: both needles rev from 0 to the redline end
  // and fall back onto the time, while the shift lights run up and drop back.
  function ignite() {
    ignition = true;
    var hands = [el.tach.hour, el.tach.min];
    hands.forEach(function (hand) {
      hand.classList.remove("settle");
      hand.style.transition = "none";
      rotate(hand, tachAngle(0));
      void hand.getBoundingClientRect();
      hand.style.transition = "";
      hand.classList.add("rev");
      rotate(hand, tachAngle(12));
    });
    setLeds(0);
    for (var i = 1; i <= el.leds.length; i++) {
      (function (n) { setTimeout(function () { if (ignition) setLeds(n); }, n * 22); })(i);
    }
    setTimeout(function () {
      var ta = tachAngles(new Date());
      hands.forEach(function (hand, i) {
        hand.classList.remove("rev");
        hand.classList.add("settle");
        rotate(hand, i ? ta.min : ta.hour);
      });
    }, 720);
    setTimeout(function () {
      ignition = false;
      tick();
      hands.forEach(function (hand) { hand.classList.remove("settle"); });
    }, 2100);
  }

  // The simple clock's seconds hand is a CSS animation started at the current second.
  function syncSeconds() {
    var now = new Date(), sec = el.simple.sec;
    sec.style.animationDelay = -(now.getSeconds() + now.getMilliseconds() / 1000) + "s";
    sec.classList.remove("run");
    void sec.getBoundingClientRect();
    sec.classList.add("run");
  }

  // dir: 1 when the new face comes in from the right, -1 from the left, 0 for a fade.
  function setFace(name, remember, dir) {
    var changed = name !== face;
    face = name === "simple" ? "simple" : "tach";
    if (remember) save(face);
    $("view-clock").setAttribute("data-dir", dir > 0 ? "next" : dir < 0 ? "prev" : "");
    $("face-tach").classList.toggle("on", face === "tach");
    $("face-simple").classList.toggle("on", face === "simple");
    Array.prototype.forEach.call($("face-dots").querySelectorAll("button"), function (b) {
      b.classList.toggle("on", b.dataset.face === face);
    });
    if (!active) return;
    if (face === "tach" && (changed || !remember)) ignite();
    if (face === "simple") syncSeconds();
  }

  function start() {
    build();
    active = true;
    tick();
    setFace(face, false, 0);
    clearInterval(timer);
    timer = setInterval(tick, 1000);
  }

  function stop() {
    active = false;
    ignition = false;
    clearInterval(timer);
    timer = null;
    if (el.simple) el.simple.sec.classList.remove("run");
  }

  return { start: start, stop: stop };
})();
