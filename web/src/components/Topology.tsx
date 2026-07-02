import { motion } from "framer-motion";
import type { Bundle } from "../sim/types";
import type { RootCause } from "../sim/analyzer";
import { Sparkline } from "./Sparkline";
import "./Topology.css";

interface Props {
  bundle: Bundle;
  ranking: RootCause[];
  culprit: string;
}

// The call chain from entry to leaf, as modeled in the simulator.
const CHAIN = ["gateway", "orders", "payments", "db"];

export function Topology({ bundle, ranking, culprit }: Props) {
  const byService = new Map(ranking.map((r) => [r.service, r]));
  const metricByService = new Map(bundle.metrics.map((m) => [m.service, m]));

  // Depth of the culprit in the chain: services at or above it inherit the
  // cascade, which is what the animated propagation shows.
  const culpritDepth = CHAIN.indexOf(culprit);

  return (
    <section className="section" id="topology" aria-labelledby="topo-title">
      <div className="wrap">
        <p className="eyebrow">step 01 / topology and timeline</p>
        <h2 id="topo-title" className="topo-heading">
          Errors cascade upstream from the fault
        </h2>
        <p className="topo-sub">
          The gateway fronts every request and calls orders, which calls payments, which depends on db.
          A failure downstream surfaces as errors in every caller above it, so the whole chain lights up.
          The spike window is shaded on each series.
        </p>

        <div className="topo-grid">
          {CHAIN.map((svc, i) => {
            const rc = byService.get(svc);
            const m = metricByService.get(svc);
            const isCulprit = svc === culprit;
            const inBlast = culpritDepth >= 0 && i <= culpritDepth;
            const err = m?.error_rate.map((p) => p.value) ?? [];
            const lat = m?.p95_latency_ms.map((p) => p.value) ?? [];
            return (
              <motion.article
                key={svc}
                className={`topo-node panel ${isCulprit ? "is-culprit" : ""} ${inBlast ? "in-blast" : ""}`}
                initial={{ opacity: 0, y: 24 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true, margin: "-60px" }}
                transition={{ duration: 0.5, delay: i * 0.08 }}
              >
                <div className="topo-node-head">
                  <span className="topo-node-name mono">{svc}</span>
                  {isCulprit ? (
                    <span className="topo-tag topo-tag-alert">fault origin</span>
                  ) : inBlast ? (
                    <span className="topo-tag topo-tag-blast">blast radius</span>
                  ) : (
                    <span className="topo-tag">nominal</span>
                  )}
                </div>

                <div className="topo-stats mono">
                  <div>
                    <span className="topo-stat-label">p95</span>
                    <span className="topo-stat-val">{rc?.latencySpikeX.toFixed(1)}x</span>
                  </div>
                  <div>
                    <span className="topo-stat-label">err peak</span>
                    <span className="topo-stat-val">{Math.round((rc?.errorRatePeak ?? 0) * 100)}%</span>
                  </div>
                  <div>
                    <span className="topo-stat-label">entry trace</span>
                    <span className="topo-stat-val">{rc?.propagationPct.toFixed(0)}%</span>
                  </div>
                </div>

                <div className="topo-sparks">
                  <div className="topo-spark">
                    <span className="topo-spark-label">error rate</span>
                    <Sparkline
                      values={err}
                      color={isCulprit ? "#ff5470" : "#7c6cff"}
                      highlightFrom={Math.floor(err.length * 0.3)}
                      label={`${svc} error rate over the incident window`}
                    />
                  </div>
                  <div className="topo-spark">
                    <span className="topo-spark-label">p95 latency</span>
                    <Sparkline
                      values={lat}
                      color={isCulprit ? "#ffc15a" : "#5b4fd6"}
                      highlightFrom={Math.floor(lat.length * 0.3)}
                      label={`${svc} p95 latency over the incident window`}
                    />
                  </div>
                </div>

                {i < CHAIN.length - 1 && (
                  <div className={`topo-edge ${inBlast ? "edge-hot" : ""}`} aria-hidden="true">
                    <span className="topo-edge-line" />
                    <span className="topo-edge-arrow">calls</span>
                  </div>
                )}
              </motion.article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
