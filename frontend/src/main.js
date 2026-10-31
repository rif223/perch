import './style.css'
import logoUrl from './assets/logo.svg'
import {
  GetState,
  Environment,
  ValidateURL,
  StartStream,
  StopStream,
  TogglePause,
  Seek,
  SeekRelative,
  OpenVideoFile,
  SetVolume,
  SetMuted,
  SetAlwaysOnTop,
  SetPlayerSize,
  SnapPlayer,
  SetOpacity,
  SetClickThrough,
  SetHidden,
  SetQuality,
  SetSpeed,
  SetSubtitle,
  RemoveRecent,
  Hotkeys,
  YTDLPVersion,
  UpdateYTDLP
} from '../wailsjs/go/main/App'
import { EventsOn, ClipboardGetText, OnFileDrop } from '../wailsjs/runtime/runtime'

const corner = (x, y) => `<svg viewBox="0 0 24 24" class="line"><rect x="3.5" y="5" width="17" height="14" rx="2.5"/><rect x="${x}" y="${y}" width="6.5" height="4.5" rx="1" fill="currentColor" stroke="none"/></svg>`

const icon = {
  play: '<svg viewBox="0 0 24 24"><path d="M8 5.5v13a1 1 0 0 0 1.5.86l10.4-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5Z"/></svg>',
  pause: '<svg viewBox="0 0 24 24"><rect x="6" y="5" width="4.2" height="14" rx="1.3"/><rect x="13.8" y="5" width="4.2" height="14" rx="1.3"/></svg>',
  stop: '<svg viewBox="0 0 24 24"><rect x="6" y="6" width="12" height="12" rx="2.2"/></svg>',
  volume: '<svg viewBox="0 0 24 24" class="line"><path d="M4 9.5v5h3.5L12 18.5v-13L7.5 9.5Z"/><path d="M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11"/></svg>',
  muted: '<svg viewBox="0 0 24 24" class="line"><path d="M4 9.5v5h3.5L12 18.5v-13L7.5 9.5Z"/><path d="m16 9.5 5 5m0-5-5 5"/></svg>',
  paste: '<svg viewBox="0 0 24 24" class="line"><rect x="7" y="4.5" width="10" height="3.5" rx="1"/><path d="M9 6H7a2 2 0 0 0-2 2v10.5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-2"/></svg>',
  clear: '<svg viewBox="0 0 24 24" class="line"><path d="m7 7 10 10M17 7 7 17"/></svg>',
  folder: '<svg viewBox="0 0 24 24" class="line"><path d="M3.5 7.5a2 2 0 0 1 2-2h4l2 2.5h7a2 2 0 0 1 2 2v7.5a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2Z"/></svg>',
  history: '<svg viewBox="0 0 24 24" class="line"><path d="M4 12a8 8 0 1 0 2.4-5.7"/><path d="M4 4v4h4"/><path d="M12 8v4l2.8 2"/></svg>',
  back: '<svg viewBox="0 0 24 24" class="line"><path d="M4 12a8 8 0 1 0 2.4-5.7"/><path d="M4 4v4h4"/><text x="12" y="15.2" text-anchor="middle" font-size="7" stroke="none" fill="currentColor" font-weight="700">10</text></svg>',
  forward: '<svg viewBox="0 0 24 24" class="line"><path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v4h-4"/><text x="12" y="15.2" text-anchor="middle" font-size="7" stroke="none" fill="currentColor" font-weight="700">10</text></svg>',
  pin: '<svg viewBox="0 0 24 24" class="line"><path d="M9 4h6l-1 5 3 3v2H7v-2l3-3Z"/><path d="M12 14v6"/></svg>',
  pointer: '<svg viewBox="0 0 24 24" class="line"><path d="M6 4l12 7-5.2 1.3L10 18Z"/></svg>',
  eye: '<svg viewBox="0 0 24 24" class="line"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="3"/></svg>',
  eyeOff: '<svg viewBox="0 0 24 24" class="line"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><path d="m4 4 16 16"/></svg>',
  opacity: '<svg viewBox="0 0 24 24" class="line"><circle cx="12" cy="12" r="8"/><path d="M12 4a8 8 0 0 1 0 16Z" fill="currentColor"/></svg>',
  link: '<svg viewBox="0 0 24 24" class="line"><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/></svg>',
  tl: corner(5.5, 7),
  tr: corner(12, 7),
  bl: corner(5.5, 12.5),
  br: corner(12, 12.5)
}

const kindLabel = {
  youtube: 'YouTube',
  twitch: 'Twitch',
  hls: 'HLS stream',
  dash: 'DASH stream',
  direct: 'Direct URL',
  file: 'Local file'
}

const statusLabel = {
  idle: 'Ready',
  starting: 'Loading …',
  playing: 'Playing',
  paused: 'Paused',
  buffering: 'Buffering …',
  stopping: 'Stopping …',
  error: 'Error'
}

const speeds = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2]

document.querySelector('#app').innerHTML = `
  <main class="app">
    <header class="header">
      <div class="brand">
        <img class="logo" src="${logoUrl}" alt="" />
        <div>
          <h1>Perch</h1>
          <p>Floating video player</p>
        </div>
      </div>
      <div id="statusPill" class="pill" data-status="idle"><span class="dot"></span><span id="statusText">Ready</span></div>
    </header>

    <section class="card source">
      <label class="card-label" for="streamUrl">Video or stream</label>
      <div class="field">
        <input id="streamUrl" type="text" spellcheck="false" autocomplete="off"
          placeholder="Paste a URL or drop a file here" />
        <button id="clearBtn" class="icon-btn subtle" title="Clear" hidden>${icon.clear}</button>
        <button id="historyBtn" class="icon-btn subtle" title="Recently played" aria-expanded="false">${icon.history}</button>
        <button id="pasteBtn" class="icon-btn subtle" title="Paste from clipboard">${icon.paste}</button>
        <button id="openBtn" class="icon-btn subtle" title="Open video file">${icon.folder}</button>
      </div>
      <div id="recentPanel" class="recent" hidden></div>
      <button id="suggest" class="suggest" hidden>
        ${icon.link}<span class="suggest-text">Play copied link: <span id="suggestUrl"></span></span>
        <span id="suggestDismiss" class="suggest-x" title="Dismiss">${icon.clear}</span>
      </button>
      <div id="hint" class="hint"></div>
      <div class="actions">
        <button id="startBtn" class="btn primary">${icon.play}<span id="startLabel">Play</span></button>
        <button id="stopBtn" class="btn secondary" disabled>${icon.stop}<span>Stop</span></button>
      </div>
    </section>

    <section class="card now" id="nowCard" data-active="false">
      <div class="now-head">
        <span class="card-label">Now playing</span>
        <span class="chips"><span id="nowLive" class="chip live" hidden>LIVE</span><span id="nowKind" class="chip" hidden></span></span>
      </div>
      <div id="nowTitle" class="now-title">Nothing playing</div>
      <div id="nowUrl" class="now-url"></div>
      <div id="timeline" class="timeline" hidden>
        <span id="timePos" class="time">0:00</span>
        <input id="seek" type="range" min="0" max="100" step="1" value="0" aria-label="Position" />
        <span id="timeDur" class="time">0:00</span>
      </div>
      <div class="controls">
        <button id="backBtn" class="icon-btn" title="Back 10 seconds" hidden>${icon.back}</button>
        <button id="pauseBtn" class="icon-btn round" title="Pause / resume" disabled>${icon.pause}</button>
        <button id="fwdBtn" class="icon-btn" title="Forward 10 seconds" hidden>${icon.forward}</button>
        <button id="muteBtn" class="icon-btn" title="Mute">${icon.volume}</button>
        <input id="volume" type="range" min="0" max="130" step="1" value="80" aria-label="Volume" />
        <span id="volumeValue" class="volume-value">80</span>
      </div>
      <div class="options">
        <label class="select-field"><span>Speed</span>
          <select id="speed" disabled>${speeds.map(v => `<option value="${v}">${v}×</option>`).join('')}</select>
        </label>
        <label class="select-field"><span>Subtitles</span>
          <select id="subs" disabled><option value="0">Off</option></select>
        </label>
        <label class="select-field" title="Maximum resolution for YouTube and Twitch"><span>Quality</span>
          <select id="quality">
            <option value="480">Up to 480p</option>
            <option value="720">Up to 720p</option>
            <option value="1080">Up to 1080p</option>
            <option value="best">Best</option>
          </select>
        </label>
      </div>
    </section>

    <section class="card">
      <span class="card-label">Player window</span>
      <div class="segmented" id="sizes" role="radiogroup">
        <button data-size="small" role="radio"><strong>Small</strong><small>320 × 180</small></button>
        <button data-size="medium" role="radio"><strong>Medium</strong><small>480 × 270</small></button>
        <button data-size="large" role="radio"><strong>Large</strong><small>640 × 360</small></button>
      </div>
      <div class="window-row">
        <div class="corners" id="corners" role="group" aria-label="Move to corner">
          <button class="icon-btn" data-corner="tl" title="Top left">${icon.tl}</button>
          <button class="icon-btn" data-corner="tr" title="Top right">${icon.tr}</button>
          <button class="icon-btn" data-corner="bl" title="Bottom left">${icon.bl}</button>
          <button class="icon-btn" data-corner="br" title="Bottom right">${icon.br}</button>
        </div>
        <button id="hideBtn" class="btn secondary small">${icon.eyeOff}<span id="hideLabel">Hide video</span></button>
      </div>
      <div class="opacity-row">
        <span class="toggle-text">${icon.opacity}<span>Opacity</span></span>
        <input id="opacity" type="range" min="20" max="100" step="5" value="100" aria-label="Opacity" />
        <span id="opacityValue" class="volume-value">100%</span>
      </div>
      <div class="toggles">
        <label class="toggle-row">
          <span class="toggle-text">${icon.pin}<span>Always on top</span></span>
          <span class="switch"><input type="checkbox" id="onTop" checked /><span class="slider"></span></span>
        </label>
        <label class="toggle-row" title="Mouse clicks go to the window behind the video. Turn off with Ctrl+Alt+T.">
          <span class="toggle-text">${icon.pointer}<span>Click-through</span></span>
          <span class="switch"><input type="checkbox" id="clickThrough" disabled /><span class="slider"></span></span>
        </label>
      </div>
    </section>

    <div id="message" class="message" hidden><span id="messageText"></span><button id="messageAction" class="btn secondary small" hidden></button></div>

    <footer>
      <div id="hotkeys" class="hotkeys"></div>
      <div class="ytdlp"><span id="ytdlpVersion"></span><button id="ytdlpUpdate" class="link-btn" hidden>Update yt-dlp</button></div>
    </footer>
  </main>
`

const $ = (id) => document.getElementById(id)
const el = {
  url: $('streamUrl'), clear: $('clearBtn'), paste: $('pasteBtn'), open: $('openBtn'), hint: $('hint'),
  historyBtn: $('historyBtn'), recent: $('recentPanel'),
  suggest: $('suggest'), suggestUrl: $('suggestUrl'), suggestDismiss: $('suggestDismiss'),
  nowLive: $('nowLive'), timeline: $('timeline'), seek: $('seek'), timePos: $('timePos'), timeDur: $('timeDur'),
  back: $('backBtn'), fwd: $('fwdBtn'),
  start: $('startBtn'), startLabel: $('startLabel'), stop: $('stopBtn'),
  pill: $('statusPill'), statusText: $('statusText'),
  nowCard: $('nowCard'), nowKind: $('nowKind'), nowTitle: $('nowTitle'), nowUrl: $('nowUrl'),
  pause: $('pauseBtn'), mute: $('muteBtn'), volume: $('volume'), volumeValue: $('volumeValue'),
  speed: $('speed'), subs: $('subs'), quality: $('quality'),
  sizes: $('sizes'), corners: $('corners'), hide: $('hideBtn'),
  opacity: $('opacity'), opacityValue: $('opacityValue'),
  onTop: $('onTop'), clickThrough: $('clickThrough'),
  message: $('message'), messageText: $('messageText'), messageAction: $('messageAction'),
  hotkeys: $('hotkeys'), ytdlpVersion: $('ytdlpVersion'), ytdlpUpdate: $('ytdlpUpdate')
}

let state = { status: 'idle', volume: 80, muted: false, onTop: true, size: 'medium', opacity: 100, recent: [] }
let busy = false
let draggingVolume = false
let draggingSeek = false
let draggingOpacity = false
let subsKey = ''

function formatTime (sec) {
  sec = Math.max(0, Math.floor(sec || 0))
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = String(sec % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`
}

const errText = (e) => String(e && e.message ? e.message : e)

// --- messages ------------------------------------------------------------

// Shown whenever there is no other message, e.g. "mpv is not installed".
let setupNotice = ''
let messageActionFn = null

// action: optional { label, run } rendered as a button in the message.
function showMessage (text, kind = 'error', action = null) {
  if (!text && setupNotice) {
    text = setupNotice
    kind = 'warning'
    action = null
  }
  if (!text) {
    el.message.hidden = true
    return
  }
  el.messageText.textContent = text
  el.message.dataset.kind = kind
  el.messageAction.hidden = !action
  el.messageAction.textContent = action ? action.label : ''
  messageActionFn = action ? action.run : null
  el.message.hidden = false
}

el.messageAction.addEventListener('click', () => messageActionFn && messageActionFn())

// YouTube and Twitch errors are often fixed by a newer yt-dlp.
function errorAction (s) {
  if (s.kind !== 'youtube' && s.kind !== 'twitch') return null
  if (/not live|unavailable or private/i.test(s.error)) return null
  return { label: 'Update yt-dlp', run: updateYTDLP }
}

// --- rendering -----------------------------------------------------------

function displayStatus (s) {
  if (s.status === 'playing' && s.buffering) return 'buffering'
  if (s.status === 'playing' && s.paused) return 'paused'
  return s.status
}

function render (s) {
  const prevError = state.error
  state = s
  const active = ['starting', 'playing', 'stopping'].includes(s.status)
  const status = displayStatus(s)

  el.pill.dataset.status = status
  el.statusText.textContent = statusLabel[status] || status

  el.startLabel.textContent = active ? 'Switch' : 'Play'
  el.start.disabled = busy || s.status === 'stopping'
  el.stop.disabled = busy || !active || s.status === 'stopping'

  el.nowCard.dataset.active = String(active)
  if (s.url && (active || s.status === 'error')) {
    el.nowTitle.textContent = s.title || (s.status === 'starting' ? 'Loading …' : s.url)
    el.nowUrl.textContent = s.url
    el.nowKind.textContent = kindLabel[s.kind] || ''
    el.nowKind.hidden = !s.kind
  } else {
    el.nowTitle.textContent = 'Nothing playing'
    el.nowUrl.textContent = ''
    el.nowKind.hidden = true
  }

  // Videos have a duration; live streams don't.
  const playing = s.status === 'playing'
  const isVideo = playing && s.duration > 0 && !s.live
  el.nowLive.hidden = !(playing && !isVideo)
  el.timeline.hidden = !isVideo
  el.back.hidden = el.fwd.hidden = !isVideo
  el.seek.disabled = el.back.disabled = el.fwd.disabled = !s.seekable
  if (isVideo && !draggingSeek) {
    el.seek.max = Math.max(1, Math.floor(s.duration))
    el.seek.value = Math.floor(s.position)
    el.timePos.textContent = formatTime(s.position)
    paintRange(el.seek)
  }
  el.timeDur.textContent = formatTime(s.duration)

  el.pause.disabled = !playing
  el.pause.innerHTML = s.paused ? icon.play : icon.pause
  el.mute.innerHTML = s.muted || s.volume === 0 ? icon.muted : icon.volume
  el.mute.classList.toggle('on', s.muted)
  if (!draggingVolume) {
    el.volume.value = s.volume
    el.volumeValue.textContent = s.volume
    paintRange(el.volume)
  }

  renderOptions(s, playing)
  renderWindow(s, active)
  renderRecent(s.recent || [])

  if (s.error) {
    if (s.error !== prevError) showMessage(s.error, 'error', errorAction(s))
  } else if (playing || s.status === 'starting') {
    if (el.message.dataset.kind === 'error') showMessage('')
  }
}

function renderOptions (s, playing) {
  el.speed.disabled = !playing
  const speed = speeds.find(v => Math.abs(v - (s.speed || 1)) < 0.01)
  if (speed !== undefined) el.speed.value = String(speed)

  const tracks = s.subTracks || []
  const key = JSON.stringify(tracks)
  if (key !== subsKey) {
    subsKey = key
    el.subs.innerHTML = '<option value="0">Off</option>' +
      tracks.map(t => `<option value="${t.id}"></option>`).join('')
    // Labels come from the stream, so set them as text, not HTML.
    tracks.forEach((t, i) => { el.subs.options[i + 1].textContent = t.label })
  }
  el.subs.disabled = !playing || tracks.length === 0
  el.subs.value = String(s.sub || 0)
  el.quality.value = s.quality || '720'
}

function renderWindow (s, active) {
  for (const b of el.sizes.querySelectorAll('button')) {
    b.setAttribute('aria-checked', String(b.dataset.size === s.size))
  }
  const hasWindow = active && s.status !== 'stopping'
  for (const b of el.corners.querySelectorAll('button')) b.disabled = !hasWindow
  el.hide.disabled = !hasWindow
  el.hide.innerHTML = (s.hidden ? icon.eye : icon.eyeOff) +
    `<span id="hideLabel">${s.hidden ? 'Show video' : 'Hide video'}</span>`
  if (!draggingOpacity) {
    el.opacity.value = s.opacity || 100
    el.opacityValue.textContent = (s.opacity || 100) + '%'
    paintRange(el.opacity, 20)
  }
  el.onTop.checked = s.onTop
  el.clickThrough.checked = !!s.clickThrough
  el.clickThrough.disabled = !hasWindow
}

function paintRange (input, min = 0) {
  const pct = ((input.value - min) / (input.max - min)) * 100
  input.style.setProperty('--fill', pct + '%')
}

async function call (fn) {
  try {
    await fn()
    return true
  } catch (e) {
    showMessage(errText(e))
    return false
  }
}

// --- URL input -------------------------------------------------------------

let validateTimer
function scheduleValidation () {
  clearTimeout(validateTimer)
  const value = el.url.value.trim()
  el.clear.hidden = !value
  if (!value) {
    el.hint.textContent = ''
    el.hint.dataset.kind = ''
    return
  }
  validateTimer = setTimeout(async () => {
    try {
      const src = await ValidateURL(value)
      if (el.url.value.trim() !== value) return
      el.hint.textContent = (kindLabel[src.kind] || 'Source') + ' detected'
      el.hint.dataset.kind = 'ok'
    } catch (e) {
      if (el.url.value.trim() !== value) return
      el.hint.textContent = errText(e)
      el.hint.dataset.kind = 'error'
    }
  }, 250)
}

async function start () {
  const value = el.url.value.trim()
  if (!value) {
    showMessage('Please enter a video or stream address first.')
    el.url.focus()
    return
  }
  busy = true
  closeRecent()
  hideSuggest()
  render(state)
  showMessage('')
  await call(() => StartStream(value))
  busy = false
  render(state)
}

function playPath (path) {
  el.url.value = path
  scheduleValidation()
  start()
}

el.url.addEventListener('input', scheduleValidation)
el.url.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') start()
})
el.clear.addEventListener('click', () => {
  el.url.value = ''
  scheduleValidation()
  el.url.focus()
})
el.paste.addEventListener('click', async () => {
  try {
    const text = (await ClipboardGetText()) || ''
    if (text.trim()) {
      el.url.value = text.trim()
      scheduleValidation()
      hideSuggest()
    }
  } catch (e) {
    showMessage('Could not read the clipboard.')
  }
  el.url.focus()
})
el.open.addEventListener('click', async () => {
  try {
    const path = await OpenVideoFile()
    if (path) playPath(path)
  } catch (e) {
    showMessage(errText(e))
  }
})

// Files dropped anywhere on the window start playing right away.
OnFileDrop((x, y, paths) => {
  if (paths && paths.length) playPath(paths[0])
}, false)

el.start.addEventListener('click', start)
el.stop.addEventListener('click', async () => {
  busy = true
  render(state)
  if (await call(StopStream)) showMessage('')
  busy = false
  render(state)
})

// --- recently played -------------------------------------------------------

function renderRecent (items) {
  el.historyBtn.disabled = items.length === 0
  if (items.length === 0) closeRecent()
  if (el.recent.hidden) return
  el.recent.innerHTML = ''
  for (const item of items) {
    const row = document.createElement('div')
    row.className = 'recent-item'
    row.innerHTML = `<button class="recent-play"><span class="chip"></span><span class="recent-title"></span></button>
      <button class="icon-btn subtle recent-remove" title="Remove">${icon.clear}</button>`
    row.querySelector('.chip').textContent = kindLabel[item.kind] || 'Link'
    row.querySelector('.recent-title').textContent = item.title || item.url
    row.querySelector('.recent-play').title = item.url
    row.querySelector('.recent-play').addEventListener('click', () => playPath(item.url))
    row.querySelector('.recent-remove').addEventListener('click', (e) => {
      e.stopPropagation()
      RemoveRecent(item.url)
    })
    el.recent.appendChild(row)
  }
  const clear = document.createElement('button')
  clear.className = 'link-btn recent-clear'
  clear.textContent = 'Clear history'
  clear.addEventListener('click', () => RemoveRecent(''))
  el.recent.appendChild(clear)
}

function closeRecent () {
  el.recent.hidden = true
  el.historyBtn.setAttribute('aria-expanded', 'false')
}

el.historyBtn.addEventListener('click', (e) => {
  e.stopPropagation()
  if (el.recent.hidden) {
    el.recent.hidden = false
    el.historyBtn.setAttribute('aria-expanded', 'true')
    renderRecent(state.recent || [])
  } else {
    closeRecent()
  }
})
document.addEventListener('click', (e) => {
  if (!el.recent.hidden && !el.recent.contains(e.target)) closeRecent()
})
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeRecent()
})

// --- clipboard suggestion ----------------------------------------------------

let suggestion = ''
let dismissedClip = ''

function hideSuggest () {
  el.suggest.hidden = true
  suggestion = ''
}

// When Perch gets focus with a video link in the clipboard, offer to play it.
async function checkClipboard () {
  try {
    const text = ((await ClipboardGetText()) || '').trim()
    if (!text || text.length > 2000 || !/^(https?:\/\/|www\.)/i.test(text) ||
        text === dismissedClip || text === el.url.value.trim() || text === state.url) {
      hideSuggest()
      return
    }
    await ValidateURL(text)
    suggestion = text
    el.suggestUrl.textContent = text
    el.suggest.hidden = false
  } catch (e) {
    hideSuggest()
  }
}

el.suggest.addEventListener('click', (e) => {
  if (el.suggestDismiss.contains(e.target)) {
    dismissedClip = suggestion
    hideSuggest()
    return
  }
  if (suggestion) playPath(suggestion)
})
window.addEventListener('focus', checkClipboard)

// --- playback controls -------------------------------------------------------

el.pause.addEventListener('click', () => call(TogglePause))
el.back.addEventListener('click', () => call(() => SeekRelative(-10)))
el.fwd.addEventListener('click', () => call(() => SeekRelative(10)))
el.seek.addEventListener('input', () => {
  draggingSeek = true
  el.timePos.textContent = formatTime(Number(el.seek.value))
  paintRange(el.seek)
})
el.seek.addEventListener('change', async () => {
  await call(() => Seek(Number(el.seek.value)))
  draggingSeek = false
})
el.mute.addEventListener('click', () => call(() => SetMuted(!state.muted)))

// Sends the latest value at most every 60 ms while a slider is dragged.
function throttled (send) {
  let timer = null
  let pending = null
  const flush = () => {
    if (pending === null) {
      timer = null
      return
    }
    const v = pending
    pending = null
    call(() => send(v))
    timer = setTimeout(flush, 60)
  }
  return (v) => {
    pending = v
    if (!timer) flush()
  }
}

const sendVolume = throttled(SetVolume)
el.volume.addEventListener('input', () => {
  draggingVolume = true
  el.volumeValue.textContent = el.volume.value
  paintRange(el.volume)
  sendVolume(Number(el.volume.value))
})
el.volume.addEventListener('change', () => {
  draggingVolume = false
})

el.speed.addEventListener('change', () => call(() => SetSpeed(Number(el.speed.value))))
el.subs.addEventListener('change', () => call(() => SetSubtitle(Number(el.subs.value))))
el.quality.addEventListener('change', () => call(() => SetQuality(el.quality.value)))

// --- window --------------------------------------------------------------

el.sizes.addEventListener('click', (e) => {
  const b = e.target.closest('button[data-size]')
  if (b) call(() => SetPlayerSize(b.dataset.size))
})
el.corners.addEventListener('click', (e) => {
  const b = e.target.closest('button[data-corner]')
  if (b) call(() => SnapPlayer(b.dataset.corner))
})
el.hide.addEventListener('click', () => call(() => SetHidden(!state.hidden)))

const sendOpacity = throttled(SetOpacity)
el.opacity.addEventListener('input', () => {
  draggingOpacity = true
  el.opacityValue.textContent = el.opacity.value + '%'
  paintRange(el.opacity, 20)
  sendOpacity(Number(el.opacity.value))
})
el.opacity.addEventListener('change', () => {
  draggingOpacity = false
})

el.onTop.addEventListener('change', () => call(() => SetAlwaysOnTop(el.onTop.checked)))
el.clickThrough.addEventListener('change', () => call(() => SetClickThrough(el.clickThrough.checked)))

// --- hotkeys and yt-dlp --------------------------------------------------

Hotkeys().then((keys) => {
  el.hotkeys.innerHTML = ''
  for (const k of keys || []) {
    const item = document.createElement('span')
    item.className = 'hotkey' + (k.active ? '' : ' unavailable')
    item.title = k.active ? k.action : `${k.action} – used by another app`
    item.innerHTML = '<kbd></kbd> <span></span>'
    item.querySelector('kbd').textContent = k.keys.replace('Ctrl+Alt+', '')
    item.querySelector('span').textContent = k.action
    el.hotkeys.appendChild(item)
  }
  if (keys && keys.length) el.hotkeys.insertAdjacentHTML('afterbegin', '<span class="hotkey-prefix">Ctrl+Alt +</span>')
}).catch(() => {})

async function refreshYTDLPVersion () {
  const v = await YTDLPVersion().catch(() => '')
  el.ytdlpVersion.textContent = v ? `yt-dlp ${v}` : ''
  el.ytdlpUpdate.hidden = !v
}

let updating = false
async function updateYTDLP () {
  if (updating) return
  updating = true
  el.ytdlpUpdate.disabled = true
  el.ytdlpUpdate.textContent = 'Updating …'
  showMessage('Updating yt-dlp … this can take a minute.', 'info')
  try {
    const result = await UpdateYTDLP()
    showMessage(result + (state.status === 'error' ? ' Press Play to try again.' : ''), 'info')
    refreshYTDLPVersion()
  } catch (e) {
    showMessage(errText(e))
  } finally {
    updating = false
    el.ytdlpUpdate.disabled = false
    el.ytdlpUpdate.textContent = 'Update yt-dlp'
  }
}
el.ytdlpUpdate.addEventListener('click', updateYTDLP)

// --- state sync --------------------------------------------------------------

EventsOn('player:state', render)
GetState().then(render).catch(() => {})
Environment().then((env) => {
  const missing = []
  if (!env.mpv) missing.push('mpv is required for playback: winget install shinchiro.mpv')
  if (!env.ytdlp) missing.push('yt-dlp is required for YouTube and Twitch: winget install yt-dlp.yt-dlp')
  if (missing.length) {
    setupNotice = missing.join('\n') + '\nRestart Perch after installing.'
    if (el.message.hidden) showMessage('')
  }
}).catch(() => {})
refreshYTDLPVersion()
checkClipboard()
paintRange(el.volume)
el.url.focus()
