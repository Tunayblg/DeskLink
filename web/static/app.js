// DeskLink panel. Plain ES2017 so it runs on iOS 15 Safari (the last iOS for iPhone 6s).
(function () {
  "use strict";

  // ---- Sideways fallback ---------------------------------------------------
  //
  // iOS 15 sometimes opens the home screen app in portrait although the phone
  // is held sideways, and only turning the phone upright and back fixes it. A
  // page cannot make iOS turn its window, so instead the panel runs in a frame
  // the size of a landscape screen, turned 90 degrees. Inside the frame
  // everything (layout, swipes, sliders) sees a normal landscape screen.
  // It starts from the "rotate your phone" screen, and after that whenever
  // the app opens upright. Turning the phone for real leaves it.
  var ROT_KEY = "desklink.rotate";
  var framed = window.parent !== window;
  // The turn direction is remembered per physical position of the phone, so the
  // sideways position it was last flipped in is not mixed up with upright.
  function rotKey() { return ROT_KEY + "." + (window.orientation || 0); }
  function rotPref() { try { return localStorage.getItem(rotKey()) || localStorage.getItem(ROT_KEY); } catch (e) { return null; } }
  function orientReport(what) {
    if (!window.DeskReport) return;
    window.DeskReport("orientation " + what + ": inner " + window.innerWidth + "x" + window.innerHeight +
      ", screen " + screen.width + "x" + screen.height + ", window.orientation " + window.orientation +
      ", saved " + rotPref() + (framed ? ", in frame" : ""), "orientation");
  }
  function startFramed() {
    var force = false;
    try { force = sessionStorage.getItem(ROT_KEY) === "now"; sessionStorage.removeItem(ROT_KEY); } catch (e) { /* no storage */ }
    var deg = rotPref();
    if (framed || !deg) return false;
    var upright = window.innerHeight > window.innerWidth;
    // A window taller than wide is the stuck-in-portrait case, whether the
    // phone is held upright or already sideways.
    if (!force && !upright) return false;

    var body = document.body;
    body.innerHTML = "";
    body.className = "rot-host";
    var frame = document.createElement("iframe");
    frame.className = "rot-frame";
    frame.setAttribute("scrolling", "no");
    frame.src = location.href;
    var flip = document.createElement("button");
    flip.type = "button";
    flip.className = "rot-flip";
    flip.setAttribute("aria-label", "Flip");
    flip.innerHTML = '<svg viewBox="0 0 24 24"><path d="M3 12a9 9 0 0 1 15.5-6.2L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-15.5 6.2L3 16"/><path d="M3 21v-5h5"/></svg>';
    // iOS keeps its status bar (clock, battery) along the top of the upright
    // window, and a page cannot hide it. The frame starts below it so the bar
    // sits beside the panel instead of over it.
    var probe = document.createElement("div");
    probe.style.paddingTop = "env(safe-area-inset-top)";
    body.appendChild(probe);
    var bar = parseFloat(getComputedStyle(probe).paddingTop) || (navigator.standalone ? 20 : 0);
    body.removeChild(probe);
    // With the keyboard up, the visible part of the (upright) window is shorter:
    // the frame is made as wide as that part, so the panel is squeezed into what
    // can be seen instead of hiding behind the keyboard. The panel tells us
    // when a text box takes the keyboard.
    var kbOpen = false;
    var vv = window.visualViewport;
    function place() {
      var visible = kbOpen && vv ? vv.height : window.innerHeight;
      var w = visible - bar, h = window.innerWidth;
      frame.style.top = (bar + (kbOpen && vv ? vv.offsetTop : 0)) + "px";
      frame.style.width = w + "px";
      frame.style.height = h + "px";
      frame.style.transform = deg === "-90" ? "rotate(-90deg) translateX(-100%)" : "rotate(90deg) translateY(-100%)";
    }
    flip.addEventListener("click", function () {
      deg = deg === "-90" ? "90" : "-90";
      try { localStorage.setItem(ROT_KEY, deg); localStorage.setItem(rotKey(), deg); } catch (e) { /* no storage */ }
      place();
    });
    function check() {
      // The window really turned to landscape: run the panel normally again.
      if (window.innerWidth > window.innerHeight) { location.reload(); return; }
      place();
    }
    window.addEventListener("resize", check);
    window.addEventListener("orientationchange", function () { setTimeout(check, 300); });
    window.addEventListener("message", function (e) {
      if (e.origin !== location.origin || !e.data || e.data.type !== "kb") return;
      kbOpen = !!e.data.on;
      place();
      if (!kbOpen) window.scrollTo(0, 0);
    });
    if (vv) {
      vv.addEventListener("resize", place);
      vv.addEventListener("scroll", place);
    }
    place();
    body.appendChild(frame);
    body.appendChild(flip);
    setTimeout(function () { orientReport("framed"); }, 1500);
    return true;
  }
  if (startFramed()) return;
  // Inside the sideways frame the panel is landscape by design, even while the
  // keyboard makes the frame narrow: never ask to rotate the phone there.
  if (framed) document.documentElement.classList.add("framed");

  var TOKEN_KEY = "desklink.token";
  var VIEW_KEY = "desklink.view";
  var VIEWS = ["home", "games", "stats", "watch", "dev", "pad", "clock", "settings"];
  var PREFS_KEY = "desklink.prefs";
  var pageVersion = ""; // the panel version this page was loaded with
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
      rotate: "Rotate your phone", rotateGo: "Show sideways", noGames: "No games yet. Add them to config.json on your PC.",
      notPlaying: "Not playing", nothingPlaying: "Nothing playing", startMusic: "Start music on your PC",
      paused: "Paused", media: "Media", play: "PLAY", pinMedia: "Pin this player", unpinMedia: "Unpin player",
      plan: "PLAN", planNext: "NEXT", planDone: "All done", planAfter: "Then {}", planTapAdd: "Tap to set an alarm", planLate: "Overdue",
      planTitle: "Alarms", planName: "Name (optional)", planTime: "Remind at", planDate: "Date", planToday: "Today", planTomorrow: "Tomorrow", planEvery: "Every day", planAdd: "Add", planNextAlarm: "NEXT ALARM", sec: "s", planSet: "Set alarm", planSetDone: "Set", planActive: "{} active", planDoneTag: "Done", planClose: "Close", planEmpty: "No alarms yet. Set one on the left.", planDel: "Delete", planNow: "Now!", planNeed: "Set a time or write a name", planToggle: "Complete / undo",
      clFive: "5 HOURS", clSeven: "7 DAYS", clLeft: "{} left", d: "d", h: "h", m: "m", clWindow: "5 HOURS", clToday: "TODAY", clWeek: "7 DAYS", clNone: "Claude Code not found on this PC", clHint: "Open Claude Code to see limits",
      launching: "Launching…", toGameMode: "Switching to game mode",
      noAnswer: "No answer from the PC", launchFailed: "Could not start the game",
      alreadyRunning: "Already running", doubleTap: "Double-tap to start",
      shortcutEmpty: "Add a shortcut from DeskLink on your PC", shortcutAdd: "Add shortcut",
      devMode: "DEVELOPER", devLost: "Another window came to the front. Live typing stopped.", setLive: "Live typing", devFront: "In front", devSendTo: "Send to", devGone: "That window is closed", devNoFocus: "Could not bring that window forward", devRefused: "Windows blocked typing there (an administrator window?)", devNoWindows: "No Claude, Codex, terminal or editor window is open", devLoading: "Loading…", devPlaceholder: "Write or dictate a prompt for Claude, Codex or any tool", devListening: "Listening… speak now",
      devSend: "Send", devType: "Type only", devSent: "Sent to the PC", devTyped: "Typed on the PC", devEmpty: "Nothing to send", devOffline: "Not connected to the PC",
      devWorking: "thinking…", devReady: "ready", devIdle: "done", devDone: "Agent finished", devNotDev: "The window in front is not a developer app. Pick one from the list.", doShell: "running a command", doEdit: "editing files", doRead: "reading code", doWeb: "searching the web", doAgent: "running a helper", doPlan: "planning", doAsk: "waiting for you", devFive: "5h", devSeven: "7d",
      devMicHelp: "Dictation is probably off. On iPhone: Settings › General › Keyboard › Enable Dictation. Then tap the prompt box and use the mic key on the keyboard.",
      devMicNet: "Dictation needs an internet connection.", devMicQuiet: "Did not hear anything. Try again.",
      settings: "SETTINGS", setHint: "Saved on this phone", setTabs: "Tabs", setDev: "Developer mode", setGeneral: "General",
      tabHome: "Home", tabGames: "Games", tabStats: "Stats", tabWatch: "Watch mode", tabDev: "Developer", tabPad: "Trackpad", tabClock: "Clock",
      setActivity: "Show what Claude / Codex is doing", setUsage: "Show usage", setAutosend: "Send after dictation", setLang: "Dictation language", setCalm: "Calm mode", setReset: "Reset tabs and settings", setResetDone: "Reset",
      setUp: "Move up", setDown: "Move down",
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
      rotate: "Telefonu yan çevir", rotateGo: "Yatay göster", noGames: "Henüz oyun yok. Bilgisayardaki config.json dosyasına ekle.",
      notPlaying: "Çalmıyor", nothingPlaying: "Şu an bir şey çalmıyor", startMusic: "Bilgisayarda müzik başlat",
      paused: "Duraklatıldı", media: "Medya", play: "OYNA", pinMedia: "Bu oynatıcıyı sabitle", unpinMedia: "Sabitlemeyi kaldır",
      plan: "PLAN", planNext: "SIRADA", planDone: "Hepsi tamam", planAfter: "Sonra {}", planTapAdd: "Alarm kurmak için dokun", planLate: "Gecikti",
      planTitle: "Alarmlar", planName: "İsim (isteğe bağlı)", planTime: "Hatırlat", planDate: "Tarih", planToday: "Bugün", planTomorrow: "Yarın", planEvery: "Her gün", planAdd: "Ekle", planNextAlarm: "SIRADAKİ ALARM", sec: "sn", planSet: "Alarm kur", planSetDone: "Kuruldu", planActive: "{} aktif", planDoneTag: "Tamamlandı", planClose: "Kapat", planEmpty: "Henüz alarm yok. Soldan kur.", planDel: "Sil", planNow: "Şimdi!", planNeed: "Bir saat seç ya da isim yaz", planToggle: "Tamamla / geri al",
      clFive: "5 SAAT", clSeven: "7 GÜN", clLeft: "{} kaldı", d: "g", h: "sa", m: "dk", clWindow: "5 SAAT", clToday: "BUGÜN", clWeek: "7 GÜN", clNone: "Bu PC'de Claude Code bulunamadı", clHint: "Limitler için Claude Code'u aç",
      launching: "Başlatılıyor…", toGameMode: "Oyun moduna geçiliyor",
      noAnswer: "Bilgisayardan yanıt gelmedi", launchFailed: "Oyun başlatılamadı",
      alreadyRunning: "Zaten çalışıyor", doubleTap: "Açmak için çift dokun",
      shortcutEmpty: "Bilgisayardaki DeskLink'ten kısayol ekle", shortcutAdd: "Kısayol ekle",
      devMode: "GELİŞTİRİCİ", devLost: "Başka pencere öne geçti, canlı yazma durdu", setLive: "Canlı yazma", devFront: "Öndeki pencere", devSendTo: "Gönderilecek yer", devGone: "O pencere kapanmış", devNoFocus: "O pencere öne getirilemedi", devRefused: "Windows oraya yazmayı engelledi (yönetici penceresi mi?)", devNoWindows: "Açık Claude, Codex, terminal ya da editör penceresi yok", devLoading: "Yükleniyor…", devPlaceholder: "Claude, Codex ya da herhangi bir araç için prompt yaz veya söyle", devListening: "Dinleniyor… konuşabilirsin",
      devSend: "Gönder", devType: "Sadece yaz", devSent: "Bilgisayara gönderildi", devTyped: "Bilgisayara yazıldı", devEmpty: "Gönderilecek bir şey yok", devOffline: "Bilgisayara bağlı değil",
      devWorking: "düşünüyor…", devReady: "hazır", devIdle: "bitti", devDone: "Ajan bitirdi", devNotDev: "Öndeki pencere bir geliştirici uygulaması değil. Listeden seç.", doShell: "komut çalıştırıyor", doEdit: "dosya düzenliyor", doRead: "kodu okuyor", doWeb: "web'de arıyor", doAgent: "yardımcı ajan çalışıyor", doPlan: "plan yapıyor", doAsk: "senden cevap bekliyor", devFive: "5sa", devSeven: "7g",
      devMicHelp: "Dikte büyük ihtimalle kapalı. iPhone'da Ayarlar › Genel › Klavye › Dikteyi Etkinleştir'i aç. Sonra yazı kutusuna dokun ve klavyedeki mikrofon tuşunu kullan.",
      devMicNet: "Dikte için internet gerekir.", devMicQuiet: "Ses duyulmadı. Tekrar dene.",
      settings: "AYARLAR", setHint: "Bu telefonda saklanır", setTabs: "Sekmeler", setDev: "Geliştirici modu", setGeneral: "Genel",
      tabHome: "Ana ekran", tabGames: "Oyunlar", tabStats: "İstatistik", tabWatch: "Dizi modu", tabDev: "Geliştirici", tabPad: "Dokunmatik yüzey", tabClock: "Saat",
      setActivity: "Claude / Codex ne yapıyor göster", setUsage: "Kullanımı göster", setAutosend: "Konuşunca hemen gönder", setLang: "Dikte dili", setCalm: "Sakin mod", setReset: "Sekmeleri ve ayarları sıfırla", setResetDone: "Sıfırlandı",
      setUp: "Yukarı al", setDown: "Aşağı al",
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
    if (!online) devOffline(); // the busy light is not known without the PC
  }

  // ---- Preferences -------------------------------------------------------

  // What the person switched on or off in Settings: which tabs show and in
  // what order, and the Claude mode options. Kept in this phone's storage.
  var DEFAULT_ORDER = ["home", "games", "stats", "watch", "dev", "pad", "clock"];
  var hasGames = true; // until the PC's layout says whether any game exists

  function loadPrefs() {
    var p = {};
    try { p = JSON.parse(load(PREFS_KEY) || "{}") || {}; } catch (e) { p = {}; }
    var out = { hidden: {}, order: DEFAULT_ORDER.slice(), agent: p.agent !== false, usage: p.usage !== false, live: p.live !== false, autosend: p.autosend === true, calm: p.calm === true, speech: p.speech === "tr" || p.speech === "en" ? p.speech : "" };
    if (p.hidden && typeof p.hidden === "object") {
      DEFAULT_ORDER.forEach(function (n) { if (n !== "home" && p.hidden[n] === true) out.hidden[n] = true; });
    }
    if (Array.isArray(p.order)) {
      var seen = {}, order = [];
      p.order.concat(DEFAULT_ORDER).forEach(function (n) {
        if (DEFAULT_ORDER.indexOf(n) >= 0 && !seen[n]) { seen[n] = true; order.push(n); }
      });
      out.order = order;
    }
    return out;
  }
  var prefs = loadPrefs();
  function savePrefs() { save(PREFS_KEY, JSON.stringify(prefs)); }

  // Home and Settings always stay; the games tab also needs a game on the PC.
  function tabHidden(name) {
    if (name === "home" || name === "settings" || name === "pair") return false;
    if (name === "games" && !hasGames) return true;
    return !!prefs.hidden[name];
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
      var p = e.touches && e.touches[0];
      if (!p || Math.abs(p.clientX - sx) > 12 || Math.abs(p.clientY - sy) > 12) tap = false;
    }, { passive: true });
    t.addEventListener("touchend", function () { if (tap) showView(t.dataset.view); tap = false; }, { passive: true });
  });

  var padReset = null; // lets go of trackpad buttons (set by the trackpad code)

  function showView(name, remember) {
    if (name !== "pair" && tabHidden(name)) name = "home"; // a tab the user switched off
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
      placePill();
    }
    if (name === "dev") devShown();
    syncStreams();
  }

  // The highlight behind the active tab. It sits where the first tab would be
  // (its CSS top) and slides down by the active tab's offset.
  function placePill() {
    var pill = $("tab-pill");
    var base = parseFloat(getComputedStyle(pill).top) || 0;
    var found = false;
    tabs.forEach(function (t) {
      var on = t.dataset.view === view && !t.hidden;
      t.classList.toggle("active", on);
      if (on) {
        found = true;
        pill.style.transform = "translateY(" + (t.offsetTop - base) + "px)";
        pill.classList.toggle("soft", t.classList.contains("settings-tab"));
      }
    });
    pill.style.visibility = found ? "" : "hidden";
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
        if (msg.version) {
          if (!pageVersion) {
            pageVersion = msg.version;
            if (window.DeskReport) window.DeskReport("panel " + msg.version + " " + window.innerWidth + "x" + window.innerHeight + (framed ? " in frame" : ""), "start");
          } else if (msg.version !== pageVersion) {
            // DeskLink was updated: load the new panel (the whole page, frame included).
            (framed ? window.parent : window).location.reload();
            return;
          }
        }
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
        hasGames = (msg.games || []).length > 0;
        applyTabs();
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
      case "windows":
        devWindows(msg.windows || []);
        break;
      case "typeres":
        devTypeResult(msg.error);
        break;
      case "claude":
        if (msg.claude) { renderClaude(msg.claude); devRender(msg.claude); }
        break;
      case "plan":
        planItems = msg.plan || [];
        renderPlan();
        break;
      case "planerr":
        toast(msg.error || "");
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
      stats: !!(live && ((view === "stats" && statsPage === 0) || (view === "home" && miniEls && miniPage === 0))),
      claude: !!(live && ((view === "home" && claudeEls && miniPage === 1) || (view === "dev" && (prefs.agent || prefs.usage)) || (prefs.agent && !tabHidden("dev")))),
      media: !!(live && view === "home" && np),
      mixer: !!(live && (view === "stats" || view === "watch"))
    };
    Object.keys(want).forEach(function (k) {
      if (want[k] !== !!streaming[k] && send({ type: k, on: want[k] })) streaming[k] = want[k];
    });
    // The busy dot on the Claude tab is only true while its numbers stream.
    if (!want.claude) {
      devPrev = "";
      var busyTab = document.querySelector(".tab.busy");
      if (busyTab) busyTab.classList.remove("busy");
    }
    clearInterval(progressTimer);
    progressTimer = want.media ? setInterval(function () { tickProgress(false); }, 1000) : null;
    // The plan counts down on the phone's own clock, so it needs no stream.
    clearInterval(planTimer);
    planTimer = null;
    if (live && view === "home" && planEls) {
      renderPlan();
      planTimer = setInterval(renderPlan, miniPage === 2 ? 1000 : 15000);
    }
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
  // A list inside the page may scroll, but only while it has somewhere to go:
  // at its top or bottom the drag would otherwise pull the whole page with it.
  function canScrollThere(el, y) {
    var dy = y - touchY;
    if (el.scrollHeight <= el.clientHeight + 1) return false;
    if (dy > 0 && el.scrollTop <= 0) return false;
    if (dy < 0 && el.scrollTop + el.clientHeight >= el.scrollHeight - 1) return false;
    return true;
  }
  document.addEventListener("touchmove", function (e) {
    if (e.target.closest && e.target.closest(".dev-text")) return; // editing text: selection handles and caret
    // Settings and the window picker scroll by code and every control there
    // acts on touchend (onTap), so no click has to survive: stop the very first
    // move. Once iOS has started panning the page it can no longer be stopped,
    // and the page slid down with the finger.
    if (e.target.closest && e.target.closest(".settings-view, .dev-sheet")) { if (e.cancelable) e.preventDefault(); return; }
    var scroller = e.target.closest && e.target.closest(".game-list, .ps-items");
    if (scroller && canScrollThere(scroller, e.touches[0].clientY)) return;
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
      np.lock = document.createElement("button");
      np.lock.type = "button";
      np.lock.className = "np-lock";
      np.lock.addEventListener("click", toggleLock);
      body.appendChild(np.lock);
      bindSwipe(card);
      body.querySelector(".np-art").addEventListener("click", openFull);
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
      // The bottom of the card is a pager: CPU and GPU, and a swipe to the side
      // shows Claude Code's token use.
      var pager = document.createElement("div");
      pager.className = "mini-pager";
      var track = document.createElement("div");
      track.className = "mini-track";
      track.appendChild(mini);
      track.appendChild(claudePage());
      track.appendChild(planPage());
      pager.appendChild(track);
      pager.insertAdjacentHTML("beforeend", '<div class="mini-dots"><i></i><i></i><i></i></div>');
      body.appendChild(pager);
      miniEls = {};
      ["cpu", "gpu"].forEach(function (k) {
        var item = mini.querySelector("." + k);
        miniEls[k] = { root: item, value: item.querySelector("b"), ring: item.querySelector(".value") };
      });
      bindPager(pager, track);
      setMiniPage(miniPage, true);
      if (claudeLast) renderClaude(claudeLast);
      renderPlan();
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

  // ---- Claude Code -------------------------------------------------------

  var miniPage = 0;       // 0: CPU and GPU, 1: Claude
  var claudeEls = null;   // the Claude page's elements, once the home screen is drawn
  var claudeLast = null;  // the last "claude" message

  // The second page of the home screen's bottom card: two rings with the plan
  // limits in percent (5 hours and 7 days), as in Claude's /usage. Until Claude
  // Code has reported them, the token counts are shown instead.
  // r = 19 → circumference 119.4
  function limitRing(key) {
    return '<div class="cl-lim ' + key + '"><div class="donut"><svg viewBox="0 0 44 44"><circle class="track" cx="22" cy="22" r="19"/>' +
      '<circle class="value" cx="22" cy="22" r="19" stroke-dasharray="119.4" stroke-dashoffset="119.4"/></svg><b>–</b></div>' +
      '<span class="mini-label"></span><span class="cl-sub"></span></div>';
  }
  function claudePage() {
    var el = document.createElement("div");
    el.className = "claude";
    el.innerHTML = '<div class="cl-limits">' + limitRing("five") + limitRing("seven") + '</div>' +
      '<div class="cl-info"><div class="cl-rows"></div><div class="cl-note"></div></div>';
    function ring(k) {
      var r = el.querySelector(".cl-lim." + k);
      return { root: r, ring: r.querySelector(".value"), pct: r.querySelector("b"), label: r.querySelector(".mini-label"), sub: r.querySelector(".cl-sub") };
    }
    claudeEls = { root: el, five: ring("five"), seven: ring("seven"), rows: el.querySelector(".cl-rows"), note: el.querySelector(".cl-note") };
    return el;
  }

  // 1234 -> 1.2K, 1234567 -> 1.2M
  function short(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(n >= 1e7 ? 0 : 1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(n >= 1e4 ? 0 : 1) + "K";
    return String(n);
  }

  // 8700 s -> "2h 25m", 400000 s -> "4d 15h"
  function until(s) {
    var m = Math.round(s / 60);
    if (m >= 1440) return Math.floor(m / 1440) + t("d") + " " + Math.floor((m % 1440) / 60) + t("h");
    if (m >= 60) return Math.floor(m / 60) + t("h") + " " + (m % 60) + t("m");
    return Math.max(1, m) + t("m");
  }

  function drawLimit(e, label, w) {
    e.label.textContent = label;
    var has = !!w;
    var pct = has ? Math.round(w.pct) : 0;
    e.root.classList.toggle("na", !has);
    e.root.classList.toggle("warn", has && pct >= 70 && pct < 90);
    e.root.classList.toggle("hot", has && pct >= 90);
    e.pct.textContent = has ? pct + "%" : "–";
    e.ring.setAttribute("stroke-dashoffset", (119.4 * (1 - (has ? Math.min(100, w.pct) : 0) / 100)).toFixed(1));
    e.sub.textContent = has && w.reset_s > 0 ? t("clLeft", until(w.reset_s)) : "";
  }

  function renderClaude(c) {
    claudeLast = c;
    var e = claudeEls;
    if (!e) return;
    var lim = c.limits;
    var show = !!(lim && (lim.five_hour || lim.seven_day));
    e.root.classList.toggle("limits", show);
    if (show) {
      drawLimit(e.five, t("clFive"), lim.five_hour);
      drawLimit(e.seven, t("clSeven"), lim.seven_day);
      return;
    }
    // No limits yet: the token counts from the logs.
    var b = c.block || {};
    if (!c.found) {
      e.rows.innerHTML = "";
      e.note.textContent = t("clNone");
      return;
    }
    var rows = [[t("clWindow"), c.active ? short(b.tokens) : "–"], [t("clToday"), short(c.today)], [t("clWeek"), short(c.week)]];
    e.rows.innerHTML = rows.map(function (r) {
      return "<div><span>" + r[0] + "</span><b>" + r[1] + "</b></div>";
    }).join("");
    e.note.textContent = t("clHint");
  }

  // ---- Developer mode ----------------------------------------------------

  // Write or dictate a prompt on the phone and have it typed into the window
  // in front on the PC: Claude Code, Codex or any other tool in a terminal or
  // editor. Optionally the header shows whether Claude Code is busy and how
  // much of the plan is used; both come from Claude Code's own logs on the PC
  // and are off until switched on in Settings.
  var devEls = null, devPrev = "", devRec = null, devGotFinal = false, devHelpTimer = null;
  var devTarget = { id: 0, name: "" }; // the program to send to; id 0 is the window in front
  var devLast = "";                    // the last text sent, to put back if it could not be delivered
  var DRAFT_KEY = "desklink.draft";
  var SpeechRec = window.SpeechRecognition || window.webkitSpeechRecognition;

  function mk(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  // Taps that always arrive. iOS sometimes loses the click of a quick tap, so
  // the end of a touch that did not move counts as the tap too (never both).
  function onTap(el, fn) {
    var sx = 0, sy = 0, tap = false, last = 0;
    function fire(e) {
      if (el.disabled) return;
      var now = Date.now();
      if (now - last < 350) return;
      last = now;
      fn(e);
    }
    el.addEventListener("click", fire);
    el.addEventListener("touchstart", function (e) {
      var p = e.touches && e.touches[0];
      sx = p ? p.clientX : 0;
      sy = p ? p.clientY : 0;
      tap = !!p && e.touches.length === 1;
    }, { passive: true });
    el.addEventListener("touchmove", function (e) {
      if (Math.abs(e.touches[0].clientX - sx) > 12 || Math.abs(e.touches[0].clientY - sy) > 12) tap = false;
    }, { passive: true });
    // The tap is used at touchend, and the click iOS makes from it is cancelled:
    // the tap often redraws the screen, and that click would land on the new
    // thing under the finger (and, say, switch a setting straight back).
    el.addEventListener("touchend", function (e) {
      if (!tap) return;
      tap = false;
      if (e.cancelable) e.preventDefault();
      fire(e);
    }, { passive: false });
  }

  // Scrolling a list by hand. Native scrolling inside the sideways frame pulls
  // the whole page along, so the lists that matter are moved by code instead
  // (with a little glide at the end).
  function dragScroll(el, inner) {
    if (!el) return;
    var y0 = 0, top0 = 0, lastY = 0, lastT = 0, speed = 0, active = false, moved = false, raf = 0;
    // With an inner layer the content is slid by transform: the phone moves a
    // picture it already has instead of painting the list again every frame.
    var pos = 0;
    function getTop() { return inner ? pos : el.scrollTop; }
    function setTop(v) {
      if (!inner) { el.scrollTop = v; return; }
      var maxTop = Math.max(0, inner.offsetHeight - el.clientHeight);
      pos = Math.max(0, Math.min(maxTop, v));
      inner.style.transform = "translate3d(0," + (-pos) + "px,0)";
    }
    if (inner) el.dragScrollTo = function (v) { setTop(v == null ? pos : v); };
    el.addEventListener("touchstart", function (e) {
      cancelAnimationFrame(raf);
      if (!e.touches || e.touches.length !== 1) { active = false; return; }
      y0 = lastY = e.touches[0].clientY;
      top0 = getTop();
      lastT = Date.now();
      speed = 0;
      active = true;
      moved = false;
    }, { passive: true });
    el.addEventListener("touchmove", function (e) {
      if (!active || !e.touches || !e.touches[0]) return;
      var y = e.touches[0].clientY;
      if (!moved && Math.abs(y - y0) < 10) return;
      moved = true;
      setTop(top0 - (y - y0));
      var now = Date.now();
      if (now > lastT) speed = (lastY - y) / (now - lastT);
      lastY = y;
      lastT = now;
    }, { passive: true });
    el.addEventListener("touchend", function () {
      if (!active) return;
      active = false;
      if (!moved) return;
      var v = speed * 16;
      (function glide() {
        if (Math.abs(v) < 0.4) return;
        var before = getTop();
        setTop(before + v);
        if (getTop() === before) return; // reached an end

        v *= 0.94;
        raf = requestAnimationFrame(glide);
      })();
    }, { passive: true });
  }

  function initDevMode() {
    var text = $("dev-text");
    if (!text) return;
    devEls = { text: text, clear: $("dev-clear"), help: $("dev-help"), mic: $("dev-mic"), chips: $("dev-chips"), live: $("dev-live"), usage: $("dev-usage") };
    text.placeholder = t("devPlaceholder");
    text.maxLength = 2000;
    text.value = load(DRAFT_KEY) || "";
    devSyncClear();
    text.addEventListener("input", function () {
      save(DRAFT_KEY, text.value);
      devSyncClear();
      devHideHelp();
      if (!text.value) devLivePaused = false;
      devLiveSync();
    });
    // The keyboard covers half of a sideways phone: the card moves to the top.
    function keyboard(on) {
      document.body.classList.toggle("kb-open", on);
      if (framed) { try { window.parent.postMessage({ type: "kb", on: on }, location.origin); } catch (err) { /* no parent */ } }
    }
    text.addEventListener("focus", function () { keyboard(true); });
    text.addEventListener("blur", function () {
      setTimeout(function () { keyboard(false); window.scrollTo(0, 0); }, 60);
    });
    // iOS pans the page to show the text box; the page never scrolls, so put it back.
    window.addEventListener("scroll", function () { if (window.scrollX || window.scrollY) window.scrollTo(0, 0); }, { passive: true });
    trackKeyboard();
    onTap(devEls.clear, function () { text.value = ""; save(DRAFT_KEY, null); devSyncClear(); devLivePaused = false; devLiveSync(); text.focus(); });
    onTap($("dev-send"), function () { devSubmit(true); });
    onTap($("dev-type"), function () { devSubmit(false); });
    onTap(devEls.mic, devDictate);
    onTap($("dev-target"), devOpenSheet);
    onTap($("dev-sheet-refresh"), function () { send({ type: "windows" }); });
    onTap($("dev-sheet"), function (ev) { if (ev && ev.target === $("dev-sheet")) $("dev-sheet").hidden = true; });
    Array.prototype.forEach.call(document.querySelectorAll(".dev-key"), function (b) {
      onTap(b, function () {
        var live = devLiveHandOff();
        if (!send({ type: "key", action: b.dataset.key, window: devTarget.id || undefined })) { toast(t("devOffline")); return; }
        if (live) devClearBox(); // the text is in the window now; the key ended the live session
        b.classList.add("pressed");
        setTimeout(function () { b.classList.remove("pressed"); }, 140);
      });
    });
    devApplyPrefs();
  }

  // The panel (outside the sideways frame) is pinned to the visible part of the
  // window: with the keyboard up, that part is shorter and iOS pans it.
  function trackKeyboard() {
    var vv = window.visualViewport;
    if (!vv || framed) return;
    function fit() {
      var up = window.innerHeight - vv.height > 90;
      document.documentElement.style.setProperty("--vh", (up ? vv.height : window.innerHeight) / 100 + "px");
      document.body.style.position = up ? "relative" : "";
      document.body.style.top = up ? vv.offsetTop + "px" : "";
    }
    vv.addEventListener("resize", fit);
    vv.addEventListener("scroll", fit);
  }

  function devSyncClear() { devEls.clear.hidden = !devEls.text.value; }

  function devShowHelp(msg) {
    devEls.help.textContent = msg;
    devEls.help.hidden = false;
    clearTimeout(devHelpTimer);
    devHelpTimer = setTimeout(devHideHelp, 15000);
  }
  function devHideHelp() { if (devEls) devEls.help.hidden = true; }

  // Types the prompt on the PC; with andEnter, presses Enter after it.
  function devSubmit(andEnter) {
    var e = devEls;
    if (!e) return;
    var txt = e.text.value.replace(/\s+$/, "");
    if (!txt.trim()) { toast(t("devEmpty")); return; }
    var win = devTarget.id || undefined;
    if (prefs.live && !devLivePaused) {
      // The text is on the PC already (or about to be): finish it, then Enter.
      clearTimeout(devLiveTimer);
      if (!send({ type: "live", text: txt, window: win })) { toast(t("devOffline")); return; }
      send({ type: "liveend", send: andEnter });
      devLiveOn = false;
      devLast = e.text.value;
    } else {
      if (!send({ type: "type", text: txt, send: andEnter, window: win })) { toast(t("devOffline")); return; }
      devLast = e.text.value;
    }
    devLivePaused = false;
    // Still listening? Stop, so late words do not fill the box again and the
    // auto-send does not send the same prompt twice.
    if (devRec) {
      devGotFinal = false;
      devRec.onresult = null;
      try { devRec.abort(); } catch (err) { /* already over */ }
    }
    e.text.value = "";
    save(DRAFT_KEY, null);
    devSyncClear();
    devHideHelp();
    toast(t(andEnter ? "devSent" : "devTyped"));
  }

  function speechLang() {
    var l = prefs.speech || LANG;
    return l === "tr" ? "tr-TR" : "en-US";
  }

  function devListening(on) {
    var e = devEls;
    e.mic.classList.toggle("on", on);
    e.text.classList.toggle("listening", on);
    e.text.placeholder = t(on ? "devListening" : "devPlaceholder");
  }

  // Safari's own speech recognition, where the page may use it. Both it and
  // the keyboard's mic key need Dictation switched on in the iPhone's
  // settings, so a failure says how to do that instead of just failing.
  function devDictate() {
    var e = devEls;
    if (devRec) { devRec.stop(); return; }
    devHideHelp();
    if (!SpeechRec) { devShowHelp(t("devMicHelp")); return; }
    var r;
    try { r = new SpeechRec(); } catch (err) { devShowHelp(t("devMicHelp")); return; }
    var base = e.text.value, gap = base && !/\s$/.test(base) ? " " : "";
    var started = false;
    devGotFinal = false;
    r.lang = speechLang();
    r.interimResults = true;
    r.continuous = false;
    r.onstart = function () { started = true; };
    r.onresult = function (ev) {
      var heard = "";
      for (var i = 0; i < ev.results.length; i++) {
        heard += ev.results[i][0].transcript;
        if (ev.results[i].isFinal) devGotFinal = true;
      }
      e.text.value = base + gap + heard;
      save(DRAFT_KEY, e.text.value);
      devSyncClear();
      devLiveSync();
    };
    r.onerror = function (ev) {
      if (ev.error === "aborted") return;
      if (ev.error === "no-speech") { devShowHelp(t("devMicQuiet")); return; }
      if (ev.error === "network") { devShowHelp(t("devMicNet")); return; }
      devShowHelp(t("devMicHelp"));
    };
    r.onend = function () {
      devRec = null;
      devListening(false);
      if (devGotFinal && prefs.autosend) devSubmit(true);
    };
    try {
      r.start();
    } catch (err) { devShowHelp(t("devMicHelp")); return; }
    devRec = r;
    devListening(true);
    // Some phones never start listening and never say why.
    setTimeout(function () {
      if (devRec === r && !started) {
        try { r.abort(); } catch (err) { /* already over */ }
        devShowHelp(t("devMicHelp"));
      }
    }, 6000);
  }

  // The program the text goes to. The list comes from the PC; a window that
  // has closed since is dropped when it is next used.
  function devSetTarget(id, name) {
    var moved = devTarget.id !== id;
    if (moved) devLiveStop();
    devTarget = { id: id, name: name };
    $("dev-target-name").textContent = id ? name : t("devFront");
    $("dev-target").classList.toggle("set", !!id);
    if (moved && devEls && devEls.text.value) devLiveSync(); // the whole text goes to the new program
  }

  function devOpenSheet() {
    var list = $("dev-sheet-list");
    list.innerHTML = "";
    list.appendChild(mk("div", "dev-none", t("devLoading")));
    $("dev-sheet").hidden = false;
    if (!send({ type: "windows" })) { $("dev-sheet").hidden = true; toast(t("devOffline")); }
  }

  var KIND_ICON = {
    front: '<rect x="3" y="4" width="18" height="13" rx="2.5"/><path d="M8 21h8"/>',
    ai: '<path d="M11 3l1.9 5.1L18 10l-5.1 1.9L11 17l-1.9-5.1L4 10l5.1-1.9z"/><path d="M18.5 15.5v4M16.5 17.5h4"/>',
    term: '<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3M13 15h4"/>',
    editor: '<path d="m8 8-4 4 4 4M16 8l4 4-4 4M13.5 5l-3 14"/>'
  };

  function devOption(id, name, sub, kind) {
    var b = mk("button", "dev-opt" + (devTarget.id === id ? " on" : ""));
    b.type = "button";
    b.setAttribute("role", "option");
    var ic = mk("span", "ic");
    ic.innerHTML = '<svg viewBox="0 0 24 24">' + (KIND_ICON[kind] || KIND_ICON.front) + "</svg>";
    b.appendChild(ic);
    var tx = mk("span", "tx");
    tx.appendChild(mk("b", "", name));
    if (sub) tx.appendChild(mk("small", "", sub));
    b.appendChild(tx);
    if (devTarget.id === id) b.insertAdjacentHTML("beforeend", '<svg viewBox="0 0 24 24"><path d="m5 12 5 5 9-10"/></svg>');
    onTap(b, function () { devSetTarget(id, name); $("dev-sheet").hidden = true; });
    return b;
  }

  function devWindows(items) {
    var list = $("dev-sheet-list");
    if (!list || $("dev-sheet").hidden) return;
    list.innerHTML = "";
    list.appendChild(devOption(0, t("devFront"), "", "front"));
    items.forEach(function (w) {
      list.appendChild(devOption(w.id, w.app, w.title && w.title !== w.app ? w.title : "", w.kind));
    });
    if (!items.length) list.appendChild(mk("div", "dev-none", t("devNoWindows")));
  }

  // Live typing: the PC shows the text as it is written or dictated. The whole
  // text is sent a moment after each change; the PC works out what changed.
  var devLiveOn = false, devLivePaused = false, devLiveTimer = null;

  function devLiveSync() {
    if (!prefs.live || devLivePaused) return;
    clearTimeout(devLiveTimer);
    devLiveTimer = setTimeout(devLivePush, 250);
  }

  function devLivePush() {
    var e = devEls;
    if (!e || !prefs.live || devLivePaused) return;
    var txt = e.text.value;
    if (!txt && !devLiveOn) return;
    if (!send({ type: "live", text: txt, window: devTarget.id || undefined })) return;
    if (txt) devLiveOn = true;
  }

  // Before a key: the last words are typed live, so the key lands after them.
  // True if a live text was on its way (the box is then emptied after the key).
  function devLiveHandOff() {
    var e = devEls;
    if (!e || !prefs.live || devLivePaused || !e.text.value) return false;
    clearTimeout(devLiveTimer);
    if (!send({ type: "live", text: e.text.value, window: devTarget.id || undefined })) return false;
    devLiveOn = false;
    return true;
  }
  function devClearBox() {
    devEls.text.value = "";
    save(DRAFT_KEY, null);
    devSyncClear();
  }

  // Ends the PC's live session; the text stays in the box.
  function devLiveStop() {
    clearTimeout(devLiveTimer);
    if (devLiveOn) send({ type: "liveend", send: false });
    devLiveOn = false;
  }

  // The PC could not deliver: say why, and give the text back.
  function devTypeResult(code) {
    if (!code || !devEls) return;
    clearTimeout(devLiveTimer);
    devLiveOn = false;
    if (prefs.live) devLivePaused = true; // do not type the same text again behind the person's back
    if (code === "gone") devSetTarget(0, "");
    toast(t(code === "gone" ? "devGone" : code === "focus" ? "devNoFocus" : code === "lost" ? "devLost" : code === "notdev" ? "devNotDev" : "devRefused"));
    if (!devEls.text.value && devLast) {
      devEls.text.value = devLast;
      save(DRAFT_KEY, devLast);
      devSyncClear();
    }
  }

  // Which of the header's two optional pieces show.
  function devApplyPrefs() {
    var e = devEls;
    if (!e) return;
    $("view-dev").classList.toggle("live", prefs.live);
    if (!prefs.live) devLiveStop();
    if (!prefs.agent) devLive("");
    if (!prefs.usage) e.usage.hidden = true;
    e.chips.hidden = e.live.hidden && e.usage.hidden;
    if (claudeLast) devRender(claudeLast);
  }

  // What a tool call means, in words: "Bash" -> "running a command".
  var DO_WORDS = { bash: "doShell", shell: "doShell", powershell: "doShell", exec_command: "doShell", local_shell: "doShell", bashoutput: "doShell",
    edit: "doEdit", write: "doEdit", multiedit: "doEdit", notebookedit: "doEdit", apply_patch: "doEdit",
    read: "doRead", grep: "doRead", glob: "doRead", ls: "doRead", view_image: "doRead",
    websearch: "doWeb", webfetch: "doWeb", web_search: "doWeb",
    agent: "doAgent", task: "doAgent", todowrite: "doPlan", update_plan: "doPlan", exitplanmode: "doPlan",
    askuserquestion: "doAsk", request_user_input: "doAsk" };
  function doingWords(tool) {
    if (!tool) return t("devWorking");
    var k = DO_WORDS[tool.toLowerCase()];
    return k ? t(k) : tool;
  }

  // The agent's state: who ("Claude", "Codex"), and what it is doing; anything else hides it.
  function devLive(s, c) {
    var e = devEls;
    if (!e) return;
    var show = prefs.agent && (s === "working" || s === "idle");
    e.live.hidden = !show;
    if (show) {
      var who = c && c.agent === "codex" ? "Codex" : "Claude";
      var what = s === "working" ? doingWords(c && c.doing) : t(c && c.state_s < 600 ? "devIdle" : "devReady");
      e.live.className = "dev-live " + s + (s === "working" && c && c.doing ? " tool" : "");
      e.live.querySelector("b").textContent = who;
      e.live.querySelector("em").textContent = what;
    }
    var tab = document.querySelector('.tab[data-view="dev"]');
    if (tab) tab.classList.toggle("busy", show && s === "working");
    e.chips.hidden = e.live.hidden && e.usage.hidden;
  }

  function devOffline() {
    devPrev = "";
    devLive("");
    // The PC forgot what it typed live: typing on after reconnecting would
    // write the whole text a second time. Wait until the box is emptied.
    if (devLiveOn) {
      clearTimeout(devLiveTimer);
      devLiveOn = false;
      devLivePaused = true;
    }
  }

  // The numbers from Claude Code's logs: activity light and plan usage.
  function devRender(c) {
    var e = devEls;
    if (!e) return;
    var s = c.state || "none";
    var finished = devPrev === "working" && s === "idle";
    devPrev = s;
    devLive(s, c);
    if (finished && prefs.agent && view !== "dev") toast(t("devDone"));
    var lim = c.limits;
    var parts = [];
    var pcts = true;
    if (prefs.usage && lim && (lim.five_hour || lim.seven_day)) {
      if (lim.five_hour) parts.push([t("devFive"), lim.five_hour.pct]);
      if (lim.seven_day) parts.push([t("devSeven"), lim.seven_day.pct]);
    } else if (prefs.usage && c.found) {
      // No plan percentages yet: the token counts from the logs.
      pcts = false;
      if (c.active && c.block) parts.push([t("devFive"), c.block.tokens]);
      parts.push([t("devSeven"), c.week]);
    }
    e.usage.innerHTML = "";
    parts.forEach(function (p) {
      var pct = Math.round(p[1]);
      var s2 = mk("span", pcts && pct >= 90 ? "hot" : pcts && pct >= 70 ? "warn" : "");
      s2.appendChild(document.createTextNode(p[0]));
      s2.appendChild(mk("b", "", pcts ? pct + "%" : short(p[1])));
      e.usage.appendChild(s2);
    });
    e.usage.hidden = !parts.length;
    e.chips.hidden = e.live.hidden && e.usage.hidden;
  }

  // Opening the screen: show the last numbers at once.
  function devShown() {
    if (devEls && claudeLast) devRender(claudeLast);
  }

  // ---- Settings ----------------------------------------------------------

  var SET_LABEL = { home: "tabHome", games: "tabGames", stats: "tabStats", watch: "tabWatch", dev: "tabDev", pad: "tabPad", clock: "tabClock" };

  // Puts the tabs in the saved order and hides the ones switched off.
  function applyTabs() {
    var rail = $("rail"), settingsTab = rail.querySelector(".settings-tab");
    var by = {};
    tabs.forEach(function (tb) { by[tb.dataset.view] = tb; });
    prefs.order.forEach(function (n) { if (by[n]) rail.insertBefore(by[n], settingsTab); });
    tabs = prefs.order.filter(function (n) { return by[n]; }).map(function (n) { return by[n]; }).concat([settingsTab]);
    tabs.forEach(function (tb) { tb.hidden = tabHidden(tb.dataset.view); });
    document.body.classList.toggle("calm", prefs.calm);
    devApplyPrefs();
    if (tabHidden(view)) showView("home", false); else placePill();
    syncStreams();
  }

  // A switch is only its look; the whole row is the button.
  function swtch(on, locked) {
    var b = mk("span", "sw" + (on ? " on" : "") + (locked ? " locked" : ""));
    b.appendChild(mk("i"));
    return b;
  }

  // A row with a switch passes onRow (and the switch's state, for screen
  // readers); taps on its arrows or buttons stay theirs.
  function setRow(label, control, icon, onRow, on) {
    var row = mk("div", "set-row");
    if (icon) {
      var ic = mk("span", "set-ic");
      ic.innerHTML = icon;
      row.appendChild(ic);
    }
    row.appendChild(mk("span", "set-tx", label));
    [].concat(control || []).forEach(function (c) { row.appendChild(c); });
    if (onRow) {
      row.classList.add("tap");
      row.tabIndex = 0;
      row.setAttribute("role", "switch");
      row.setAttribute("aria-checked", on ? "true" : "false");
      onTap(row, function (e) {
        if (e && e.target && e.target.closest && e.target.closest(".set-mv, .seg")) return;
        onRow();
      });
    }
    return row;
  }

  function arrow(dir, label, disabled, go) {
    var b = mk("button", "set-mv");
    b.type = "button";
    b.setAttribute("aria-label", label);
    b.disabled = disabled;
    b.innerHTML = '<svg viewBox="0 0 24 24"><path d="' + (dir < 0 ? "m6 15 6-6 6 6" : "m6 9 6 6 6-6") + '"/></svg>';
    onTap(b, go);
    return b;
  }

  function changed() {
    if (window.DeskSettingsNote) window.DeskSettingsNote("changed");
    savePrefs();
    applyTabs();
    renderSettings();
  }

  // One preference that is a plain on/off.
  function flag(key, label) {
    return setRow(label, swtch(prefs[key]), null, function () { prefs[key] = !prefs[key]; changed(); }, prefs[key]);
  }

  function renderSettings() {
    var box = $("set-scroll");
    if (!box) return;
    var top = box.scrollTop;
    var tabsBox = $("set-tabs"), devBox = $("set-dev"), general = $("set-general");
    [tabsBox, devBox, general].forEach(function (b) { b.innerHTML = ""; });

    prefs.order.forEach(function (n, i) {
      var tabEl = document.querySelector('.tab[data-view="' + n + '"]');
      var icon = tabEl ? tabEl.querySelector("svg").outerHTML : "";
      var locked = n === "home";
      var controls = [
        arrow(-1, t("setUp"), i === 0, function () { prefs.order.splice(i, 1); prefs.order.splice(i - 1, 0, n); changed(); }),
        arrow(1, t("setDown"), i === prefs.order.length - 1, function () { prefs.order.splice(i, 1); prefs.order.splice(i + 1, 0, n); changed(); }),
        swtch(!prefs.hidden[n], locked)
      ];
      tabsBox.appendChild(setRow(t(SET_LABEL[n]), controls, icon, locked ? null : function () {
        if (prefs.hidden[n]) delete prefs.hidden[n]; else prefs.hidden[n] = true;
        changed();
      }, !prefs.hidden[n]));
    });

    devBox.appendChild(flag("live", t("setLive")));
    devBox.appendChild(flag("usage", t("setUsage")));
    devBox.appendChild(flag("agent", t("setActivity")));
    if (SpeechRec) {
      devBox.appendChild(flag("autosend", t("setAutosend")));
      var seg = mk("div", "seg");
      [["", "Auto"], ["tr", "TR"], ["en", "EN"]].forEach(function (o) {
        var b = mk("button", prefs.speech === o[0] ? "on" : "", o[1]);
        b.type = "button";
        onTap(b, function () { prefs.speech = o[0]; changed(); });
        seg.appendChild(b);
      });
      devBox.appendChild(setRow(t("setLang"), seg));
    }

    general.appendChild(flag("calm", t("setCalm")));
    var reset = mk("button", "set-reset", t("setReset"));
    reset.type = "button";
    onTap(reset, function () {
      save(PREFS_KEY, null);
      prefs = loadPrefs();
      changed();
      toast(t("setResetDone"));
    });
    general.appendChild(reset);
    if (box.dragScrollTo) box.dragScrollTo(); else box.scrollTop = top;
  }

  (function watchSettingsTaps() {
    var box = $("view-settings"), notes = [], sent = 0, timer = null;
    if (!box || !window.DeskReport) return;
    function note(s) {
      if (sent >= 4 || notes.length > 12) return;
      notes.push(s);
      clearTimeout(timer);
      timer = setTimeout(function () { sent++; window.DeskReport("settings taps: " + notes.join(" | "), "settings"); notes = []; }, 2500);
    }
    function where(e) {
      var el = e.target, p = e.changedTouches && e.changedTouches[0];
      var name = el && el.className && typeof el.className === "string" ? el.className.split(" ")[0] : el && el.nodeName;
      var row = el && el.closest && el.closest(".set-row");
      return name + (row ? "@row" : "") + (p ? " " + Math.round(p.clientX) + "," + Math.round(p.clientY) : "");
    }
    ["touchstart", "touchend", "click"].forEach(function (type) {
      box.addEventListener(type, function (e) { note(type + " " + where(e) + (e.defaultPrevented ? " prevented" : "")); }, true);
    });
    window.DeskSettingsNote = note;
  })();
  initDevMode();
  dragScroll($("set-scroll"), $("set-inner"));
  dragScroll($("dev-sheet-list"));
  renderSettings();
  applyTabs();
  // The settings tab sits at the bottom, so its place depends on the screen's
  // height: measure again once the page has its size, and when it changes.
  requestAnimationFrame(placePill);
  window.addEventListener("resize", placePill);

  // ---- Day plan ----------------------------------------------------------

  var PAGES = 3;          // pager pages: CPU and GPU, Claude, plan
  var planItems = [];     // the plan from the PC
  var planEls = null;     // the plan page's elements, once the home screen is drawn
  var planTimer = null;
  var alerted = "";       // the last reminder that rang on this phone
  var sheet = null;       // the plan sheet, while it is open

  function planPage() {
    var el = document.createElement("div");
    el.className = "plan-page";
    el.tabIndex = 0;
    el.setAttribute("role", "button");
    el.setAttribute("aria-label", t("planTitle"));
    el.addEventListener("keydown", function (ev) { if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); openPlan(); } });
    el.innerHTML = '<div class="pl-ring"><div class="donut"><svg viewBox="0 0 44 44"><circle class="track" cx="22" cy="22" r="19"/>' +
      '<circle class="value" cx="22" cy="22" r="19" stroke-dasharray="119.4" stroke-dashoffset="119.4"/></svg>' +
      '<b><span class="pl-time">+</span><span class="pl-unit"></span></b></div></div>' +
      '<div class="pl-info"><div class="pl-label"></div><div class="pl-title"></div><div class="pl-sub"></div><div class="pl-next"></div></div>';
    planEls = { root: el, ring: el.querySelector(".value"), time: el.querySelector(".pl-time"), unit: el.querySelector(".pl-unit"),
      label: el.querySelector(".pl-label"), title: el.querySelector(".pl-title"), sub: el.querySelector(".pl-sub"), next: el.querySelector(".pl-next"), state: "" };
    el.addEventListener("click", openPlan);
    return el;
  }

  function pad2(n) { return (n < 10 ? "0" : "") + n; }
  function dayKey(d) { return d.getFullYear() + "-" + pad2(d.getMonth() + 1) + "-" + pad2(d.getDate()); }
  function minutesOf(hhmm) { return +hhmm.slice(0, 2) * 60 + +hhmm.slice(3); }

  function isDone(it, key) { return !!it.done_on && (!!it.date || it.done_on === key); }
  function sortTasks(list, key) {
    return list.sort(function (a, b) {
      var done = Number(isDone(a, key)) - Number(isDone(b, key));
      if (done) return done;
      var at = a.time || "99:99", bt = b.time || "99:99";
      return at < bt ? -1 : at > bt ? 1 : 0;
    });
  }
  function itemsOn(key) {
    return sortTasks(planItems.filter(function (it) { return !it.date || it.date === key; }), key);
  }
  function planText(el, value) { if (el.textContent !== value) el.textContent = value; }

  // The home card shows the next alarm as a live countdown: hours and minutes
  // while it is far off, minutes and seconds in the last hour with the ring
  // running down once a second, and a pulse when it is due.
  function renderPlan() {
    var e = planEls, now = new Date(), key = dayKey(now);
    if (e) {
      var today = itemsOn(key), pending = today.filter(function (it) { return !isDone(it, key); });
      var next = pending[0], after = pending[1];
      var center = "+", unit = "", ratio = 0, state = "idle", title = t("planTapAdd"), sub = "";
      if (next && next.time) {
        var rem = minutesOf(next.time) * 60 - (now.getHours() * 3600 + now.getMinutes() * 60 + now.getSeconds());
        title = next.time;
        sub = next.title || "";
        if (rem > 0) {
          if (rem >= 3600) { center = Math.floor(rem / 3600) + ":" + pad2(Math.floor(rem % 3600 / 60)); unit = t("h") + " " + t("m"); ratio = 1; state = "run"; }
          else { center = pad2(Math.floor(rem / 60)) + ":" + pad2(rem % 60); unit = t("m") + " " + t("sec"); ratio = rem / 3600; state = rem <= 60 ? "soon" : "run"; }
        } else {
          center = "!"; unit = rem > -60 ? t("planNow") : t("planLate"); ratio = 1; state = "late";
        }
        if (rem <= 0 && rem > -120 && alerted !== next.id + key + next.time) {
          alerted = next.id + key + next.time;
          toast(next.time + (next.title ? " · " + next.title : ""));
          try { if (navigator.vibrate) navigator.vibrate([300, 150, 300]); } catch (x) { /* not supported */ }
        }
      } else if (next) {
        center = "•"; ratio = 0; state = "run"; title = next.title;
      } else if (today.length) {
        center = "✓"; ratio = 1; state = "done"; title = t("planDone");
      }
      planText(e.time, center);
      planText(e.unit, unit);
      planText(e.label, state === "idle" ? t("plan") : t("planNextAlarm"));
      planText(e.title, title);
      planText(e.sub, sub);
      planText(e.next, after ? t("planAfter", (after.time ? after.time + " " : "") + (after.title || "")) : "");
      if (e.state !== state) {
        e.root.classList.remove("st-idle", "st-run", "st-soon", "st-late", "st-done");
        e.root.classList.add("st-" + state);
        e.state = state;
      }
      var offset = (119.4 * (1 - ratio)).toFixed(1);
      if (e.ring.getAttribute("stroke-dashoffset") !== offset) e.ring.setAttribute("stroke-dashoffset", offset);
    }
    if (sheet) fillSheet();
  }

  // A drum for picking a number (hours, minutes). It loops round, is dragged
  // with the finger and keeps rolling a little after letting go, then settles on
  // one number. Only seven rows exist at a time; they are placed from the
  // position, so dragging costs almost nothing. Own touch handling (not native
  // scrolling) so it behaves the same every time.
  var DRUM_H = 40;
  function drum(count, onChange) {
    var box = document.createElement("div"), rows = [], pos = 0, shown = 0, raf = 0, anim = null;
    box.className = "drum";
    box.setAttribute("role", "spinbutton");
    box.tabIndex = 0;
    for (var i = 0; i < 7; i++) {
      var r = document.createElement("i");
      rows.push(r);
      box.appendChild(r);
    }
    function mod(n) { return ((n % count) + count) % count; }
    function paint() {
      var base = Math.round(pos);
      for (var k = -3; k <= 3; k++) {
        var d = base + k - pos, ad = Math.abs(d), r = rows[k + 3];
        var text = pad2(mod(base + k));
        if (r.textContent !== text) r.textContent = text;
        r.style.transform = "translateY(" + (d * DRUM_H).toFixed(1) + "px) rotateX(" + (-d * 26).toFixed(1) + "deg) scale(" + Math.max(0.6, 1 - ad * 0.1).toFixed(3) + ")";
        r.style.opacity = Math.max(0, 1 - ad * 0.32).toFixed(2);
        r.className = ad < 0.5 ? "on" : "";
      }
      var v = mod(base);
      if (v !== shown) { shown = v; box.setAttribute("aria-valuenow", String(v)); if (onChange) onChange(); }
    }
    function stop() { if (raf) cancelAnimationFrame(raf); raf = 0; anim = null; }
    // Roll to a position (in rows) with an ease-out.
    function roll(target, ms) {
      stop();
      var from = pos, t0 = Date.now();
      ms = ms || 280;
      anim = function () {
        var k = Math.min(1, (Date.now() - t0) / ms), e = 1 - Math.pow(1 - k, 3);
        pos = from + (target - from) * e;
        paint();
        raf = k < 1 ? requestAnimationFrame(anim) : 0;
      };
      anim();
    }
    var down = false, y0 = 0, p0 = 0, hist = [];
    box.addEventListener("pointerdown", function (e) {
      down = true; y0 = e.clientY; p0 = pos; hist = [[Date.now(), e.clientY]];
      stop();
      try { box.setPointerCapture(e.pointerId); } catch (x) { /* not supported */ }
    });
    box.addEventListener("pointermove", function (e) {
      if (!down) return;
      pos = p0 - (e.clientY - y0) / DRUM_H;
      var now = Date.now();
      hist.push([now, e.clientY]);
      while (hist.length > 2 && now - hist[0][0] > 120) hist.shift();
      paint();
    });
    function release(e) {
      if (!down) return;
      down = false;
      var moved = Math.abs(e.clientY - y0);
      if (moved < 6) {
        // A tap: the row above or below the middle steps one way.
        var rect = box.getBoundingClientRect(), off = (e.clientY - rect.top - rect.height / 2) / DRUM_H;
        roll(Math.round(pos) + (Math.abs(off) < 0.5 ? 0 : Math.round(off)), 220);
        return;
      }
      var first = hist[0], last = hist[hist.length - 1], dt = Math.max(1, last[0] - first[0]);
      var v = (first[1] - last[1]) / dt / DRUM_H;          // rows per ms, positive: towards bigger numbers
      if (Date.now() - last[0] > 90) v = 0;                // held still before letting go
      var target = Math.round(pos + Math.max(-14, Math.min(14, v * 260)));
      roll(target, 260 + Math.min(500, Math.abs(target - pos) * 45));
    }
    box.addEventListener("pointerup", release);
    box.addEventListener("pointercancel", release);
    box.addEventListener("wheel", function (e) { e.preventDefault(); roll(Math.round(pos) + (e.deltaY > 0 ? 1 : -1), 160); }, { passive: false });
    box.addEventListener("keydown", function (e) {
      if (e.key === "ArrowUp" || e.key === "ArrowDown") { e.preventDefault(); roll(Math.round(pos) + (e.key === "ArrowDown" ? 1 : -1), 160); }
    });
    paint();
    return {
      el: box,
      get: function () { return mod(Math.round(pos)); },
      // Rolls to a number by the shortest way round.
      set: function (v, animate) {
        var cur = Math.round(pos), diff = mod(v) - mod(cur);
        if (diff > count / 2) diff -= count; else if (diff < -count / 2) diff += count;
        if (animate) roll(cur + diff, 380); else { stop(); pos = cur + diff; paint(); }
      }
    };
  }

  function openPlan() {
    if (sheet) return;
    var el = document.createElement("div");
    el.className = "plan-sheet";
    el.innerHTML = '<div class="ps-box" role="dialog" aria-modal="true" aria-labelledby="ps-heading">' +
      '<div class="ps-header"><h2 class="ps-head" id="ps-heading"></h2><span class="ps-count"></span><button class="ps-close" type="button"></button></div>' +
      '<div class="ps-body">' +
      '<form class="ps-form" autocomplete="off">' +
      '<div class="ps-clock"><div class="ps-drums"><i class="ps-band"></i><span class="ps-colon">:</span></div><div class="ps-eta"></div></div>' +
      '<div class="ps-side"><div class="ps-quick"></div>' +
      '<input class="ps-name" maxlength="60" enterkeyhint="done">' +
      '<div class="ps-days" role="radiogroup"></div>' +
      '<button type="submit" class="ps-add"></button></div></form>' +
      '<div class="ps-items"></div></div></div>';
    el.querySelector(".ps-head").textContent = t("planTitle");
    var name = el.querySelector(".ps-name"), addBtn = el.querySelector(".ps-add");
    name.placeholder = t("planName");
    name.setAttribute("aria-label", t("planName"));
    addBtn.textContent = t("planSet");
    var closeBtn = el.querySelector(".ps-close");
    closeBtn.textContent = "×";
    closeBtn.setAttribute("aria-label", t("planClose"));

    var etaEl = el.querySelector(".ps-eta"), drums = el.querySelector(".ps-drums");
    var hours = drum(24, eta), mins = drum(60, eta);
    drums.insertBefore(hours.el, drums.children[1]);
    drums.appendChild(mins.el);
    hours.el.setAttribute("aria-label", t("h"));
    mins.el.setAttribute("aria-label", t("m"));
    function untilText(m) {
      return (m >= 60 ? Math.floor(m / 60) + " " + t("h") + " " : "") + (m % 60) + " " + t("m");
    }
    // How long until the chosen time (the next time it comes round).
    function eta() {
      var now = new Date(), m = hours.get() * 60 + mins.get() - now.getHours() * 60 - now.getMinutes();
      if (m < 0) m += 1440;
      etaEl.textContent = t("clLeft", untilText(m));
    }

    // Quick picks: a time counted from now.
    var quick = el.querySelector(".ps-quick");
    [[10, "+10 " + t("m")], [30, "+30 " + t("m")], [60, "+1 " + t("h")]].forEach(function (q) {
      var b = document.createElement("button");
      b.type = "button"; b.textContent = q[1];
      b.addEventListener("click", function () {
        var at = new Date(Date.now() + q[0] * 60000);
        hours.set(at.getHours(), true); mins.set(at.getMinutes(), true);
      });
      quick.appendChild(b);
    });

    var day = "today", days = el.querySelector(".ps-days");
    [["today", t("planToday")], ["tomorrow", t("planTomorrow")], ["every", t("planEvery")]].forEach(function (d) {
      var b = document.createElement("button");
      b.type = "button"; b.textContent = d[1];
      b.setAttribute("role", "radio");
      b.className = d[0] === day ? "on" : "";
      b.setAttribute("aria-checked", String(d[0] === day));
      b.addEventListener("click", function () {
        day = d[0];
        Array.prototype.forEach.call(days.children, function (c) { c.classList.toggle("on", c === b); c.setAttribute("aria-checked", String(c === b)); });
      });
      days.appendChild(b);
    });

    function close() {
      if (window.visualViewport) { window.visualViewport.removeEventListener("resize", fit); window.visualViewport.removeEventListener("scroll", fit); }
      sheet = null; el.remove();
      if (planEls) planEls.root.focus();
    }
    function fit() {
      var v = window.visualViewport;
      if (v) { el.style.height = v.height + "px"; el.style.top = v.offsetTop + "px"; }
    }
    closeBtn.addEventListener("click", close);
    el.addEventListener("click", function (ev) { if (ev.target === el) close(); });
    el.addEventListener("keydown", function (ev) { if (ev.key === "Escape") close(); });
    var doneTimer = null;
    el.querySelector(".ps-form").addEventListener("submit", function (ev) {
      ev.preventDefault();
      var when = new Date(); if (day === "tomorrow") when.setDate(when.getDate() + 1);
      var at = pad2(hours.get()) + ":" + pad2(mins.get());
      // The name is optional: an alarm needs only its time.
      if (send({ type: "planadd", title: name.value.trim(), time: at, date: day === "every" ? "" : dayKey(when) })) {
        name.value = ""; name.blur();
        addBtn.classList.add("ok"); addBtn.textContent = "✓ " + t("planSetDone");
        clearTimeout(doneTimer);
        doneTimer = setTimeout(function () { addBtn.classList.remove("ok"); addBtn.textContent = t("planSet"); }, 1400);
      } else toast(t("notConnected"));
    });
    el.querySelector(".ps-items").addEventListener("click", function (ev) {
      var button = ev.target.closest("button[data-id]");
      if (button && !send({ type: button.dataset.action, id: button.dataset.id, date: dayKey(new Date()) })) toast(t("notConnected"));
    });
    document.body.appendChild(el);
    sheet = el;
    if (window.visualViewport) { window.visualViewport.addEventListener("resize", fit); window.visualViewport.addEventListener("scroll", fit); }
    fit(); fillSheet();
    // Starts on the next full hour.
    var soon = new Date(Date.now() + 3600000);
    hours.set(soon.getHours()); mins.set(0);
    eta();
  }

  // The alarm list on the right of the sheet: the next one first, done ones last.
  function fillSheet() {
    var now = new Date(), key = dayKey(now), nowMin = now.getHours() * 60 + now.getMinutes();
    var list = sortTasks(planItems.slice(), key);
    var stamp = key + ":" + nowMin + ":" + JSON.stringify(planItems);
    if (sheet._stamp === stamp) return;
    sheet._stamp = stamp;
    var box = sheet.querySelector(".ps-items"), scroll = box.scrollTop;
    var active = document.activeElement, focusID = active && active.dataset && active.dataset.id, focusAction = active && active.dataset && active.dataset.action;
    box.textContent = "";
    var pending = list.filter(function (it) { return !isDone(it, key); }).length;
    sheet.querySelector(".ps-count").textContent = list.length ? t("planActive", pending) : "";
    if (!list.length) {
      var empty = document.createElement("div");
      empty.className = "ps-empty";
      empty.innerHTML = svg('<circle cx="12" cy="13" r="8"/><path d="M12 9v4l3 2M5 3 2 6M19 3l3 3"/>') ;
      empty.appendChild(textNode("", t("planEmpty")));
      box.appendChild(empty);
    }
    list.forEach(function (it) {
      var done = isDone(it, key), row = document.createElement("div");
      var late = false, sub = "";
      if (done) sub = t("planDoneTag");
      else if (it.time) {
        var rem = minutesOf(it.time) - nowMin;
        late = rem < 0;
        sub = rem === 0 ? t("planNow") : late ? t("planLate") : t("clLeft", (rem >= 60 ? Math.floor(rem / 60) + " " + t("h") + " " : "") + rem % 60 + " " + t("m"));
      }
      row.className = "ps-row" + (done ? " done" : "") + (late ? " late" : "");
      var toggle = document.createElement("button");
      toggle.type = "button"; toggle.className = "ps-toggle";
      toggle.dataset.id = it.id; toggle.dataset.action = "plantoggle";
      toggle.setAttribute("role", "checkbox"); toggle.setAttribute("aria-checked", String(done));
      toggle.setAttribute("aria-label", t("planToggle") + ": " + (it.title || it.time));
      toggle.innerHTML = svg('<path d="m5 12.5 4.5 4.5L19 7.5"/>');
      row.appendChild(toggle);
      var text = document.createElement("div");
      text.className = "ps-text";
      text.appendChild(textNode("ps-time-text", it.time || "—"));
      text.appendChild(textNode("ps-what", it.title || (it.date ? (it.date === key ? t("planToday") : it.date) : t("planEvery"))));
      text.appendChild(textNode("ps-sub", (it.title ? (it.date ? (it.date === key ? t("planToday") : it.date) : t("planEvery")) + (sub ? " · " : "") : "") + sub));
      row.appendChild(text);
      var del = document.createElement("button");
      del.type = "button"; del.className = "ps-del";
      del.dataset.id = it.id; del.dataset.action = "plandel";
      del.setAttribute("aria-label", t("planDel") + ": " + (it.title || it.time)); del.textContent = "×";
      row.appendChild(del); box.appendChild(row);
      if (focusID === it.id) (focusAction === "plantoggle" ? toggle : del).focus();
    });
    box.scrollTop = scroll;
  }

  // Moves the pager to a page (0, 1 or 2). instant skips the animation.
  function setMiniPage(p, instant) {
    miniPage = p;
    var track = document.querySelector(".mini-track");
    if (track) {
      track.style.transition = instant ? "none" : "";
      track.style.transform = p ? "translateX(-" + (p * 100 / PAGES).toFixed(3) + "%)" : "";
      var dots = track.parentNode.querySelectorAll(".mini-dots i");
      for (var i = 0; i < dots.length; i++) dots[i].classList.toggle("on", i === p);
    }
    syncStreams();
  }

  // A horizontal swipe on the card follows the finger and switches page.
  function bindPager(el, track) {
    var x0 = null, y0 = 0, axis = null, dx = 0, w = 1;
    el.addEventListener("touchstart", function (e) {
      x0 = e.touches[0].clientX;
      y0 = e.touches[0].clientY;
      axis = null;
      dx = 0;
      w = el.offsetWidth || 1;
    }, { passive: true });
    el.addEventListener("touchmove", function (e) {
      if (x0 === null) return;
      dx = e.touches[0].clientX - x0;
      var dy = e.touches[0].clientY - y0;
      if (!axis && (Math.abs(dx) > 8 || Math.abs(dy) > 8)) axis = Math.abs(dx) > Math.abs(dy) ? "x" : "y";
      if (axis !== "x") return;
      e.preventDefault();
      // Pulling past the first or last page gives a little, then resists.
      var base = -miniPage * w;
      var pos = base + dx;
      var low = -(PAGES - 1) * w;
      if (pos > 0) pos *= 0.3;
      if (pos < low) pos = low + (pos - low) * 0.3;
      track.style.transition = "none";
      track.style.transform = "translateX(" + pos.toFixed(1) + "px)";
    }, { passive: false });
    function end() {
      if (x0 === null) return;
      x0 = null;
      if (axis !== "x") return;
      var next = miniPage;
      if (dx < -40) next = Math.min(PAGES - 1, miniPage + 1); else if (dx > 40) next = Math.max(0, miniPage - 1);
      setMiniPage(next);
    }
    el.addEventListener("touchend", end);
    el.addEventListener("touchcancel", end);
  }

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
    claudeEls = null;
    planEls = null;
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
    if (locked && !list.some(function (s) { return s.id === locked; })) locked = null;
    if (!locked && Date.now() - pickedAt > FOLLOW_AFTER) {
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
    // The full screen player stays on the source it was opened on.
    var held = fullHold && by(function (s) { return s.id === fullHold; });
    if (held) return held;
    // A locked source stays until the user swipes. A picked one is only kept
    // while it plays (or was just picked): once it is paused, whatever else
    // is playing takes over instead of a stale paused tab staying on screen.
    var lk = locked && by(function (s) { return s.id === locked; });
    if (lk) return lk;
    var sel = selected && by(function (s) { return s.id === selected; });
    if (sel && (sel.playing || Date.now() - pickedAt < 15000 || !by(function (s) { return s.playing; }))) return sel;
    return by(function (s) { return s.playing && s.id === cur; }) ||
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
    if (locked) locked = id; // swiping moves the lock along
    if (byUser) pickedAt = Date.now();
  }

  var locked = null;        // ID of the source the user pinned, null when unpinned
  var LOCK_ICON = '<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>';
  var UNLOCK_ICON = '<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 7.5-2"/>';

  function toggleLock() {
    var cur = shownSource();
    locked = locked ? null : cur ? cur.id : null;
    if (!locked) { selected = null; pickedAt = 0; }
    renderMedia(true);
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
    r.classList.add("sliding");
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
      setTimeout(function () { r.classList.remove("sliding"); }, 600);
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
    var lockState = (locked ? "1" : "0") + (m.id ? "1" : "0");
    if (np.lockState !== lockState) {
      np.lockState = lockState;
      np.lock.classList.toggle("on", !!locked);
      np.lock.setAttribute("aria-pressed", String(!!locked));
      np.lock.setAttribute("aria-label", t(locked ? "unpinMedia" : "pinMedia"));
      np.lock.innerHTML = svg(locked ? LOCK_ICON : UNLOCK_ICON);
    }
    np.lock.style.display = m.id ? "" : "none";
    np.root.classList.toggle("idle", !has);
    np.root.classList.toggle("playing", has && m.playing);

    var app = has ? (m.app || t("media")) + (m.playing ? "" : " · " + t("paused")) : t("notPlaying");
    var title = has ? m.title : t("nothingPlaying");
    var artist = has ? (m.artist || m.album || "") : t("startMusic");
    // Compare with the text the card is heading for, not the one on screen: a
    // swap may be waiting, and a reading that goes back to the old track must
    // cancel it instead of leaving the wrong track up.
    var wanted = title + " " + artist;
    if (instant) {
      clearTimeout(np.textTimer);
      np.wanted = wanted;
      np.meta.classList.remove("out", "in");
      np.title.textContent = title;
      np.artist.textContent = artist;
    } else if (np.wanted !== wanted) {
      np.wanted = wanted;
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
    // The server repeats this about every second; only touch the DOM on a change.
    if (np.appText !== app) { np.appText = app; np.app.textContent = app; }
    var playing = has && m.playing;
    if (np.glyphPlaying !== playing) { np.glyphPlaying = playing; np.play.innerHTML = svg(playing ? GLYPH.pause : GLYPH.play); }

    playback = has ? { pos: m.position_ms, dur: m.duration_ms, playing: m.playing, t: Date.now() } : null;
    tickProgress(true);
    // The slider shows the app's own volume, or the PC's if the app is not in the mixer.
    var own = m.id && m.volume >= 0;
    np.volSource = own ? m.id : "";
    np.slider.classList.toggle("app", !!own);
    var v = own ? m.volume : mediaState.volume, muted = own ? m.muted : mediaState.muted;
    if (v >= 0 && Date.now() > volumeLockUntil) setVolume(v, muted);
    if (full) fillFull();
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
      r.classList.add("sliding");
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
      setTimeout(function () { r.classList.remove("sliding"); }, 500);
    }
    el.addEventListener("touchend", end);
    el.addEventListener("touchcancel", end);
  }

  // ---- Full screen player --------------------------------------------------
  // Tapping the cover opens a big player; a tap anywhere but the volume
  // slider and the buttons closes it again.

  var full = null;
  var fullTimer = null;
  var fullHold = null;      // ID of the source the full screen player is showing

  function buildFull() {
    var el = document.createElement("div");
    el.className = "np-full";
    el.innerHTML = '<div class="nf-bg"><i></i><i></i><i></i></div>' +
      '<div class="nf-art">' + svg('<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>') + '<img alt=""></div>' +
      '<div class="nf-side"><div class="nf-meta"><span class="nf-app"></span><span class="nf-title"></span><span class="nf-artist"></span></div>' +
      '<div class="np-progress"><div class="track"><i></i></div><div class="times"><span class="pos">0:00</span><span class="dur">0:00</span></div></div>' +
      '<div class="transport"></div>' +
      '<div class="vol">' + svg('<path d="M11 5 6 9H2v6h4l5 4V5z"/><path d="M15.5 8.5a5 5 0 0 1 0 7"/>', "nf-spk") +
      '<div class="slider" role="slider" aria-label="Volume" aria-valuemin="0" aria-valuemax="100"><i></i><span class="pct"></span></div></div></div>';
    var transport = el.querySelector(".transport");
    var glyphs = [GLYPH.prev, GLYPH.play, GLYPH.next], acts = ["prev", "toggle", "next"];
    glyphs.forEach(function (g, i) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "tbtn" + (i === 1 ? " main" : "");
      b.innerHTML = svg(g);
      b.addEventListener("click", function () { mediaControl(acts[i]); });
      transport.appendChild(b);
    });
    var slider = el.querySelector(".slider");
    bindSlider(slider);
    el.addEventListener("click", function (e) {
      if (e.target.closest && e.target.closest(".slider, .tbtn")) return;
      closeFull();
    });
    document.body.appendChild(el);
    full = {
      root: el, bg: el.querySelector(".nf-bg"), art: el.querySelector(".nf-art"), img: el.querySelector(".nf-art img"),
      app: el.querySelector(".nf-app"), title: el.querySelector(".nf-title"), artist: el.querySelector(".nf-artist"),
      pos: el.querySelector(".pos"), dur: el.querySelector(".dur"), bar: el.querySelector(".track i"),
      play: transport.children[1], slider: slider, level: slider.querySelector("i"), spk: el.querySelector(".nf-spk"),
      pct: el.querySelector(".pct"), vol: el.querySelector(".vol"), src: "", bgCss: "", glyphPlaying: null
    };
  }

  function openFull() {
    if (!np || !mediaState || !shownSource() || !shownSource().title) return;
    if (!full) buildFull();
    clearTimeout(fullTimer);
    fullHold = shownSource().id;
    fillFull();
    if (np.state) setVolume(np.state.volume, np.state.muted);
    tickProgress(true);
    void full.root.offsetWidth;
    full.root.classList.add("on");
  }

  function closeFull() {
    if (!full) return;
    var was = full.root.classList.contains("on");
    full.root.classList.remove("on");
    fullHold = null;
    if (was && np && mediaState) renderMedia(true);
  }

  // Copies what the card shows onto the full screen player.
  function fillFull() {
    if (!full || !np) return;
    var m = shownSource() || {};
    if (!m.title) { closeFull(); return; }
    if (np.lastArt && full.src !== np.lastArt) {
      full.src = np.lastArt;
      full.img.src = np.lastArt;
    }
    full.art.classList.toggle("has-art", !!np.lastArt);
    // The backdrop drifts in the cover's two colors; on a new cover it fades out, takes the new colors and fades back.
    var pal = np.pals && np.pals[np.front];
    var key = pal ? pal[0] + "|" + pal[1] : "";
    if (pal && key !== full.bgCss) {
      var first = !full.bgCss;
      full.bgCss = key;
      clearTimeout(full.bgTimer);
      var paint = function () {
        full.bg.style.setProperty("--c1", pal[0]);
        full.bg.style.setProperty("--c2", pal[1]);
        full.bg.style.opacity = "1";
      };
      if (first) paint();
      else { full.bg.style.opacity = "0"; full.bgTimer = setTimeout(paint, 450); }
    }
    var app = (m.app || t("media")) + (m.playing ? "" : " · " + t("paused"));
    planText(full.app, app);
    planText(full.title, m.title);
    planText(full.artist, m.artist || m.album || "");
    var playing = !!m.playing;
    if (full.glyphPlaying !== playing) { full.glyphPlaying = playing; full.play.innerHTML = svg(playing ? GLYPH.pause : GLYPH.play); }
    var own = m.id && m.volume >= 0;
    full.vol.classList.toggle("app", !!own);
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
    if (full) {
      full.pos.textContent = np.pos.textContent;
      full.dur.textContent = np.dur.textContent;
      full.bar.style.transition = np.bar.style.transition;
      full.bar.style.transform = np.bar.style.transform;
    }
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
    if (full) {
      full.level.style.transform = np.level.style.transform;
      if (full.spk.innerHTML !== glyph) full.spk.innerHTML = glyph;
      full.pct.textContent = v;
      full.vol.classList.toggle("muted", muted);
    }
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

  function makeTrackpad(el, hint) {
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
      if (hint && !hint.classList.contains("gone")) hint.classList.add("gone");
      for (var i = 0; i < e.changedTouches.length; i++) {
        var t = e.changedTouches[i];
        var zone = t.target.closest && t.target.closest(".pad-zone, .dev-zone");
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
    function reset() {
      Object.keys(zones).forEach(function (id) {
        zones[id].classList.remove("down");
        pad({ action: "up", button: zones[id].dataset.btn });
      });
      if (g && g.drag) pad({ action: "up", button: "left" });
      zones = {};
      touches = {};
      g = null;
      pend.x = pend.y = pend.sx = pend.sy = 0;
    }
    document.addEventListener("visibilitychange", function () { if (document.visibilityState !== "visible") reset(); });
    return reset;
  }
  // The trackpad screen, and the small one beside the prompt in developer mode.
  var bigPadReset = makeTrackpad($("view-pad"), $("pad-hint"));
  var miniPadReset = $("dev-pad") ? makeTrackpad($("dev-pad"), null) : function () {};
  padReset = function () { bigPadReset(); miniPadReset(); };

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
    np.pals = np.pals || [];
    np.pals[layer] = c;
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

  var standalone = framed || !!(navigator.standalone || (window.matchMedia && window.matchMedia("(display-mode: standalone)").matches));
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

  // "Show sideways" on the rotate screen: open the panel in the turned frame.
  var rotateGo = $("rotate-go");
  if (rotateGo) {
    if (framed) rotateGo.hidden = true;
    rotateGo.addEventListener("click", function () {
      orientReport("button");
      try {
        if (!rotPref()) localStorage.setItem(ROT_KEY, "90");
        sessionStorage.setItem(ROT_KEY, "now");
      } catch (e) { /* no storage */ }
      setTimeout(function () { location.reload(); }, 300);
    });
  }
  // Tell the PC log what the phone reports when it opens upright, to see
  // which way iOS got it wrong.
  setTimeout(function () { if (window.innerHeight > window.innerWidth) orientReport("upright at start"); }, 1200);

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
