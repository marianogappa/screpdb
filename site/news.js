(function () {
  "use strict";

  var feed = document.getElementById("news-feed");
  var more = document.getElementById("news-more");
  var COLLAPSED = 5;
  var groups = [];
  var expanded = false;

  function isKo() { return document.documentElement.lang === "ko"; }

  function t(key, params) {
    var text = (isKo() && window.SCREPDB_KO[key]) || window.SCREPDB_EN[key] || key;
    return text.replace(/\{(\w+)\}/g, function (whole, name) { return params && params[name] != null ? params[name] : whole; });
  }

  function local(entry, field) {
    return (isKo() && entry.ko && entry.ko[field]) || entry[field];
  }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text != null) node.textContent = text;
    return node;
  }

  function formatDate(iso) {
    var parts = iso.split("-").map(Number);
    var date = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]));
    return new Intl.DateTimeFormat(isKo() ? "ko-KR" : "en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }).format(date);
  }

  function youtubeId(url) {
    var match = /[?&]v=([\w-]{11})/.exec(url);
    return match ? match[1] : null;
  }

  function groupEntries(list) {
    var out = [];
    var used = {};
    list.forEach(function (entry) {
      var last = out[out.length - 1];
      if (entry.kind === "video" && last && last.kind === "video" && last.date === entry.date) {
        last.videos.push(entry);
        return;
      }
      var group = entry.kind === "video" ? { kind: "video", date: entry.date, videos: [entry] } : entry;
      var id = group.kind === "release" ? "v" + group.version : (group.kind === "video" ? "videos-" : "article-") + group.date;
      var n = (used[id] || 0) + 1;
      used[id] = n;
      group.id = n > 1 ? id + "-" + n : id;
      out.push(group);
    });
    return out;
  }

  function videoCard(video) {
    var a = el("a", "news-video");
    a.href = video.url;
    var id = youtubeId(video.url);
    if (id) {
      var img = el("img");
      img.loading = "lazy";
      img.alt = "";
      img.width = 320;
      img.height = 180;
      img.src = "https://i.ytimg.com/vi/" + id + "/mqdefault.jpg";
      a.appendChild(img);
    }
    a.appendChild(el("span", null, local(video, "title")));
    return a;
  }

  function renderGroup(group) {
    var li = el("li", "news-item is-" + group.kind);
    li.id = group.id;
    if (location.hash === "#" + group.id) li.classList.add("is-linked");

    var meta = el("div", "news-meta");
    var permalink = el("a", "news-permalink");
    permalink.href = "#" + group.id;
    var time = el("time", null, formatDate(group.date));
    time.dateTime = group.date;
    permalink.appendChild(time);
    meta.appendChild(permalink);
    meta.appendChild(el("span", "news-kind", group.kind === "release" ? "v" + group.version : t("news.kind." + group.kind)));
    li.appendChild(meta);

    var body = el("div", "news-body");
    if (group.kind === "release") {
      body.appendChild(el("h3", null, local(group, "title")));
      var ul = el("ul");
      local(group, "items").forEach(function (item) { ul.appendChild(el("li", null, item)); });
      body.appendChild(ul);
    } else if (group.kind === "video") {
      if (group.videos.length > 1) body.appendChild(el("h3", null, t("news.videos.n", { n: group.videos.length })));
      var grid = el("div", "news-videos");
      group.videos.forEach(function (video) { grid.appendChild(videoCard(video)); });
      body.appendChild(grid);
    } else {
      var h3 = el("h3");
      var link = el("a", null, local(group, "title"));
      link.href = group.url;
      h3.appendChild(link);
      body.appendChild(h3);
      if (group.summary) body.appendChild(el("p", null, local(group, "summary")));
    }
    li.appendChild(body);
    return li;
  }

  function render() {
    var shown = expanded ? groups : groups.slice(0, COLLAPSED);
    feed.textContent = "";
    shown.forEach(function (group) { feed.appendChild(renderGroup(group)); });
    more.hidden = shown.length === groups.length;
    feed.querySelectorAll('a[href^="http"]').forEach(function (a) {
      a.target = "_blank";
      a.rel = "noopener";
      if (!a.classList.contains("news-video")) a.classList.add("ext");
    });
  }

  function showLinkedItem() {
    var id = decodeURIComponent(location.hash.slice(1));
    var index = groups.findIndex(function (g) { return g.id === id; });
    if (index >= COLLAPSED) expanded = true;
    render();
    if (index < 0) return;
    document.getElementById(id).scrollIntoView({ block: "start", behavior: "instant" });
    if (typeof window.gtag === "function") window.gtag("event", "news_permalink", { item: id });
  }

  more.addEventListener("click", function () {
    expanded = true;
    render();
  });

  window.addEventListener("hashchange", function () { if (groups.length) showLinkedItem(); });

  document.addEventListener("screpdb:locale", function () { if (groups.length) render(); });

  fetch("news.json")
    .then(function (r) { return r.ok ? r.json() : []; })
    .then(function (list) {
      groups = groupEntries(list);
      showLinkedItem();
    })
    .catch(function () {});
})();
