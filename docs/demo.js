(() => {
  const root = document.querySelector("[data-demo]");
  const code = document.querySelector("#council-demo code");
  if (!root || !code) return;

  const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const steps = ["cmd", "ping", "plan", "critique", "synth", "done"];

  const finalHTML = [
    `<span class="dim">$</span> council "Should we migrate REST to gRPC?"`,
    `<span class="ok">✅</span> Claude · <span class="ok">✅</span> Codex · <span class="ok">✅</span> Gemini · <span class="ok">✅</span> Cursor`,
    `<span class="phase">— Phase 1: Planning —</span>`,
    `<span class="dim">plan.claude.txt · plan.codex.txt · plan.gemini.txt · plan.cursor.txt</span>`,
    `<span class="phase">— Phase 2: Critique + Ranking —</span>`,
    `<span class="warn">🎭</span> Codex Devil’s Advocate · <span class="ok">FINAL RANKING: B &gt; A &gt; C</span>`,
    `<span class="phase">— Phase 3: Synthesis —</span>`,
    `<span class="ok">synthesis.txt</span> · rankings.json`,
    `<span class="ok">Council adjourned</span> → council_runs/run_…/`,
  ].join("\n");

  const setStep = (name) => {
    root.querySelectorAll(".demo-rail span").forEach((el) => {
      el.classList.toggle("on", el.dataset.step === name);
      if (steps.indexOf(el.dataset.step) <= steps.indexOf(name)) {
        el.classList.add("seen");
      }
    });
  };

  if (reduce) {
    code.innerHTML = finalHTML;
    setStep("done");
    root.querySelectorAll(".demo-rail span").forEach((el) => el.classList.add("seen", "on"));
    return;
  }

  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  const typeLine = async (html, charDelay = 18) => {
    const tmp = document.createElement("div");
    tmp.innerHTML = html;
    const plain = tmp.textContent || "";
    let buf = code.innerHTML;
    if (buf && !buf.endsWith("\n") && buf.length) buf += "\n";
    for (let i = 0; i < plain.length; i++) {
      code.innerHTML = buf + plain.slice(0, i + 1) + `<span class="caret"></span>`;
      await sleep(charDelay);
    }
    code.innerHTML = buf + html;
  };

  const appendLine = async (html, delayAfter = 420) => {
    const prefix = code.innerHTML ? code.innerHTML + "\n" : "";
    code.innerHTML = prefix + html;
    await sleep(delayAfter);
  };

  const run = async () => {
    for (;;) {
      code.innerHTML = "";
      root.querySelectorAll(".demo-rail span").forEach((el) => {
        el.classList.remove("on", "seen");
      });

      setStep("cmd");
      await typeLine(`<span class="dim">$</span> council "Should we migrate REST to gRPC?"`, 16);
      await sleep(500);

      setStep("ping");
      await appendLine(`<span class="pulse">🏓</span> Pre-flight ping…`, 380);
      await appendLine(`<span class="ok">✅</span> Claude · <span class="ok">✅</span> Codex · <span class="ok">✅</span> Gemini · <span class="ok">✅</span> Cursor`, 650);

      setStep("plan");
      await appendLine(`<span class="phase">— Phase 1: Planning —</span>`, 400);
      await appendLine(`<span class="dim writing">plan.claude.txt</span>`, 280);
      await appendLine(`<span class="dim writing">plan.codex.txt</span>`, 280);
      await appendLine(`<span class="dim writing">plan.gemini.txt · plan.cursor.txt</span>`, 520);
      await appendLine(`<span class="ok">Valid plans: 4/4</span>`, 520);

      setStep("critique");
      await appendLine(`<span class="phase">— Phase 2: Critique + Ranking —</span>`, 400);
      await appendLine(`<span class="warn">🎭</span> Codex assigned Devil’s Advocate`, 500);
      await appendLine(`<span class="ok">FINAL RANKING: B &gt; A &gt; C</span>`, 520);

      setStep("synth");
      await appendLine(`<span class="phase">— Phase 3: Chairman Synthesis —</span>`, 400);
      await appendLine(`<span class="ok">synthesis.txt</span> · rankings.json`, 650);

      setStep("done");
      await appendLine(`<span class="ok">Council adjourned</span> → council_runs/run_…/`, 2200);
      await sleep(900);
    }
  };

  run();
})();
