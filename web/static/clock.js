// Clock screen: five faces on a black screen. Swipe (or use the dots) to switch.
//   tach:   a yellow sports-car rev counter. The scale runs 1 to 12 over 270°
//           (bottom left round to bottom right); a black hour hand and a long
//           red minute needle both sweep it, and the minute needle drops back
//           at the top of the hour like revs after a gear change. Three strips
//           of shift lights (green, amber, red) fill one light every 2 s.
//   simple: a plain round analog clock with the date and a small digital time.
//   digital: glowing neon digits with a seconds strip and the week days.
//   info:    the time, the weather of a city and two widgets (PC network, CPU, device batteries...).
//   modern: big thin digits, the date above and a hairline that fills each minute.
// Only transforms, opacity and SVG fills change, and nothing runs while hidden.
window.DeskClock = (function () {
  "use strict";

  var FACE_KEY = "desklink.clockface";
  var FACES = ["tach", "simple", "modern", "digital", "info"];
  var $ = function (id) { return document.getElementById(id); };
  var timer = null, built = false, active = false, ignition = false;
  var face = norm(load());
  var el = {};

  function load() { try { return localStorage.getItem(FACE_KEY); } catch (e) { return null; } }
  function norm(v) { return FACES.indexOf(v) > 0 ? v : "tach"; }
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

  // ---- Modern face ---------------------------------------------------------

  function modernHTML() {
    return '<div class="m-date"></div>' +
      '<div class="m-time"><span class="m-h"></span><span class="m-sep">:</span><span class="m-m"></span><span class="m-sec"></span></div>' +
      '<div class="m-bar"><i></i></div>';
  }

  // ---- Digital face --------------------------------------------------------

  function digitalHTML() {
    var dots = "", days = "";
    for (var i = 0; i < 60; i++) dots += "<i></i>";
    // Week days, Monday first, named by the phone's language.
    for (var d = 0; d < 7; d++) {
      days += "<span>" + new Date(2024, 0, 1 + d).toLocaleDateString([], { weekday: "short" }).slice(0, 3) + "</span>";
    }
    return '<div class="d-panel"><div class="d-time"><span class="d-h"></span><span class="d-colon">:</span><span class="d-m"></span></div>' +
      '<div class="d-secs">' + dots + '</div></div><div class="d-days">' + days + '</div><div class="d-date"></div>';
  }

  // ---- Info face: time, weather and two widgets -----------------------------

  var CITY_KEY = "desklink.city", WIDGET_KEY = "desklink.widgets";
  var WX_EVERY = 10 * 60 * 1000, WX_RETRY = 20 * 1000;
  var wx = null, wxAsked = 0, wxFailed = false, widgetKeys = null, phoneBat = null, wxTried = false;

  function lsGet(k) { try { return localStorage.getItem(k); } catch (e) { return null; } }
  function lsSet(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }
  function city() { return lsGet(CITY_KEY) || "Izmit"; }

  // The last weather is kept, so the face has something to show before the PC answers.
  try { wx = JSON.parse(lsGet("desklink.weather") || "null"); } catch (e) { wx = null; }

  function wxIcon(code, day) {
    if (code === 0) return day ? "☀️" : "🌙";
    if (code === 1) return day ? "🌤️" : "🌙";
    if (code === 2) return day ? "⛅" : "☁️";
    if (code === 3) return "☁️";
    if (code === 45 || code === 48) return "🌫️";
    if (code >= 51 && code <= 57) return "🌦️";
    if ((code >= 61 && code <= 67) || (code >= 80 && code <= 82)) return "🌧️";
    if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "❄️";
    if (code >= 95) return "⛈️";
    return "🌡️";
  }

  function infoHTML() {
    return '<div class="i-main"><div class="i-time"><span class="i-hm"></span><span class="i-ss"></span></div><div class="i-date"></div>' +
      '<button type="button" class="i-wx" aria-label="Weather"><span class="i-ico"></span><span class="i-t"></span>' +
      '<span class="i-sub"><b class="i-city"></b><em class="i-mm"></em></span></button></div>' +
      '<div class="i-side"><button type="button" class="i-w"></button><button type="button" class="i-w"></button></div>';
  }

  function askWeather() {
    if (!window.DeskData) return;
    if (window.DeskData.send({ type: "weather", text: city() })) wxAsked = Date.now();
  }

  function onWeather(msg) {
    if (msg.weather) {
      wx = msg.weather;
      wxFailed = false;
      lsSet("desklink.weather", JSON.stringify(wx));
    } else {
      wxFailed = true;
    }
    if (active) tick();
  }

  // Everything a widget can show now, in a fixed order.
  function widgetPages() {
    var d = window.DeskData, s = d && d.stats(), out = [];
    ((d && d.batteries()) || []).forEach(function (b) { out.push("dev:" + b.name); });
    if (phoneBat) out.push("phone");
    out.push("net", "cpu");
    if (s && s.gpu) out.push("gpu");
    out.push("ram");
    return out;
  }

  function loadWidgets() {
    try {
      var v = JSON.parse(lsGet(WIDGET_KEY) || "null");
      if (Array.isArray(v) && v.length === 2) return v.map(function (k) { return typeof k === "string" ? k : null; });
    } catch (e) {}
    return [null, null];
  }

  function rate(b) {
    if (b >= 1000 * 1024) { var mb = b / 1048576; return (mb >= 99.5 ? Math.round(mb) : mb.toFixed(mb >= 9.95 ? 0 : 1)) + " MB/s"; }
    return Math.round(b / 1024) + " KB/s";
  }

  function pctView(label, p) {
    p = Math.max(0, Math.min(100, p));
    return { ring: p / 100, big: Math.round(p) + "%", label: label, over: p > 95 };
  }

  function widgetView(key) {
    var d = window.DeskData, s = d && d.stats(), i;
    if (key.indexOf("dev:") === 0) {
      var list = (d && d.batteries()) || [], dev = null;
      for (i = 0; i < list.length; i++) if (list[i].name === key.slice(4)) dev = list[i];
      if (!dev) return { ring: 0, big: "–", label: key.slice(4).replace(/^razer\s+/i, ""), off: true };
      var p = Math.max(0, Math.min(100, Math.round(+dev.percent || 0)));
      return { ring: p / 100, big: p + "%", label: String(dev.name).replace(/^razer\s+/i, ""), cls: p <= 20 ? "low" : p <= 40 ? "mid" : "" };
    }
    if (key === "phone") {
      if (!phoneBat) return { ring: 0, big: "–", label: "Phone", off: true };
      var q = Math.round(phoneBat.level * 100);
      return { ring: q / 100, big: q + "%", label: "Phone" + (phoneBat.charging ? " ⚡" : ""), cls: q <= 20 ? "low" : q <= 40 ? "mid" : "" };
    }
    if (key === "net") {
      var n = s && s.net;
      return n ? { rows: "↓ " + rate(n.down) + "<br>↑ " + rate(n.up), label: "Network", net: true } : { ring: 0, big: "–", label: "Network", off: true };
    }
    if (key === "cpu") return s ? pctView("CPU", s.cpu.usage) : { ring: 0, big: "–", label: "CPU", off: true };
    if (key === "gpu") return s && s.gpu ? pctView("GPU", s.gpu.usage) : { ring: 0, big: "–", label: "GPU", off: true };
    return s ? pctView("RAM", s.ram.usage) : { ring: 0, big: "–", label: "RAM", off: true };
  }

  function widgetHTML(v) {
    var ring = v.rows ? "" : '<svg viewBox="0 0 44 44"><circle class="tr" cx="22" cy="22" r="19"/><circle class="vl" cx="22" cy="22" r="19" stroke-dasharray="119.4" stroke-dashoffset="' +
      (119.4 * (1 - v.ring)).toFixed(1) + '"/></svg>';
    var big = v.rows ? '<b class="i-net">' + v.rows + "</b>" : "<b" + (v.over ? ' class="over"' : "") + ">" + v.big + "</b>";
    return ring + '<div class="i-wt">' + big + "<span>" + v.label.replace(/</g, "&lt;") + "</span></div>";
  }

  function stepWidget(i, dir) {
    if (!widgetKeys) widgetKeys = loadWidgets();
    var pages = widgetPages(), mine = widgetKeys[i], other = widgetKeys[1 - i];
    var list = pages.filter(function (k) { return k === mine || k !== other; });
    if (mine && list.indexOf(mine) < 0) list.push(mine);
    if (list.length < 2) return;
    widgetKeys[i] = list[(list.indexOf(mine) + dir + list.length) % list.length];
    lsSet(WIDGET_KEY, JSON.stringify(widgetKeys));
    renderInfoWidgets(i);
  }

  function renderInfoWidgets(swapped) {
    if (!el.info) return;
    if (!widgetKeys) widgetKeys = loadWidgets();
    var pages = widgetPages();
    // A fresh widget starts with the first device (or CPU) and the network.
    var firstDev = pages.filter(function (k) { return k.indexOf("dev:") === 0; })[0];
    var order = [firstDev || "cpu", "net", "cpu", "ram"];
    for (var i = 0; i < 2; i++) {
      if (widgetKeys[i]) continue;
      var taken = widgetKeys.filter(Boolean);
      widgetKeys[i] = order.filter(function (k) { return taken.indexOf(k) < 0; })[0] || null;
    }
    for (var j = 0; j < 2; j++) {
      var node = el.info.w[j], v = widgetView(widgetKeys[j] || "cpu"), html = widgetHTML(v);
      node.className = "i-w" + (v.cls ? " " + v.cls : "") + (v.off ? " off" : "");
      if (node._html !== html) {
        node._html = html;
        node.innerHTML = html;
        if (swapped === j) { node.classList.remove("swap"); void node.offsetWidth; node.classList.add("swap"); }
      }
    }
  }

  function renderInfo(now) {
    var h = now.getHours(), m = now.getMinutes(), s = now.getSeconds(), inf = el.info;
    inf.hm.textContent = pad(h) + ":" + pad(m);
    inf.ss.textContent = pad(s);
    inf.date.textContent = now.toLocaleDateString([], { weekday: "long", day: "numeric", month: "long" });
    if (wx) {
      inf.ico.textContent = wxIcon(wx.code, wx.day);
      inf.t.textContent = Math.round(wx.temp) + "°";
      inf.city.textContent = wx.city;
      inf.mm.textContent = "↓ " + Math.round(wx.min) + "°  ↑ " + Math.round(wx.max) + "°";
    } else {
      inf.ico.textContent = "🌡️";
      inf.t.textContent = "–";
      inf.city.textContent = city();
      inf.mm.textContent = wxFailed ? "?" : "…";
    }
    renderInfoWidgets();
    var due = wxFailed ? WX_RETRY : WX_EVERY;
    if (Date.now() - wxAsked > due) askWeather();
  }

  // ---- Running -------------------------------------------------------------

  function build() {
    if (built) return;
    built = true;
    var t = $("face-tach"), s = $("face-simple"), mo = $("face-modern");
    t.innerHTML = tachSVG();
    s.innerHTML = simpleSVG();
    mo.innerHTML = modernHTML();
    var inf = $("face-info");
    inf.innerHTML = infoHTML();
    var ws = inf.querySelectorAll(".i-w");
    el.info = { hm: inf.querySelector(".i-hm"), ss: inf.querySelector(".i-ss"), date: inf.querySelector(".i-date"), ico: inf.querySelector(".i-ico"),
      t: inf.querySelector(".i-t"), city: inf.querySelector(".i-city"), mm: inf.querySelector(".i-mm"), w: [ws[0], ws[1]] };
    inf.querySelector(".i-wx").addEventListener("click", function () {
      var c = window.prompt("City", city());
      if (c && c.trim()) { lsSet(CITY_KEY, c.trim().slice(0, 60)); wx = null; wxFailed = false; askWeather(); tick(); }
    });
    // A tap or a swipe in any direction moves the widget on (the other way for a swipe left or up),
    // as in game mode. The face's own swipe (changing faces) must not start from here.
    Array.prototype.forEach.call(ws, function (node, i) {
      var x0 = 0, y0 = 0, down = false, swiped = 0;
      node.addEventListener("touchstart", function (e) {
        e.stopPropagation();
        var p = e.touches[0];
        x0 = p.clientX; y0 = p.clientY; down = e.touches.length === 1;
      }, { passive: true });
      node.addEventListener("touchend", function (e) {
        e.stopPropagation();
        if (!down) return;
        down = false;
        var p = e.changedTouches[0], dx = p.clientX - x0, dy = p.clientY - y0;
        var far = Math.max(Math.abs(dx), Math.abs(dy));
        if (far < 10) return; // a tap: the click that follows does it
        if (far >= 22) {
          swiped = Date.now();
          var v = Math.abs(dx) > Math.abs(dy) ? dx : dy;
          stepWidget(i, v < 0 ? 1 : -1);
        }
      }, { passive: true });
      node.addEventListener("click", function () {
        if (Date.now() - swiped < 500) return;
        stepWidget(i, 1);
      });
    });
    if (navigator.getBattery) navigator.getBattery().then(function (b) { phoneBat = b; }).catch(function () {});
    var dg = $("face-digital");
    dg.innerHTML = digitalHTML();
    el.digital = { h: dg.querySelector(".d-h"), m: dg.querySelector(".d-m"), dots: dg.querySelectorAll(".d-secs i"),
      days: dg.querySelectorAll(".d-days span"), date: dg.querySelector(".d-date"), lit: -1, day: -1 };
    el.modern = { date: mo.querySelector(".m-date"), h: mo.querySelector(".m-h"), m: mo.querySelector(".m-m"),
      sec: mo.querySelector(".m-sec"), bar: mo.querySelector(".m-bar i") };
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
    var dateText = now.toLocaleDateString([], { weekday: "short", day: "numeric", month: "short" });
    el.simple.date.textContent = dateText;
    el.modern.date.textContent = dateText;
    el.modern.h.textContent = pad(h);
    el.modern.m.textContent = pad(m);
    el.modern.sec.textContent = pad(s);

    renderInfo(now);

    var dg = el.digital;
    dg.h.textContent = pad(h);
    dg.m.textContent = pad(m);
    dg.date.textContent = dateText;
    // Dots light up one per second and all go out at the new minute.
    if (dg.lit !== s + 1) {
      for (var k = 0; k < 60; k++) dg.dots[k].classList.toggle("on", k <= s);
      dg.lit = s + 1;
    }
    var wd = (now.getDay() + 6) % 7;
    if (dg.day !== wd) {
      for (var j = 0; j < 7; j++) dg.days[j].classList.toggle("on", j === wd);
      dg.day = wd;
    }
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

  // The modern face's hairline fills over one minute, started at the current second.
  function syncBar() {
    var now = new Date(), bar = el.modern.bar;
    bar.style.animationDelay = -(now.getSeconds() + now.getMilliseconds() / 1000) + "s";
    bar.classList.remove("run");
    void bar.getBoundingClientRect();
    bar.classList.add("run");
  }

  // dir: 1 when the new face comes in from the right, -1 from the left, 0 for a fade.
  function setFace(name, remember, dir) {
    var changed = name !== face;
    face = norm(name);
    if (remember) save(face);
    $("view-clock").setAttribute("data-dir", dir > 0 ? "next" : dir < 0 ? "prev" : "");
    $("face-tach").classList.toggle("on", face === "tach");
    $("face-simple").classList.toggle("on", face === "simple");
    $("face-modern").classList.toggle("on", face === "modern");
    $("face-digital").classList.toggle("on", face === "digital");
    $("face-info").classList.toggle("on", face === "info");
    Array.prototype.forEach.call($("face-dots").querySelectorAll("button"), function (b) {
      b.classList.toggle("on", b.dataset.face === face);
    });
    if (!active) return;
    if (face === "tach" && (changed || !remember)) ignite();
    if (face === "simple") syncSeconds();
    if (face === "modern") syncBar();
    if (window.DeskData) window.DeskData.sync();
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
    if (el.modern) el.modern.bar.classList.remove("run");
  }

  return { start: start, stop: stop, onWeather: onWeather, wantsData: function () { return active && face === "info"; } };
})();
