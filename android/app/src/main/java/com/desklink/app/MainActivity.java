package com.desklink.app;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.graphics.Color;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;

import com.google.mlkit.vision.barcode.common.Barcode;
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions;
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning;

import org.json.JSONObject;

/**
 * DeskLink for Android: a full-screen window onto the panel the PC serves.
 *
 * Two WebViews live here. The panel (loaded from the PC) has no bridge to the app. The
 * connection screen (our own page from assets) does, so only our own code can call it.
 */
public class MainActivity extends Activity {
    private static final String PREFS = "desklink";
    private static final String KEY_URL = "url";
    private static final int DEFAULT_PORT = 8765;
    private static final String BG = "#0e0f12";

    private WebView panel;
    private WebView setup;
    private boolean panelLoaded = false;

    @SuppressLint({"SetJavaScriptEnabled", "AddJavascriptInterface"})
    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        if (Build.VERSION.SDK_INT >= 28) {
            getWindow().getAttributes().layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
        }

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.parseColor(BG));

        panel = new WebView(this);
        configure(panel);
        panel.setWebViewClient(new PanelClient());
        root.addView(panel);

        setup = new WebView(this);
        configure(setup);
        setup.addJavascriptInterface(new Bridge(), "DeskLinkApp");
        // The connection screen has a bridge into the app, so it must never navigate away from itself.
        setup.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                return true;
            }
        });
        setup.loadUrl("file:///android_asset/setup.html");
        root.addView(setup);

        setContentView(root);

        String saved = saved();
        if (saved != null) {
            openPanel(saved);
        }
    }

    @SuppressLint("SetJavaScriptEnabled")
    private void configure(WebView v) {
        v.setBackgroundColor(Color.parseColor(BG));
        WebSettings s = v.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setMediaPlaybackRequiresUserGesture(false);
        s.setAllowFileAccess(false);
        v.setOverScrollMode(View.OVER_SCROLL_NEVER);
        CookieManager.getInstance().setAcceptCookie(true);
    }

    // ---- Address handling --------------------------------------------------

    private String saved() {
        return getSharedPreferences(PREFS, MODE_PRIVATE).getString(KEY_URL, null);
    }

    /** Turns "192.168.1.20", "192.168.1.20:8765" or a full QR link into a loadable URL, or null. */
    static String normalize(String input) {
        if (input == null) return null;
        String s = input.trim();
        if (s.isEmpty()) return null;
        String lower = s.toLowerCase();
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
            s = "http://" + s;
        }
        int schemeEnd = s.indexOf("://") + 3;
        String scheme = s.substring(0, schemeEnd);
        String rest = s.substring(schemeEnd);
        String hash = "";
        int h = rest.indexOf('#');
        if (h >= 0) {
            hash = rest.substring(h);
            rest = rest.substring(0, h);
        }
        int slash = rest.indexOf('/');
        String host = slash >= 0 ? rest.substring(0, slash) : rest;
        String path = slash >= 0 ? rest.substring(slash) : "/";
        if (host.isEmpty() || host.contains(" ")) return null;
        if (!host.contains(":")) host = host + ":" + DEFAULT_PORT;
        return scheme + host + path + hash;
    }

    /**
     * DeskLink lives on the home network (the PC itself refuses anything else), so the app only
     * connects to private addresses and plain host names. That keeps it from being used as a
     * general browser, and from being pointed at a site on the internet by a mistyped address.
     */
    static boolean isLocalHost(String host) {
        if (host == null || host.isEmpty()) return false;
        String h = host.toLowerCase();
        if (h.startsWith("[") && h.endsWith("]")) {
            h = h.substring(1, h.length() - 1);
            return h.equals("::1") || h.startsWith("fe80:") || h.startsWith("fc") || h.startsWith("fd");
        }
        String[] parts = h.split("\\.", -1);
        boolean numeric = parts.length == 4;
        int[] octets = new int[4];
        for (int i = 0; numeric && i < 4; i++) {
            try {
                octets[i] = Integer.parseInt(parts[i]);
                numeric = octets[i] >= 0 && octets[i] <= 255 && !parts[i].isEmpty();
            } catch (NumberFormatException e) {
                numeric = false;
            }
        }
        if (numeric) {
            return octets[0] == 10
                    || octets[0] == 127
                    || (octets[0] == 192 && octets[1] == 168)
                    || (octets[0] == 172 && octets[1] >= 16 && octets[1] <= 31)
                    || (octets[0] == 169 && octets[1] == 254);
        }
        return !h.contains(".") || h.endsWith(".local") || h.endsWith(".lan") || h.endsWith(".home.arpa");
    }

    private void openPanel(String url) {
        int hash = url.indexOf('#');
        String clean = hash >= 0 ? url.substring(0, hash) : url;
        getSharedPreferences(PREFS, MODE_PRIVATE).edit().putString(KEY_URL, clean).apply();
        panelLoaded = true;
        setup.setVisibility(View.GONE);
        panel.loadUrl(url);
    }

    private void showSetup() {
        setup.setVisibility(View.VISIBLE);
        setup.evaluateJavascript("window.refresh && window.refresh()", null);
    }

    private void setupError(String code) {
        showSetup();
        setup.evaluateJavascript("window.showError && window.showError(" + JSONObject.quote(code) + ")", null);
    }

    // ---- Panel navigation --------------------------------------------------------

    private class PanelClient extends WebViewClient {
        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            // Stay inside the PC's panel. Anything else is not ours to open.
            String saved = saved();
            if (saved == null) return true;
            String host = request.getUrl().getHost();
            return host == null || !host.equals(Uri.parse(saved).getHost());
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            if (request.isForMainFrame()) {
                panelLoaded = false;
                setupError("unreachable");
            }
        }
    }

    // ---- The connection screen's bridge (our own page only) ---------------------

    private class Bridge {
        @JavascriptInterface
        public String savedAddress() {
            String s = saved();
            return s == null ? "" : s.replaceFirst("^https?://", "").replaceFirst("/$", "");
        }

        @JavascriptInterface
        public boolean hasPanel() {
            return panelLoaded;
        }

        @JavascriptInterface
        public void connect(final String address) {
            runOnUiThread(() -> {
                String url = normalize(address);
                if (url == null) {
                    setupError("badAddress");
                    return;
                }
                if (!isLocalHost(Uri.parse(url).getHost())) {
                    setupError("notLocal");
                    return;
                }
                openPanel(url);
            });
        }

        @JavascriptInterface
        public void resume() {
            runOnUiThread(() -> {
                if (panelLoaded) setup.setVisibility(View.GONE);
            });
        }

        @JavascriptInterface
        public void scan() {
            runOnUiThread(MainActivity.this::startScan);
        }

        @JavascriptInterface
        public void exit() {
            runOnUiThread(MainActivity.this::finishAndRemoveTask);
        }
    }

    private void startScan() {
        GmsBarcodeScannerOptions options = new GmsBarcodeScannerOptions.Builder()
                .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
                .build();
        GmsBarcodeScanning.getClient(this, options).startScan()
                .addOnSuccessListener(barcode -> {
                    String url = normalize(barcode.getRawValue());
                    if (url == null || !isLocalHost(Uri.parse(url).getHost())) {
                        setupError("badQr");
                        return;
                    }
                    openPanel(url);
                })
                .addOnFailureListener(e -> setupError("scanFailed"));
    }

    // ---- Window --------------------------------------------------------------

    @Override
    public void onBackPressed() {
        if (setup.getVisibility() == View.VISIBLE) {
            if (panelLoaded) {
                setup.setVisibility(View.GONE);
            } else {
                finish();
            }
        } else {
            showSetup();
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        panel.onResume();
        immersive();
    }

    @Override
    protected void onPause() {
        panel.onPause();
        super.onPause();
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) immersive();
    }

    @SuppressWarnings("deprecation")
    private void immersive() {
        getWindow().getDecorView().setSystemUiVisibility(
                View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                        | View.SYSTEM_UI_FLAG_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION);
    }
}
