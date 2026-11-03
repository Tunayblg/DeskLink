// DeskLink panel. Plain ES2017 so it runs on iOS 15 Safari (the last iOS for iPhone 6s).
(function () {
  "use strict";

  var TOKEN_KEY = "desklink.token";
  var VIEW_KEY = "desklink.view";
  var VIEWS = ["home", "games", "stats", "clock"];
  var RETRY_MAX = 10000;
  var HISTORY = 60;
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
      gameMode: "GAME MODE", load60: "LOAD · 60s", gpuFan: "GPU FAN", gpuFanIdle: "GPU FAN · IDLE",
      coreClock: "CORE CLOCK", noGpu: "No NVIDIA GPU found. GPU stats are only available for NVIDIA cards for now."
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
      gameMode: "OYUN MODU", load60: "YÜK · 60 sn", gpuFan: "GPU FANI", gpuFanIdle: "GPU FANI · DURDU",
      coreClock: "ÇEKİRDEK HIZI", noGpu: "NVIDIA ekran kartı bulunamadı. GPU verileri şimdilik sadece NVIDIA kartlarda var."
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
  });

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
    }
    document.body.classList.toggle("pairing", name === "pair");
    // The clock screen is a plain black screen: the rail hides until a tap.
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
        if (on) $("tab-pill").style.transform = "translateY(" + i * 52 + "px)";
      });
    }
    syncStreams();
  }

  // ---- Connection --------------------------------------------------------

  function connect() {
    clearTimeout(retryTimer);
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;

    var token = load(TOKEN_KEY);
    if (!token) { showPairing(); return; }

    setStatus("connecting", t("connecting"));
    var sock = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws");
    ws = sock;
    streaming = {};

    sock.onopen = function () { sock.send(JSON.stringify({ type: "auth", token: token })); };
    sock.onmessage = function (ev) {
      var msg;
      try { msg = JSON.parse(ev.data); } catch (e) { return; }
      handle(msg);
    };
    sock.onclose = function () {
      if (ws !== sock) return;
      ws = null;
      streaming = {};
      if (load(TOKEN_KEY)) scheduleRetry();
    };
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
        (msg.games || []).forEach(function (g) { gamesById[g.id] = g; });
        renderHome(msg.groups || []);
        renderGames(msg.games || []);
        setStatus("online", "Connected");
        showView(view, false);
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
        if (msg.media) renderMedia(msg.media, msg.art);
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
      stats: !!(live && (view === "stats" || (view === "home" && miniEls))),
      media: !!(live && view === "home" && np)
    };
    Object.keys(want).forEach(function (k) {
      if (want[k] !== !!streaming[k] && send({ type: k, on: want[k] })) streaming[k] = want[k];
    });
    clearInterval(progressTimer);
    progressTimer = want.media ? setInterval(function () { tickProgress(false); }, 1000) : null;
  }

  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") { retryDelay = 1000; connect(); }
    if (view === "clock" && window.DeskClock && $("view-clock")) {
      if (document.visibilityState === "visible") window.DeskClock.start(); else window.DeskClock.stop();
    }
    syncStreams();
  });

  // On the clock screen a tap brings back the rail and the face picker for a few seconds.
  var peekTimer = null;
  if ($("view-clock")) $("view-clock").addEventListener("click", function () {
    document.body.classList.add("rail-peek");
    clearTimeout(peekTimer);
    peekTimer = setTimeout(function () { document.body.classList.remove("rail-peek"); }, 4000);
  });
  window.addEventListener("online", function () { retryDelay = 1000; connect(); });

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
      buttons.slice(0, 3).forEach(function (b, i) {
        var el = control("button", "tbtn" + (i === 1 ? " main" : ""), b.id, null, function () { press(b); });
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
        muteBtn = control("button", "vbtn", mb.id, null, function () { press(mb); toggleMuteLocally(); });
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
        vol: vol,
        slider: vol.querySelector(".slider"),
        level: vol.querySelector(".slider i"),
        spk: vol.querySelector(".slider .spk"),
        pct: vol.querySelector(".pct"),
        mute: muteBtn,
        state: null
      };
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
    var el = control("button", "gtile", b.id, g.tint || null, function () {
      flash(b.id, "pressed", 180);
      startLaunch(b.id, g, img && g.kind === "icon" ? img : el);
    });
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

  function renderMedia(m, art) {
    if (!np) return;
    var has = !!m.title;
    if (!has) showArt(null);
    else if (art) showArt(art);
    else if (np.lastArt) revealArt(true);
    np.root.classList.toggle("idle", !has);
    np.root.classList.toggle("playing", has && m.playing);

    var app = has ? (m.app || t("media")) + (m.playing ? "" : " · " + t("paused")) : t("notPlaying");
    var title = has ? m.title : t("nothingPlaying");
    var artist = has ? (m.artist || m.album || "") : t("startMusic");
    if (title !== np.title.textContent || artist !== np.artist.textContent) {
      // New track: fade the text out, swap it, fade back in.
      np.meta.classList.add("swap");
      setTimeout(function () {
        np.title.textContent = title;
        np.artist.textContent = artist;
        np.meta.classList.remove("swap");
      }, np.title.textContent ? 200 : 0);
    }
    np.app.textContent = app;
    np.play.innerHTML = svg(has && m.playing ? GLYPH.pause : GLYPH.play);

    playback = has ? { pos: m.position_ms, dur: m.duration_ms, playing: m.playing, t: Date.now() } : null;
    tickProgress(true);
    if (m.volume >= 0 && Date.now() > volumeLockUntil) setVolume(m.volume, m.muted);
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
        send({ type: "volume", level: pending });
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
  function showArt(src) {
    if (!np) return;
    if (!src) { revealArt(false); return; }
    if (src === np.lastArt) { revealArt(true); return; }
    np.lastArt = src;
    var back = 1 - np.front, img = np.imgs[back];
    img.onload = function () {
      var c = palette(img);
      if (c) {
        // Two soft glows in the cover's colors from opposite corners: the card
        // takes on the mood of the cover without showing a smeared copy of it.
        np.bgs[back].style.background =
          "radial-gradient(90% 85% at 0% 0%, rgba(" + c[0] + ",.55), transparent 62%)," +
          "radial-gradient(80% 90% at 100% 100%, rgba(" + c[1] + ",.42), transparent 60%)," +
          "linear-gradient(160deg, rgba(" + c[0] + ",.14), rgba(" + c[1] + ",.06))";
      }
      np.front = back;
      revealArt(true);
    };
    img.src = src;
  }

  function revealArt(on) {
    for (var i = 0; i < 2; i++) {
      var show = on && i === np.front;
      np.imgs[i].classList.toggle("shown", show);
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
      var el = control("button", "game", g.id, g.tint || null, function () {
        flash(g.id, "pressed", 180);
        // Fly from the picture itself: the whole card for covers, the icon otherwise.
        startLaunch(g.id, g, img && g.kind !== "cover" ? img : el);
      });
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
    if (!ok) { launchFailed(error || t("launchFailed")); return; }
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

  var history = { cpu: [], gpu: [] };

  function pushHistory(key, v) {
    var h = history[key];
    h.push(v);
    if (h.length > HISTORY) h.shift();
  }

  function drawSeries(key) {
    var h = history[key];
    if (h.length < 2) return;
    var step = 300 / (HISTORY - 1);
    var x0 = 300 - (h.length - 1) * step;
    var pts = h.map(function (v, i) {
      return (x0 + i * step).toFixed(1) + " " + (98 - v * 0.94).toFixed(1);
    });
    $("line-" + key).setAttribute("d", "M" + pts.join("L"));
    $("area-" + key).setAttribute("d", "M" + x0.toFixed(1) + " 100L" + pts.join("L") + "L300 100Z");
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
    pushHistory("cpu", s.cpu.usage);
    if (g) pushHistory("gpu", g.usage);
    drawMini("cpu", s.cpu.usage);
    drawMini("gpu", g ? g.usage : null);
    // On the home screen only the mini charts are visible; skip the rest.
    if (view !== "stats") return;

    setGauge("cpu", s.cpu.usage);
    setGauge("ram", s.ram.usage, gb(s.ram.used_mb) + " / " + gb(s.ram.total_mb) + " GB");
    drawSeries("cpu");

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
    drawSeries("gpu");

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

  function pair(body) {
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
    });
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

  // 100vh is wrong in iOS Safari while the toolbar is visible; use the real height.
  function fixHeight() {
    document.documentElement.style.setProperty("--vh", window.innerHeight / 100 + "px");
  }
  window.addEventListener("resize", fixHeight);
  window.addEventListener("orientationchange", function () { setTimeout(fixHeight, 250); });

  buildGauges();
  fixHeight();
  showView(view, false);
  // Enable transitions only after the first paint so nothing animates in from the wrong place.
  requestAnimationFrame(function () {
    requestAnimationFrame(function () { document.body.classList.remove("booting"); });
  });
  startup();
})();
