import { motion } from "framer-motion";
import type { RootCause as RC } from "../sim/analyzer";
import { W_SIGNATURE, W_SPIKE, W_PROPAGATION } from "../sim/analyzer";
import "./RootCause.css";

interface Props {
  ranking: RC[];
  scenario: string;
  seed: number;
  onScenario: (s: string) => void;
  onSeed: (n: number) => void;
}

const SCENARIO_OPTIONS = [
  { id: "payments-outage", label: "payments outage" },
  { id: "db-slowdown", label: "db slowdown" },
  { id: "healthy", label: "healthy" },
];

export function RootCause({ ranking, scenario, seed, onScenario, onSeed }: Props) {
  const top = ranking[0];
  const topScore = top.score || 1;

  return (
    <section className="section" id="rootcause" aria-labelledby="rc-title">
      <div className="wrap">
        <p className="eyebrow">step 03 / root-cause ranking</p>
        <h2 className="rc-heading">One ranked, explainable answer</h2>
        <p className="rc-sub">
          Each service scores as a weighted sum of three normalized signals: signature density
          ({Math.round(W_SIGNATURE * 100)}%), the metric spike ({Math.round(W_SPIKE * 100)}%), and
          dependency propagation ({Math.round(W_PROPAGATION * 100)}%). Change the scenario or seed to
          re-run the whole pipeline in your browser and watch the culprit move.
        </p>

        <div className="rc-controls panel" role="group" aria-label="Scenario and seed selector">
          <div className="rc-control">
            <span className="rc-control-label">scenario</span>
            <div className="rc-scenario-btns">
              {SCENARIO_OPTIONS.map((o) => (
                <button
                  key={o.id}
                  type="button"
                  className={`rc-chip mono ${scenario === o.id ? "is-active" : ""}`}
                  aria-pressed={scenario === o.id}
                  onClick={() => onScenario(o.id)}
                >
                  {o.label}
                </button>
              ))}
            </div>
          </div>
          <div className="rc-control">
            <label className="rc-control-label" htmlFor="seed-input">
              seed
            </label>
            <div className="rc-seed">
              <button
                type="button"
                className="rc-chip mono"
                aria-label="Decrease seed"
                onClick={() => onSeed(Math.max(0, seed - 1))}
              >
                -
              </button>
              <input
                id="seed-input"
                className="rc-seed-input mono"
                type="number"
                min={0}
                value={seed}
                onChange={(e) => onSeed(Math.max(0, Number(e.target.value) || 0))}
              />
              <button type="button" className="rc-chip mono" aria-label="Increase seed" onClick={() => onSeed(seed + 1)}>
                +
              </button>
            </div>
          </div>
          <div className="rc-verdict-mini">
            <span className="rc-control-label">verdict</span>
            <span className="rc-verdict-mini-val mono">
              {top.service} <span className="rc-verdict-mini-score">{(top.score * 100).toFixed(0)}%</span>
            </span>
          </div>
        </div>

        <ol className="rc-list" aria-label="Ranked services by root-cause likelihood">
          {ranking.map((r, i) => (
            <motion.li
              key={r.service}
              layout
              className={`rc-card panel ${i === 0 ? "is-top" : ""}`}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: i * 0.05 }}
            >
              <div className="rc-card-head">
                <span className="rc-rank mono">{i + 1}</span>
                <span className="rc-service mono">{r.service}</span>
                <span className="rc-score mono">{r.score.toFixed(3)}</span>
              </div>

              <div className="rc-scorebar" aria-hidden="true">
                <motion.span
                  className={`rc-scorefill ${i === 0 ? "fill-top" : ""}`}
                  initial={{ width: 0 }}
                  animate={{ width: `${(r.score / topScore) * 100}%` }}
                  transition={{ duration: 0.7, ease: "easeOut" }}
                />
              </div>

              <div className="rc-breakdown mono">
                <span>{r.signatureCount} sig / {r.signatureErrors} lines</span>
                <span>{r.latencySpikeX.toFixed(1)}x p95</span>
                <span>{Math.round(r.errorRatePeak * 100)}% err</span>
                <span>{r.propagationPct.toFixed(0)}% entry trace</span>
              </div>

              <ul className="rc-reasons">
                {r.reasons.map((reason, k) => (
                  <li key={k}>{reason}</li>
                ))}
              </ul>
            </motion.li>
          ))}
        </ol>
      </div>
    </section>
  );
}
