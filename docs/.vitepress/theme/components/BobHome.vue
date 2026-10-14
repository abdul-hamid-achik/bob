<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'

type Row = { text: string; cls: string; pause?: number }

/* The transcript is the sales pitch: plan, refuse the conflict,
 * apply cleanly, converge. Deterministic — it loops identically. */
const transcript: Row[] = [
  { text: '$ bob plan ./payments-cli', cls: 'cmd', pause: 500 },
  { text: 'recipe go-agent-tool@6 · 30 files', cls: 'meta', pause: 300 },
  { text: '', cls: 'meta' },
  { text: 'create     .github/workflows/ci.yml        [missing] scaffold', cls: 'create' },
  { text: 'create     cmd/payments-cli/main.go        [missing] scaffold', cls: 'create' },
  { text: 'create     Taskfile.yml                    [missing] scaffold', cls: 'create' },
  { text: '… 27 more', cls: 'meta', pause: 300 },
  { text: '', cls: 'meta' },
  { text: '30 create, 0 update, 0 adopt, 0 unchanged, 0 retire, 0 conflict', cls: 'meta', pause: 900 },
  { text: '$ bob apply ./payments-cli', cls: 'cmd mut', pause: 500 },
  { text: 'applied: 30 written, 0 adopted, 0 unchanged; lock written: true', cls: 'create', pause: 1000 },
  { text: '$ echo "// hand edit" >> internal/cli/root.go', cls: 'cmd', pause: 500 },
  { text: '$ bob plan ./payments-cli', cls: 'cmd', pause: 400 },
  { text: 'conflict   internal/cli/root.go             [managed_hash_mismatch] contract_drift', cls: 'conflict', pause: 700 },
  { text: '0 create, 0 update, 0 adopt, 29 unchanged, 0 retire, 1 conflict', cls: 'meta', pause: 700 },
  { text: '$ bob apply ./payments-cli', cls: 'cmd mut', pause: 500 },
  { text: 'bob: apply: plan contains conflicts; run bob plan for details', cls: 'conflict', pause: 700 },
  { text: '(exit 2 — the handled edit was not overwritten, nothing was written.)', cls: 'meta', pause: 1000 },
  { text: '$ git checkout internal/cli/root.go && bob apply ./payments-cli', cls: 'cmd mut', pause: 500 },
  { text: 'applied: 0 written, 0 adopted, 30 unchanged; lock written: false', cls: 'converged', pause: 900 },
  { text: 'run it again — same answer. that is the point.', cls: 'converged', pause: 3600 },
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
    body: 'One conflict means zero writes: Bob preflights the complete plan and refuses before it writes anything. (Multi-file apply is not one global transaction — a crash can leave a partial publish, and the next plan reports it honestly.)',
  },
  {
    tag: 'DRIFT',
    kind: 'read',
    title: 'Drift detection for CI',
    body: 'bob check exits non-zero the moment generated infrastructure wanders from the contract — and emits --sarif for code scanning or --junit for CI test reporting. Your pipeline finds out before your users do.',
  },
  {
    tag: 'DIGEST',
    kind: 'write',
    title: 'Reviewed, then refused unless it matches',
    body: 'bob apply and bob remove take an expect digest. Bob fresh-plans while holding the lock and writes nothing unless the fresh result equals the plan you reviewed. A removal digest never authorizes an apply, and a plan digest never authorizes a removal. One mutation per workspace; a second fails fast as workspace_busy.',
  },
  {
    tag: 'AGENT',
    kind: 'read',
    title: 'Agent-native surfaces',
    body: 'Versioned JSON envelopes on every command, a one-shot bob learn briefing, and an MCP server whose read tools never write and whose one apply tool is opt-in and digest-gated. Machines are first-class readers here.',
  },
  {
    tag: 'SCOPE',
    kind: 'write',
    title: 'Honest boundaries',
    body: 'No LLM inside. No secret management. No verification theater. Bob builds the repository; proving your feature works is your job.',
  },
]

const refusals = [
  'Run a model or call a network (the one opt-in exception is bob inspect --probe-integrations).',
  'Overwrite a file it cannot prove it owns.',
  'Generate your Next.js, FastAPI, or Cargo app. Hygiene, not the product.',
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
            Turn a small <code>bob.yaml</code> into a reviewable repository (early alpha) —
            a packaged CLI scaffold (Go, Rust, Python, TypeScript, Java, or C#),
            seed-once hygiene on 27 existing language and infrastructure stacks,
            or a file tree you declare. Bob plans before it writes, proves it owns
            every file it touches, and refuses to guess. No model. No magic. A ledger.
          </p>
          <div class="cta-row">
            <a class="btn btn-safety" href="/getting-started">Get started</a>
            <a class="btn btn-draft" href="/guides/existing-repository">Existing repo</a>
            <a class="btn btn-draft" href="/agents">For coding agents</a>
          </div>
          <button class="install" type="button" @click="copyInstall" aria-label="Copy install command">
            <span class="install-cmd">brew tap abdul-hamid-achik/tap &amp;&amp; brew install --cask bob</span>
            <span class="install-copy">{{ copied ? 'copied' : 'copy' }}</span>
          </button>
          <p class="badges">
            <span>Model-free</span><span>Local-first</span><span>MIT</span>
            <span>Deterministic — same plan, every run</span>
            <span>Safety pinned by terminal behavior specs</span>
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
          <li class="station write">
            <span class="station-tag">bob remove</span>
            <strong>Teardown</strong>
            <p>Deletes only proven lock-owned files. <code>bob.yaml</code> survives.</p>
          </li>
          <li class="station write">
            <span class="station-tag">bob upgrade</span>
            <strong>Upgrade</strong>
            <p>Migrates <code>bob.lock</code> to the current recipe version.</p>
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
          <span class="key">start: <code>bob new</code> / <code>bob init</code> · evolve: <code>bob upgrade</code> · teardown: <code>bob remove</code> · every command speaks <code>--json</code></span>
        </p>
      </div>
    </section>

    <!-- THREE PATHS ------------------------------------------------------ -->
    <section class="grid-section paths">
      <div class="wrap">
        <h2 class="h2">Three paths. Same ledger.</h2>
        <p class="body-copy paths-lede">
          Bob does not generate your application. It makes the repository
          contract visible and refuses to smash living files.
        </p>
        <div class="cards">
          <a class="card read path-card" href="/guides/existing-repository">
            <span class="card-tag">STACK</span>
            <h3>Existing repo</h3>
            <p>
              Twenty-seven stacks — Go, TypeScript, Python, Rust, Java, C#/.NET,
              PHP, Ruby, and 19 more, including Docker, Terraform, Nix, Helm, and
              Kubernetes. Seed docs and ignores once; never own <code>src/</code>.
            </p>
          </a>
          <a class="card write path-card" href="/guides/any-repository">
            <span class="card-tag">FILES</span>
            <h3>Declared tree</h3>
            <p>
              You write the paths in <code>bob.yaml</code>. Bob materializes
              them with the same plan, apply, and lock rules.
            </p>
          </a>
          <a class="card read path-card" href="/getting-started">
            <span class="card-tag">FACTORY</span>
            <h3>Six CLI scaffolds</h3>
            <p>
              Go/Cobra, Rust, Python, TypeScript, Java, and C#. Each owns its
              <code>src/</code> tree, tests, CI, and release files, while its
              living files (<code>README</code>, <code>CHANGELOG</code>, the
              language manifest) seed once and stay in human hands.
            </p>
          </a>
        </div>
      </div>
    </section>

    <!-- CATALOG ---------------------------------------------------------- -->
    <section class="grid-section">
      <div class="wrap">
        <h2 class="h2">What you actually get</h2>
        <p class="body-copy paths-lede">
          Thirty-four recipes in three shapes. Pick one; Bob renders it under the
          same plan, lock, and refusal rules.
        </p>
        <div class="cards">
          <article class="card write">
            <span class="card-tag">FACTORY · 6</span>
            <h3>Public CLI scaffolds</h3>
            <p>
              <code>go-agent-tool</code>, <code>rust-tool</code>, <code>python-tool</code>,
              <code>ts-tool</code>, <code>java-tool</code>, <code>csharp-tool</code>. A thin
              entry point over a tested pure function, human and JSON output, tests, CI,
              and release plumbing in one command. The living files (<code>README</code>,
              <code>CHANGELOG</code>, the language manifest) seed once and stay yours.
            </p>
          </article>
          <article class="card read">
            <span class="card-tag">HYGIENE · 27</span>
            <h3>Seed-once stack hygiene</h3>
            <p>
              Go, TypeScript, JavaScript, Vue, Nuxt, Deno, Python, Ruby, PHP, Java,
              Kotlin, .NET, Rust, Swift, Elixir, Dart, Zig, Scala, Lua, C/C++, plus
              Docker, Terraform, Nix, Ansible, Helm, Kubernetes, and static web.
              Seeds <code>README</code>, <code>AGENTS</code>, <code>SECURITY</code>,
              ignores, and a CI stub only when missing — never updated by Bob, and
              never owning <code>src/</code>. (Delete a seed and the next apply
              recreates it.)
            </p>
          </article>
          <article class="card read">
            <span class="card-tag">FILES · 1</span>
            <h3>A tree you declare</h3>
            <p>
              <code>recipe: files</code>. Write the paths in <code>bob.yaml</code>; Bob
              materializes them under the same plan, apply, and lock rules — existence,
              mode, and byte-for-byte convergence, not a template engine in disguise.
            </p>
          </article>
        </div>
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

    <!-- HOW SAFETY WORKS ------------------------------------------------- -->
    <section class="grid-section">
      <div class="wrap">
        <h2 class="h2">The mechanism, not the motto.</h2>
        <ol class="stations">
          <li class="station read"><span class="station-tag">bob.yaml</span><strong>Intent</strong><p>Your recipe and product identity, in one small, strict file.</p></li>
          <li class="station read"><span class="station-tag">bob.lock</span><strong>Ownership</strong><p>A SHA-256 digest of every whole file Bob manages.</p></li>
          <li class="station write"><span class="station-tag">hash match</span><strong>Proof to update</strong><p>A managed file changes only while its current hash still matches the lock.</p></li>
          <li class="station write"><span class="station-tag">atomic</span><strong>Atomic per file</strong><p>Each file is staged as a temp sibling and published by atomic rename; <code>bob.lock</code> is written last.</p></li>
        </ol>
        <p class="legend">
          <span class="key read">Hand-edit a managed file and it becomes a conflict, not a silent overwrite.</span>
          <span class="key write">One conflict blocks the entire apply; symlinks and special files are always conflicts.</span>
        </p>
        <p class="body-copy">
          Two ownership classes: <strong>seed-once</strong> files (created if missing,
          never in the lock, never a conflict) and <strong>managed</strong> files
          (hash-owned, so Bob can prove what is safe to change). Mark any path
          <code>ownership.release</code> and Bob seeds it once and leaves it to you — your
          hand-edit is never a permanent apply block. When a recipe stops declaring a file,
          Bob calls it <strong>retired</strong>: delete it deliberately with
          <code>--allow-retire</code>, or leave it as a conflict. It never silently discards
          ownership. <a href="/ownership-and-safety">More on ownership &amp; safety</a>
        </p>
      </div>
    </section>

    <!-- CI ---------------------------------------------------------------- -->
    <section class="grid-section">
      <div class="wrap agents-grid">
        <div>
          <h2 class="h2">Your pipeline finds out first.</h2>
          <p class="body-copy">
            <code>bob check</code> is the gate: it exits non-zero the moment generated
            infrastructure drifts from the contract. The same plan also renders as a
            SARIF 2.1.0 log for code scanning or a JUnit XML report for your test
            reporter — no adapter, no wrapper.
          </p>
          <pre class="snippet"><code>exit 0   bob check: clean (bob plan always exits 0 — a report, not a gate)
exit 2   an ownership conflict · apply would refuse
exit 3   drift, no conflict · apply would proceed
exit 4   invalid input or manifest
exit 5   fresh plan ≠ reviewed digest · nothing written</code></pre>
        </div>
        <div>
          <h2 class="h2">Two flags, done.</h2>
          <pre class="snippet"><code><span class="c-cmd">$ bob check --sarif</span>    <span class="c-meta"># SARIF 2.1.0 → GitHub code scanning</span>
<span class="c-cmd">$ bob check --junit</span>    <span class="c-meta"># JUnit XML → your CI test reporter</span>
<span class="c-cmd">$ bob check --conflicts-only --json</span>  <span class="c-meta"># just the blockers</span></code></pre>
          <p class="body-copy">
            A code-scanning upload fails the build exactly as a plain
            <code>bob check</code> would: same exit code, same answer. Local-only and
            model-free — the sole exception is the opt-in
            <code>bob inspect --probe-integrations</code>, which runs Codemap/Vecgrep
            status commands you explicitly asked for.
          </p>
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
  "data": { "product": "deterministic repository factory, contract compiler, guidance provider, and lifecycle reconciler", … }
}</code></pre>
        </div>
        <div>
          <h2 class="h2">Ten MCP tools.<br />One can write, only if you say so.</h2>
          <p class="body-copy">
            <code>bob mcp serve</code> exposes nine read-only tools over stdio,
            plus <code>bob_apply</code>: off unless you pass
            <code>--allow-apply-workspace</code>, and gated on the exact
            reviewed plan digest. Diagnostics go to stderr; stdout stays
            JSON-RPC-clean.
          </p>
          <p class="body-copy">
            The read side is the point. <code>bob context --json</code> describes
            the one repository you are standing in — recipe, capabilities,
            ownership, and the current plan digest — with a
            <code>--profile</code> from <code>compact</code> to <code>full</code>.
            Before editing, <code>bob path internal/cli/root.go</code> answers
            <code>will_conflict</code> or <code>outside_bob_ownership</code>, and
            <code>--batch</code> checks seven paths at once. Closed
            <code>bob playbook</code> procedures (<code>add-cli-command</code>,
            <code>resolve-ownership-conflict</code>, …) come with typed inputs,
            never prose to parse.
          </p>
          <pre class="snippet"><code><span class="c-cmd">$ claude mcp add bob -- bob mcp serve .</span>
<span class="c-meta"># the one write, only when you say so:</span>
<span class="c-cmd">$ bob mcp serve . --allow-apply-workspace "$(pwd)"</span></code></pre>
          <ul class="tools">
            <li>bob_context</li><li>bob_path</li><li>bob_playbook</li>
            <li>bob_inspect</li><li>bob_plan</li><li>bob_check</li>
            <li>bob_validate_manifest</li><li>bob_recipe_describe</li><li>bob_stats</li>
            <li>bob_apply (opt-in)</li>
          </ul>
          <p class="stamp-line">All nine stamped <span class="ro">READ-ONLY</span>. <code>bob_apply</code> is opt-in and digest-gated.</p>
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

    <!-- WHY NOT A GENERATOR ---------------------------------------------- -->
    <section class="grid-section">
      <div class="wrap">
        <h2 class="h2">Why not just a generator</h2>
        <div class="cards">
          <article class="card read">
            <span class="card-tag">RECONCILE</span>
            <h3>Generators spray. Bob reconciles.</h3>
            <p>A generator writes a tree once and forgets. Bob records <code>bob.lock</code> and reports drift on every later run, for the life of the repo.</p>
          </article>
          <article class="card write">
            <span class="card-tag">REFUSE</span>
            <h3>Generators guess. Bob refuses.</h3>
            <p>Where <code>cookiecutter</code> or <code>copier</code> overwrite whatever is in the way, Bob stops at the first file it cannot prove it owns.</p>
          </article>
          <article class="card read">
            <span class="card-tag">CONVERGE</span>
            <h3>Generators exit. Bob converges.</h3>
            <p>Run <code>bob apply</code> twice; the second is a no-op. That is the contract, not a side effect.</p>
          </article>
        </div>
      </div>
    </section>

    <!-- FINAL CTA ---------------------------------------------------------- -->
    <section class="final">
      <div class="wrap final-grid">
        <div>
          <h2 class="display display-small">Put on the hard hat.</h2>
          <p class="body-copy">
            One manifest, one plan, one explicit apply. Review the diff like a
            professional. Already have a TypeScript or Python repo? Seed
            hygiene — do not ask Bob to invent the app.
          </p>
        </div>
        <div class="final-code">
          <pre class="snippet"><code><span class="c-cmd">$ brew tap abdul-hamid-achik/tap</span>
<span class="c-cmd">$ brew install --cask bob</span>
<span class="c-meta"># or</span>
<span class="c-cmd">$ go install github.com/abdul-hamid-achik/bob/cmd/bob@latest</span>
<span class="c-meta"># then, in one motion:</span>
<span class="c-cmd">$ bob new acme-tool --recipe go-agent-tool --module github.com/acme/acme-tool --write</span>
<span class="c-cmd">$ bob check</span>
<span class="c-meta"># existing repo instead:</span>
<span class="c-cmd">$ bob init . --write</span>
<span class="c-cmd">$ bob plan &amp;&amp; bob apply &amp;&amp; bob check</span></code></pre>
          <div class="cta-row">
            <a class="btn btn-safety" href="/getting-started">Build something reviewable</a>
            <a class="btn btn-draft" href="/guides/existing-repository">Bring Bob to an existing repo</a>
          </div>
        </div>
      </div>
      <p class="foot-line">Deterministic plans · Explicit authority · Honest integration boundaries</p>
      <p class="foot-line">Proven, not asserted — CI runs the full Go test suite with <code>-race</code>, and the safety behavior is pinned by Glyphrun terminal-behavior specs.</p>
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
.paths-lede {
  max-width: 62ch;
  margin: -6px 0 28px;
}
.path-card {
  display: block;
  text-decoration: none;
  color: inherit;
  transition: border-color 0.15s ease, transform 0.12s ease;
}
.path-card:hover {
  border-color: var(--draft);
  transform: translateY(-2px);
}
.path-card code {
  color: var(--draft);
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.86em;
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
