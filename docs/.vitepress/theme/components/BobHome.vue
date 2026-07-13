<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'

type Row = { text: string; cls: string; pause?: number }

/* The transcript is the sales pitch: plan, refuse the conflict,
 * apply cleanly, converge. Deterministic — it loops identically. */
const transcript: Row[] = [
  { text: '$ bob plan ./payments-cli', cls: 'cmd', pause: 500 },
  { text: 'recipe go-agent-tool · 14 desired files', cls: 'meta', pause: 300 },
  { text: '', cls: 'meta' },
  { text: '  + create   .github/workflows/ci.yml', cls: 'create' },
  { text: '  + create   cmd/payments-cli/main.go', cls: 'create' },
  { text: '  + create   Taskfile.yml', cls: 'create' },
  { text: '  ~ update   README.md', cls: 'update' },
  { text: '  ! conflict docs/index.md   unmanaged file differs', cls: 'conflict', pause: 700 },
  { text: '', cls: 'meta' },
  { text: 'plan: 3 create · 1 update · 1 conflict · 0 files written', cls: 'meta', pause: 900 },
  { text: '$ bob apply ./payments-cli', cls: 'cmd mut', pause: 500 },
  { text: 'apply: refused — 1 conflict. nothing was written.', cls: 'conflict', pause: 500 },
  { text: '(nothing is ever half-written.)', cls: 'meta', pause: 1100 },
  { text: '$ mv docs/index.md docs/notes.md', cls: 'cmd', pause: 400 },
  { text: '$ bob apply ./payments-cli', cls: 'cmd mut', pause: 500 },
  { text: 'apply: 4 files written · bob.lock updated', cls: 'create', pause: 900 },
  { text: '$ bob plan ./payments-cli', cls: 'cmd', pause: 500 },
  { text: 'no changes. run it again if you like — same answer.', cls: 'converged', pause: 4200 },
]

const lines = ref<Row[]>([])
const typing = ref('')
const typingCls = ref('cmd')
const done = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null
let reduced = false

function schedule(fn: () => void, ms: number) {
  timer = setTimeout(fn, ms)
}

function playRow(index: number) {
  if (index >= transcript.length) {
    lines.value = []
    schedule(() => playRow(0), 400)
    return
  }
  const row = transcript[index]
  if (row.cls.startsWith('cmd')) {
    typingCls.value = row.cls
    let i = 0
    const tick = () => {
      typing.value = row.text.slice(0, i)
      i += 1
      if (i <= row.text.length) {
        schedule(tick, 24)
      } else {
        lines.value = [...lines.value, row]
        typing.value = ''
        schedule(() => playRow(index + 1), row.pause ?? 250)
      }
    }
    tick()
  } else {
    lines.value = [...lines.value, row]
    schedule(() => playRow(index + 1), row.pause ?? 120)
  }
}

onMounted(() => {
  reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (reduced) {
    lines.value = transcript
    done.value = true
    return
  }
  schedule(() => playRow(0), 600)
})

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
})

const copied = ref(false)
function copyInstall() {
  navigator.clipboard
    .writeText('brew tap abdul-hamid-achik/tap && brew install --cask bob')
    .then(() => {
      copied.value = true
      setTimeout(() => (copied.value = false), 1600)
    })
}

const ledger = [
  {
    tag: 'PLAN',
    kind: 'read',
    title: 'Plan before mutation',
    body: 'Every create, update, adoption, and conflict lands on paper before a single byte lands on disk. Review the plan like a diff, because it is one.',
  },
  {
    tag: 'LOCK',
    kind: 'write',
    title: 'Ownership you can audit',
    body: 'bob.lock records a content hash for every file Bob manages. A managed file updates only when its current hash matches the lock. No hash, no touch.',
  },
  {
    tag: 'HALT',
    kind: 'write',
    title: 'Conflicts stop the line',
    body: 'One conflict means zero writes. Bob preflights the complete plan and refuses to half-apply anything, ever.',
  },
  {
    tag: 'DRIFT',
    kind: 'read',
    title: 'Drift detection for CI',
    body: 'bob check exits non-zero the moment generated infrastructure wanders from the contract. Your pipeline finds out before your users do.',
  },
  {
    tag: 'AGENT',
    kind: 'read',
    title: 'Agent-native surfaces',
    body: 'Versioned JSON envelopes on every command, a one-shot bob learn briefing, and six read-only MCP tools. Machines are first-class readers here.',
  },
  {
    tag: 'SCOPE',
    kind: 'write',
    title: 'Honest boundaries',
    body: 'No LLM inside. No secret management. No verification theater. Bob builds the repository; proving your feature works is your job.',
  },
]

const refusals = [
  'Run a model, call a network, or guess.',
  'Overwrite a file it cannot prove it owns.',
  'Touch .git, bob.yaml, or anything outside the workspace.',
  'Write half a plan. One conflict, zero writes.',
  'Manage your secrets.',
  'Declare your feature “verified.” Bob builds; evidence tools judge.',
]
</script>

<template>
  <div class="bob-home">
    <!-- HERO ------------------------------------------------------------ -->
    <section class="hero">
      <div class="wrap hero-grid">
        <div class="hero-copy">
          <p class="eyebrow">Deterministic repository factory</p>
          <h1 class="display">
            Bob has <span class="mark">one job.</span>
          </h1>
          <p class="lede">
            Turn a small <code>bob.yaml</code> into a public-ready repository —
            the packaged Go/Cobra recipe, or a file tree you declare yourself.
            Bob plans before it writes, proves it owns every file it touches,
            and refuses to guess. No model. No magic. A ledger.
          </p>
          <div class="cta-row">
            <a class="btn btn-safety" href="/getting-started">Get started</a>
            <a class="btn btn-draft" href="/agents">For coding agents</a>
          </div>
          <button class="install" type="button" @click="copyInstall" aria-label="Copy install command">
            <span class="install-cmd">brew tap abdul-hamid-achik/tap &amp;&amp; brew install --cask bob</span>
            <span class="install-copy">{{ copied ? 'copied' : 'copy' }}</span>
          </button>
          <p class="badges">
            <span>Model-free</span><span>Local-first</span><span>MIT</span>
            <span class="alpha">Early alpha — read the plan. Bob insists.</span>
          </p>
        </div>

        <!-- Signature: the plan ledger -->
        <div class="term" role="img" aria-label="Animated terminal: bob plan finds a conflict, bob apply refuses to write, the conflict is resolved, apply succeeds, and a second plan reports no changes — the repository has converged.">
          <div class="term-bar">
            <span class="dot"></span><span class="dot"></span><span class="dot"></span>
            <span class="term-title">the ledger</span>
            <span class="term-stamp">DETERMINISTIC</span>
          </div>
          <div class="term-body">
            <div v-for="(l, i) in lines" :key="i" :class="['tl', l.cls]">{{ l.text || ' ' }}</div>
            <div v-if="typing" :class="['tl', typingCls]">{{ typing }}<span class="caret"></span></div>
            <div v-else-if="!done" class="tl"><span class="caret"></span></div>
          </div>
        </div>
      </div>
    </section>

    <!-- CONTRACT STRIP --------------------------------------------------- -->
    <section class="contract">
      <div class="wrap">
        <ol class="stations">
          <li class="station read">
            <span class="station-tag">bob plan</span>
            <strong>Plan</strong>
            <p>Read-only. Compares recipe, lock, and working tree.</p>
          </li>
          <li class="station write">
            <span class="station-tag">bob apply</span>
            <strong>Apply</strong>
            <p>Explicit. Writes only files with proven ownership.</p>
          </li>
          <li class="station read">
            <span class="station-tag">bob check</span>
            <strong>Check</strong>
            <p>CI gate. Non-zero exit the moment anything drifts.</p>
          </li>
          <li class="station converge">
            <span class="station-tag">no-op</span>
            <strong>Converge</strong>
            <p>Run apply twice. The second run does nothing. That is the feature.</p>
          </li>
        </ol>
        <p class="legend">
          <span class="key read">blue — read-only authority</span>
          <span class="key write">orange — explicit mutation</span>
        </p>
      </div>
    </section>

    <!-- LEDGER GRID ------------------------------------------------------ -->
    <section class="grid-section">
      <div class="wrap">
        <h2 class="h2">The spec sheet</h2>
        <div class="cards">
          <article v-for="c in ledger" :key="c.tag" :class="['card', c.kind]">
            <span class="card-tag">{{ c.tag }}</span>
            <h3>{{ c.title }}</h3>
            <p>{{ c.body }}</p>
          </article>
        </div>
      </div>
    </section>

    <!-- FOR AGENTS ------------------------------------------------------- -->
    <section class="agents">
      <div class="wrap agents-grid">
        <div>
          <h2 class="h2">Your agent can read the manual.<br />It is one command long.</h2>
          <p class="body-copy">
            <code>bob learn --json</code> emits a versioned, machine-clean brief:
            the lifecycle, every command, the JSON envelope, and the safety
            invariants an agent can rely on. Run it once at session start and
            your agent knows exactly what Bob will and will not do. Every plan
            action carries a stable conflict code, too — branch on that, not
            the prose next to it.
          </p>
          <pre class="snippet"><code><span class="c-cmd">$ bob learn --json</span>
{
  "schema_version": 1,
  "ok": true,
  "command": "learn",
  "data": { "product": "deterministic repository factory", … }
}</code></pre>
        </div>
        <div>
          <h2 class="h2">Six MCP tools.<br />Zero of them write.</h2>
          <p class="body-copy">
            <code>bob mcp serve</code> exposes a compact, repository-read-only
            surface over stdio. Diagnostics go to stderr; stdout stays
            JSON-RPC-clean.
          </p>
          <pre class="snippet"><code><span class="c-cmd">$ claude mcp add bob -- bob mcp serve .</span></code></pre>
          <ul class="tools">
            <li>bob_plan</li><li>bob_check</li><li>bob_inspect</li>
            <li>bob_stats</li><li>bob_recipe_describe</li><li>bob_validate_manifest</li>
          </ul>
          <p class="stamp-line">All six stamped <span class="ro">READ-ONLY</span>.</p>
        </div>
      </div>
    </section>

    <!-- REFUSALS ----------------------------------------------------------- -->
    <section class="refusals">
      <div class="wrap">
        <h2 class="h2">Things Bob will not do</h2>
        <ul class="refusal-list">
          <li v-for="r in refusals" :key="r">{{ r }}</li>
        </ul>
        <p class="load-bearing">This list is load-bearing.</p>
      </div>
    </section>

    <!-- FINAL CTA ---------------------------------------------------------- -->
    <section class="final">
      <div class="wrap final-grid">
        <div>
          <h2 class="display display-small">Put on the hard hat.</h2>
          <p class="body-copy">
            One manifest, one plan, one explicit apply. Review the diff like a
            professional and ship a repository you can defend.
          </p>
        </div>
        <div class="final-code">
          <pre class="snippet"><code><span class="c-cmd">$ brew tap abdul-hamid-achik/tap</span>
<span class="c-cmd">$ brew install --cask bob</span>
<span class="c-meta"># or</span>
<span class="c-cmd">$ go install github.com/abdul-hamid-achik/bob/cmd/bob@latest</span></code></pre>
          <a class="btn btn-safety" href="/getting-started">Build something reviewable</a>
        </div>
      </div>
      <p class="foot-line">Deterministic plans · Explicit authority · Honest integration boundaries</p>
    </section>
  </div>
</template>

<style scoped>
.bob-home {
  --ink: #0e1116;
  --steel: #151b23;
  --panel: #10151c;
  --line: #29323e;
  --bone: #e9edf2;
  --dim: #97a2b0;
  --safety: #f25c1a;
  --safety-bright: #ff7a33;
  --draft: #6ca8ff;
  --create: #3ecf8e;
  --update: #d9b23d;
  --conflict: #ff5c5c;
  background:
    linear-gradient(rgba(108, 168, 255, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(108, 168, 255, 0.045) 1px, transparent 1px),
    var(--ink);
  background-size: 32px 32px, 32px 32px, auto;
  color: var(--bone);
  font-family: var(--vp-font-family-base);
}

.wrap {
  max-width: 1120px;
  margin: 0 auto;
  padding: 0 24px;
}

.display {
  font-family: 'Archivo Black', 'Archivo', sans-serif;
  font-size: clamp(2.6rem, 6.5vw, 4.6rem);
  line-height: 1.02;
  letter-spacing: -0.015em;
  margin: 0 0 20px;
  color: var(--bone);
}
.display-small {
  font-size: clamp(1.9rem, 4vw, 2.8rem);
}
.mark {
  color: var(--safety-bright);
  white-space: nowrap;
}

.h2 {
  font-family: 'Archivo Black', 'Archivo', sans-serif;
  font-size: clamp(1.4rem, 2.6vw, 2rem);
  line-height: 1.15;
  margin: 0 0 18px;
  border: none;
  padding: 0;
  color: var(--bone);
}

.eyebrow {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.75rem;
  letter-spacing: 0.28em;
  text-transform: uppercase;
  color: var(--draft);
  margin: 0 0 18px;
}

/* Hero */
.hero {
  padding: 88px 0 72px;
  border-bottom: 1px solid var(--line);
}
.hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr);
  gap: 56px;
  align-items: center;
}
.lede {
  font-size: 1.1rem;
  line-height: 1.65;
  color: var(--dim);
  max-width: 46ch;
  margin: 0 0 28px;
}
.lede code {
  color: var(--draft);
  background: rgba(108, 168, 255, 0.1);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.92em;
}

.cta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
}
.btn {
  display: inline-block;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.88rem;
  font-weight: 700;
  padding: 12px 22px;
  border-radius: 6px;
  text-decoration: none;
  transition: transform 0.12s ease, background 0.15s ease;
}
.btn:hover {
  transform: translateY(-1px);
}
.btn-safety {
  background: var(--safety);
  color: #0e1116;
}
.btn-safety:hover {
  background: var(--safety-bright);
}
.btn-draft {
  border: 1px solid var(--draft);
  color: var(--draft);
}
.btn-draft:hover {
  background: rgba(108, 168, 255, 0.12);
}

.install {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 10px 14px;
  cursor: pointer;
  color: var(--bone);
  max-width: 100%;
}
.install-cmd {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.8rem;
  color: var(--dim);
  overflow-x: auto;
  white-space: nowrap;
}
.install-copy {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--safety-bright);
  flex-shrink: 0;
}

.badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-top: 20px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  letter-spacing: 0.06em;
  color: var(--dim);
}
.badges .alpha {
  color: var(--update);
}

/* Terminals stay terminals in both themes: pin the dark palette on
 * the ledger, snippets, and the install pill. */
.term,
.snippet,
.install {
  --panel: #10151c;
  --steel: #151b23;
  --line: #29323e;
  --bone: #e9edf2;
  --dim: #97a2b0;
  --draft: #6ca8ff;
  --safety-bright: #ff7a33;
  --create: #3ecf8e;
  --update: #d9b23d;
  --conflict: #ff5c5c;
  color: var(--bone);
}

/* Terminal */
.term {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45);
  min-width: 0;
}
.term-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--line);
  background: var(--steel);
}
.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--line);
}
.term-title {
  margin-left: 10px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.72rem;
  color: var(--dim);
}
.term-stamp {
  margin-left: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.62rem;
  letter-spacing: 0.2em;
  color: var(--draft);
  border: 1px solid rgba(108, 168, 255, 0.4);
  padding: 2px 8px;
  border-radius: 3px;
}
.term-body {
  padding: 18px 18px 22px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.8rem;
  line-height: 1.7;
  min-height: 396px;
  overflow-x: auto;
}
.tl {
  white-space: pre;
}
.tl.cmd {
  color: var(--draft);
  font-weight: 700;
}
.tl.cmd.mut {
  color: var(--safety-bright);
}
.tl.meta {
  color: var(--dim);
}
.tl.create {
  color: var(--create);
}
.tl.update {
  color: var(--update);
}
.tl.conflict {
  color: var(--conflict);
}
.tl.converged {
  color: var(--create);
  font-weight: 700;
}
.caret {
  display: inline-block;
  width: 8px;
  height: 1em;
  background: var(--safety-bright);
  vertical-align: text-bottom;
  animation: blink 1s steps(1) infinite;
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .caret {
    animation: none;
  }
}

/* Contract strip */
.contract {
  padding: 64px 0;
  border-bottom: 1px solid var(--line);
}
.stations {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  list-style: none;
  margin: 0;
  padding: 0;
  counter-reset: station;
}
.station {
  border-top: 3px solid var(--line);
  padding: 16px 4px 0;
}
.station.read {
  border-top-color: var(--draft);
}
.station.write {
  border-top-color: var(--safety);
}
.station.converge {
  border-top-color: var(--create);
}
.station-tag {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.7rem;
  letter-spacing: 0.14em;
  color: var(--dim);
}
.station strong {
  display: block;
  font-family: 'Archivo Black', 'Archivo', sans-serif;
  font-size: 1.15rem;
  margin: 6px 0;
}
.station p {
  color: var(--dim);
  font-size: 0.88rem;
  line-height: 1.55;
  margin: 0;
}
.legend {
  display: flex;
  gap: 24px;
  margin-top: 28px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.7rem;
  letter-spacing: 0.08em;
}
.key.read {
  color: var(--draft);
}
.key.write {
  color: var(--safety-bright);
}

/* Cards */
.grid-section {
  padding: 72px 0;
  border-bottom: 1px solid var(--line);
}
.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
}
.card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 22px;
  position: relative;
}
.card-tag {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.62rem;
  letter-spacing: 0.22em;
  padding: 3px 8px;
  border-radius: 3px;
}
.card.read .card-tag {
  color: var(--draft);
  border: 1px solid rgba(108, 168, 255, 0.35);
}
.card.write .card-tag {
  color: var(--safety-bright);
  border: 1px solid rgba(242, 92, 26, 0.4);
}
.card h3 {
  font-family: 'Archivo Black', 'Archivo', sans-serif;
  font-size: 1.05rem;
  margin: 14px 0 8px;
  color: var(--bone);
}
.card p {
  color: var(--dim);
  font-size: 0.88rem;
  line-height: 1.6;
  margin: 0;
}

/* Agents */
.agents {
  padding: 72px 0;
  border-bottom: 1px solid var(--line);
}
.agents-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
}
.body-copy {
  color: var(--dim);
  line-height: 1.65;
  font-size: 0.98rem;
}
.body-copy code {
  color: var(--safety-bright);
  background: rgba(242, 92, 26, 0.1);
  padding: 1px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.9em;
}
.snippet {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 16px 18px;
  overflow-x: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.8rem;
  line-height: 1.7;
  color: var(--dim);
  margin: 18px 0;
}
.c-cmd {
  color: var(--draft);
  font-weight: 700;
}
.c-meta {
  color: var(--dim);
}
.tools {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  list-style: none;
  padding: 0;
  margin: 18px 0 10px;
}
.tools li {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.74rem;
  color: var(--draft);
  border: 1px solid rgba(108, 168, 255, 0.35);
  border-radius: 4px;
  padding: 4px 10px;
}
.stamp-line {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.74rem;
  color: var(--dim);
}
.ro {
  color: var(--create);
  letter-spacing: 0.15em;
}

/* Refusals */
.refusals {
  padding: 72px 0;
  border-bottom: 1px solid var(--line);
}
.refusal-list {
  list-style: none;
  padding: 0;
  margin: 0;
  max-width: 640px;
}
.refusal-list li {
  padding: 14px 0 14px 34px;
  border-bottom: 1px dashed var(--line);
  color: var(--bone);
  font-size: 1.02rem;
  position: relative;
}
.refusal-list li::before {
  content: '✕';
  position: absolute;
  left: 4px;
  color: var(--conflict);
  font-family: 'JetBrains Mono', monospace;
  font-weight: 700;
}
.load-bearing {
  margin-top: 20px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.78rem;
  letter-spacing: 0.1em;
  color: var(--dim);
}

/* Final */
.final {
  padding: 80px 0 56px;
}
.final-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: center;
}
.final-code .btn {
  margin-top: 6px;
}
.foot-line {
  text-align: center;
  margin: 64px auto 0;
  padding: 0 24px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.68rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--dim);
}

/* Responsive */
@media (max-width: 900px) {
  .hero {
    padding: 56px 0 48px;
  }
  .hero-grid,
  .agents-grid,
  .final-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 36px;
  }
  .hero-grid > *,
  .agents-grid > *,
  .final-grid > * {
    min-width: 0;
  }
  .mark {
    white-space: normal;
  }
  .cards {
    grid-template-columns: 1fr;
  }
  .stations {
    grid-template-columns: 1fr 1fr;
  }
  .term-body {
    min-height: 0;
  }
}
@media (max-width: 520px) {
  .stations {
    grid-template-columns: 1fr;
  }
}
</style>
