package web

// The page, as one document. No script, no external asset, no build step: the
// template below and the process's own state are the entire dependency list,
// which is what lets the chart expose this behind any gateway without a
// content policy conversation. Styling follows the system's light/dark choice
// because the page has no storage to remember one of its own.
//
// Everything dynamic arrives pre-formatted in the view; the template ranges
// and branches and says nothing. html/template escapes every finding, title
// and note on the way through, which matters here more than most places:
// finding text quotes cluster objects, and pull request titles are whatever
// the bump wrote.
//
// THE PALETTE IS THE SITE'S, and it is not restated from taste. Every value
// below maps to a token in site/src/styles/theme.css, which holds the
// Integratn design system's tokens, the ones integratn.io and Specmarshal
// share. There is one palette and the site is where this repository takes it
// from; this page is a consumer of it. If a colour here has no counterpart
// there, it is a bug rather than a variation.
//
// Dark is the base and light is the override, which is the site's structure
// and for the site's reason: you read this next to a terminal.
//
// WHICH TREATMENT IS THREE-STATE, and the shape below is what makes all three
// reachable. Left alone the page follows the reader's system preference. The
// chart's `web.theme` stamps `data-theme` on <html> -- the same attribute the
// site's own toggle writes -- and an explicit value has to beat the media
// query in BOTH directions: `dark` on a reader whose system asks for light,
// and `light` on one whose system asks for dark. So the media query is
// guarded with :not([data-theme='dark']) and the explicit light block is
// repeated after it. Defining a colour only inside the media query would
// leave `data-theme="light"` half-applied on a dark system, which renders as
// dark text on dark ground rather than as an error.
//
// The fonts are the platform's own sans and mono, as on the site, so nothing
// is fetched: an external asset would break a page whose whole publishing
// story is that it has none.
const pageHTML = `<!doctype html>
<html lang="en"{{with .Theme}} data-theme="{{.}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="60">
<link rel="icon" type="image/svg+xml" href="mark.svg">
<title>{{.Brand}} — promotion pipeline</title>
<style>
/* Dark, and the base rather than a media query -- see the note above the
   template. Names on the left are this page's; names in the comments are the
   site's tokens they come from: the Integratn design system's, which the docs
   site and integratn.io share. */
:root {
  --bg: #0a0c10;            /* --ground */
  --card: #1d2126;          /* --panel */
  --fg: #cfd5dc;            /* --said */
  --strong: #f3f7fc;        /* --text */
  --muted: #acb2b9;         /* --dim */
  --line: #393e43;          /* --border */
  --link: #f3f7fc;          /* --accent */
  --ok: #42c161;            /* --ok */
  --ok-bg: #0c2b13;         /* --ok-surface */
  --blocking: #f47983;      /* --bad */
  --blocking-bg: #39191b;   /* --bad-surface */
  --degraded: #cf9a43;      /* --warn */
  --degraded-bg: #2f210c;   /* --warn-surface */
  --neutral: #acb2b9;       /* --dim */
  --neutral-bg: #262a30;    /* --accent-surface */
  --code-bg: #14171d;       /* --well */
  --radius: 6px;            /* --radius */
  --font: system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  --font-display: var(--font);
  --font-mono: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
/* Light: the design system's light surface, not an inversion of the above. */
@media (prefers-color-scheme: light) {
  :root:not([data-theme='dark']) {
    --bg: #dee2e7;          /* --ground */
    --card: #e8ebf1;        /* --panel */
    --fg: #262a30;          /* --said */
    --strong: #0a0c10;      /* --text */
    --muted: #393e43;       /* --dim */
    --line: #acb2b9;        /* --border */
    --link: #0a0c10;        /* --accent */
    --ok: #006c24;          /* --ok */
    --ok-bg: #eafaec;       /* --ok-surface */
    --blocking: #8f3640;    /* --bad */
    --blocking-bg: #ffefef; /* --bad-surface */
    --degraded: #764f00;    /* --warn */
    --degraded-bg: #fff3e3; /* --warn-surface */
    --neutral: #393e43;     /* --dim */
    --neutral-bg: #f3f7fc;  /* --accent-surface */
    --code-bg: #dee2e7;     /* --well */
  }
}
/* And again for an explicit light, which must beat a dark system preference.
   Byte-identical to the block above by construction; TestLightBlocksAgree fails
   if they ever stop agreeing. */
:root[data-theme='light'] {
  --bg: #dee2e7;          /* --ground */
  --card: #e8ebf1;        /* --panel */
  --fg: #262a30;          /* --said */
  --strong: #0a0c10;      /* --text */
  --muted: #393e43;       /* --dim */
  --line: #acb2b9;        /* --border */
  --link: #0a0c10;        /* --accent */
  --ok: #006c24;          /* --ok */
  --ok-bg: #eafaec;       /* --ok-surface */
  --blocking: #8f3640;    /* --bad */
  --blocking-bg: #ffefef; /* --bad-surface */
  --degraded: #764f00;    /* --warn */
  --degraded-bg: #fff3e3; /* --warn-surface */
  --neutral: #393e43;     /* --dim */
  --neutral-bg: #f3f7fc;  /* --accent-surface */
  --code-bg: #dee2e7;     /* --well */
}
* { box-sizing: border-box; }
body {
  margin: 0; background: var(--bg); color: var(--fg);
  font: 15px/1.5 var(--font);
}
a { color: var(--link); text-decoration: none; }
a:hover { text-decoration: underline; }
main { max-width: 1040px; margin: 0 auto; padding: 24px 16px 48px; }
header { display: flex; justify-content: space-between; align-items: baseline; flex-wrap: wrap; gap: 4px 16px; margin-bottom: 16px; }
header .lockup { display: flex; align-items: center; gap: 10px; }
/* The badge lockup at its intended size. It is a rounded square with its own
   navy ground, so it needs no ring or shadow from this page. */
header .mark { width: 32px; height: 32px; border-radius: 7px; flex: none; }
header h1 { font-size: 20px; margin: 0; font-family: var(--font-display); letter-spacing: -.01em; color: var(--strong); }
.banner h2, section.findings > h2, article.finding h3 { font-family: var(--font-display); color: var(--strong); }
.sub, .stamp, .muted { color: var(--muted); }
.sub { margin: 2px 0 0; font-size: 13px; }
.stamp { font-size: 13px; }
.banner { border: 1px solid var(--line); border-left: 6px solid var(--neutral); background: var(--card); border-radius: var(--radius); padding: 14px 18px; margin-bottom: 16px; }
.banner h2 { margin: 0; font-size: 17px; }
.banner p { margin: 6px 0 0; color: var(--muted); }
.banner.ok { border-left-color: var(--ok); background: var(--ok-bg); }
.banner.blocking { border-left-color: var(--blocking); background: var(--blocking-bg); }
.banner.degraded { border-left-color: var(--degraded); background: var(--degraded-bg); }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); align-items: start; gap: 12px; margin-bottom: 12px; }
.card { background: var(--card); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px 16px; }
.card h2 { margin: 0 0 8px; font-size: 13px; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); }
.card p { margin: 4px 0; }
.card .err { color: var(--blocking); }
/* The gate's own card is a table, so it gets the width a table needs rather
   than a third of the row: a wrapped pull request title reads as four rows. */
.card.wide { margin-bottom: 12px; }
table.prs { width: 100%; border-collapse: collapse; margin: 8px 0 4px; }
table.prs td { padding: 5px 10px 5px 0; border-top: 1px solid var(--line); vertical-align: top; }
table.prs td:first-child { white-space: nowrap; }
table.prs td.title { width: 100%; overflow-wrap: anywhere; }
table.prs td:last-child { padding-right: 0; text-align: right; }
.chip { display: inline-block; font-size: 12px; line-height: 1; padding: 4px 8px; border-radius: 999px; background: var(--neutral-bg); color: var(--neutral); white-space: nowrap; }
.chip.passing { background: var(--ok-bg); color: var(--ok); }
.chip.failing, .chip.error { background: var(--blocking-bg); color: var(--blocking); }
.chip.running { background: var(--degraded-bg); color: var(--degraded); }
ul.features { list-style: none; margin: 4px 0; padding: 0; }
ul.features li { margin: 2px 0; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; background: var(--neutral); vertical-align: 1px; }
.dot.on { background: var(--ok); }
section.findings { margin-bottom: 20px; }
section.findings > h2 { font-size: 16px; margin: 0 0 2px; }
section.findings > .blurb { margin: 0 0 10px; color: var(--muted); }
article.finding { background: var(--card); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 16px; margin-bottom: 10px; }
article.finding h3 { margin: 0; font-size: 15px; display: inline; }
article.finding .chip { margin-left: 8px; }
article.finding .detail { margin: 8px 0 0; white-space: pre-line; }
pre { background: var(--code-bg); border: 1px solid var(--line); border-radius: 6px; padding: 10px 12px; margin: 10px 0 2px; overflow-x: auto; font: 13px/1.45 var(--font-mono); }
footer { border-top: 1px solid var(--line); margin-top: 24px; padding-top: 12px; font-size: 13px; color: var(--muted); }
footer p { margin: 4px 0; }
</style>
</head>
<body>
<main>
<header>
  <div>
    <div class="lockup">
      <img class="mark" src="mark.svg" width="32" height="32" alt="">
      <h1>{{.Brand}}</h1>
    </div>
    <p class="sub">
      watching {{if .RepoLink}}<a href="{{.RepoLink}}">{{.Repo}}</a>{{else}}{{.Repo}}{{end}}
      · check “{{.CheckName}}” · model {{.Model}} · {{.Version}}
    </p>
  </div>
  <p class="stamp">as of {{.Now}} · refreshes every minute</p>
</header>

<section class="banner {{.BannerClass}}">
  <h2>{{.Banner}}</h2>
  {{with .BannerNote}}<p>{{.}}</p>{{end}}
</section>

{{with .Gate}}
<section class="card wide">
  <h2>Gate</h2>
  <p class="muted">swept {{.SweptAgo}}{{with .Poll}}, polling every {{.}}{{end}}</p>
  {{with .Err}}<p class="err">The last sweep could not list pull requests: {{.}}</p>{{end}}
  {{if .Open}}
  <table class="prs">
    {{range .Open}}<tr>
      <td>{{if .URL}}<a href="{{.URL}}">#{{.Number}}</a>{{else}}#{{.Number}}{{end}}</td>
      <td class="title">{{.Title}}</td>
      <td><span class="chip {{.State}}">{{.State}}</span></td>
    </tr>{{end}}
  </table>
  {{else}}<p class="muted">No open pull requests.</p>{{end}}
  <p class="muted">{{.Summary}}</p>
</section>
{{end}}

<div class="grid">
{{with .Triage}}
  <section class="card">
    <h2>Triage</h2>
    <p>{{.Active}}</p>
    {{with .Queued}}<p>{{.}}</p>{{end}}
    <p class="muted">{{.Totals}}</p>
  </section>
{{end}}
{{with .Sweep}}
  <section class="card">
    <h2>Pipeline sweep</h2>
    {{if .On}}
    <p class="muted">swept {{if .SweptAgo}}{{.SweptAgo}}{{else}}never{{end}}, every {{.Every}}</p>
    {{else}}
    <p class="muted">Off. Nothing is watching for the promotions that never happened.</p>
    {{end}}
  </section>
{{end}}
  <section class="card">
    <h2>Posture</h2>
    <ul class="features">
    {{range .Features}}<li><span class="dot{{if .On}} on{{end}}"></span>{{.Name}}</li>
    {{end}}</ul>
    {{with .Egress}}<p class="muted">{{.}}</p>{{end}}
    <p class="muted">{{.Clusters}}</p>
  </section>
</div>

{{range .Sections}}
<section class="findings">
  <h2>{{.Mark}} {{.Title}}</h2>
  <p class="blurb">{{.Blurb}}</p>
  {{range .Finds}}
  <article class="finding">
    <h3>{{.Summary}}</h3>{{with .Since}}<span class="chip">{{.}}</span>{{end}}
    {{with .Detail}}<p class="detail">{{.}}</p>{{end}}
    {{with .Remedy}}<pre><code>{{.}}</code></pre>{{end}}
  </article>
  {{end}}
</section>
{{end}}

<footer>
  {{with .Checked}}<p>{{.}}</p>{{end}}
  {{range .Notes}}<p>⚠ Could not check everything — {{.}}</p>
  {{end}}
  <p>This report as <a href="/pipeline?format=markdown">markdown</a> or <a href="/pipeline?format=text">plain text</a>. Metrics stay on the in-cluster port.</p>
</footer>
</main>
</body>
</html>
`
