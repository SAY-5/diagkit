import { motion } from "framer-motion";
import type { Bundle } from "../sim/types";
import { normalize } from "../sim/fingerprint";
import "./Signatures.css";

interface Props {
  bundle: Bundle;
}

// TemplatePart renders a normalized template, styling the placeholder tokens so
// the collapse from raw values to a template reads at a glance.
function TemplatePart({ template }: { template: string }) {
  const parts = template.split(/(<[A-Z]+>)/g);
  return (
    <>
      {parts.map((p, i) =>
        /^<[A-Z]+>$/.test(p) ? (
          <span key={i} className="sig-token">
            {p}
          </span>
        ) : (
          <span key={i}>{p}</span>
        )
      )}
    </>
  );
}

export function Signatures({ bundle }: Props) {
  // Collect up to three raw error examples per top signature for the collapse
  // demo. Examples come straight from the generated logs.
  const topSig = bundle.signatures[0];
  const rawExamples = bundle.logs
    .filter((l) => l.level === "error" && normalize(l.message) === topSig.template)
    .slice(0, 3)
    .map((l) => l.message);

  const totalLines = bundle.signatures.reduce((s, sig) => s + sig.count, 0);

  return (
    <section className="section" id="signatures" aria-labelledby="sig-title">
      <div className="wrap">
        <p className="eyebrow">step 02 / signature clustering</p>
        <h2 className="sig-heading">Thousands of lines collapse into a handful of signatures</h2>
        <p className="sig-sub">
          Every log message is normalized: volatile tokens like ids, durations, and hex become
          placeholders, so lines that differ only in their variable parts group into one signature.
          {" "}
          {totalLines.toLocaleString()} error lines reduce to {bundle.signatures.length} templates.
        </p>

        <div className="sig-collapse panel">
          <div className="sig-raw">
            <span className="sig-mini-label">raw log lines</span>
            {rawExamples.map((m, i) => (
              <motion.pre
                key={i}
                className="sig-rawline mono"
                initial={{ opacity: 0, x: -12 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.4, delay: i * 0.12 }}
              >
                {m}
              </motion.pre>
            ))}
          </div>
          <div className="sig-arrow" aria-hidden="true">
            collapses to
          </div>
          <motion.div
            className="sig-template"
            initial={{ opacity: 0, scale: 0.96 }}
            whileInView={{ opacity: 1, scale: 1 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.4 }}
          >
            <span className="sig-mini-label">one template</span>
            <pre className="sig-templateline mono">
              <TemplatePart template={topSig.template} />
            </pre>
            <span className="sig-count mono">x{topSig.count}</span>
          </motion.div>
        </div>

        <h3 className="sig-rank-title">Top recurring signatures by volume</h3>
        <ul className="sig-list" aria-label="Top error signatures ranked by volume">
          {bundle.signatures.map((sig, i) => {
            const width = (sig.count / bundle.signatures[0].count) * 100;
            return (
              <motion.li
                key={sig.template}
                className="sig-row"
                initial={{ opacity: 0, y: 12 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.4, delay: i * 0.06 }}
              >
                <span className="sig-row-count mono">{sig.count}</span>
                <span className="sig-row-svc mono">{sig.services.join(", ")}</span>
                <span className="sig-row-tmpl mono">
                  <TemplatePart template={sig.template} />
                </span>
                <span className="sig-row-bar" aria-hidden="true">
                  <motion.span
                    className="sig-row-fill"
                    initial={{ width: 0 }}
                    whileInView={{ width: `${width}%` }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.8, delay: 0.1 + i * 0.06, ease: "easeOut" }}
                  />
                </span>
              </motion.li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
