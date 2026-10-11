// Engagement analytics for bosun.integratn.io: which tool counts visits, and the
// events the site sends it. The same switch and the same event names as
// integratn.io's src/analytics.ts, so the two sites read as one in the tool.
//
// ANALYTICS is null until a tool is chosen, and while it is null the site loads
// nothing and sends nothing. Every supported tool is cookieless, so no consent
// banner is needed. The loader only runs on the production host, so `astro dev`
// and `astro preview` never add to the numbers.
//
// On Cloudflare Pages the tracker is first-party: functions/c.js and
// functions/api/c.js pass it through to Umami on the homelab, so the value to
// set is { tool: 'umami', src: '/c.js', websiteId: '<id from Umami>' }.
//
// What is counted beyond page views:
//   click     a landing-page button (.bo-btn), or anything marked data-track
//   outbound  any link to another site (GitHub, integratn.io), with host and path
//   read      on a docs page, the reader reached half-way (50) or the end (100)

/**
 * One of:
 *   { tool: 'umami', src: 'https://stats.example/script.js', websiteId: '...' }
 *   { tool: 'plausible', domain: 'bosun.integratn.io' }
 *   { tool: 'goatcounter', endpoint: 'https://NAME.goatcounter.com/count' }
 */
export const ANALYTICS = { tool: 'umami', src: '/c.js', websiteId: 'a4dc99d0-79d0-46df-97d1-67a6b64a9935' }

/** The hosts whose visits count. Anything else loads nothing. */
export const ANALYTICS_HOSTS = ['bosun.integratn.io']

/** Starlight `head` entries: none while analytics is off. */
export function analyticsHead() {
  if (!ANALYTICS) return []
  const options = { clicks: '[data-track], .bo-btn', read: '.sl-markdown-content' }
  return [
    {
      tag: 'script',
      content: `(${tracker.toString()})(${JSON.stringify(ANALYTICS)},${JSON.stringify(ANALYTICS_HOSTS)},${JSON.stringify(options)});`,
    },
  ]
}

// Runs in the browser, shipped as its source text: nothing from outside its body.
function tracker(config, hosts, options) {
  if (hosts.indexOf(location.hostname) < 0) return

  const s = document.createElement('script')
  s.defer = true
  if (config.tool === 'umami') {
    s.src = config.src
    s.setAttribute('data-website-id', config.websiteId)
  } else if (config.tool === 'plausible') {
    s.src = config.src || 'https://plausible.io/js/script.js'
    s.setAttribute('data-domain', config.domain)
  } else {
    s.src = 'https://gc.zgo.at/count.js'
    s.setAttribute('data-goatcounter', config.endpoint)
  }
  document.head.appendChild(s)

  // Events wait in a queue until the tool's script has loaded.
  const queue = []
  function send(name, data) {
    if (config.tool === 'umami' && window.umami) window.umami.track(name, data)
    else if (config.tool === 'plausible' && window.plausible) window.plausible(name, { props: data })
    else if (config.tool === 'goatcounter' && window.goatcounter && window.goatcounter.count)
      window.goatcounter.count({ path: name + ':' + Object.values(data).join(':'), title: name, event: true })
    else return false
    return true
  }
  function track(name, data) {
    if (!send(name, data)) queue.push([name, data])
  }
  s.addEventListener('load', () => {
    queue.splice(0).forEach(([name, data]) => send(name, data))
  })

  document.addEventListener(
    'click',
    (e) => {
      const target = e.target
      const marked = target && target.closest && target.closest(options.clicks)
      if (marked) {
        const name = marked.getAttribute('data-track') || (marked.textContent || '').trim().slice(0, 60)
        track('click', { name, page: location.pathname })
      }
      const link = target && target.closest && target.closest('a[href]')
      if (link && link.host && link.host !== location.host)
        track('outbound', { url: link.host + link.pathname.replace(/\/$/, ''), page: location.pathname })
    },
    true,
  )

  // Read depth, once per mark per page view. This script runs in the head, so
  // the content is looked for once the page has been parsed.
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', readDepth)
  else readDepth()
  function readDepth() {
    const article = document.querySelector(options.read)
    if (!article) return
    const marks = []
    for (const depth of [50, 100]) {
      const mark = document.createElement('span')
      mark.setAttribute('aria-hidden', 'true')
      mark.style.cssText = 'display:block;height:1px;margin:0;'
      if (depth === 100) article.appendChild(mark)
      else {
        // The first block that starts below the middle of the text, by height.
        const middle = article.getBoundingClientRect().top + article.scrollHeight / 2
        const half = Array.from(article.children).find((el) => el.getBoundingClientRect().top >= middle)
        if (!half) continue
        article.insertBefore(mark, half)
      }
      marks.push([depth, mark])
    }
    // A mark counts once it has come into view or gone past the top, so a fast
    // scroll that jumps over it still counts. Checked on scroll and once now.
    let pending = marks.slice()
    function check() {
      pending = pending.filter(([depth, m]) => {
        if (m.getBoundingClientRect().top > window.innerHeight) return true
        track('read', { depth: String(depth), page: location.pathname })
        return false
      })
      if (!pending.length) window.removeEventListener('scroll', onScroll)
    }
    let queued = false
    function onScroll() {
      if (queued) return
      queued = true
      requestAnimationFrame(() => {
        queued = false
        check()
      })
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    check()
  }
}
