(function () {
  "use strict";

  var LOCALE_KEY = "screpdb_site_locale";
  var SUPPORTED = ["en", "ko"];

  function normalizeLocale(value) {
    var base = String(value || "").trim().toLowerCase().split(/[-_]/)[0];
    return SUPPORTED.indexOf(base) >= 0 ? base : null;
  }

  function readStoredLocale() {
    try { return localStorage.getItem(LOCALE_KEY); } catch (e) { return null; }
  }

  function writeStoredLocale(locale) {
    try { localStorage.setItem(LOCALE_KEY, locale); } catch (e) {}
  }

  function detectLocale() {
    var fromQuery = normalizeLocale(new URLSearchParams(location.search).get("lang"));
    if (fromQuery) return fromQuery;
    var stored = normalizeLocale(readStoredLocale());
    if (stored) return stored;
    var languages = navigator.languages || [navigator.language];
    for (var i = 0; i < languages.length; i++) {
      var match = normalizeLocale(languages[i]);
      if (match) return match;
    }
    return "en";
  }

  function detectOS() {
    var ua = navigator.userAgent || "";
    var platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || "";
    if (/Android|iPhone|iPad|iPod/i.test(ua)) return "mobile";
    if (/Win/i.test(platform) || /Windows/i.test(ua)) return "windows";
    if (/Mac/i.test(platform) || /Mac OS X/i.test(ua)) return "macos";
    if (/Linux|X11|CrOS/i.test(platform + ua)) return "linux";
    return "unknown";
  }

  var reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var english = {};
  var englishAttr = {};
  var locale = detectLocale();
  var os = detectOS();
  var releaseTag = null;

  function t(key, params) {
    var dict = locale === "ko" ? window.SCREPDB_KO : window.SCREPDB_EN;
    var text = dict[key] || window.SCREPDB_EN[key] || english[key] || key;
    return text.replace(/\{(\w+)\}/g, function (whole, name) { return params && params[name] != null ? params[name] : whole; });
  }

  function translateAttr(attr, dataAttr) {
    document.querySelectorAll("[" + dataAttr + "]").forEach(function (el) {
      var key = el.getAttribute(dataAttr);
      if (!(key in englishAttr)) englishAttr[key] = el.getAttribute(attr);
      el.setAttribute(attr, locale === "ko" ? window.SCREPDB_KO[key] || englishAttr[key] : englishAttr[key]);
    });
  }

  function track(name, params) {
    if (typeof window.gtag === "function") window.gtag("event", name, params || {});
  }

  document.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest("a");
    if (a && /\/releases\/latest\/download\//.test(a.href)) track("download", { file: a.href.split("/").pop() });
  });

  function markExternalLinks() {
    document.querySelectorAll('a[href^="http"]').forEach(function (a) {
      if (/\/releases\/latest\/download\//.test(a.href)) return;
      a.target = "_blank";
      a.rel = "noopener";
      a.classList.add("ext");
    });
  }

  var storyManifest = {};

  function applyVersion() {
    if (releaseTag) document.getElementById("version-fact").textContent = t("fact.version", { v: releaseTag });
  }

  function placeRings() {
    document.querySelectorAll(".path-shot[data-step]").forEach(function (figure) {
      var old = figure.querySelector(".hl");
      if (old) old.remove();
      var step = figure.getAttribute("data-step");
      var entry = storyManifest[locale === "ko" ? step + ".ko" : step] || storyManifest[step];
      var hl = entry && entry.highlight;
      if (!hl) return;
      var ring = document.createElement("span");
      ring.className = "hl";
      ring.setAttribute("aria-hidden", "true");
      ring.style.left = hl.x * 100 + "%";
      ring.style.top = hl.y * 100 + "%";
      ring.style.width = hl.w * 100 + "%";
      ring.style.height = hl.h * 100 + "%";
      figure.appendChild(ring);
    });
  }

  function applyLocale() {
    document.documentElement.lang = locale;
    document.querySelectorAll("[data-i18n]").forEach(function (el) {
      var key = el.getAttribute("data-i18n");
      if (!(key in english)) english[key] = el.innerHTML;
      el.innerHTML = locale === "ko" ? window.SCREPDB_KO[key] || english[key] : window.SCREPDB_EN[key] || english[key];
    });
    translateAttr("content", "data-i18n-content");
    translateAttr("alt", "data-i18n-alt");
    translateAttr("aria-label", "data-i18n-aria");

    document.querySelectorAll('[src^="images/"], [poster^="images/"]').forEach(function (el) {
      ["src", "poster"].forEach(function (attr) {
        var value = el.getAttribute(attr);
        if (!value) return;
        var base = value.replace(/\.ko(\.\w+)$/, "$1");
        var next = locale === "ko" ? base.replace(/(\.\w+)$/, ".ko$1") : base;
        if (next !== value) {
          el.setAttribute(attr, next);
          if (el.tagName === "VIDEO" && attr === "src") el.load();
        }
      });
    });
    document.querySelectorAll(".shot-video").forEach(function (video) {
      if (reducedMotion) {
        video.removeAttribute("autoplay");
        video.pause();
      } else {
        video.play().catch(function () {});
      }
    });

    var toggle = document.getElementById("lang-toggle");
    toggle.textContent = locale === "ko" ? "English" : "한국어";
    toggle.lang = locale === "ko" ? "en" : "ko";

    markExternalLinks();
    placeRings();
    applyVersion();
  }

  applyLocale();

  document.getElementById("lang-toggle").addEventListener("click", function () {
    locale = locale === "ko" ? "en" : "ko";
    track("language", { locale: locale });
    writeStoredLocale(locale);
    applyLocale();
  });


  var tabs = Array.prototype.slice.call(document.querySelectorAll('[role="tab"]'));
  function selectTab(tab) {
    tabs.forEach(function (tb) {
      var on = tb === tab;
      tb.setAttribute("aria-selected", on ? "true" : "false");
      tb.tabIndex = on ? 0 : -1;
      document.getElementById(tb.getAttribute("aria-controls")).hidden = !on;
    });
  }
  tabs.forEach(function (tab, i) {
    tab.addEventListener("click", function () {
      selectTab(tab);
      track("install_tab", { os: tab.id.replace("tab-", "") });
    });
    tab.addEventListener("keydown", function (e) {
      var d = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
      if (!d) return;
      var next = tabs[(i + d + tabs.length) % tabs.length];
      selectTab(next);
      next.focus();
    });
  });
  var initial = document.getElementById("tab-" + os);
  if (initial) selectTab(initial);

  document.querySelectorAll("pre.cmd").forEach(function (pre) {
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy";
    btn.setAttribute("data-i18n", "copy");
    btn.textContent = t("copy");
    btn.addEventListener("click", function () {
      navigator.clipboard.writeText(pre.querySelector("code").textContent).then(function () {
        btn.textContent = t("copied");
        setTimeout(function () { btn.textContent = t("copy"); }, 1500);
      });
    });
    pre.appendChild(btn);
  });

  var facade = document.querySelector(".video-facade");
  facade.addEventListener("click", function () {
    track("video_play", { video: "tour" });
    var iframe = document.createElement("iframe");
    var captions = locale === "ko" ? "&cc_load_policy=1&cc_lang_pref=ko&hl=ko" : "";
    iframe.src = "https://www.youtube-nocookie.com/embed/p2KgiVLyyZE?autoplay=1&rel=0" + captions;
    iframe.title = "screpdb";
    iframe.allow = "autoplay; encrypted-media; picture-in-picture; fullscreen";
    iframe.allowFullscreen = true;
    facade.replaceWith(iframe);
  });

  var storyVideos = new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      if (reducedMotion) return;
      if (entry.isIntersecting) entry.target.play().catch(function () {});
      else entry.target.pause();
    });
  }, { threshold: 0.25 });
  document.querySelectorAll(".story-video").forEach(function (video) { storyVideos.observe(video); });

  var steps = Array.prototype.slice.call(document.querySelectorAll(".path-step"));
  var path = document.querySelector(".path");
  var progress = document.querySelector(".path-progress");
  function onStoryScroll() {
    var mark = window.innerHeight * 0.55;
    var top = path.getBoundingClientRect().top;
    var end = 0;
    steps.forEach(function (step) {
      var node = step.querySelector(".path-n").getBoundingClientRect();
      step.classList.toggle("seen", node.top < mark);
      end = node.top + node.height / 2 - top;
    });
    progress.style.height = Math.max(0, Math.min(end, mark - top)) + "px";
  }
  window.addEventListener("scroll", onStoryScroll, { passive: true });
  window.addEventListener("resize", onStoryScroll);
  onStoryScroll();

  fetch("images/story/story.json")
    .then(function (r) { return r.ok ? r.json() : {}; })
    .then(function (manifest) { storyManifest = manifest; placeRings(); })
    .catch(function () {});

  var nav = document.querySelector(".nav");
  function onScroll() { nav.classList.toggle("scrolled", window.scrollY > 8); }
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  fetch("https://api.github.com/repos/marianogappa/screpdb/releases/latest")
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (rel) {
      if (!rel || !rel.tag_name) return;
      releaseTag = rel.tag_name;
      applyVersion();
    })
    .catch(function () {});
})();
