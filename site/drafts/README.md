# Drafts

Articles here are merged but never deployed: `make pages` drops this folder.

To publish one:

1. `git mv site/drafts site/articles` (or move just the article and its index entry).
2. Add its entry to the top of `site/news.json` with `"kind": "article"` and its `https://marianogappa.github.io/screpdb/articles/<slug>/` url.
