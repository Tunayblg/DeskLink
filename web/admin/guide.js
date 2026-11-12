// User guide: two books (iPhone and Android) that turn their pages.
// All text here is written by us and inserted as HTML; nothing comes from outside.
(function () {
  "use strict";
  var root = document.getElementById("guide-root");
  if (!root) return;

  var LANG = /^tr\b/i.test(navigator.language || "") ? "tr" : "en";
  var BOOK_W = 880, BOOK_H = 560;

  var UI = {
    en: {
      title: "User guide", exit: "Close",
      pickSub: "Which device are you setting up?",
      iosName: "iPhone / iPad", iosSub: "Safari, then Add to Home Screen. Works on old devices too.",
      andName: "Android", andSub: "Install the DeskLink app (APK) for a real full-screen experience.",
      open: "Open the guide", change: "Change device", page: "Page {} of {}", prev: "Previous page", next: "Next page"
    },
    tr: {
      title: "Kılavuz", exit: "Kapat",
      pickSub: "Hangi cihazı kuruyorsun?",
      iosName: "iPhone / iPad", iosSub: "Safari, ardından Ana Ekrana Ekle. Eski cihazlarda da çalışır.",
      andName: "Android", andSub: "Tam ekran deneyim için DeskLink uygulamasını (APK) kur.",
      open: "Kılavuzu aç", change: "Cihazı değiştir", page: "Sayfa {} / {}", prev: "Önceki sayfa", next: "Sonraki sayfa"
    }
  }[LANG];

  // ---- Little drawings, one per page ---------------------------------------

  var ART = {
    wifi: '<svg viewBox="0 0 160 120"><path class="f" d="M80 100l-9-9a13 13 0 0 1 18 0z"/><path d="M30 60a70 70 0 0 1 100 0"/><path d="M47 77a45 45 0 0 1 66 0"/><path d="M64 93a20 20 0 0 1 32 0"/><circle class="d" cx="80" cy="104" r="5"/></svg>',
    pc: '<svg viewBox="0 0 160 120"><rect class="f" x="20" y="14" width="120" height="76" rx="8"/><rect x="20" y="14" width="120" height="76" rx="8"/><path d="M62 108h36M80 90v18"/><path class="s" d="M44 70V42h28v28zM86 42h30M86 56h22"/></svg>',
    qr: '<svg viewBox="0 0 160 120"><rect class="f" x="40" y="10" width="80" height="100" rx="8"/><path d="M52 24h20v20H52zM88 24h20v20H88zM52 62h20v20H52z"/><path class="s" d="M88 62h8M104 62h4M88 74h20M96 86h12M88 86h2"/></svg>',
    code: '<svg viewBox="0 0 160 120"><path d="M14 44h20v32H14zM40 44h20v32H40zM66 44h20v32H66zM96 44h20v32H96zM122 44h20v32h-20z" class="f"/><path d="M14 44h20v32H14zM40 44h20v32H40zM66 44h20v32H66zM96 44h20v32H96zM122 44h20v32h-20z"/><path class="s" d="M24 54v12M50 54v12M76 54v12M106 54v12M132 54v12"/></svg>',
    check: '<svg viewBox="0 0 160 120"><circle class="f" cx="80" cy="60" r="44"/><circle cx="80" cy="60" r="44"/><path class="s" d="M60 62l14 14 28-30"/></svg>',
    share: '<svg viewBox="0 0 160 120"><rect class="f" x="42" y="42" width="76" height="64" rx="10"/><path d="M42 58V52a10 10 0 0 1 10-10h12M118 58V52a10 10 0 0 0-10-10H96"/><path class="s" d="M80 76V14M62 32l18-18 18 18"/></svg>',
    home: '<svg viewBox="0 0 160 120"><rect class="f" x="36" y="8" width="88" height="104" rx="12"/><rect x="36" y="8" width="88" height="104" rx="12"/><rect class="s" x="48" y="22" width="18" height="18" rx="5"/><rect class="s" x="71" y="22" width="18" height="18" rx="5"/><rect x="94" y="22" width="18" height="18" rx="5"/><rect class="s" x="48" y="46" width="18" height="18" rx="5"/><path d="M80 52v12M74 58h12" /></svg>',
    apk: '<svg viewBox="0 0 160 120"><rect class="f" x="30" y="46" width="100" height="58" rx="12"/><path d="M42 46a38 36 0 0 1 76 0"/><path d="M60 22l-8-10M100 22l8-10"/><circle class="d" cx="62" cy="34" r="3"/><circle class="d" cx="98" cy="34" r="3"/><path class="s" d="M80 60v26M68 76l12 12 12-12"/></svg>',
    lock: '<svg viewBox="0 0 160 120"><rect class="f" x="42" y="52" width="76" height="56" rx="10"/><rect x="42" y="52" width="76" height="56" rx="10"/><path d="M58 52V38a22 22 0 0 1 44 0v14"/><circle class="d" cx="80" cy="78" r="6"/></svg>',
    ip: '<svg viewBox="0 0 160 120"><rect class="f" x="16" y="30" width="128" height="60" rx="10"/><rect x="16" y="30" width="128" height="60" rx="10"/><path class="s" d="M32 60h22M62 60h22M92 60h22"/><circle class="d" cx="124" cy="60" r="4"/><path d="M80 90v16M56 106h48"/></svg>',
    shield: '<svg viewBox="0 0 160 120"><path class="f" d="M80 10l46 16v30c0 28-20 46-46 54-26-8-46-26-46-54V26z"/><path d="M80 10l46 16v30c0 28-20 46-46 54-26-8-46-26-46-54V26z"/><path class="s" d="M60 58l14 14 26-28"/></svg>',
    help: '<svg viewBox="0 0 160 120"><circle class="f" cx="80" cy="60" r="46"/><circle cx="80" cy="60" r="46"/><circle cx="80" cy="60" r="18"/><path d="M48 28l16 16M112 28L96 44M48 92l16-16M112 92L96 76"/></svg>',
    deck: '<svg viewBox="0 0 160 120"><rect class="f" x="30" y="12" width="100" height="96" rx="12"/><rect x="30" y="12" width="100" height="96" rx="12"/><rect class="s" x="42" y="26" width="34" height="26" rx="6"/><rect x="84" y="26" width="34" height="26" rx="6"/><rect x="42" y="62" width="34" height="26" rx="6"/><rect class="s" x="84" y="62" width="34" height="26" rx="6"/></svg>',
    link: '<svg viewBox="0 0 160 120"><rect class="f" x="26" y="26" width="108" height="68" rx="12"/><rect x="26" y="26" width="108" height="68" rx="12"/><path class="s" d="M70 52l-8 8a8 8 0 0 0 11 11l5-5M90 68l8-8a8 8 0 0 0-11-11l-5 5"/><path d="M44 106h72"/></svg>',
    watch: '<svg viewBox="0 0 160 120"><rect class="f" x="18" y="14" width="124" height="78" rx="10"/><rect x="18" y="14" width="124" height="78" rx="10"/><path class="s" d="M72 38v28l24-14z"/><path d="M60 108h40M80 92v16"/></svg>',
    folder: '<svg viewBox="0 0 160 120"><path class="f" d="M20 34a8 8 0 0 1 8-8h32l12 14h60a8 8 0 0 1 8 8v46a8 8 0 0 1-8 8H28a8 8 0 0 1-8-8z"/><path d="M20 34a8 8 0 0 1 8-8h32l12 14h60a8 8 0 0 1 8 8v46a8 8 0 0 1-8 8H28a8 8 0 0 1-8-8z"/></svg>',
    dev: '<svg viewBox="0 0 160 120"><rect class="f" x="18" y="16" width="124" height="88" rx="10"/><rect x="18" y="16" width="124" height="88" rx="10"/><path class="s" d="M58 46L42 60l16 14M102 46l16 14-16 14M88 40l-16 40"/></svg>',
    gear: '<svg viewBox="0 0 160 120"><circle class="f" cx="80" cy="60" r="40"/><circle cx="80" cy="60" r="40"/><circle class="s" cx="80" cy="60" r="14"/><path d="M80 12v14M80 94v14M32 60h14M114 60h14M46 26l10 10M104 84l10 10M46 94l10-10M104 36l10-10"/></svg>'
  };

  // ---- The books --------------------------------------------------------------
  // Each page: { art, step, h, p: [..], ol: [..], ul: [..], tip, warn }.
  // The first page is the cover, the last one is the back cover. Always an even count.

  var ADDR = '<code>http://192.168.1.20:8765</code>';

  var BOOKS = {
    ios: {
      en: [
        { cover: true, sub: "iPhone & iPad guide", text: "From the first connection to a full-screen deck on your desk.", tag: "About 1 minute · no App Store" },
        { art: "deck", h: "What you need", ul: [
            "DeskLink running on your PC. Its icon sits in the tray, next to the clock.",
            "An iPhone or iPad, even an old one.",
            "Both on the <b>same Wi-Fi</b>. Mobile data and guest Wi-Fi will not work.",
            "<b>Safari</b>. iOS only adds home screen apps from Safari."],
          tip: "No account, no cloud, no App Store. Everything stays on your home network." },
        { art: "pc", step: "Step 1", h: "Open the pairing screen", ol: [
            "Click the DeskLink icon in the tray to open the dashboard.",
            "Press <b>Add device</b> at the bottom left.",
            "A QR code and a 6-digit code appear."],
          tip: "The code changes every 30 seconds, so nobody can reuse an old one." },
        { art: "qr", step: "Step 2", h: "Scan the QR code", ol: [
            "Open the <b>Camera</b> app on the iPhone.",
            "Point it at the QR code on the PC screen.",
            "Tap the yellow banner that pops up. Safari opens DeskLink and pairs by itself."],
          tip: "Safari does not open? Press and hold the banner and choose <b>Open in Safari</b>." },
        { art: "check", step: "Step 3", h: "You are paired", p: [
            "When the phone shows the home screen, you are connected. DeskLink remembers this phone, so there is no pairing next time.",
            "On the PC, the <b>Devices</b> page lists every paired phone. You can remove one there at any time."] },
        { art: "share", step: "Step 4", h: "Add it to the Home Screen", ol: [
            "Tap the <b>Share</b> button (the square with an arrow) at the bottom of Safari.",
            "Scroll down and tap <b>Add to Home Screen</b>.",
            "Tap <b>Add</b>."],
          tip: "Do this in Safari. Chrome on iPhone cannot add full-screen apps." },
        { art: "home", step: "Step 5", h: "Open it like an app", p: [
            "Start DeskLink from its icon on the home screen. There is no address bar, it runs full screen.",
            "Tablets and iPads get a wider layout on their own. Swipe the desk clock for the analog face."],
          tip: "Tip: put the iPhone on a stand and let it charge. It makes a great desk companion." },
        { art: "link", h: "Tiles on the home screen", p: [
            "The home screen has six tiles. Each opens a game, a website or a program on the PC. Empty ones show a <b>+</b>."],
          ol: [
            "On the PC open DeskLink and go to <b>Games</b>. The <b>Home screen</b> list is below your games.",
            "Tap a tile. Pick one of your games, <b>Netflix</b>, <b>YouTube</b> and others, or paste any web address or program.",
            "Press <b>Save</b>. The phone updates at once."],
          tip: "The <b>Games</b> tab on the phone only shows up once you have added a game." },
        { art: "watch", h: "Watch mode", ul: [
            "Open the fourth tab, the screen with a play button.",
            "<b>Left:</b> the PC's volume. <b>Right:</b> screen brightness.",
            "The small button under the volume switches the sound between speakers, headphones and your monitor.",
            "<b>Middle:</b> rewind, play or pause and fast forward. They press keys, so the player window must be in front.",
            "The red power button shuts the PC down, after you confirm."],
          tip: "Brightness uses your monitor when it allows it (<b>DDC/CI</b>), a laptop's own panel, or otherwise dims the picture in software, so the bar works on any screen. Software dimming never goes fully black. Brightness is still in beta and may not work properly with every monitor." },
        { art: "dev", h: "Developer mode", ul: [
            "The <b>&lt;/&gt;</b> tab sends prompts to Claude, Codex, ChatGPT, a terminal or your editor on the PC.",
            "Type, or tap the <b>microphone</b> and speak. With <b>live typing</b> the words appear on the PC while you talk.",
            "The pill at the top picks the window. Only developer apps are listed, never chat or other programs.",
            "The bar on top shows what Claude or Codex is doing: thinking, running a command, editing files, or done.",
            "The small pad on the left is a mouse; the keys below press Esc, Tab, the arrows, Ctrl+C and Enter."],
          tip: "Dictation needs Siri and Dictation switched on in the phone's settings." },
        { art: "gear", h: "Settings", ul: [
            "The quiet gear at the bottom of the tab bar opens Settings.",
            "Switch any tab off, or move it up and down with the arrows.",
            "Turn live typing, usage, the agent status, sending after dictation and the dictation language on or off.",
            "<b>Calm mode</b> stops all animation."],
          tip: "Settings are saved on the phone itself, so each phone can have its own layout." },
        { art: "code", h: "Connecting manually", p: ["No camera, or the QR code will not open? Use the address instead."], ol: [
            "On the PC press <b>Add device</b>, then <b>Trouble scanning? Connect manually</b>.",
            "In Safari on the phone, type the address shown there, like " + ADDR + ".",
            "Type the 6-digit code from the PC and tap <b>Pair</b>."],
          tip: "Type the whole address: <b>http://</b>, the numbers and the <b>port</b> after the colon." },
        { art: "ip", h: "Finding your PC's address", ol: [
            "Press <b>Win + R</b>, type <code>cmd</code> and press Enter.",
            "Type <code>ipconfig</code> and press Enter.",
            "Look for <b>IPv4 Address</b> under your Wi-Fi or Ethernet, for example <code>192.168.1.20</code>.",
            "Your address is <code>http://</code> + that number + <code>:8765</code>."],
          tip: "8765 is the default port. The dashboard already shows the full address for you." },
        { art: "shield", h: "Windows Firewall", p: [
            "The first time DeskLink runs, Windows asks if it may use the network. Choose <b>Private networks</b> and press <b>Allow access</b>."],
          ol: ["If you pressed Cancel, open <b>Windows Security → Firewall &amp; network protection</b>.", "Choose <b>Allow an app through firewall</b> and tick DeskLink under <b>Private</b>."],
          warn: "Never forward the port on your router. DeskLink is meant for your home network only." },
        { art: "help", h: "If something goes wrong", ul: [
            "<b>The page does not open:</b> are both devices on the same Wi-Fi? Guest networks often block devices from seeing each other.",
            "<b>Offline banner:</b> the PC is asleep or DeskLink was closed. Check the tray.",
            "<b>\"No longer paired\":</b> remove the phone in Devices and scan a fresh QR code.",
            "<b>Worked yesterday, not today:</b> the router may have given the PC a new address. Scan the QR code again."] },
        { back: true, h: "Enjoy your deck", ul: [
            "<b>Games:</b> add them on the PC, tap to launch. The phone turns into a live CPU and GPU monitor.",
            "<b>Mixer:</b> set the volume for each program on its own.",
            "<b>Trackpad:</b> use the phone as a mouse for the PC. The <b>Windows</b> button lists the open programs (not background helpers) and brings one to the front.",
            "<b>Clock:</b> swipe left or right to change the face, up or down for the next or previous tab."] }
      ],
      tr: [
        { cover: true, sub: "iPhone ve iPad kılavuzu", text: "İlk bağlantıdan masandaki tam ekran kumanda paneline kadar.", tag: "Yaklaşık 1 dakika · App Store yok" },
        { art: "deck", h: "Neler lazım", ul: [
            "Bilgisayarında çalışan DeskLink. Simgesi saatin yanındaki tepsidedir.",
            "Bir iPhone veya iPad, eski olması sorun değil.",
            "İkisi de <b>aynı Wi-Fi</b>'da olmalı. Mobil veri ve misafir Wi-Fi çalışmaz.",
            "<b>Safari</b>. iOS ana ekrana uygulamayı yalnızca Safari'den ekler."],
          tip: "Hesap yok, bulut yok, App Store yok. Her şey ev ağında kalır." },
        { art: "pc", step: "Adım 1", h: "Eşleştirme ekranını aç", ol: [
            "Panoyu açmak için tepsideki DeskLink simgesine tıkla.",
            "Sol alttaki <b>Cihaz ekle</b> düğmesine bas.",
            "Bir QR kod ve 6 haneli bir kod görünür."],
          tip: "Kod 30 saniyede bir değişir, eski bir kod tekrar kullanılamaz." },
        { art: "qr", step: "Adım 2", h: "QR kodu okut", ol: [
            "iPhone'da <b>Kamera</b> uygulamasını aç.",
            "Bilgisayar ekranındaki QR koda doğrult.",
            "Çıkan sarı banner'a dokun. Safari DeskLink'i açar ve kendiliğinden eşleşir."],
          tip: "Safari açılmıyorsa banner'a basılı tut ve <b>Safari'de Aç</b>'ı seç." },
        { art: "check", step: "Adım 3", h: "Eşleştin", p: [
            "Telefon ana ekranı gösteriyorsa bağlandın. DeskLink bu telefonu hatırlar, bir dahaki sefer eşleştirme gerekmez.",
            "Bilgisayarda <b>Cihazlar</b> sayfası eşleşen tüm telefonları listeler. İstediğin zaman oradan kaldırabilirsin."] },
        { art: "share", step: "Adım 4", h: "Ana Ekrana ekle", ol: [
            "Safari'nin altındaki <b>Paylaş</b> düğmesine (oklu kare) dokun.",
            "Aşağı kaydır ve <b>Ana Ekrana Ekle</b>'ye dokun.",
            "<b>Ekle</b>'ye dokun."],
          tip: "Bunu Safari'de yap. iPhone'da Chrome tam ekran uygulama ekleyemez." },
        { art: "home", step: "Adım 5", h: "Uygulama gibi aç", p: [
            "DeskLink'i ana ekrandaki simgesinden başlat. Adres çubuğu yok, tam ekran çalışır.",
            "Tablet ve iPad'ler geniş düzeni kendiliğinden alır. Masa saatinde kaydırarak analog yüzü görebilirsin."],
          tip: "İpucu: iPhone'u bir standa koy ve şarjda bırak. Harika bir masa arkadaşı olur." },
        { art: "link", h: "Ana ekrandaki kutular", p: [
            "Ana ekranda altı kutu var. Her biri bilgisayarda bir oyun, web sitesi ya da program açar. Boş olanlarda <b>+</b> görünür."],
          ol: [
            "Bilgisayarda DeskLink'i aç ve <b>Oyunlar</b>'a git. <b>Ana ekran</b> listesi oyunlarının altında.",
            "Bir kutuya dokun. Oyunlarından birini, <b>Netflix</b>, <b>YouTube</b> ve diğerlerini seç ya da herhangi bir web adresi veya program yapıştır.",
            "<b>Kaydet</b>'e bas. Telefon hemen güncellenir."],
          tip: "Telefondaki <b>Oyunlar</b> sekmesi, en az bir oyun eklediğinde görünür." },
        { art: "watch", h: "Dizi modu", ul: [
            "Dördüncü sekmeyi aç, oynat işaretli ekran.",
            "<b>Solda:</b> bilgisayarın sesi. <b>Sağda:</b> ekran parlaklığı.",
            "Sesin altındaki küçük düğme sesi hoparlör, kulaklık ve monitör arasında değiştirir.",
            "<b>Ortada:</b> geri sar, oynat veya durdur ve ileri sar. Tuş bastıkları için oynatıcı penceresi önde olmalı.",
            "Kırmızı güç düğmesi, onayladıktan sonra bilgisayarı kapatır."],
          tip: "Parlaklık, monitör izin veriyorsa monitörün kendisini (<b>DDC/CI</b>), laptopta panelini, olmazsa görüntüyü yazılımla karartır. Böylece çubuk her ekranda çalışır. Yazılımla karartma ekranı hiçbir zaman tamamen karartmaz. Parlaklık henüz beta aşamasında, her monitörle düzgün çalışmayabilir." },
        { art: "dev", h: "Geliştirici modu", ul: [
            "<b>&lt;/&gt;</b> sekmesi bilgisayardaki Claude, Codex, ChatGPT, terminal ya da kod editörüne prompt gönderir.",
            "Yaz ya da <b>mikrofona</b> dokunup konuş. <b>Canlı yazma</b> açıksa sen konuşurken yazı bilgisayarda belirir.",
            "Üstteki düğmeden pencereyi seç. Yalnızca geliştirici uygulamaları listelenir, sohbet ve başka programlar asla.",
            "Üstteki çubuk Claude ya da Codex'in ne yaptığını gösterir: düşünüyor, komut çalıştırıyor, dosya düzenliyor ya da bitti.",
            "Soldaki küçük alan bir faredir; alttaki tuşlar Esc, Tab, oklar, Ctrl+C ve Enter'a basar."],
          tip: "Dikte için telefonun ayarlarında Siri ve Dikte açık olmalı." },
        { art: "gear", h: "Ayarlar", ul: [
            "Sekme çubuğunun en altındaki sade dişli Ayarlar'ı açar.",
            "İstediğin sekmeyi kapat ya da oklarla yukarı aşağı taşı.",
            "Canlı yazmayı, kullanımı, ajan durumunu, dikteden sonra göndermeyi ve dikte dilini aç ya da kapat.",
            "<b>Sakin mod</b> tüm animasyonları durdurur."],
          tip: "Ayarlar telefonun kendisinde saklanır, her telefonun kendi düzeni olabilir." },
        { art: "code", h: "Elle bağlanma", p: ["Kamera yok mu ya da QR kod açılmıyor mu? Adresi kullan."], ol: [
            "Bilgisayarda <b>Cihaz ekle</b>'ye, sonra <b>Okutamıyor musun? Elle bağlan</b>'a bas.",
            "Telefonda Safari'ye orada yazan adresi gir, örneğin " + ADDR + ".",
            "Bilgisayardaki 6 haneli kodu yaz ve <b>Eşleştir</b>'e dokun."],
          tip: "Adresi tam yaz: <b>http://</b>, sayılar ve iki noktadan sonra <b>port</b>." },
        { art: "ip", h: "Bilgisayarın adresini bulma", ol: [
            "<b>Win + R</b>'ye bas, <code>cmd</code> yaz ve Enter'a bas.",
            "<code>ipconfig</code> yaz ve Enter'a bas.",
            "Wi-Fi veya Ethernet altında <b>IPv4 Adresi</b>'ni bul, örneğin <code>192.168.1.20</code>.",
            "Adresin: <code>http://</code> + o sayı + <code>:8765</code>."],
          tip: "8765 varsayılan porttur. Pano tam adresi zaten senin için gösterir." },
        { art: "shield", h: "Windows Güvenlik Duvarı", p: [
            "DeskLink ilk çalıştığında Windows ağı kullanmasına izin verip vermeyeceğini sorar. <b>Özel ağlar</b>'ı seç ve <b>Erişime izin ver</b>'e bas."],
          ol: ["İptal'e bastıysan <b>Windows Güvenliği → Güvenlik duvarı ve ağ koruması</b>'nı aç.", "<b>Güvenlik duvarı üzerinden bir uygulamaya izin ver</b>'i seç ve DeskLink'i <b>Özel</b> altında işaretle."],
          warn: "Router'da portu asla dışarı açma. DeskLink yalnızca ev ağın için tasarlandı." },
        { art: "help", h: "Bir şey ters giderse", ul: [
            "<b>Sayfa açılmıyor:</b> iki cihaz da aynı Wi-Fi'da mı? Misafir ağlar cihazların birbirini görmesini çoğu zaman engeller.",
            "<b>Çevrimdışı uyarısı:</b> bilgisayar uykuda ya da DeskLink kapalı. Tepsiye bak.",
            "<b>\"Eşleşme kaldırıldı\":</b> telefonu Cihazlar'dan kaldır ve yeni bir QR kod okut.",
            "<b>Dün çalışıyordu, bugün çalışmıyor:</b> router bilgisayara yeni bir adres vermiş olabilir. QR kodu tekrar okut."] },
        { back: true, h: "İyi eğlenceler", ul: [
            "<b>Oyunlar:</b> bilgisayardan ekle, dokunup başlat. Telefon canlı CPU ve GPU monitörüne dönüşür.",
            "<b>Mikser:</b> her programın sesini ayrı ayrı ayarla.",
            "<b>Trackpad:</b> telefonu bilgisayar için fare olarak kullan. <b>Pencereler</b> düğmesi açık programları (arka plan yardımcıları hariç) listeler ve seçtiğini öne getirir.",
            "<b>Saat:</b> sağa sola kaydırarak yüzünü değiştir, yukarı aşağı kaydırarak sonraki ya da önceki sekmeye geç."] }
      ]
    },

    android: {
      en: [
        { cover: true, sub: "Android guide", text: "Install the DeskLink app and turn your Android into a Stream Deck.", tag: "About 2 minutes · no Play Store" },
        { art: "deck", h: "What you need", ul: [
            "DeskLink running on your PC. Its icon sits in the tray, next to the clock.",
            "An Android phone or tablet (Android 5 or newer).",
            "Both on the <b>same Wi-Fi</b>. Mobile data and guest Wi-Fi will not work.",
            "The <b>DeskLink.apk</b> app file from the Releases page."],
          tip: "The app is only a small full-screen window onto your PC. It does not collect anything." },
        { art: "apk", step: "Step 1", h: "Download the app", p: [
            "On the phone, open the DeskLink releases page in your browser and download <b>DeskLink.apk</b>:",
            "<code>github.com/Tunayblg/DeskLink/releases/latest</code>"],
          tip: "Easier on a computer? Download it there and send the file to the phone over USB or cloud storage." },
        { art: "lock", step: "Step 2", h: "Install it", ol: [
            "Open the downloaded file.",
            "If Android says installing from this source is blocked, tap <b>Settings</b> and allow it for your browser or file manager.",
            "Go back and tap <b>Install</b>."],
          warn: "DeskLink is not on the Play Store, so Android shows a warning. That is normal." },
        { art: "qr", step: "Step 3", h: "Connect to your PC", ol: [
            "Open the DeskLink app.",
            "Tap <b>Scan QR code</b>.",
            "On the PC open the dashboard, press <b>Add device</b> and aim the phone at the QR code."],
          tip: "The scanner needs Google Play services. If it is missing, use the address box instead." },
        { art: "check", step: "Step 4", h: "You are paired", p: [
            "When the home screen appears, you are connected. DeskLink remembers the PC and your pairing, so next time it opens straight into the deck.",
            "The <b>Devices</b> page on the PC lists every paired phone. You can remove one there at any time."] },
        { art: "home", step: "Step 5", h: "Everyday use", p: [
            "The app runs full screen without an address bar and keeps the screen on while it is open.",
            "Press the <b>Back</b> button to open the connection screen. There you can scan again, change the PC or exit."],
          tip: "Tablets get a wider layout on their own. Swipe the desk clock for the analog face." },
        { art: "link", h: "Tiles on the home screen", p: [
            "The home screen has six tiles. Each opens a game, a website or a program on the PC. Empty ones show a <b>+</b>."],
          ol: [
            "On the PC open DeskLink and go to <b>Games</b>. The <b>Home screen</b> list is below your games.",
            "Tap a tile. Pick one of your games, <b>Netflix</b>, <b>YouTube</b> and others, or paste any web address or program.",
            "Press <b>Save</b>. The phone updates at once."],
          tip: "The <b>Games</b> tab on the phone only shows up once you have added a game." },
        { art: "watch", h: "Watch mode", ul: [
            "Open the fourth tab, the screen with a play button.",
            "<b>Left:</b> the PC's volume. <b>Right:</b> screen brightness.",
            "The small button under the volume switches the sound between speakers, headphones and your monitor.",
            "<b>Middle:</b> rewind, play or pause and fast forward. They press keys, so the player window must be in front.",
            "The red power button shuts the PC down, after you confirm."],
          tip: "Brightness uses your monitor when it allows it (<b>DDC/CI</b>), a laptop's own panel, or otherwise dims the picture in software, so the bar works on any screen. Software dimming never goes fully black. Brightness is still in beta and may not work properly with every monitor." },
        { art: "dev", h: "Developer mode", ul: [
            "The <b>&lt;/&gt;</b> tab sends prompts to Claude, Codex, ChatGPT, a terminal or your editor on the PC.",
            "Type, or tap the <b>microphone</b> and speak. With <b>live typing</b> the words appear on the PC while you talk.",
            "The pill at the top picks the window. Only developer apps are listed, never chat or other programs.",
            "The bar on top shows what Claude or Codex is doing: thinking, running a command, editing files, or done.",
            "The small pad on the left is a mouse; the keys below press Esc, Tab, the arrows, Ctrl+C and Enter."],
          tip: "Dictation needs Siri and Dictation switched on in the phone's settings." },
        { art: "gear", h: "Settings", ul: [
            "The quiet gear at the bottom of the tab bar opens Settings.",
            "Switch any tab off, or move it up and down with the arrows.",
            "Turn live typing, usage, the agent status, sending after dictation and the dictation language on or off.",
            "<b>Calm mode</b> stops all animation."],
          tip: "Settings are saved on the phone itself, so each phone can have its own layout." },
        { art: "code", h: "Connecting manually", p: ["The QR scanner does not work for you? Type the address instead."], ol: [
            "On the PC press <b>Add device</b>, then <b>Trouble scanning? Connect manually</b>.",
            "In the app, type the address shown there into the address box, like " + ADDR + ", and tap <b>Connect</b>.",
            "Type the 6-digit code from the PC and tap <b>Pair</b>."],
          tip: "The <b>http://</b> part is optional in the app. The port after the colon is not." },
        { art: "ip", h: "Finding your PC's address", ol: [
            "Press <b>Win + R</b>, type <code>cmd</code> and press Enter.",
            "Type <code>ipconfig</code> and press Enter.",
            "Look for <b>IPv4 Address</b> under your Wi-Fi or Ethernet, for example <code>192.168.1.20</code>.",
            "Your address is that number + <code>:8765</code>."],
          tip: "8765 is the default port. The dashboard already shows the full address for you." },
        { art: "shield", h: "Windows Firewall", p: [
            "The first time DeskLink runs, Windows asks if it may use the network. Choose <b>Private networks</b> and press <b>Allow access</b>."],
          ol: ["If you pressed Cancel, open <b>Windows Security → Firewall &amp; network protection</b>.", "Choose <b>Allow an app through firewall</b> and tick DeskLink under <b>Private</b>."],
          warn: "Never forward the port on your router. DeskLink is meant for your home network only." },
        { art: "help", h: "If something goes wrong", ul: [
            "<b>Cannot connect:</b> are both devices on the same Wi-Fi? Guest networks often block devices from seeing each other.",
            "<b>Offline banner:</b> the PC is asleep or DeskLink was closed. Check the tray.",
            "<b>Address changed:</b> the router may give the PC a new address. Scan the QR code again.",
            "<b>No APK?</b> In Chrome open the address, tap the menu and choose <b>Add to Home screen</b>. It works, but keeps the browser bar."] },
        { back: true, h: "Enjoy your deck", ul: [
            "<b>Games:</b> add them on the PC, tap to launch. The phone turns into a live CPU and GPU monitor.",
            "<b>Mixer:</b> set the volume for each program on its own.",
            "<b>Trackpad:</b> use the phone as a mouse for the PC. The <b>Windows</b> button lists the open programs (not background helpers) and brings one to the front.",
            "<b>Clock:</b> swipe left or right to change the face, up or down for the next or previous tab."] }
      ],
      tr: [
        { cover: true, sub: "Android kılavuzu", text: "DeskLink uygulamasını kur, Android'ini Stream Deck'e çevir.", tag: "Yaklaşık 2 dakika · Play Store yok" },
        { art: "deck", h: "Neler lazım", ul: [
            "Bilgisayarında çalışan DeskLink. Simgesi saatin yanındaki tepsidedir.",
            "Bir Android telefon veya tablet (Android 5 ve üzeri).",
            "İkisi de <b>aynı Wi-Fi</b>'da olmalı. Mobil veri ve misafir Wi-Fi çalışmaz.",
            "Releases sayfasından <b>DeskLink.apk</b> uygulama dosyası."],
          tip: "Uygulama, bilgisayarına açılan küçük bir tam ekran penceredir. Hiçbir şey toplamaz." },
        { art: "apk", step: "Adım 1", h: "Uygulamayı indir", p: [
            "Telefonda tarayıcıyla DeskLink sürümler sayfasını aç ve <b>DeskLink.apk</b> dosyasını indir:",
            "<code>github.com/Tunayblg/DeskLink/releases/latest</code>"],
          tip: "Bilgisayardan daha kolaysa orada indir ve dosyayı USB ya da bulut ile telefona gönder." },
        { art: "lock", step: "Adım 2", h: "Kur", ol: [
            "İndirilen dosyayı aç.",
            "Android bu kaynaktan kurulumun engelli olduğunu söylerse <b>Ayarlar</b>'a dokun ve tarayıcın ya da dosya yöneticin için izin ver.",
            "Geri dön ve <b>Yükle</b>'ye dokun."],
          warn: "DeskLink Play Store'da olmadığı için Android bir uyarı gösterir. Bu normaldir." },
        { art: "qr", step: "Adım 3", h: "Bilgisayara bağlan", ol: [
            "DeskLink uygulamasını aç.",
            "<b>QR kodu tara</b>'ya dokun.",
            "Bilgisayarda panoyu aç, <b>Cihaz ekle</b>'ye bas ve telefonu QR koda doğrult."],
          tip: "Tarayıcı Google Play hizmetleri ister. Yoksa adres kutusunu kullan." },
        { art: "check", step: "Adım 4", h: "Eşleştin", p: [
            "Ana ekran göründüyse bağlandın. DeskLink bilgisayarı ve eşleşmeni hatırlar, bir dahaki sefer doğrudan kumanda paneline açılır.",
            "Bilgisayarda <b>Cihazlar</b> sayfası eşleşen tüm telefonları listeler. İstediğin zaman oradan kaldırabilirsin."] },
        { art: "home", step: "Adım 5", h: "Günlük kullanım", p: [
            "Uygulama adres çubuğu olmadan tam ekran çalışır ve açıkken ekranı uyanık tutar.",
            "<b>Geri</b> tuşuna basınca bağlantı ekranı açılır. Oradan yeniden tarayabilir, bilgisayarı değiştirebilir ya da çıkabilirsin."],
          tip: "Tabletler geniş düzeni kendiliğinden alır. Masa saatinde kaydırarak analog yüzü görebilirsin." },
        { art: "link", h: "Ana ekrandaki kutular", p: [
            "Ana ekranda altı kutu var. Her biri bilgisayarda bir oyun, web sitesi ya da program açar. Boş olanlarda <b>+</b> görünür."],
          ol: [
            "Bilgisayarda DeskLink'i aç ve <b>Oyunlar</b>'a git. <b>Ana ekran</b> listesi oyunlarının altında.",
            "Bir kutuya dokun. Oyunlarından birini, <b>Netflix</b>, <b>YouTube</b> ve diğerlerini seç ya da herhangi bir web adresi veya program yapıştır.",
            "<b>Kaydet</b>'e bas. Telefon hemen güncellenir."],
          tip: "Telefondaki <b>Oyunlar</b> sekmesi, en az bir oyun eklediğinde görünür." },
        { art: "watch", h: "Dizi modu", ul: [
            "Dördüncü sekmeyi aç, oynat işaretli ekran.",
            "<b>Solda:</b> bilgisayarın sesi. <b>Sağda:</b> ekran parlaklığı.",
            "Sesin altındaki küçük düğme sesi hoparlör, kulaklık ve monitör arasında değiştirir.",
            "<b>Ortada:</b> geri sar, oynat veya durdur ve ileri sar. Tuş bastıkları için oynatıcı penceresi önde olmalı.",
            "Kırmızı güç düğmesi, onayladıktan sonra bilgisayarı kapatır."],
          tip: "Parlaklık, monitör izin veriyorsa monitörün kendisini (<b>DDC/CI</b>), laptopta panelini, olmazsa görüntüyü yazılımla karartır. Böylece çubuk her ekranda çalışır. Yazılımla karartma ekranı hiçbir zaman tamamen karartmaz. Parlaklık henüz beta aşamasında, her monitörle düzgün çalışmayabilir." },
        { art: "dev", h: "Geliştirici modu", ul: [
            "<b>&lt;/&gt;</b> sekmesi bilgisayardaki Claude, Codex, ChatGPT, terminal ya da kod editörüne prompt gönderir.",
            "Yaz ya da <b>mikrofona</b> dokunup konuş. <b>Canlı yazma</b> açıksa sen konuşurken yazı bilgisayarda belirir.",
            "Üstteki düğmeden pencereyi seç. Yalnızca geliştirici uygulamaları listelenir, sohbet ve başka programlar asla.",
            "Üstteki çubuk Claude ya da Codex'in ne yaptığını gösterir: düşünüyor, komut çalıştırıyor, dosya düzenliyor ya da bitti.",
            "Soldaki küçük alan bir faredir; alttaki tuşlar Esc, Tab, oklar, Ctrl+C ve Enter'a basar."],
          tip: "Dikte için telefonun ayarlarında Siri ve Dikte açık olmalı." },
        { art: "gear", h: "Ayarlar", ul: [
            "Sekme çubuğunun en altındaki sade dişli Ayarlar'ı açar.",
            "İstediğin sekmeyi kapat ya da oklarla yukarı aşağı taşı.",
            "Canlı yazmayı, kullanımı, ajan durumunu, dikteden sonra göndermeyi ve dikte dilini aç ya da kapat.",
            "<b>Sakin mod</b> tüm animasyonları durdurur."],
          tip: "Ayarlar telefonun kendisinde saklanır, her telefonun kendi düzeni olabilir." },
        { art: "code", h: "Elle bağlanma", p: ["QR tarayıcı işine yaramadı mı? Adresi yaz."], ol: [
            "Bilgisayarda <b>Cihaz ekle</b>'ye, sonra <b>Okutamıyor musun? Elle bağlan</b>'a bas.",
            "Uygulamada orada yazan adresi adres kutusuna gir, örneğin " + ADDR + ", ve <b>Bağlan</b>'a dokun.",
            "Bilgisayardaki 6 haneli kodu yaz ve <b>Eşleştir</b>'e dokun."],
          tip: "Uygulamada <b>http://</b> kısmı isteğe bağlıdır. İki noktadan sonraki port değildir." },
        { art: "ip", h: "Bilgisayarın adresini bulma", ol: [
            "<b>Win + R</b>'ye bas, <code>cmd</code> yaz ve Enter'a bas.",
            "<code>ipconfig</code> yaz ve Enter'a bas.",
            "Wi-Fi veya Ethernet altında <b>IPv4 Adresi</b>'ni bul, örneğin <code>192.168.1.20</code>.",
            "Adresin: o sayı + <code>:8765</code>."],
          tip: "8765 varsayılan porttur. Pano tam adresi zaten senin için gösterir." },
        { art: "shield", h: "Windows Güvenlik Duvarı", p: [
            "DeskLink ilk çalıştığında Windows ağı kullanmasına izin verip vermeyeceğini sorar. <b>Özel ağlar</b>'ı seç ve <b>Erişime izin ver</b>'e bas."],
          ol: ["İptal'e bastıysan <b>Windows Güvenliği → Güvenlik duvarı ve ağ koruması</b>'nı aç.", "<b>Güvenlik duvarı üzerinden bir uygulamaya izin ver</b>'i seç ve DeskLink'i <b>Özel</b> altında işaretle."],
          warn: "Router'da portu asla dışarı açma. DeskLink yalnızca ev ağın için tasarlandı." },
        { art: "help", h: "Bir şey ters giderse", ul: [
            "<b>Bağlanamıyor:</b> iki cihaz da aynı Wi-Fi'da mı? Misafir ağlar cihazların birbirini görmesini çoğu zaman engeller.",
            "<b>Çevrimdışı uyarısı:</b> bilgisayar uykuda ya da DeskLink kapalı. Tepsiye bak.",
            "<b>Adres değişti:</b> router bilgisayara yeni bir adres vermiş olabilir. QR kodu tekrar okut.",
            "<b>APK yok mu?</b> Chrome'da adresi aç, menüye dokun ve <b>Ana ekrana ekle</b>'yi seç. Çalışır ama tarayıcı çubuğu kalır."] },
        { back: true, h: "İyi eğlenceler", ul: [
            "<b>Oyunlar:</b> bilgisayardan ekle, dokunup başlat. Telefon canlı CPU ve GPU monitörüne dönüşür.",
            "<b>Mikser:</b> her programın sesini ayrı ayrı ayarla.",
            "<b>Trackpad:</b> telefonu bilgisayar için fare olarak kullan. <b>Pencereler</b> düğmesi açık programları (arka plan yardımcıları hariç) listeler ve seçtiğini öne getirir.",
            "<b>Saat:</b> sağa sola kaydırarak yüzünü değiştir, yukarı aşağı kaydırarak sonraki ya da önceki sekmeye geç."] }
      ]
    }
  };

  // ---- Building the pages ---------------------------------------------------------

  function list(tag, items) {
    return items && items.length ? "<" + tag + "><li>" + items.join("</li><li>") + "</li></" + tag + ">" : "";
  }

  function pageHTML(pg, num, total) {
    if (pg.cover) {
      return '<div class="g-page g-cover"><img class="g-logo" src="/icon-180.png" alt=""><h2>DeskLink</h2><div class="g-sub">' + pg.sub +
        "</div><p>" + pg.text + '</p><div class="g-tag">' + pg.tag + '</div><div class="g-hintrow">' +
        (LANG === "tr" ? "Sayfayı çevirmek için tıkla ya da → tuşuna bas" : "Click the page or press → to turn it") + "</div></div>";
    }
    var h = '<div class="g-page">';
    if (pg.art) h += '<div class="g-art">' + ART[pg.art] + "</div>";
    if (pg.step) h += '<div class="g-step">' + pg.step + "</div>";
    h += "<h3>" + pg.h + "</h3>";
    if (pg.p) h += "<p>" + pg.p.join("</p><p>") + "</p>";
    h += list("ol", pg.ol) + list("ul", pg.ul);
    if (pg.tip) h += '<div class="tip">' + pg.tip + "</div>";
    if (pg.warn) h += '<div class="tip warn">' + pg.warn + "</div>";
    return h + '<div class="g-num">' + num + " / " + total + "</div></div>";
  }

  // ---- The book itself ---------------------------------------------------------------

  var book, stage, leaves, flipped, count, bar, prevBtn, nextBtn, platform, view;
  var timers = [];

  function el(tag, cls, html) { var e = document.createElement(tag); if (cls) e.className = cls; if (html !== undefined) e.innerHTML = html; return e; }

  var CHEV_L = '<svg viewBox="0 0 24 24"><path d="M15 5l-7 7 7 7"/></svg>';
  var CHEV_R = '<svg viewBox="0 0 24 24"><path d="M9 5l7 7-7 7"/></svg>';
  var PHONE = '<svg viewBox="0 0 24 24"><rect x="6" y="2" width="12" height="20" rx="2.5"/><path d="M11 18h2"/></svg>';
  var ROBOT = '<svg viewBox="0 0 24 24"><rect x="4" y="9" width="16" height="11" rx="3"/><path d="M6 9a6 6 0 0 1 12 0M8.5 4.5L7 2.5M15.5 4.5L17 2.5M9 14h.01M15 14h.01"/></svg>';

  // The guide takes the whole window; this goes back to the dashboard.
  function exitButton() {
    var b = el("button", "btn", "");
    b.textContent = "✕ " + UI.exit;
    b.onclick = leave;
    return b;
  }
  function leave() { document.dispatchEvent(new CustomEvent("deskguide:exit")); }

  function showPicker() {
    view = "pick";
    stopTimers();
    root.className = "";
    root.innerHTML = "";
    var wrap = el("div", "g-wrap");
    var top = el("div", "g-bar");
    top.appendChild(exitButton());
    wrap.appendChild(top);
    var heading = el("h2", "g-title", "");
    heading.textContent = UI.title;
    wrap.appendChild(heading);
    var lead = el("p", "g-lead", "");
    lead.textContent = UI.pickSub;
    wrap.appendChild(lead);
    var row = el("div", "g-pick");
    [["ios", "g-ios", PHONE, UI.iosName, UI.iosSub], ["android", "g-and", ROBOT, UI.andName, UI.andSub]].forEach(function (c) {
      var b = el("button", "g-card " + c[1], c[2] + "<b></b><span></span><div class=\"go\"></div>");
      b.querySelector("b").textContent = c[3];
      b.querySelector("span").textContent = c[4];
      b.querySelector(".go").textContent = UI.open + " →";
      b.onclick = function () { openBook(c[0]); };
      row.appendChild(b);
    });
    wrap.appendChild(row);
    root.appendChild(wrap);
  }

  function openBook(which) {
    platform = which;
    view = "book";
    var pages = BOOKS[which][LANG];
    var n = pages.length / 2;
    flipped = 0;
    leaves = [];
    root.className = which === "ios" ? "g-ios" : "g-and";
    root.innerHTML = "";

    var wrap = el("div", "g-wrap");
    bar = el("div", "g-bar");
    bar.appendChild(exitButton());
    var change = el("button", "btn", "");
    change.textContent = "← " + UI.change;
    change.onclick = showPicker;
    count = el("span", "count", "");
    bar.appendChild(change);
    bar.appendChild(el("span", "grow"));
    bar.appendChild(count);
    wrap.appendChild(bar);

    var fit = el("div", "g-fit");
    stage = el("div", "g-stage");
    book = el("div", "g-book cover");
    for (var k = 0; k < n; k++) {
      var leaf = el("div", "g-leaf");
      var front = el("div", "g-face g-front", pageHTML(pages[2 * k], 2 * k + 1, pages.length));
      var back = el("div", "g-face g-back", pageHTML(pages[2 * k + 1], 2 * k + 2, pages.length));
      leaf.appendChild(front);
      leaf.appendChild(back);
      (function (index) {
        leaf.onclick = function () { index < flipped ? turn(-1) : turn(1); };
      })(k);
      book.appendChild(leaf);
      leaves.push(leaf);
    }
    stage.appendChild(book);
    fit.appendChild(stage);
    prevBtn = el("button", "g-hit g-prev", CHEV_L);
    nextBtn = el("button", "g-hit g-next", CHEV_R);
    prevBtn.title = UI.prev;
    nextBtn.title = UI.next;
    prevBtn.onclick = function () { turn(-1); };
    nextBtn.onclick = function () { turn(1); };
    fit.appendChild(prevBtn);
    fit.appendChild(nextBtn);
    wrap.appendChild(fit);
    root.appendChild(wrap);
    layout();
    settle(-1);
    update();
  }

  // The leaf that is turning rides on top; the others stack by their position.
  function settle(moving) {
    var n = leaves.length;
    leaves.forEach(function (leaf, k) {
      leaf.classList.toggle("flipped", k < flipped);
      leaf.style.zIndex = k === moving ? n + 2 : (k < flipped ? k + 1 : n - k);
    });
  }

  function turn(dir) {
    var next = flipped + dir, n = leaves.length;
    if (next < 0 || next > n) return;
    var k = dir > 0 ? flipped : next;
    flipped = next;
    settle(k);
    update();
    timers.push(setTimeout(function () { settle(-1); }, 820));
  }

  function update() {
    var n = leaves.length;
    book.classList.toggle("cover", flipped === 0);
    book.classList.toggle("end", flipped === n);
    prevBtn.disabled = flipped === 0;
    nextBtn.disabled = flipped === n;
    // Two pages are visible at a time: the back of the last turned leaf and the front of the next.
    var left = flipped * 2, right = flipped * 2 + 1, total = n * 2;
    var shown = flipped === 0 ? 1 : flipped === n ? total : left;
    count.textContent = UI.page.replace("{}", shown).replace("{}", total);
  }

  function stopTimers() { timers.forEach(clearTimeout); timers = []; }

  // The book is drawn at a fixed size and scaled down to fit the window.
  function layout() {
    if (view !== "book" || !stage) return;
    var fit = stage.parentNode;
    var s = Math.min(2.1, (fit.clientWidth - 16) / BOOK_W, (fit.clientHeight - 24) / BOOK_H);
    stage.style.transform = "scale(" + Math.max(0.3, s).toFixed(3) + ")";
  }
  window.addEventListener("resize", layout);

  document.addEventListener("keydown", function (ev) {
    if (!view || root.closest("[hidden]")) return;
    if (ev.key === "Escape") { leave(); return; }
    if (view !== "book") return;
    if (ev.key === "ArrowRight") turn(1);
    else if (ev.key === "ArrowLeft") turn(-1);
  });

  window.DeskGuide = {
    show: function () { if (!view) showPicker(); requestAnimationFrame(layout); },
    leave: stopTimers
  };
})();
