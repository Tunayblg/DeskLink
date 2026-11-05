// DeskLink panel. Plain ES2017 so it runs on iOS 15 Safari (the last iOS for iPhone 6s).
(function () {
  "use strict";

  var TOKEN_KEY = "desklink.token";
  var VIEW_KEY = "desklink.view";
  var VIEWS = ["home", "games", "stats", "watch", "pad", "clock"];
  var RETRY_MAX = 10000;
  var STATS_AFTER_LAUNCH = 1600; // ms before switching to stats after a game starts
  var SHIFT_EVERY = 3 * 60 * 1000;

  var $ = function (id) { return document.getElementById(id); };
  var ICONS = window.DESKLINK_ICONS || {};

  // UI text. The phone's language picks the table; English is the fallback.
  var STR = {
    en: {
      connecting: "Connecting…", offline: "PC offline · retrying in {}s", notPaired: "Not paired",
      notConnected: "Not connected to PC", unpaired: "This device is no longer paired. Scan the QR code on your PC again.",
      codeDigits: "The code has 6 digits.", pairFailed: "Pairing failed", unreachable: "Cannot reach the PC.",
      badCode: "Wrong or expired code. Try the one on screen now.", tooMany: "Too many tries. Wait for the next code.",
      expiredLink: "This link has expired. Enter the code shown on your PC.",
      a2hs: "Paired. For full screen, tap Share, then Add to Home Screen.",
      pairTitle: "Pair this device", pairHint: "Scan the QR code on your PC with the camera, or enter the code shown under it.", pair: "Pair",
      rotate: "Rotate your phone", noGames: "No games yet. Add them to config.json on your PC.",
      notPlaying: "Not playing", nothingPlaying: "Nothing playing", startMusic: "Start music on your PC",
      paused: "Paused", media: "Media", play: "PLAY",
      launching: "Launching…", toGameMode: "Switching to game mode",
      noAnswer: "No answer from the PC", launchFailed: "Could not start the game",
      alreadyRunning: "Already running", doubleTap: "Double-tap to start",
      shortcutEmpty: "Add a shortcut from DeskLink on your PC", shortcutAdd: "Add shortcut",
      watchMode: "WATCH MODE", brightness: "BRIGHTNESS", shutdownQ: "Shut down the PC?",
      gameMode: "GAME MODE", gpuFan: "GPU FAN", gpuFanIdle: "GPU FAN · IDLE",
      coreClock: "CORE CLOCK", noGpu: "No NVIDIA GPU found. GPU stats are only available for NVIDIA cards for now.",
      mic: "MIC", noMic: "NO MIC", padHint: "Slide to move · tap to click · two fingers: right click · three: middle click", sound: "SOUND", soundHint: "Swipe back for stats", master: "PC", game: "GAME", noApps: "Programs that play sound show up here."
    },
    tr: {
      connecting: "Bağlanıyor…", offline: "Bilgisayar kapalı · {} sn sonra tekrar", notPaired: "Eşleşmedi",
      notConnected: "Bilgisayara bağlı değil", unpaired: "Bu cihazın eşleşmesi kaldırıldı. Bilgisayardaki QR kodu tekrar okut.",
      codeDigits: "Kod 6 haneli olmalı.", pairFailed: "Eşleşme başarısız", unreachable: "Bilgisayara ulaşılamıyor.",
      badCode: "Kod yanlış ya da süresi doldu. Ekrandaki güncel kodu dene.", tooMany: "Çok fazla deneme. Sonraki kodu bekle.",
      expiredLink: "Bu bağlantının süresi doldu. Bilgisayardaki kodu gir.",
      a2hs: "Eşleşti. Tam ekran için Paylaş, sonra Ana Ekrana Ekle'ye dokun.",
      pairTitle: "Bu cihazı eşleştir", pairHint: "Bilgisayardaki QR kodu kamerayla okut ya da altındaki kodu gir.", pair: "Eşleştir",
      rotate: "Telefonu yan çevir", noGames: "Henüz oyun yok. Bilgisayardaki config.json dosyasına ekle.",
      notPlaying: "Çalmıyor", nothingPlaying: "Şu an bir şey çalmıyor", startMusic: "Bilgisayarda müzik başlat",
      paused: "Duraklatıldı", media: "Medya", play: "OYNA",
      launching: "Başlatılıyor…", toGameMode: "Oyun moduna geçiliyor",
      noAnswer: "Bilgisayardan yanıt gelmedi", launchFailed: "Oyun başlatılamadı",
      alreadyRunning: "Zaten çalışıyor", doubleTap: "Açmak için çift dokun",
      shortcutEmpty: "Bilgisayardaki DeskLink'ten kısayol ekle", shortcutAdd: "Kısayol ekle",
      watchMode: "DİZİ MODU", brightness: "PARLAKLIK", shutdownQ: "Bilgisayar kapatılsın mı?",
      gameMode: "OYUN MODU", gpuFan: "GPU FANI", gpuFanIdle: "GPU FANI · DURDU",
      coreClock: "ÇEKİRDEK HIZI", noGpu: "NVIDIA ekran kartı bulunamadı. GPU verileri şimdilik sadece NVIDIA kartlarda var.",
      mic: "MİKROFON", noMic: "MİKROFON YOK", padHint: "Kaydırarak hareket ettir · dokun: tıkla · iki parmak: sağ tık · üç parmak: orta tık", sound: "SES", soundHint: "İstatistikler için geri kaydır", master: "ANA SES", game: "OYUN", noApps: "Ses çalan programlar burada görünür."
    }
  };
  var LANG = /^tr\b/i.test(navigator.language || "") ? "tr" : "en";
  function t(key, arg) {
    var s = STR[LANG][key] || STR.en[key] || key;
    return arg === undefined ? s : s.replace("{}", arg);
  }
  // Static text in index.html is marked with data-t="key".
  Array.prototype.forEach.call(document.querySelectorAll("[data-t]"), function (el) {
    el.textContent = t(el.dataset.t);
  });
  document.documentElement.lang = LANG;

  var ws = null;
  var retryDelay = 1000;
  var retryTimer = null;
  var layout = null;
  var view = VIEWS.indexOf(load(VIEW_KEY)) >= 0 ? load(VIEW_KEY) : "home";
  var streaming = {};
  var gameIDs = {};   // ids of buttons and tiles that launch a game
  var gamesById = {}; // game id -> game from the layout (name, art)
  var shortcuts = [];  // the three home screen shortcut tiles from the layout

  function load(k) { try { return localStorage.getItem(k); } catch (e) { return null; } }
  function save(k, v) { try { v ? localStorage.setItem(k, v) : localStorage.removeItem(k); } catch (e) {} }

  function svg(inner, cls) {
    return '<svg viewBox="0 0 24 24"' + (cls ? ' class="' + cls + '"' : "") + ">" + inner + "</svg>";
  }

  var toastTimer = null;
  function toast(msg) {
    var el = $("toast");
    el.textContent = msg;
    el.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.classList.remove("show"); }, 2400);
  }

  // ---- Status ------------------------------------------------------------

  function setStatus(state, text) {
    var online = state === "online";
    $("status").className = "logo " + state;
    $("conn-text").textContent = text;
    $("conn").classList.toggle("show", !online && !document.body.classList.contains("pairing"));
    $("view-home").classList.toggle("disabled", !online);
    $("view-games").classList.toggle("disabled", !online);
    $("view-stats").classList.toggle("stale", !online);
  }

  // ---- Views -------------------------------------------------------------

  var tabs = Array.prototype.slice.call(document.querySelectorAll(".tab"));
  tabs.forEach(function (t) {
    t.addEventListener("click", function () { showView(t.dataset.view); });
    // Belt and braces for iOS: if a tap's click never arrives, the end of the
    // touch still switches the screen. Doing both is harmless.
    var sx = 0, sy = 0, tap = false;
    t.addEventListener("touchstart", function (e) { sx = e.touches[0].clientX; sy = e.touches[0].clientY; tap = e.touches.length === 1; }, { passive: true });
    t.addEventListener("touchmove", function (e) {
      if (Math.abs(e.touches[0].clientX - sx) > 12 || Math.abs(e.touches[0].clientY - sy) > 12) tap = false;
    }, { passive: true });
    t.addEventListener("touchend", function () { if (tap) showView(t.dataset.view); tap = false; }, { passive: true });
  });

  var padReset = null; // lets go of trackpad buttons (set by the trackpad code)

  function showView(name, remember) {
    var prev = document.querySelector(".view.active");
    var next = $("view-" + name);
    if (prev !== next) {
      if (prev) {
        prev.classList.remove("active");
        prev.classList.add("leaving");
        setTimeout(function () { prev.classList.remove("leaving"); }, 450);
      }
      next.classList.add("active");
      if (name === "stats") statsHoldUntil = Date.now() + 480;
      if (name === "watch") askBrightness();
    }
    if (name !== "pad" && padReset) padReset();
    document.body.classList.toggle("pairing", name === "pair");
    // The clock screen is a plain black screen without the rail; remember
    // where it was opened from for its back button.
    if (name === "clock" && view !== "clock" && VIEWS.indexOf(view) >= 0) beforeClock = view;
    document.body.classList.toggle("clock-mode", name === "clock");
    document.body.classList.remove("rail-peek");
    if (window.DeskClock && $("view-clock")) {
      if (name === "clock" && document.visibilityState === "visible") window.DeskClock.start();
      else window.DeskClock.stop();
    }
    if (name !== "pair") {
      view = name;
      if (remember !== false) save(VIEW_KEY, name);
      tabs.forEach(function (t, i) {
        var on = t.dataset.view === name;
        t.classList.toggle("active", on);
        if (on) $("tab-pill").style.transform = "translateY(" + (t.offsetTop - tabs[0].offsetTop) + "px)";
      });
    }
    syncStreams();
  }

  // ---- Connection --------------------------------------------------------

  // Keeping the link healthy on a phone:
  //  - a socket that is still connecting after CONNECT_TIMEOUT is given up and
  //    retried (iOS can leave one hanging right after waking up);
  //  - the page pings every PING_EVERY, and treats SILENCE without any message
  //    from the PC as a dead link (the PC also streams data and answers pings).
  var CONNECT_TIMEOUT = 5000, LAYOUT_TIMEOUT = 30000, PING_EVERY = 15000, SILENCE = 35000;
  var lastHeard = 0, cookieTried = false, sessionSynced = false;

  function connect() {
    clearTimeout(retryTimer);
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;

    // Without a token in localStorage, try once anyway: the PC may know this
    // phone from the cookie it set at pairing.
    var token = load(TOKEN_KEY) || "";
    if (!token && cookieTried) { showPairing(); return; }

    setStatus("connecting", t("connecting"));
    var sock = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws");
    ws = sock;
    streaming = {};
    // Only the handshake is timed out. Once connected, the first message (the
    // layout, with game art) may take a while on weak Wi-Fi, so it gets longer.
    var giveUp = setTimeout(function () { if (sock.readyState === WebSocket.CONNECTING) sock.close(); }, CONNECT_TIMEOUT);
    var noLayout = setTimeout(function () { if (!layout) sock.close(); }, LAYOUT_TIMEOUT);

    sock.onopen = function () {
      lastHeard = Date.now();
      sock.send(JSON.stringify({ type: "auth", token: token }));
    };
    sock.onmessage = function (ev) {
      lastHeard = Date.now();
      var msg;
      try { msg = JSON.parse(ev.data); } catch (e) { return; }
      if (msg.type === "layout") clearTimeout(noLayout);
      if (msg.type === "unauthorized" && !token) {
        // Not known by cookie either: this phone needs pairing.
        cookieTried = true;
        showPairing();
        return;
      }
      handle(msg);
    };
    sock.onclose = function (ev) {
      clearTimeout(giveUp);
      clearTimeout(noLayout);
      if (ws !== sock) return;
      ws = null;
      layout = null;
      streaming = {};
      if (!document.body.classList.contains("pairing")) scheduleRetry();
    };
  }

  setInterval(function () {
    if (!ws || ws.readyState !== WebSocket.OPEN || document.visibilityState !== "visible") return;
    if (Date.now() - lastHeard > SILENCE) { ws.close(); return; }
    send({ type: "ping" });
  }, PING_EVERY);

  // Phones paired before the PC kept a cookie get one now, so they stay
  // paired even if iOS clears the page's storage.
  function syncSession() {
    var token = load(TOKEN_KEY);
    if (sessionSynced || !token) return;
    sessionSynced = true;
    fetch("/api/session", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token: token }) })
      .catch(function () { sessionSynced = false; });
  }

  function scheduleRetry() {
    setStatus("offline", t("offline", Math.round(retryDelay / 1000)));
    retryTimer = setTimeout(connect, retryDelay);
    retryDelay = Math.min(retryDelay * 2, RETRY_MAX);
  }

  function send(obj) {
    if (ws && ws.readyState === WebSocket.OPEN) { ws.send(JSON.stringify(obj)); return true; }
    return false;
  }

  function handle(msg) {
    switch (msg.type) {
      case "layout":
        retryDelay = 1000;
        layout = msg;
        gameIDs = {};
        gamesById = {};
        shortcuts = msg.shortcuts || [];
        (msg.games || []).forEach(function (g) { gamesById[g.id] = g; });
        renderHome(msg.groups || []);
        renderGames(msg.games || []);
        setStatus("online", "Connected");
        // The games screen is only worth a tab once a game has been added on the PC.
        var hasGames = (msg.games || []).length > 0;
        tabs.forEach(function (tb) { if (tb.dataset.view === "games") tb.hidden = !hasGames; });
        if (!hasGames && view === "games") view = "home";
        showView(view, false);
        if (view === "watch") askBrightness();
        syncSession();
        break;
      case "result":
        if (gameIDs[msg.id]) {
          launchResult(msg.id, msg.ok, msg.error);
        } else if (!msg.ok) {
          flash(msg.id, "failed", 450);
          if (msg.error) toast(msg.error);
        }
        break;
      case "stats":
        if (msg.stats) queueStats(msg.stats);
        break;
      case "media":
        if (msg.media) onMedia(msg.media, msg.art);
        break;
      case "mixer":
        if (msg.mixer) renderMixer(msg.mixer, msg.icons);
        break;
      case "brightness":
        onBrightness(msg);
        break;
      case "micack":
        // Answers arrive in order, so once the last one is in, this is the truth.
        if (micPending) micPending--;
        if (!micPending) renderMic(msg.mic ? msg.mic.muted : null);
        break;
      case "unauthorized":
        save(TOKEN_KEY, null);
        showPairing(t("unpaired"));
        break;
    }
  }

  // Live data is only streamed for the screen that is showing: PC stats on the
  // stats screen, now playing on the home screen.
  function syncStreams() {
    var live = !document.body.classList.contains("pairing") &&
      document.visibilityState === "visible" && ws && ws.readyState === WebSocket.OPEN;
    var want = {
      stats: !!(live && ((view === "stats" && statsPage === 0) || (view === "home" && miniEls))),
      media: !!(live && view === "home" && np),
      mixer: !!(live && (view === "stats" || view === "watch"))
    };
    Object.keys(want).forEach(function (k) {
      if (want[k] !== !!streaming[k] && send({ type: k, on: want[k] })) streaming[k] = want[k];
    });
    clearInterval(progressTimer);
    progressTimer = want.media ? setInterval(function () { tickProgress(false); }, 1000) : null;
  }

  // iOS pans (rubber-bands) the whole page on a drag even though nothing
  // scrolls. Only the games list scrolls; everything else stays fixed.
  //
  // But a finger never lies perfectly still: a tap moves a pixel or two, and
  // on iOS cancelling that tiny move cancels the click as well. Phones with a
  // big, sensitive touch area (iPhone 15/16 Pro) then needed two or three taps.
  // So only a move past TAP_SLOP is treated as a drag; a smaller one is left
  // alone and still becomes a click.
  var TAP_SLOP = 12;
  var touchX = 0, touchY = 0, touchDragging = false;
  document.addEventListener("touchstart", function (e) {
    touchX = e.touches[0].clientX;
    touchY = e.touches[0].clientY;
    touchDragging = e.touches.length > 1;
  }, { passive: true });
  document.addEventListener("touchmove", function (e) {
    if (e.target.closest && e.target.closest(".game-list")) return;
    if (!touchDragging) {
      var t = e.touches[0];
      if (Math.abs(t.clientX - touchX) < TAP_SLOP && Math.abs(t.clientY - touchY) < TAP_SLOP) return;
      touchDragging = true;
    }
    e.preventDefault();
  }, { passive: false });

  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") { retryDelay = 1000; connect(); }
    if (view === "clock" && window.DeskClock && $("view-clock")) {
      if (document.visibilityState === "visible") window.DeskClock.start(); else window.DeskClock.stop();
    }
    syncStreams();
  });

  // On the clock screen a tap shows the back button and the face picker for a
  // few seconds. Back returns to the screen the clock was opened from.
  var peekTimer = null, beforeClock = "home";
  // The back arrow is small, so the whole top left corner counts as pressing it:
  // anyone who taps there wants to go back.
  var BACK_ZONE_W = 110, BACK_ZONE_H = 90;
  if ($("view-clock")) $("view-clock").addEventListener("click", function (e) {
    var r = $("view-clock").getBoundingClientRect();
    if (e.clientX - r.left < BACK_ZONE_W && e.clientY - r.top < BACK_ZONE_H) {
      showView(beforeClock);
      return;
    }
    document.body.classList.add("rail-peek");
    clearTimeout(peekTimer);
    peekTimer = setTimeout(function () { document.body.classList.remove("rail-peek"); }, 4000);
  });
  if ($("clock-exit")) $("clock-exit").addEventListener("click", function (e) {
    e.stopPropagation();
    showView(beforeClock);
  });
  window.addEventListener("online", function () { retryDelay = 1000; connect(); });
  // iOS can restore the page from its cache without a visibility change.
  window.addEventListener("pageshow", function () { retryDelay = 1000; connect(); });

  // ---- Controls ----------------------------------------------------------

  // control creates a pressable element with ripple feedback.
  function control(tag, cls, id, accent, onPress) {
    var el = document.createElement(tag);
    el.className = "ctl " + cls;
    if (tag === "button") el.type = "button";
    el.dataset.id = id;
    if (accent) el.style.setProperty("--a", accent);
    el.addEventListener("touchstart", function (ev) { ripple(el, ev.touches[0]); }, { passive: true });
    el.addEventListener("mousedown", function (ev) { ripple(el, ev); });
    el.addEventListener("click", onPress);
    return el;
  }

  function ripple(el, point) {
    var r = el.getBoundingClientRect();
    var dot = document.createElement("span");
    dot.className = "ripple";
    dot.style.left = point.clientX - r.left + "px";
    dot.style.top = point.clientY - r.top + "px";
    dot.addEventListener("animationend", function () { dot.remove(); });
    el.appendChild(dot);
  }

  function flash(id, cls, ms) {
    var el = document.querySelector('.ctl[data-id="' + id + '"]');
    if (!el) return;
    el.classList.remove(cls);
    void el.offsetWidth; // restart the animation if it is already running
    el.classList.add(cls);
    setTimeout(function () { el.classList.remove(cls); }, ms);
  }

  function iconOf(b) { return ICONS[b.icon]; }
  function accentOf(b) { var i = iconOf(b); return i ? i[1] : null; }

  function chip(b) {
    var i = iconOf(b);
    if (i) return '<span class="chip">' + svg(i[0]) + "</span>";
    var span = document.createElement("span");
    span.textContent = b.icon || "•";
    return '<span class="chip text">' + span.innerHTML + "</span>";
  }

  function textNode(cls, text) {
    var s = document.createElement("span");
    s.className = cls;
    s.textContent = text;
    return s;
  }

  function press(b) {
    if (b.page) { flash(b.id, "pressed", 160); showView(b.page); return; }
    if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
    if (b.confirm && !window.confirm(b.label + "?")) return;
    send({ type: "press", id: b.id });
    flash(b.id, "pressed", 160);
    // Remember which way the next track change goes, for the cover slide.
    if (np && (b.id === np.prevId || b.id === np.nextId)) {
      np.dir = b.id === np.prevId ? -1 : 1;
      np.dirAt = Date.now();
    }
  }

  // ---- Home --------------------------------------------------------------

  // Filled, rounded transport glyphs in the style of iOS (backward/forward are
  // two triangles, as in Apple Music).
  var GLYPH = {
    prev: '<path d="M11.6 6.4v11.2a1 1 0 0 1-1.6.8L2.9 12.8a1 1 0 0 1 0-1.6L10 5.6a1 1 0 0 1 1.6.8z"/><path d="M21.4 6.4v11.2a1 1 0 0 1-1.6.8l-7.1-5.6a1 1 0 0 1 0-1.6l7.1-5.6a1 1 0 0 1 1.6.8z"/>',
    next: '<path d="M2.6 6.4v11.2a1 1 0 0 0 1.6.8l7.1-5.6a1 1 0 0 0 0-1.6L4.2 5.6a1 1 0 0 0-1.6.8z"/><path d="M12.4 6.4v11.2a1 1 0 0 0 1.6.8l7.1-5.6a1 1 0 0 0 0-1.6L14 5.6a1 1 0 0 0-1.6.8z"/>',
    play: '<path d="M7 4.6v14.8a1.2 1.2 0 0 0 1.8 1l12-7.4a1.2 1.2 0 0 0 0-2l-12-7.4A1.2 1.2 0 0 0 7 4.6z"/>',
    pause: '<rect x="5.5" y="4" width="4.6" height="16" rx="1.4"/><rect x="13.9" y="4" width="4.6" height="16" rx="1.4"/>'
  };
  var np = null; // now-playing elements, set when the media card is rendered

  var renderers = {
    // media: a now-playing card. Buttons 1-3 are previous / play-pause / next,
    // button 4 (mute) sits next to the volume slider.
    media: function (body, buttons) {
      var card = body.parentNode;
      // Blurred cover behind everything: two layers so a new cover can cross-fade in.
      card.insertAdjacentHTML("afterbegin", '<div class="np-bg"><i></i><i></i></div>');
      body.insertAdjacentHTML("beforeend",
        '<div class="np idle"><div class="np-art">' + svg('<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>') +
        '<img alt=""><img alt=""></div><div class="np-meta"><span class="np-app"><i></i><b></b></span>' +
        '<span class="np-title"></span><span class="np-artist"></span></div></div>' +
        '<div class="np-progress"><div class="track"><i></i></div>' +
        '<div class="times"><span class="pos">0:00</span><span class="dur">0:00</span></div></div>');
      var transport = document.createElement("div");
      transport.className = "transport";
      var glyphs = [GLYPH.prev, GLYPH.play, GLYPH.next];
      var acts = ["prev", "toggle", "next"];
      buttons.slice(0, 3).forEach(function (b, i) {
        var el = control("button", "tbtn" + (i === 1 ? " main" : ""), b.id, null, function () { mediaControl(acts[i], b); });
        el.setAttribute("aria-label", b.label);
        el.innerHTML = svg(glyphs[i]);
        transport.appendChild(el);
      });
      body.appendChild(transport);

      // Volume: [mute] [slider] — the fourth button (if any) is the mute key.
      var vol = document.createElement("div");
      vol.className = "vol";
      var muteBtn = null;
      if (buttons[3]) {
        var mb = buttons[3];
        muteBtn = control("button", "vbtn", mb.id, null, function () { mediaControl("mute", mb); toggleMuteLocally(); });
        muteBtn.setAttribute("aria-label", mb.label);
        var mic = iconOf(mb);
        muteBtn.innerHTML = mic ? svg(mic[0]) : "";
        vol.appendChild(muteBtn);
      }
      vol.insertAdjacentHTML("beforeend",
        '<div class="slider" role="slider" aria-label="Volume" aria-valuemin="0" aria-valuemax="100">' +
        '<i></i>' + svg('<path d="M11 5 6 9H2v6h4l5 4V5z"/><path d="M15.5 8.5a5 5 0 0 1 0 7"/>', "spk") +
        '<span class="pct"></span></div>');
      body.appendChild(vol);
      bindSlider(vol.querySelector(".slider"));

      np = {
        card: card,
        root: body.querySelector(".np"),
        imgs: body.querySelectorAll(".np-art img"),
        bgs: card.querySelectorAll(".np-bg i"),
        front: 0,
        meta: body.querySelector(".np-meta"),
        app: body.querySelector(".np-app b"),
        title: body.querySelector(".np-title"),
        artist: body.querySelector(".np-artist"),
        pos: body.querySelector(".np-progress .pos"),
        dur: body.querySelector(".np-progress .dur"),
        bar: body.querySelector(".np-progress .track i"),
        play: transport.children[1],
        prevId: buttons[0] && buttons[0].id,
        nextId: buttons[2] && buttons[2].id,
        dir: 1,
        dirAt: 0,
        vol: vol,
        slider: vol.querySelector(".slider"),
        level: vol.querySelector(".slider i"),
        spk: vol.querySelector(".slider .spk"),
        pct: vol.querySelector(".pct"),
        mute: muteBtn,
        dots: body.appendChild(document.createElement("div")),
        volSource: "",
        state: null
      };
      np.dots.className = "np-dots";
      bindSwipe(card);
      if (mediaState) renderMedia();
    },
    // list: one row per button; consecutive game buttons share one row as tiles.
    list: function (body, buttons) {
      var row = null;
      buttons.forEach(function (b) {
        if (b.game && gamesById[b.game]) {
          if (!row) {
            row = document.createElement("div");
            row.className = "game-row";
            body.appendChild(row);
          }
          row.appendChild(gameTile(b, gamesById[b.game]));
          return;
        }
        row = null;
        var el = control("button", "row", b.id, accentOf(b), function () { press(b); });
        el.innerHTML = chip(b);
        el.appendChild(textNode("label", b.label));
        el.insertAdjacentHTML("beforeend", svg('<path d="m9 6 6 6-6 6"/>', "chev"));
        body.appendChild(el);
      });
    },
    // quick: [game tiles] / [icon buttons] / [CPU and GPU mini charts]
    quick: function (body, buttons) {
      var games = buttons.filter(function (b) { return b.game && gamesById[b.game]; });
      var others = buttons.filter(function (b) { return !(b.game && gamesById[b.game]); });
      if (games.length) {
        var row = document.createElement("div");
        row.className = "game-row";
        games.forEach(function (b) { row.appendChild(gameTile(b, gamesById[b.game])); });
        body.appendChild(row);
      }
      // The home screen tiles: six, in two rows of three. Each is a game, a website or a program.
      for (var r = 0; r < shortcuts.length; r += 3) body.appendChild(shortcutRow(shortcuts.slice(r, r + 3)));
      if (others.length) {
        var icons = document.createElement("div");
        icons.className = "icon-row";
        others.forEach(function (b) {
          var el = control("button", "ibtn", b.id, accentOf(b), function () { press(b); });
          el.setAttribute("aria-label", b.label);
          el.innerHTML = chip(b);
          icons.appendChild(el);
        });
        body.appendChild(icons);
      }
      var mini = document.createElement("div");
      mini.className = "mini";
      // r = 19 → circumference 119.4
      mini.innerHTML = ["cpu", "gpu"].map(function (k) {
        return '<div class="mini-item ' + k + '"><div class="donut"><svg viewBox="0 0 44 44">' +
          '<circle class="track" cx="22" cy="22" r="19"/>' +
          '<circle class="value" cx="22" cy="22" r="19" stroke-dasharray="119.4" stroke-dashoffset="119.4"/>' +
          '</svg><b>–</b></div><span class="mini-label">' + k.toUpperCase() + "</span></div>";
      }).join("");
      body.appendChild(mini);
      miniEls = {};
      ["cpu", "gpu"].forEach(function (k) {
        var item = mini.querySelector("." + k);
        miniEls[k] = { root: item, value: item.querySelector("b"), ring: item.querySelector(".value") };
      });
    },
    grid: function (body, buttons) {
      body.classList.add("grid-body");
      var cols = Math.ceil(Math.sqrt(buttons.length));
      body.style.gridTemplateColumns = "repeat(" + cols + ", 1fr)";
      buttons.forEach(function (b) {
        var el = control("button", "tile", b.id, accentOf(b), function () { press(b); });
        el.innerHTML = chip(b);
        el.appendChild(textNode("label", b.label));
        body.appendChild(el);
      });
    }
  };

  // The icon version of a game, for small square tiles: games with a cover
  // also carry a separate icon (e.g. Steam games), which is used here.
  function asIcon(g) {
    if (!g.icon) return g;
    return { id: g.id, name: g.name, image: g.icon, kind: "icon", tint: g.icon_tint || g.tint };
  }

  // A small game tile showing the game's icon, for game buttons on the home screen.
  function gameTile(b, game) {
    var g = asIcon(game);
    var img = null;
    var el = control("button", "gtile", b.id, g.tint || null, doubleTap(b.id, function () {
      startLaunch(b.id, g, img && g.kind === "icon" ? img : el);
    }));
    el.setAttribute("aria-label", b.label || g.name);
    gameIDs[b.id] = true;
    if (g.image) {
      img = document.createElement("img");
      img.className = g.kind === "cover" ? "cover" : "icon";
      img.src = g.image;
      img.alt = "";
      el.appendChild(img);
    } else {
      el.appendChild(textNode("initial", g.name.charAt(0)));
    }
    return el;
  }

  var clockEl = null;
  var miniEls = null; // CPU/GPU mini charts on the home screen, if any

  // One row of home screen tiles. A filled tile opens its game, website or
  // program on the PC; an empty one shows a "+" and says where to add it.
  function shortcutRow(tiles) {
    var row = document.createElement("div");
    row.className = "game-row shortcut-row";
    tiles.forEach(function (sc) {
      var game = sc.kind === "game" ? asIcon(gamesById[sc.game] || { id: "", name: "" }) : null;
      var filled = !!sc.name && (!game || !!gamesById[sc.game]);
      var image = game ? game.image : sc.image;
      var tint = game ? game.tint : sc.tint;
      var img = null;
      var handler;
      var el = control("button", "gtile stile" + (filled ? "" : " empty"), sc.id, tint || null, function () { handler(); });
      if (!filled) {
        handler = function () { toast(t("shortcutEmpty")); };
        el.innerHTML = svg('<path d="M12 5v14M5 12h14"/>', "plus");
      } else if (game) {
        // A game opens with the launch animation, and needs a double tap so it is not started by accident.
        gameIDs[sc.id] = true;
        handler = doubleTap(sc.id, function () { startLaunch(sc.id, game, img || el); });
      } else {
        handler = function () {
          if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
          send({ type: "press", id: sc.id });
          flash(sc.id, "pressed", 160);
        };
      }
      if (filled) {
        var brand = !game && sc.brand && window.DeskBrands && window.DeskBrands[sc.brand];
        if (brand) {
          // A well-known service (Netflix, YouTube …) is drawn with its own icon.
          el.insertAdjacentHTML("beforeend", brand);
          el.lastChild.setAttribute("class", "icon svc-icon");
        } else if (image) {
          img = document.createElement("img");
          img.className = "icon";
          img.src = image;
          img.alt = "";
          el.appendChild(img);
        } else {
          el.appendChild(textNode("initial", sc.name.charAt(0).toUpperCase()));
        }
        el.appendChild(textNode("stile-name", sc.name));
      }
      el.setAttribute("aria-label", filled ? sc.name : t("shortcutAdd"));
      row.appendChild(el);
    });
    return row;
  }

  function renderHome(groups) {
    var home = $("view-home");
    home.innerHTML = "";
    np = null;
    clockEl = null;
    miniEls = null;
    home.style.gridTemplateColumns = groups.map(function (g) {
      return g.style === "media" ? "1.55fr" : "1fr";
    }).join(" ") || "1fr";
    groups.forEach(function (g, i) {
      var card = document.createElement("div");
      card.className = "group " + g.style;
      card.style.setProperty("--i", i);
      // The first card other than the media card carries the clock as its header.
      if (!clockEl && g.style !== "media") {
        clockEl = document.createElement("div");
        clockEl.className = "home-clock";
        clockEl.innerHTML = '<span class="time"></span><span class="date"></span>';
        card.appendChild(clockEl);
        updateClock();
      } else if (g.title) {
        card.appendChild(textNode("group-title", g.title));
      }
      var body = document.createElement("div");
      body.className = "group-body";
      card.appendChild(body);
      (renderers[g.style] || renderers.grid)(body, g.buttons || []);
      home.appendChild(card);
    });
  }

  // ---- Now playing -------------------------------------------------------

  var playback = null;      // { pos, dur, playing, t } from the last media message
  var progressTimer = null;
  var volumeLockUntil = 0;  // ignore server volume briefly after a local change

  function fmt(ms) {
    var s = Math.max(0, Math.floor(ms / 1000));
    return Math.floor(s / 60) + ":" + ("0" + (s % 60)).slice(-2);
  }

  // Several apps can be playing (Spotify, a browser tab…). The card shows one
  // source at a time; swiping it switches source, and the transport, mute and
  // volume slider act on the source shown (its own volume in the Windows mixer).
  var mediaState = null;    // the last media message: { sources, current, volume, muted }
  var arts = {};            // art ID -> data URI, each sent once per connection
  var selected = null;      // ID of the source the user picked, null to follow Windows
  var pickedAt = 0;         // when the user last swiped
  var FOLLOW_AFTER = 10000; // a source that starts playing takes over unless the user swiped this recently

  function onMedia(m, art) {
    if (art) Object.keys(art).forEach(function (id) { arts[id] = art[id]; });
    var prev = mediaState;
    var list = m.sources || [];
    var wasPlaying = {};
    if (prev) (prev.sources || []).forEach(function (s) { wasPlaying[s.id] = s.playing; });
    if (selected && !list.some(function (s) { return s.id === selected; })) selected = null;
    if (Date.now() - pickedAt > FOLLOW_AFTER) {
      list.forEach(function (s) {
        if (s.playing && !wasPlaying[s.id] && prev && s.id !== selected) pickSource(s.id, false);
      });
    }
    // Forget covers no source shows any more.
    var keep = {};
    list.forEach(function (s) { if (s.art_id && arts[s.art_id]) keep[s.art_id] = arts[s.art_id]; });
    arts = keep;
    mediaState = m;
    renderMedia();
  }

  // shownSource is the picked source, else one that is playing (Windows' own
  // choice first), else Windows' choice, else the first.
  function shownSource() {
    var list = mediaState && mediaState.sources || [];
    var by = function (f) { for (var i = 0; i < list.length; i++) if (f(list[i])) return list[i]; return null; };
    var cur = mediaState && mediaState.current;
    return by(function (s) { return s.id === selected; }) ||
      by(function (s) { return s.playing && s.id === cur; }) ||
      by(function (s) { return s.playing; }) ||
      by(function (s) { return s.id === cur; }) || list[0] || null;
  }

  // pickSource shows a source; the cover slides in from the side it is on.
  function pickSource(id, byUser) {
    var list = mediaState && mediaState.sources || [];
    var from = shownSource(), a = -1, b = -1;
    for (var i = 0; i < list.length; i++) {
      if (from && list[i].id === from.id) a = i;
      if (list[i].id === id) b = i;
    }
    if (np && a >= 0 && b >= 0 && a !== b) { np.dir = b > a ? 1 : -1; np.dirAt = Date.now(); }
    selected = id;
    if (byUser) pickedAt = Date.now();
  }

  function swipeSource(step) {
    var list = mediaState && mediaState.sources || [];
    var cur = shownSource();
    if (list.length < 2 || !cur) return;
    var i = 0;
    while (i < list.length && list[i].id !== cur.id) i++;
    var next = list[(i + step + list.length) % list.length];
    pickSource(next.id, true);
    volumeLockUntil = 0;
    renderMedia(true);
  }

  // slideSource switches source with the card's top part (cover and titles)
  // sliding out the way the finger went and the next one sliding in; the
  // progress bar, buttons and slider stay put.
  function slideSource(step) {
    var r = np.root;
    r.style.transition = "transform .2s cubic-bezier(.4,0,1,1), opacity .2s ease-in";
    r.style.transform = "translateX(" + (-step * 70) + "px)";
    r.style.opacity = "0";
    setTimeout(function () {
      if (!np || np.root !== r) return;
      swipeSource(step);
      r.style.transition = "none";
      r.style.transform = "translateX(" + (step * 70) + "px)";
      void r.offsetWidth;
      r.style.transition = "transform .55s cubic-bezier(.22,1,.36,1), opacity .4s ease-out";
      r.style.transform = "";
      r.style.opacity = "";
    }, 200);
  }

  // Transport and mute act on the source shown; without one they fall back to
  // the configured buttons (media keys).
  function mediaControl(action, b) {
    var src = shownSource();
    if (!src || (action === "mute" && src.volume < 0)) { if (b) press(b); return false; }
    if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return false; }
    send({ type: "mediactl", source: src.id, action: action });
    if (b) flash(b.id, "pressed", 160);
    if (np && (action === "prev" || action === "next")) { np.dir = action === "prev" ? -1 : 1; np.dirAt = Date.now(); }
    return true;
  }

  // renderMedia shows the source on the card. instant skips the track-change
  // animations (used while the card itself slides to another source).
  function renderMedia(instant) {
    if (!np || !mediaState) return;
    var m = shownSource() || {};
    var list = mediaState.sources || [];
    var art = m.art_id ? arts[m.art_id] : null;
    var has = !!m.title;
    var sameSource = np.lastSource === m.id;
    np.lastSource = m.id;
    if (!has) showArt(null);
    else if (art) showArt(art, instant);
    // A new track's title often arrives before its cover: keep the old cover
    // until then instead of flashing the empty frame.
    else if (np.lastArt && sameSource) revealArt(true);
    else showArt(null);
    renderDots(list, m.id);
    np.root.classList.toggle("idle", !has);
    np.root.classList.toggle("playing", has && m.playing);

    var app = has ? (m.app || t("media")) + (m.playing ? "" : " · " + t("paused")) : t("notPlaying");
    var title = has ? m.title : t("nothingPlaying");
    var artist = has ? (m.artist || m.album || "") : t("startMusic");
    if (instant) {
      clearTimeout(np.textTimer);
      np.meta.classList.remove("out", "in");
      np.title.textContent = title;
      np.artist.textContent = artist;
    } else if (title !== np.title.textContent || artist !== np.artist.textContent) {
      // New track: the old text lifts away, then the new text rises into place.
      var meta = np.meta, first = !np.title.textContent;
      clearTimeout(np.textTimer);
      meta.classList.add("out");
      np.textTimer = setTimeout(function () {
        np.title.textContent = title;
        np.artist.textContent = artist;
        meta.classList.remove("out");
        meta.classList.add("in");
        void meta.offsetWidth;
        meta.classList.remove("in");
      }, first ? 0 : 230);
    }
    np.app.textContent = app;
    np.play.innerHTML = svg(has && m.playing ? GLYPH.pause : GLYPH.play);

    playback = has ? { pos: m.position_ms, dur: m.duration_ms, playing: m.playing, t: Date.now() } : null;
    tickProgress(true);
    // The slider shows the app's own volume, or the PC's if the app is not in the mixer.
    var own = m.id && m.volume >= 0;
    np.volSource = own ? m.id : "";
    np.slider.classList.toggle("app", !!own);
    var v = own ? m.volume : mediaState.volume, muted = own ? m.muted : mediaState.muted;
    if (v >= 0 && Date.now() > volumeLockUntil) setVolume(v, muted);
  }

  function renderDots(list, id) {
    var html = "";
    if (list.length > 1) list.forEach(function (s) { html += '<i class="' + (s.id === id ? "on" : "") + '"></i>'; });
    if (np.dots.innerHTML !== html) np.dots.innerHTML = html;
  }

  // Horizontal swipes on the card switch source (the volume slider keeps its
  // own drag). The cover and titles follow the finger; with a single source
  // they only give a little and spring back.
  function bindSwipe(el) {
    var x0 = null, y0 = 0, axis = null, dx = 0;
    function many() { return !!(mediaState && mediaState.sources && mediaState.sources.length > 1); }
    el.addEventListener("touchstart", function (e) {
      x0 = e.target.closest && e.target.closest(".slider") ? null : e.touches[0].clientX;
      y0 = e.touches[0].clientY;
      axis = null;
      dx = 0;
    }, { passive: true });
    el.addEventListener("touchmove", function (e) {
      if (x0 === null || !np) return;
      dx = e.touches[0].clientX - x0;
      var dy = e.touches[0].clientY - y0;
      if (!axis && (Math.abs(dx) > 8 || Math.abs(dy) > 8)) axis = Math.abs(dx) > Math.abs(dy) ? "x" : "y";
      if (axis !== "x") return;
      e.preventDefault();
      var r = np.root;
      r.style.transition = "none";
      r.style.transform = "translateX(" + (dx * (many() ? 0.45 : 0.12)).toFixed(1) + "px)";
      r.style.opacity = many() ? Math.max(0.3, 1 - Math.abs(dx) / 400).toFixed(2) : "";
    }, { passive: false });
    function end() {
      if (x0 === null) return;
      x0 = null;
      if (axis !== "x" || !np) return;
      if (many() && Math.abs(dx) > 45) { slideSource(dx < 0 ? 1 : -1); return; }
      var r = np.root;
      r.style.transition = "transform .45s cubic-bezier(.22,1,.36,1), opacity .3s ease-out";
      r.style.transform = "";
      r.style.opacity = "";
    }
    el.addEventListener("touchend", end);
    el.addEventListener("touchcancel", end);
  }

  // Moves the progress bar on the phone between server updates.
  function tickProgress(jump) {
    if (!np) return;
    var p = 0, d = 0;
    if (playback) {
      d = playback.dur;
      p = Math.min(d, playback.pos + (playback.playing ? Date.now() - playback.t : 0));
    }
    np.pos.textContent = fmt(p);
    // Remaining time, as in Apple Music.
    np.dur.textContent = d > 0 ? "-" + fmt(d - p) : "0:00";
    np.bar.style.transition = jump || !playback || !playback.playing ? "none" : "transform 1s linear";
    // Slide the fill (not scale it) so its rounded end keeps its shape.
    np.bar.style.transform = "translateX(" + (d > 0 ? (p / d * 100 - 100).toFixed(2) : -100) + "%)";
  }

  // Speaker glyph for a volume level: muted, then 0, 1 or 2 sound waves.
  var SPEAKER = '<path d="M11 5 6 9H2v6h4l5 4V5z"/>';
  var WAVE1 = '<path d="M15.5 8.5a5 5 0 0 1 0 7"/>';
  var WAVE2 = '<path d="M19 5a10 10 0 0 1 0 14"/>';
  function speakerFor(v, muted) {
    if (muted || v === 0) return SPEAKER + '<path d="m22 9-6 6M16 9l6 6"/>';
    return SPEAKER + (v > 33 ? WAVE1 : "") + (v > 66 ? WAVE2 : "");
  }

  function setVolume(v, muted) {
    np.state = { volume: v, muted: muted };
    np.level.style.transform = "translateX(" + (v - 100) + "%)";
    var glyph = speakerFor(v, muted);
    if (np.spk.innerHTML !== glyph) np.spk.innerHTML = glyph;
    np.pct.textContent = v;
    np.slider.setAttribute("aria-valuenow", v);
    np.vol.classList.toggle("muted", muted);
    if (np.mute) np.mute.classList.toggle("on", muted);
  }

  function toggleMuteLocally() {
    if (!np || !np.state) return;
    volumeLockUntil = Date.now() + 1500;
    setVolume(np.state.volume, !np.state.muted);
  }

  // Drag (or tap) anywhere on the slider to set the volume. While dragging,
  // the level is sent at most every 80 ms, plus once more on release.
  function bindSlider(el) {
    var dragging = false, lastSent = 0, pending = null, timer = null;

    function levelAt(x) {
      var r = el.getBoundingClientRect();
      return Math.round(Math.max(0, Math.min(1, (x - r.left) / r.width)) * 100);
    }
    function push(v, force) {
      pending = v;
      var wait = 80 - (Date.now() - lastSent);
      if (force || wait <= 0) {
        clearTimeout(timer);
        timer = null;
        lastSent = Date.now();
        var msg = { type: "volume", level: pending };
        if (np && np.volSource) msg.source = np.volSource;
        send(msg);
      } else if (!timer) {
        timer = setTimeout(function () { timer = null; push(pending, true); }, wait);
      }
    }
    function move(x, force) {
      if (!np) return;
      var v = levelAt(x);
      volumeLockUntil = Date.now() + 1500;
      setVolume(v, np.state ? np.state.muted && v === 0 : false);
      push(v, force);
    }
    function start(x) { dragging = true; el.classList.add("dragging"); move(x); }
    function end(x) {
      if (!dragging) return;
      dragging = false;
      el.classList.remove("dragging");
      if (x !== undefined) move(x, true); else if (pending !== null) push(pending, true);
    }

    el.addEventListener("touchstart", function (e) { e.preventDefault(); start(e.touches[0].clientX); }, { passive: false });
    el.addEventListener("touchmove", function (e) { e.preventDefault(); if (dragging) move(e.touches[0].clientX); }, { passive: false });
    el.addEventListener("touchend", function () { end(); });
    el.addEventListener("touchcancel", function () { end(); });
    el.addEventListener("mousedown", function (e) {
      start(e.clientX);
      function mm(ev) { move(ev.clientX); }
      function mu(ev) { document.removeEventListener("mousemove", mm); document.removeEventListener("mouseup", mu); end(ev.clientX); }
      document.addEventListener("mousemove", mm);
      document.addEventListener("mouseup", mu);
    });
  }

  // ---- Volume mixer (second page of game mode) ----------------------------

  var statsPage = 0;      // 0: PC stats, 1: volume mixer
  var mixIcons = {};      // mixer app ID -> icon data URI, each sent once per connection
  var mixLock = {};       // app ID -> ignore server levels until this time (after a local change)
  var micMuted = null;    // null without a microphone
  var micPending = 0;     // micmute requests the PC has not answered yet
  var micPendingAt = 0;
  // The mixer is drawn twice: in full on its own page, and small (the PC plus
  // the first few programs, those playing sound first) on the stats page.
  var mixPanels = [
    { row: "mix-row", limit: 8, mini: false, cols: {}, order: "" },
    { row: "mini-mix", limit: 4, mini: true, cols: {}, order: "" },
    { row: "watch-vol", limit: 1, mini: false, single: true, cols: {}, order: null }
  ];
  var lastMixerApps = null; // the PC and its programs from the last mixer message

  function showStatsPage(p) {
    statsPage = p;
    $("stats-pager").classList.toggle("on-mixer", p === 1);
    var dots = $("stats-dots").children;
    for (var i = 0; i < dots.length; i++) dots[i].classList.toggle("on", i === p);
    syncStreams();
  }

  (function bindStatsPager() {
    var el = $("view-stats"), x0 = null, y0 = 0;
    el.addEventListener("touchstart", function (e) {
      x0 = e.target.closest && e.target.closest(".vslider") ? null : e.touches[0].clientX;
      y0 = e.touches[0].clientY;
    }, { passive: true });
    el.addEventListener("touchend", function (e) {
      if (x0 === null) return;
      var dx = e.changedTouches[0].clientX - x0, dy = e.changedTouches[0].clientY - y0;
      x0 = null;
      if (Math.abs(dx) > 45 && Math.abs(dx) > Math.abs(dy) * 1.5) showStatsPage(dx < 0 ? 1 : 0);
    });
    Array.prototype.forEach.call($("stats-dots").children, function (d, i) {
      d.addEventListener("click", function () { showStatsPage(i); });
    });
    $("mic-btn").addEventListener("click", function () {
      if (micMuted === null) return;
      if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
      // Ask for a state, not a flip, so a second tap or a slow link cannot undo
      // the first. Until the PC answers, mixer updates (which may predate the
      // change) are ignored for the microphone.
      var want = !micMuted;
      send({ type: "micmute", on: want });
      micPending++;
      micPendingAt = Date.now();
      renderMic(want);
    });
  })();

  // ---- Watch mode: the PC's volume, play/pause, jump back and forward,
  // screen brightness and a shut down button ----------------------------------

  var brightCol = null;   // the brightness slider, once the PC says the screen can be controlled
  var brightLock = 0;     // ignore the PC's level until then (after a local change)

  function askBrightness() { send({ type: "brightness" }); }

  function onBrightness(msg) {
    var wrap = $("watch-bright");
    if (typeof msg.brightness !== "number") { wrap.hidden = true; return; } // this screen cannot be controlled
    if (!brightCol) {
      var root = document.createElement("div");
      root.className = "mix bright";
      root.innerHTML =
        '<div class="mix-icon">' + svg('<circle cx="12" cy="12" r="4"/><path d="M12 2.5v2.5M12 19v2.5M2.5 12H5M19 12h2.5M5.3 5.3l1.8 1.8M16.9 16.9l1.8 1.8M5.3 18.7l1.8-1.8M16.9 7.1l1.8-1.8"/>') + "</div>" +
        '<span class="mix-name"></span>' +
        '<div class="vslider" role="slider" aria-valuemin="0" aria-valuemax="100"><i></i><span class="pct"></span></div>';
      root.querySelector(".mix-name").textContent = t("brightness");
      brightCol = { fill: root.querySelector(".vslider i"), pct: root.querySelector(".pct"), slider: root.querySelector(".vslider") };
      brightCol.slider.setAttribute("aria-label", t("brightness"));
      bindVSlider(brightCol.slider, function (v) {
        brightLock = Date.now() + 2500;
        showBrightness(v);
        return { type: "brightness", level: v };
      });
      $("watch-bright-row").appendChild(root);
    }
    wrap.hidden = false;
    if (!(brightLock > Date.now())) showBrightness(msg.brightness);
  }

  function showBrightness(v) {
    brightCol.fill.style.transform = "translateY(" + (100 - v) + "%)";
    brightCol.pct.textContent = v;
    brightCol.slider.classList.toggle("low", v < 12);
    brightCol.slider.setAttribute("aria-valuenow", v);
  }

  (function bindWatch() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-watch]"), function (b) {
      b.addEventListener("click", function () {
        if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
        send({ type: "watch", action: b.dataset.watch });
        b.classList.add("pressed");
        setTimeout(function () { b.classList.remove("pressed"); }, 160);
      });
    });
    $("watch-power").addEventListener("click", function () {
      if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
      if (window.confirm(t("shutdownQ"))) send({ type: "press", id: "shutdown" });
    });
  })();

  function renderMic(muted) {
    micMuted = muted;
    var b = $("mic-btn");
    b.classList.toggle("muted", muted === true);
    b.classList.toggle("none", muted === null);
    b.querySelector("span").textContent = muted === null ? t("noMic") : t("mic");
  }

  function renderMixer(m, icons) {
    if (icons) Object.keys(icons).forEach(function (id) { mixIcons[id] = icons[id]; });
    var apps = [{ id: "", name: t("master"), volume: m.volume, muted: m.muted, active: true, master: true }].concat(m.apps || []);
    lastMixerApps = apps;
    mixPanels.forEach(function (p) { renderMixPanel(p, apps.slice(0, p.limit)); });
    // A mixer update may have been sent before our last tap reached the PC; wait for the answer (but not forever).
    if (micPending && Date.now() - micPendingAt > 4000) micPending = 0;
    if (!micPending) renderMic(m.mic ? m.mic.muted : null);
  }

  function renderMixPanel(p, apps) {
    var order = apps.map(function (a) { return a.id; }).join("\n");
    var row = $(p.row);
    if (!row) return; // this panel is not on screen (e.g. the home screen has game tiles)
    if (order !== p.order) {
      p.order = order;
      p.cols = {};
      row.innerHTML = "";
      apps.forEach(function (a) { row.appendChild(mixColumn(a, p)); });
      if (apps.length === 1 && !p.mini && !p.single) row.insertAdjacentHTML("beforeend", '<p class="mix-empty">' + t("noApps") + "</p>");
    }
    apps.forEach(function (a) {
      var col = p.cols[a.id];
      if (!col) return;
      col.name.textContent = a.name;
      col.root.classList.toggle("active", !!a.active);
      if (!col.icon.firstChild && mixIcons[a.id]) col.icon.innerHTML = '<img alt="" src="' + mixIcons[a.id] + '">';
      if (a.volume >= 0 && !(mixLock[a.id] > Date.now())) setMixLevel(col, a.volume, a.muted);
    });
  }

  // eachCol runs f for an app's column in every panel that shows it.
  function eachCol(id, f) {
    mixPanels.forEach(function (p) { if (p.cols[id]) f(p.cols[id]); });
  }

  function mixColumn(a, panel) {
    var root = document.createElement("div");
    root.className = "mix" + (panel.mini ? " mini" : "") + (a.master ? " master" : "") + (a.game ? " is-game" : "");
    root.innerHTML =
      '<div class="mix-icon"></div>' +
      '<span class="mix-name"></span>' +
      (a.game && !panel.mini ? '<span class="mix-tag">' + t("game") + "</span>" : "") +
      '<div class="vslider" role="slider" aria-valuemin="0" aria-valuemax="100"><i></i><span class="pct"></span></div>' +
      (panel.mini ? "" : '<button type="button" class="mix-mute" aria-label="Mute"></button>');
    var col = {
      root: root,
      icon: root.querySelector(".mix-icon"),
      name: root.querySelector(".mix-name"),
      slider: root.querySelector(".vslider"),
      fill: root.querySelector(".vslider i"),
      pct: root.querySelector(".vslider .pct"),
      mute: root.querySelector(".mix-mute"),
      level: 0,
      muted: false
    };
    if (a.master) col.icon.innerHTML = svg('<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>');
    col.slider.setAttribute("aria-label", a.name);
    panel.cols[a.id] = col;
    bindVSlider(col.slider, function (v) {
      mixLock[a.id] = Date.now() + 1500;
      eachCol(a.id, function (c) { setMixLevel(c, v, c.muted && v === 0); });
      var msg = { type: "volume", level: v };
      if (a.id) msg.app = a.id;
      return msg;
    });
    if (col.mute) col.mute.addEventListener("click", function () {
      if (!ws || ws.readyState !== WebSocket.OPEN) { toast(t("notConnected")); return; }
      send(a.id ? { type: "mute", app: a.id } : { type: "mute" });
      mixLock[a.id] = Date.now() + 1500;
      var muted = !col.muted;
      eachCol(a.id, function (c) { setMixLevel(c, c.level, muted); });
    });
    return root;
  }
  function setMixLevel(col, v, muted) {
    col.level = v;
    col.muted = muted;
    col.fill.style.transform = "translateY(" + (100 - v) + "%)";
    col.pct.textContent = v;
    col.slider.classList.toggle("low", v < 12);
    col.slider.setAttribute("aria-valuenow", v);
    col.root.classList.toggle("muted", muted);
    var glyph = speakerFor(v, muted);
    if (col.mute && col.glyph !== glyph) { col.glyph = glyph; col.mute.innerHTML = svg(glyph); }
  }

  // A vertical slider: drag (or tap) to set the level. update(v) shows the
  // level and returns the message to send, at most every 80 ms plus once on release.
  function bindVSlider(el, update) {
    var dragging = false, lastSent = 0, pending = null, timer = null;
    function levelAt(y) {
      var r = el.getBoundingClientRect();
      return Math.round(Math.max(0, Math.min(1, (r.bottom - y) / r.height)) * 100);
    }
    function push(msg, force) {
      pending = msg;
      var wait = 80 - (Date.now() - lastSent);
      if (force || wait <= 0) {
        clearTimeout(timer);
        timer = null;
        lastSent = Date.now();
        send(pending);
      } else if (!timer) {
        timer = setTimeout(function () { timer = null; push(pending, true); }, wait);
      }
    }
    function move(y, force) { push(update(levelAt(y)), force); }
    function start(y) { dragging = true; el.classList.add("dragging"); move(y); }
    function end(y) {
      if (!dragging) return;
      dragging = false;
      el.classList.remove("dragging");
      if (y !== undefined) move(y, true); else if (pending) push(pending, true);
    }
    el.addEventListener("touchstart", function (e) { e.preventDefault(); start(e.touches[0].clientY); }, { passive: false });
    el.addEventListener("touchmove", function (e) { e.preventDefault(); if (dragging) move(e.touches[0].clientY); }, { passive: false });
    el.addEventListener("touchend", function () { end(); });
    el.addEventListener("touchcancel", function () { end(); });
    el.addEventListener("mousedown", function (e) {
      start(e.clientY);
      function mm(ev) { move(ev.clientY); }
      function mu(ev) { document.removeEventListener("mousemove", mm); document.removeEventListener("mouseup", mu); end(ev.clientY); }
      document.addEventListener("mousemove", mm);
      document.addEventListener("mouseup", mu);
    });
  }

  // ---- Trackpad -------------------------------------------------------------
  //
  // One finger moves the pointer (faster swipes go further), a tap clicks, two
  // fingers right-click (or scroll when they slide), three fingers middle-click.
  // Tap, then touch again quickly and hold: a drag. The two glowing areas at the
  // bottom are the left and right buttons; they can be held with one finger
  // while another moves the pointer.

  (function trackpad() {
    var el = $("view-pad"), hint = $("pad-hint");
    var TAP_MS = 300, DOUBLE_MS = 320, DEAD = 4, TAP_MOVE = 10;
    var touches = {};   // surface touch id -> { x, y, sx, sy, t, armed }
    var zones = {};     // zone touch id -> zone element
    var g = null;       // the current surface gesture
    var lastTap = 0;    // when a lone finger last tapped (for tap-and-drag)
    var pend = { x: 0, y: 0, sy: 0, sx: 0 }, raf = 0;

    function pad(msg) {
      msg.type = "mouse";
      return send(msg);
    }

    function count() { return Object.keys(touches).length; }

    function flush() {
      raf = 0;
      var dx = Math.trunc(pend.x), dy = Math.trunc(pend.y);
      if (dx || dy) { pend.x -= dx; pend.y -= dy; pad({ action: "move", dx: dx, dy: dy }); }
      var sy = Math.trunc(pend.sy), sx = Math.trunc(pend.sx);
      if (sy) { pend.sy -= sy; pad({ action: "scroll", dy: sy }); }
      if (sx) { pend.sx -= sx; pad({ action: "hscroll", dy: sx }); }
    }
    function queue() { if (!raf) raf = requestAnimationFrame(flush); }

    function ripple(x, y, rgb) {
      var r = document.createElement("i");
      r.className = "pad-ripple";
      var box = el.getBoundingClientRect();
      r.style.left = (x - box.left) + "px";
      r.style.top = (y - box.top) + "px";
      if (rgb) r.style.setProperty("--c", rgb);
      el.appendChild(r);
      setTimeout(function () { if (r.parentNode) r.parentNode.removeChild(r); }, 600);
    }

    el.addEventListener("touchstart", function (e) {
      if (!hint.classList.contains("gone")) hint.classList.add("gone");
      for (var i = 0; i < e.changedTouches.length; i++) {
        var t = e.changedTouches[i];
        var zone = t.target.closest && t.target.closest(".pad-zone");
        if (zone) {
          zones[t.identifier] = zone;
          zone.classList.add("down");
          pad({ action: "down", button: zone.dataset.btn });
          continue;
        }
        var now = Date.now();
        touches[t.identifier] = { x: t.clientX, y: t.clientY, sx: t.clientX, sy: t.clientY, t: now, armed: false };
        if (!g) {
          g = { t0: now, maxN: 0, moved: false, drag: false };
          // Touching again right after a tap and holding: drag with the left button.
          if (now - lastTap < DOUBLE_MS) { g.drag = true; pad({ action: "down", button: "left" }); }
        }
        g.maxN = Math.max(g.maxN, count());
      }
    }, { passive: true });

    el.addEventListener("touchmove", function (e) {
      if (!g) return;
      var n = count();
      for (var i = 0; i < e.changedTouches.length; i++) {
        var t = e.changedTouches[i], p = touches[t.identifier];
        if (!p) continue;
        if (Math.hypot(t.clientX - p.sx, t.clientY - p.sy) > TAP_MOVE) g.moved = true;
        // A small dead zone, so a tap does not nudge the pointer.
        if (!p.armed) {
          if (Math.hypot(t.clientX - p.sx, t.clientY - p.sy) < DEAD) continue;
          p.armed = true;
          p.x = t.clientX; p.y = t.clientY; p.t = Date.now();
          continue;
        }
        var dx = t.clientX - p.x, dy = t.clientY - p.y, now = Date.now();
        var dt = Math.max(8, now - p.t);
        p.x = t.clientX; p.y = t.clientY; p.t = now;
        if (n === 1 && (g.maxN === 1 || g.drag)) {
          // Faster swipes get more reach, as on a laptop trackpad.
          var gain = 2 + Math.min(Math.hypot(dx, dy) / dt * 1.4, 3);
          pend.x += dx * gain;
          pend.y += dy * gain;
        } else if (n === 2 && !g.drag) {
          pend.sy += dy / 2 * 5;      // content follows the fingers
          pend.sx += -dx / 2 * 5;
        }
      }
      queue();
    }, { passive: true });

    function lift(e, cancelled) {
      for (var i = 0; i < e.changedTouches.length; i++) {
        var t = e.changedTouches[i];
        if (zones[t.identifier]) {
          var z = zones[t.identifier];
          delete zones[t.identifier];
          z.classList.remove("down");
          pad({ action: "up", button: z.dataset.btn });
          continue;
        }
        if (!touches[t.identifier]) continue;
        delete touches[t.identifier];
        if (count() > 0 || !g) continue;
        // The last finger is up: a quick, still touch is a tap.
        flush();
        var now = Date.now(), tap = !cancelled && !g.moved && !g.drag && now - g.t0 < TAP_MS;
        if (tap) {
          var btn = g.maxN === 1 ? "left" : g.maxN === 2 ? "right" : "middle";
          pad({ action: "click", button: btn });
          ripple(t.clientX, t.clientY, btn === "right" ? "167, 139, 250" : btn === "middle" ? "52, 211, 153" : null);
          lastTap = g.maxN === 1 ? now : 0;
        } else {
          lastTap = 0;
        }
        if (g.drag) pad({ action: "up", button: "left" });
        g = null;
      }
    }
    el.addEventListener("touchend", function (e) { lift(e, false); }, { passive: true });
    el.addEventListener("touchcancel", function (e) { lift(e, true); }, { passive: true });

    // Let go of everything: leaving the screen, locking the phone, losing the PC.
    padReset = function () {
      Object.keys(zones).forEach(function (id) {
        zones[id].classList.remove("down");
        pad({ action: "up", button: zones[id].dataset.btn });
      });
      if (g && g.drag) pad({ action: "up", button: "left" });
      zones = {};
      touches = {};
      g = null;
      pend.x = pend.y = pend.sx = pend.sy = 0;
    };
    document.addEventListener("visibilitychange", function () { if (document.visibilityState !== "visible") padReset(); });
  })();

  // ---- Clock -------------------------------------------------------------

  function updateClock() {
    if (!clockEl) return;
    var now = new Date();
    clockEl.firstChild.textContent = now.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    clockEl.lastChild.textContent = now.toLocaleDateString([], { weekday: "long", day: "numeric", month: "long" });
  }

  // Tick on the minute rather than every second.
  (function scheduleClock() {
    var now = new Date();
    setTimeout(function () { updateClock(); scheduleClock(); }, 60000 - now.getSeconds() * 1000 - now.getMilliseconds() + 50);
  })();

  // Cover art: the new cover loads into the hidden layer, then both the art
  // and the color backdrop cross-fade. src null hides the art.
  function showArt(src, instant) {
    if (!np) return;
    if (!src) { revealArt(false); return; }
    if (src === np.lastArt) { revealArt(true); return; }
    np.lastArt = src;
    var back = 1 - np.front, img = np.imgs[back], old = np.imgs[np.front];
    if (instant) {
      // Swap without the slide; the old cover stays until the new one has loaded.
      img.onload = function () {
        paintGlow(back, img);
        np.front = back;
        old.classList.add("instant");
        img.classList.add("instant");
        img.classList.remove("gone");
        revealArt(true);
        void img.offsetWidth;
        old.classList.remove("instant");
        img.classList.remove("instant");
      };
      img.src = src;
      return;
    }
    // Slide direction: "previous" pressed in the last few seconds brings the
    // new cover in from the left, anything else (next, autoplay) from the right.
    var dir = np.dirAt && Date.now() - np.dirAt < 8000 ? np.dir : 1;
    np.dirAt = 0;
    // Park the incoming cover on its entry side without animating there.
    img.classList.add("instant");
    img.classList.remove("shown", "gone");
    img.style.setProperty("--dir", dir);
    void img.offsetWidth;
    img.classList.remove("instant");
    img.onload = function () {
      if (old.classList.contains("shown")) {
        old.style.setProperty("--dir", dir);
        old.classList.add("gone");
      }
      paintGlow(back, img);
      np.front = back;
      revealArt(true);
    };
    img.src = src;
  }

  // Two soft glows in the cover's colors from opposite corners: the card
  // takes on the mood of the cover without showing a smeared copy of it.
  function paintGlow(layer, img) {
    var c = palette(img);
    if (!c) return;
    np.bgs[layer].style.background =
      "radial-gradient(90% 85% at 0% 0%, rgba(" + c[0] + ",.55), transparent 62%)," +
      "radial-gradient(80% 90% at 100% 100%, rgba(" + c[1] + ",.42), transparent 60%)," +
      "linear-gradient(160deg, rgba(" + c[0] + ",.14), rgba(" + c[1] + ",.06))";
  }

  function revealArt(on) {
    np.imgs[0].parentNode.classList.toggle("has-art", on);
    for (var i = 0; i < 2; i++) {
      var show = on && i === np.front;
      np.imgs[i].classList.toggle("shown", show);
      if (show || !on) np.imgs[i].classList.remove("gone");
      np.bgs[i].classList.toggle("shown", show);
    }
  }

  // The cover's two main colors as "r,g,b": the most vivid one, and the most
  // vivid one with a clearly different hue (or a deeper shade of the first).
  function palette(img) {
    try {
      var c = document.createElement("canvas");
      c.width = c.height = 16;
      var ctx = c.getContext("2d");
      ctx.drawImage(img, 0, 0, 16, 16);
      var d = ctx.getImageData(0, 0, 16, 16).data, px = [];
      for (var i = 0; i < d.length; i += 4) {
        var r = d[i], g = d[i + 1], b = d[i + 2];
        var max = Math.max(r, g, b), min = Math.min(r, g, b);
        var sat = max ? (max - min) / max : 0;
        px.push({ r: r, g: g, b: b, hue: hue(r, g, b, max, min), score: sat * (max / 255) + max / 2550 });
      }
      px.sort(function (a, b) { return b.score - a.score; });
      var first = px[0];
      var second = null;
      for (var j = 1; j < px.length && !second; j++) {
        var dh = Math.abs(px[j].hue - first.hue);
        if (Math.min(dh, 360 - dh) > 40 && px[j].score > 0.15) second = px[j];
      }
      var fmtc = function (p, k) { return Math.round(p.r * k) + "," + Math.round(p.g * k) + "," + Math.round(p.b * k); };
      return [fmtc(first, 1), second ? fmtc(second, 1) : fmtc(first, 0.55)];
    } catch (e) {
      return null;
    }
  }

  function hue(r, g, b, max, min) {
    if (max === min) return 0;
    var h = max === r ? (g - b) / (max - min) : max === g ? 2 + (b - r) / (max - min) : 4 + (r - g) / (max - min);
    return (h * 60 + 360) % 360;
  }

  // ---- Games -------------------------------------------------------------

  var PLAY = '<path d="M7 4v16l13-8z"/>';

  function renderGames(games) {
    var list = $("game-list");
    list.innerHTML = "";
    $("games-empty").hidden = games.length > 0;
    games.forEach(function (g, i) {
      gameIDs[g.id] = true;
      var img = null;
      var el = control("button", "game", g.id, g.tint || null, doubleTap(g.id, function () {
        // Fly from the picture itself: the whole card for covers, the icon otherwise.
        startLaunch(g.id, g, img && g.kind !== "cover" ? img : el);
      }));
      el.style.setProperty("--i", i);
      if (g.image) {
        img = document.createElement("img");
        img.className = g.kind === "cover" ? "cover" : "icon";
        img.src = g.image;
        img.alt = "";
        el.appendChild(img);
      } else {
        el.appendChild(textNode("initial", g.name.charAt(0)));
      }
      var info = document.createElement("span");
      info.className = "info";
      info.appendChild(textNode("name", g.name));
      info.insertAdjacentHTML("beforeend", '<span class="play">' + svg(PLAY) + t("play") + "</span>");
      el.appendChild(info);
      list.appendChild(el);
    });
  }

  // ---- Launch animation --------------------------------------------------
  //
  // Pressing a game: its art flies from the tile to the centre of the screen,
  // the PC confirms the launch, and the overlay hands over to the stats screen.

  var launching = null; // { id, t0, timers[] }

  // Starting a game takes two quick taps, so a stray touch or a hand resting
  // on the screen never opens one. The first tap only says so.
  var DOUBLE_TAP_MS = 500;
  function doubleTap(id, start) {
    var first = 0;
    return function () {
      var now = Date.now();
      if (first && now - first < DOUBLE_TAP_MS) {
        first = 0;
        flash(id, "pressed", 180);
        start();
        return;
      }
      first = now;
      toast(t("doubleTap"));
    };
  }

  function startLaunch(id, g, fromEl) {
    if (launching) return;
    if (!send({ type: "press", id: id })) { toast(t("notConnected")); return; }

    var ov = $("launch"), art = $("launch-art");
    ov.className = "launch";
    ov.style.setProperty("--a", g.tint || "120,130,150");
    $("launch-name").textContent = g.name;
    setLaunchStatus(t("launching"));

    // Measure the overlay itself (it is laid out even while hidden); window
    // sizes can disagree with it on iOS while the toolbar animates.
    var box = ov.getBoundingClientRect();
    var cover = g.image && g.kind === "cover";
    var h = Math.min(box.height * 0.5, 190);
    var w = cover ? h * 2 / 3 : h * 0.62;
    if (!cover) h = w;
    art.className = "launch-art" + (cover ? "" : " icon");
    art.innerHTML = "";
    if (g.image) {
      var img = document.createElement("img");
      img.src = g.image;
      img.alt = "";
      art.appendChild(img);
    }
    art.style.width = w + "px";
    art.style.height = h + "px";
    var cx = box.width / 2, cy = box.height * 0.4;
    // The art is centred with CSS (top 40%, left 50%) and the text is placed
    // relative to that same 40%, so both stay aligned if the viewport resizes.
    art.style.margin = (-h / 2) + "px 0 0 " + (-w / 2) + "px";
    ov.querySelector(".launch-info").style.top = "calc(40% + " + Math.round(h / 2 + 18) + "px)";

    // FLIP: start exactly over the tile, then animate to the centre.
    var r = fromEl.getBoundingClientRect();
    var s = Math.min(r.width / w, r.height / h);
    art.style.transition = "none";
    art.style.transform = "translate(" + (r.left + r.width / 2 - cx) + "px," + (r.top + r.height / 2 - cy) + "px) scale(" + s.toFixed(3) + ")";
    void art.offsetWidth;
    ov.classList.add("show");
    requestAnimationFrame(function () {
      art.style.transition = "transform .65s cubic-bezier(.2, .8, .2, 1)";
      art.style.transform = "none";
      ov.classList.add("run");
    });

    launching = { id: id, t0: Date.now(), timers: [] };
    // If the PC never answers, do not leave the overlay up.
    later(4000, function () { launchFailed(t("noAnswer")); });
  }

  function later(ms, f) {
    if (launching) launching.timers.push(setTimeout(f, ms));
  }

  function setLaunchStatus(text) {
    var el = $("launch-status");
    el.classList.add("swap");
    setTimeout(function () { el.textContent = text; el.classList.remove("swap"); }, el.textContent ? 180 : 0);
  }

  function endLaunch(delay) {
    var l = launching;
    if (!l) return;
    l.timers.forEach(clearTimeout);
    launching = null;
    var ov = $("launch");
    setTimeout(function () {
      ov.classList.add("leave");
      setTimeout(function () { ov.className = "launch"; }, 460);
    }, delay);
  }

  // Called with the PC's answer to a game press.
  function launchResult(id, ok, error) {
    if (!launching || launching.id !== id) return;
    if (!ok) { launchFailed(error === "already_running" ? t("alreadyRunning") : error || t("launchFailed")); return; }
    var wait = Math.max(0, launching.t0 + 1000 - Date.now()); // let the art land first
    var l = launching;
    l.timers.forEach(clearTimeout);
    l.timers = [];
    later(wait, function () {
      setLaunchStatus(t("toGameMode"));
      $("launch").classList.add("done");
    });
    later(wait + STATS_AFTER_LAUNCH - 700, function () { showView("stats"); });
    later(wait + STATS_AFTER_LAUNCH - 600, function () { endLaunch(0); });
  }

  function launchFailed(msg) {
    if (!launching) return;
    $("launch").classList.add("fail");
    setLaunchStatus(msg);
    endLaunch(1600);
  }

  // ---- Stats -------------------------------------------------------------

  // A 270° arc: radius 42, circumference 263.9, visible length 197.9.
  var ARC = 197.9;
  var GAUGES = [
    { key: "cpu", name: "CPU", unit: "%", color: "var(--cyan)", warn: 70, hot: 90 },
    { key: "gpu", name: "GPU", unit: "%", color: "var(--violet)", warn: 85, hot: 97 },
    { key: "temp", name: "GPU TEMP", unit: "°C", color: "var(--green)", warn: 70, hot: 83 },
    { key: "ram", name: "RAM", unit: "%", color: "var(--pink)", warn: 75, hot: 90 }
  ];
  var gaugeEls = {};

  function buildGauges() {
    var root = $("gauges");
    GAUGES.forEach(function (g, i) {
      var el = document.createElement("div");
      el.className = "gauge";
      el.style.setProperty("--i", i);
      el.style.setProperty("--base", g.color);
      el.innerHTML =
        '<div class="ring"><svg viewBox="0 0 100 100">' +
        '<g transform="rotate(135 50 50)">' +
        '<circle class="track" cx="50" cy="50" r="42" stroke-dasharray="197.9 263.9"/>' +
        '<circle class="glow" cx="50" cy="50" r="42" stroke-dasharray="197.9 263.9" stroke-dashoffset="197.9"/>' +
        '<circle class="value" cx="50" cy="50" r="42" stroke-dasharray="197.9 263.9" stroke-dashoffset="197.9"/>' +
        "</g></svg>" +
        '<div class="center"><span class="num">0</span><span class="unit">' + g.unit + "</span>" +
        '<span class="sub"></span></div></div>' +
        '<div class="name">' + g.name + "</div>";
      root.appendChild(el);
      gaugeEls[g.key] = {
        root: el,
        arcs: el.querySelectorAll(".glow, .value"),
        num: el.querySelector(".num"),
        sub: el.querySelector(".sub"),
        def: g
      };
    });
  }

  function setGauge(key, value, sub) {
    var g = gaugeEls[key];
    var na = value === null;
    g.root.classList.toggle("na", na);
    var v = na ? 0 : Math.max(0, Math.min(100, value));
    var offset = (ARC * (1 - v / 100)).toFixed(1);
    for (var i = 0; i < g.arcs.length; i++) g.arcs[i].setAttribute("stroke-dashoffset", offset);
    var d = g.def;
    g.root.classList.toggle("hot", !na && v >= d.hot);
    g.root.classList.toggle("warn", !na && v >= d.warn && v < d.hot);
    if (na) g.num.textContent = "–"; else tween(g.num, v, 0);
    g.sub.textContent = sub || "";
  }

  // Counts a number up or down with an ease-out curve.
  function tween(el, to, decimals) {
    var from = parseFloat(el.dataset.v || "0");
    el.dataset.v = to;
    if (from === to) { el.textContent = to.toFixed(decimals); return; }
    var id = (el._tween || 0) + 1;
    el._tween = id;
    var start = null;
    function step(ts) {
      if (el._tween !== id) return;
      if (start === null) start = ts;
      var k = Math.min((ts - start) / 700, 1);
      k = 1 - Math.pow(1 - k, 3);
      el.textContent = (from + (to - from) * k).toFixed(decimals);
      if (k < 1) requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }

  // While the stats screen slides in, hold readings back: animating the gauges
  // during the slide is what made it stutter on the iPhone 6s.
  var statsHoldUntil = 0, pendingStats = null, holdTimer = null;

  function queueStats(s) {
    if (view !== "stats") { renderStats(s); return; }
    var wait = statsHoldUntil - Date.now();
    if (wait <= 0) { renderStats(s); return; }
    pendingStats = s;
    if (holdTimer) return;
    holdTimer = setTimeout(function () {
      holdTimer = null;
      if (pendingStats) { renderStats(pendingStats); pendingStats = null; }
    }, wait);
  }

  function gb(mb) { return (mb / 1024).toFixed(1); }

  // Home screen rings: current usage as a filled share of the circle.
  function drawMini(key, value) {
    var m = miniEls && miniEls[key];
    if (!m) return;
    var na = value === null, v = na ? 0 : Math.max(0, Math.min(100, value));
    m.root.classList.toggle("na", na);
    m.root.classList.toggle("hot", v >= 90);
    m.value.textContent = na ? "–" : Math.round(v) + "%";
    m.ring.setAttribute("stroke-dashoffset", (119.4 * (1 - v / 100)).toFixed(1));
  }

  function renderStats(s) {
    var g = s.gpu;
    drawMini("cpu", s.cpu.usage);
    drawMini("gpu", g ? g.usage : null);
    // On the home screen only the mini charts are visible; skip the rest.
    if (view !== "stats") return;

    setGauge("cpu", s.cpu.usage);
    setGauge("ram", s.ram.usage, gb(s.ram.used_mb) + " / " + gb(s.ram.total_mb) + " GB");

    $("no-gpu").hidden = !!g;
    $("details").style.opacity = g ? "" : ".35";
    if (!g) {
      setGauge("gpu", null);
      setGauge("temp", null);
      return;
    }
    $("gpu-name").textContent = g.name.replace(/^NVIDIA\s+/, "");
    setGauge("gpu", g.usage);
    setGauge("temp", g.temp);

    tween($("fan-val"), g.fan, 0);
    $("fan-sub").textContent = g.fan > 0 ? t("gpuFan") : t("gpuFanIdle");
    var fan = $("fan");
    fan.classList.toggle("spinning", g.fan > 0);
    // Faster fan, faster spin: 1.6 s per turn at idle speed down to 0.25 s at 100%.
    fan.firstChild.style.animationDuration = (1.6 - 1.35 * Math.min(g.fan, 100) / 100).toFixed(2) + "s";

    tween($("clock-val"), g.clock_mhz, 0);
    $("vram-text").textContent = gb(g.vram_used_mb) + " / " + gb(g.vram_total_mb) + " GB";
    $("vram-bar").style.transform = "scaleX(" + (g.vram_used_mb / g.vram_total_mb).toFixed(3) + ")";
    $("power-text").textContent = Math.round(g.power_w) + " / " + Math.round(g.power_limit_w) + " W";
    $("power-bar").style.transform = "scaleX(" + Math.min(1, g.power_w / g.power_limit_w).toFixed(3) + ")";
  }

  // ---- Pairing -----------------------------------------------------------

  // Pairing: scan the QR code on the PC (the page opens with #pair=<code> and
  // pairs by itself) or type the 6-digit code, which changes every 30 seconds.
  //
  // iOS gives a home screen app its own storage, separate from Safari. So after
  // pairing in Safari the page puts a single-use handoff code in its URL
  // (#h=<code>); "Add to Home Screen" saves that URL, and the home screen app
  // trades the code for the same token on first launch.

  var standalone = !!(navigator.standalone || (window.matchMedia && window.matchMedia("(display-mode: standalone)").matches));
  var isiOS = /iPhone|iPad|iPod/.test(navigator.userAgent);

  function showPairing(err) {
    showView("pair");
    setStatus("", t("notPaired"));
    $("pair-error").textContent = err || "";
  }

  function deviceName() {
    var ua = navigator.userAgent;
    return /iPad/.test(ua) ? "iPad" : /iPhone/.test(ua) ? "iPhone" : /Android/.test(ua) ? "Android" : "Device";
  }

  function setHash(h) {
    try { history.replaceState(null, "", location.pathname + location.search + (h ? "#" + h : "")); } catch (e) {}
  }

  var PAIR_ERRORS = { bad_code: "badCode", too_many: "tooMany", expired_link: "expiredLink" };

  var pairing = false;

  function pair(body) {
    // One request at a time: typing the sixth digit and tapping Pair would
    // otherwise pair the phone twice.
    if (pairing) return Promise.resolve();
    pairing = true;
    $("pair-error").textContent = "";
    return fetch("/api/pair", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }).then(function (r) {
      return r.json().then(function (res) { return { ok: r.ok, body: res }; });
    }).then(function (res) {
      if (!res.ok) {
        showPairing(t(PAIR_ERRORS[res.body.error] || "pairFailed"));
        return;
      }
      save(TOKEN_KEY, res.body.token);
      $("pin").value = "";
      $("pin").blur();
      if (res.body.handoff && isiOS && !standalone) {
        // Keep the handoff in the URL for "Add to Home Screen", and say so.
        setHash("h=" + res.body.handoff);
        $("a2hs").hidden = false;
      } else {
        setHash("");
      }
      connect();
    }).catch(function () {
      showPairing(t("unreachable"));
    }).then(function () { pairing = false; });
  }

  function submitCode() {
    var code = $("pin").value.replace(/\D/g, "");
    if (code.length !== 6) { $("pair-error").textContent = t("codeDigits"); return; }
    pair({ code: code, name: deviceName() });
  }

  $("pair-form").addEventListener("submit", function (ev) { ev.preventDefault(); submitCode(); });
  // Send as soon as the sixth digit is in.
  $("pin").addEventListener("input", function () {
    if ($("pin").value.replace(/\D/g, "").length === 6) submitCode();
  });
  $("a2hs").addEventListener("click", function () { $("a2hs").hidden = true; });

  // First thing on load: a QR code (#pair=) or a home screen handoff (#h=).
  function startup() {
    var m = /[#&]pair=([a-z2-7]{16})/.exec(location.hash);
    // A home screen app can be saved with the QR link in its URL (iOS keeps the
    // address the page was opened with). That code has long expired; such an
    // app is already paired by its token or cookie, so just connect.
    if (m && (standalone || load(TOKEN_KEY))) {
      setHash("");
      m = null;
    }
    if (m) {
      setHash("");
      pair({ code: m[1], name: deviceName() });
      return;
    }
    m = /[#&]h=([0-9a-f]{32})/.exec(location.hash);
    if (m && !load(TOKEN_KEY)) {
      setHash("");
      pair({ handoff: m[1] });
      return;
    }
    if (m && standalone) setHash("");
    connect();
  }

  // ---- Screen care -------------------------------------------------------

  // Pixel shift: every few minutes the whole UI drifts by a pixel or two, slowly
  // enough to be invisible, so static parts never sit on exactly the same pixels.
  var SHIFTS = [[0, 0], [1, 0], [1, 1], [0, 1], [-1, 1], [-1, 0], [-1, -1], [0, -1], [1, -1], [2, 0], [0, 2], [-2, 0], [0, -2]];
  var shiftIndex = 0;
  setInterval(function () {
    shiftIndex = (shiftIndex + 1) % SHIFTS.length;
    var s = SHIFTS[shiftIndex];
    document.body.style.transform = s[0] || s[1] ? "translate(" + s[0] + "px," + s[1] + "px)" : "";
  }, SHIFT_EVERY);

  // ---- Viewport ----------------------------------------------------------

  // Tablets: the layout is sized for a phone in landscape (about 667×375 CSS
  // px). On a bigger touch screen it is scaled up through the viewport, so the
  // page lays out at about 760 CSS px wide and every control grows with it,
  // instead of phone-sized controls floating in large, empty cards. Touch
  // coordinates stay in CSS px, so nothing else has to know about it.
  var viewportMeta = document.querySelector('meta[name="viewport"]');
  var viewportBase = viewportMeta ? viewportMeta.getAttribute("content") : "";
  var TABLET_WIDTH = 760, TABLET_HEIGHT = 440, TABLET_MAX_SCALE = 1.6;
  function fitViewport() {
    if (!viewportMeta) return;
    var sw = screen.width, sh = screen.height;
    var touch = navigator.maxTouchPoints > 0 || "ontouchstart" in window;
    var content = viewportBase;
    if (touch && Math.min(sw, sh) >= 600) {
      // iOS keeps screen.width/height in portrait terms; Android swaps them.
      var landscape = screen.orientation && screen.orientation.type ?
        screen.orientation.type.indexOf("landscape") === 0 :
        typeof window.orientation === "number" ? Math.abs(window.orientation) === 90 : sw > sh;
      var w = landscape ? Math.max(sw, sh) : Math.min(sw, sh);
      var h = landscape ? Math.min(sw, sh) : Math.max(sw, sh);
      var s = Math.min(w / TABLET_WIDTH, h / TABLET_HEIGHT, TABLET_MAX_SCALE);
      if (s > 1.05) {
        s = Math.round(s * 100) / 100;
        content = "width=" + Math.round(w / s) + ", initial-scale=" + s + ", minimum-scale=" + s +
          ", maximum-scale=" + s + ", user-scalable=no, viewport-fit=cover";
      }
    }
    if (viewportMeta.getAttribute("content") !== content) viewportMeta.setAttribute("content", content);
  }

  // 100vh is wrong in iOS Safari while the toolbar is visible; use the real height.
  function fixHeight() {
    document.documentElement.style.setProperty("--vh", window.innerHeight / 100 + "px");
  }
  window.addEventListener("resize", fixHeight);
  window.addEventListener("orientationchange", function () {
    fitViewport();
    setTimeout(fixHeight, 250);
  });

  fitViewport();
  buildGauges();
  fixHeight();
  showView(view, false);
  // Enable transitions only after the first paint so nothing animates in from the wrong place.
  requestAnimationFrame(function () {
    requestAnimationFrame(function () { document.body.classList.remove("booting"); });
  });
  startup();
})();
