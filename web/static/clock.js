// Clock screen: six faces on a black screen. Swipe (or use the dots) to switch.
//   tach:   a yellow sports-car rev counter. The scale runs 1 to 12 over 270°
//           (bottom left round to bottom right); a black hour hand and a long
//           red minute needle both sweep it, and the minute needle drops back
//           at the top of the hour like revs after a gear change. Three strips
//           of shift lights (green, amber, red) fill one light every 2 s.
//   simple: a plain round analog clock with the date and a small digital time.
//   digital: glowing neon digits with a seconds strip and the week days.
//   rings:   three glowing rings (hours, minutes, seconds) around big digits.
//   info:    the time, the weather of a city and two widgets (PC network, CPU, device batteries...).
//   modern: big thin digits, the date above and a hairline that fills each minute.
// Only transforms, opacity and SVG fills change, and nothing runs while hidden.
window.DeskClock = (function () {
  "use strict";

  var FACE_KEY = "desklink.clockface";
  var FACES = ["tach", "simple", "modern", "digital", "rings", "info"];
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

  // ---- Rings face: hours, minutes and seconds as three filling rings ----------

  var RING_R = [168, 134, 100], RING_C = ["#ff375f", "#a3e635", "#38bdf8"];

  function ringsSVG() {
    var s = '<svg viewBox="0 0 400 400" aria-hidden="true">';
    for (var i = 0; i < 3; i++) {
      var len = (2 * Math.PI * RING_R[i]).toFixed(2);
      s += '<circle cx="200" cy="200" r="' + RING_R[i] + '" fill="none" stroke="' + RING_C[i] + '" stroke-opacity=".16" stroke-width="22"/>' +
        '<circle class="rg r' + i + '" cx="200" cy="200" r="' + RING_R[i] + '" fill="none" stroke="' + RING_C[i] + '" stroke-width="22" stroke-linecap="round"' +
        ' stroke-dasharray="' + len + '" stroke-dashoffset="' + len + '" data-len="' + len + '" transform="rotate(-90 200 200)"/>';
    }
    return s + '<text x="200" y="196" class="rt"></text><text x="200" y="244" class="rd"></text></svg>';
  }

  // ---- Info face: usage on top, time and weather in the middle, devices below ----

  var CITY_KEY = "desklink.city";
  var WX_EVERY = 10 * 60 * 1000, WX_RETRY = 20 * 1000;
  var wx = null, wxAsked = 0, wxFailed = false, phoneBat = null;

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

  var USAGE = ["NET", "RAM", "CPU", "GPU"];

  function infoHTML() {
    var top = "";
    USAGE.forEach(function (n) {
      top += '<div class="i-m" data-k="' + n + '"><span>' + n + '</span><b>–</b><i class="bar"><u></u></i><em></em></div>';
    });
    return '<div class="i-top">' + top + "</div>" +
      '<div class="i-center"><div class="i-time"><span class="i-hm"></span><span class="i-ss"></span></div><div class="i-date"></div>' +
      '<button type="button" class="i-wx" aria-label="Weather"><span class="i-ico"></span><span class="i-t"></span>' +
      '<span class="i-city"></span><span class="i-mm"></span></button></div>' +
      '<div class="i-bot"></div>';
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

  function rateShort(b) {
    if (b >= 1000 * 1024) { var mb = b / 1048576; return (mb >= 99.5 ? Math.round(mb) : mb.toFixed(mb >= 9.95 ? 0 : 1)) + " MB/s"; }
    return Math.round(b / 1024) + " KB/s";
  }

  // Sets a text only when it changed, so the face does not repaint for nothing.
  function setText(node, v) { if (node.textContent !== v) node.textContent = v; }

  function renderUsage() {
    var d = window.DeskData, s = d && d.stats(), cells = el.info.top;
    cells.forEach(function (c) {
      var k = c.root.getAttribute("data-k"), value = "–", sub = "", pct = null;
      if (s) {
        if (k === "NET" && s.net) { value = "↓ " + rateShort(s.net.down); sub = "↑ " + rateShort(s.net.up); }
        else if (k === "RAM") { pct = s.ram.usage; value = Math.round(pct) + "%"; sub = (s.ram.used_mb / 1024).toFixed(1) + " GB"; }
        else if (k === "CPU") { pct = s.cpu.usage; value = Math.round(pct) + "%"; }
        else if (k === "GPU" && s.gpu) { pct = s.gpu.usage; value = Math.round(pct) + "%"; sub = s.gpu.temp + "°C"; }
      }
      setText(c.val, value);
      setText(c.sub, sub);
      c.val.classList.toggle("over", pct !== null && pct > 95);
      c.root.classList.toggle("off", value === "–");
      c.root.classList.toggle("hasbar", pct !== null);
      c.root.classList.toggle("hot", pct !== null && pct >= 90);
      if (pct !== null) c.fill.style.transform = "scaleX(" + (Math.max(0, Math.min(100, pct)) / 100).toFixed(3) + ")";
    });
  }

  // What the bottom row shows: connected devices first; when there are few, the phone's
  // own battery and the graphics card's temperature, memory and power fill it up.
  function chipList() {
    var d = window.DeskData, s = d && d.stats(), out = [];
    ((d && d.batteries()) || []).forEach(function (b) {
      var p = Math.max(0, Math.min(100, Math.round(+b.percent || 0)));
      out.push({ name: String(b.name).replace(/^razer\s+/i, ""), val: p + "%", ring: p / 100, cls: p <= 20 ? "low" : p <= 40 ? "mid" : "" });
    });
    if (phoneBat) {
      var q = Math.round(phoneBat.level * 100);
      out.push({ name: "Phone" + (phoneBat.charging ? " ⚡" : ""), val: q + "%", ring: q / 100, cls: q <= 20 ? "low" : q <= 40 ? "mid" : "" });
    }
    var g = s && s.gpu;
    if (g) {
      out.push({ name: "GPU temp", val: Math.round(g.temp) + "°", ring: Math.min(1, g.temp / 100), cls: g.temp >= 85 ? "low" : g.temp >= 75 ? "mid" : "" });
      if (g.vram_total_mb) {
        var u = g.vram_used_mb / g.vram_total_mb;
        out.push({ name: "VRAM", val: Math.round(u * 100) + "%", ring: u, cls: u >= .9 ? "low" : u >= .75 ? "mid" : "" });
      }
      if (g.power_limit_w) out.push({ name: "Power", val: Math.round(g.power_w) + " W", ring: Math.min(1, g.power_w / g.power_limit_w), cls: "" });
    }
    return out.slice(0, 4);
  }

  function renderChips() {
    var list = chipList(), box = el.info.bot, html = "";
    list.forEach(function (c) {
      html += '<div class="i-c' + (c.cls ? " " + c.cls : "") + '"><span>' + c.name.replace(/</g, "&lt;") + "</span><b>" + c.val +
        '</b><i class="bar"><u style="transform:scaleX(' + c.ring.toFixed(3) + ')"></u></i></div>';
    });
    if (box._html !== html) { box._html = html; box.innerHTML = html; }
  }

  function renderInfo(now) {
    var h = now.getHours(), m = now.getMinutes(), s = now.getSeconds(), inf = el.info;
    setText(inf.hm, pad(h) + ":" + pad(m));
    setText(inf.ss, pad(s));
    setText(inf.date, now.toLocaleDateString([], { weekday: "long", day: "numeric", month: "long" }));
    if (wx) {
      setText(inf.ico, wxIcon(wx.code, wx.day));
      setText(inf.t, Math.round(wx.temp) + "°");
      setText(inf.city, wx.city);
      setText(inf.mm, "↓ " + Math.round(wx.min) + "°  ↑ " + Math.round(wx.max) + "°");
    } else {
      setText(inf.ico, "🌡️");
      setText(inf.t, "–");
      setText(inf.city, city());
      setText(inf.mm, wxFailed ? "?" : "…");
    }
    renderUsage();
    renderChips();
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
    var rg = $("face-rings");
    rg.innerHTML = ringsSVG();
    el.rings = { arcs: rg.querySelectorAll(".rg"), last: [0, 0, 0], t: rg.querySelector(".rt"), d: rg.querySelector(".rd") };
    var inf = $("face-info");
    inf.innerHTML = infoHTML();
    el.info = { hm: inf.querySelector(".i-hm"), ss: inf.querySelector(".i-ss"), date: inf.querySelector(".i-date"), ico: inf.querySelector(".i-ico"),
      t: inf.querySelector(".i-t"), city: inf.querySelector(".i-city"), mm: inf.querySelector(".i-mm"), bot: inf.querySelector(".i-bot"),
      top: Array.prototype.map.call(inf.querySelectorAll(".i-m"), function (n) {
        return { root: n, val: n.querySelector("b"), sub: n.querySelector("em"), fill: n.querySelector("u") };
      }) };
    inf.querySelector(".i-wx").addEventListener("click", function () {
      var c = window.prompt("City", city());
      if (c && c.trim()) { lsSet(CITY_KEY, c.trim().slice(0, 60)); wx = null; wxFailed = false; askWeather(); tick(); }
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

  // Each ring fills as its unit passes; when a ring wraps to empty it jumps back without a reverse sweep.
  function renderRings(now) {
    var r = el.rings, s = now.getSeconds(), m = now.getMinutes() + s / 60, h = (now.getHours() % 12) + m / 60;
    var fr = [h / 12, m / 60, (s + 1) / 60];
    for (var i = 0; i < 3; i++) {
      var arc = r.arcs[i], len = parseFloat(arc.getAttribute("data-len"));
      if (fr[i] < r.last[i]) arc.style.transition = "none";
      else arc.style.transition = "";
      arc.style.strokeDashoffset = (len * (1 - fr[i])).toFixed(2);
      r.last[i] = fr[i];
    }
    setText(r.t, pad(now.getHours()) + ":" + pad(now.getMinutes()));
    setText(r.d, now.toLocaleDateString([], { weekday: "short", day: "numeric", month: "short" }));
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

    renderRings(now);
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
    $("face-rings").classList.toggle("on", face === "rings");
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
