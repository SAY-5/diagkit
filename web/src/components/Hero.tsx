import { motion } from "framer-motion";
import type { RootCause } from "../sim/analyzer";
import { AnimatedNumber } from "./AnimatedNumber";
import "./Hero.css";

interface Props {
  top: RootCause;
  services: string[];
  logs: number;
  traces: number;
}

const NODES = ["gateway", "orders", "payments", "db"];

export function Hero({ top, logs, traces }: Props) {
  const confidence = Math.round(top.score * 100);

  return (
    <header className="hero" aria-labelledby="hero-title">
      <div className="wrap hero-grid">
        <div className="hero-copy">
          <motion.p
            className="eyebrow"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6 }}
          >
            diagkit / incident command
          </motion.p>

          <motion.h1
            id="hero-title"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.05 }}
          >
            A cascade at 2am.
            <br />
            <span className="hero-em">Which service actually broke?</span>
          </motion.h1>

          <motion.p
            className="hero-lede"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.15 }}
          >
            Alerts fire everywhere at once. diagkit pulls {logs.toLocaleString()} logs and{" "}
            {traces.toLocaleString()} trace spans for the window, collapses the noise into recurring
            failure signatures, and correlates them with metric spikes and dependency blast radius to
            name one culprit, with the evidence to back it.
          </motion.p>

          <motion.div
            className="hero-verdict"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.3 }}
          >
            <span className="hero-verdict-label">likely root cause</span>
            <span className="hero-verdict-service mono">{top.service}</span>
            <span className="hero-verdict-conf">
              <AnimatedNumber value={confidence} suffix="%" durationMs={1600} /> confidence
            </span>
          </motion.div>

          <motion.div
            className="hero-evidence"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.8, delay: 0.55 }}
          >
            <span>
              p95 <strong className="mono"><AnimatedNumber value={top.latencySpikeX} decimals={1} suffix="x" /></strong>
            </span>
            <span>
              error rate{" "}
              <strong className="mono">
                <AnimatedNumber value={Math.round(top.errorRatePeak * 100)} suffix="%" />
              </strong>
            </span>
            <span>
              entry errors{" "}
              <strong className="mono">
                <AnimatedNumber value={top.propagationPct} decimals={0} suffix="%" />
              </strong>
            </span>
          </motion.div>

          <motion.a
            className="hero-cta"
            href="#topology"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.6, delay: 0.7 }}
          >
            walk the diagnosis
          </motion.a>
        </div>

        <motion.div
          className="hero-visual"
          role="img"
          aria-label={`Service topology showing ${top.service} as the failing node with errors propagating upstream`}
          initial={{ opacity: 0, scale: 0.94 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.9, delay: 0.2 }}
        >
          <div className="hero-scan" aria-hidden="true" />
          <svg viewBox="0 0 320 420" className="hero-svg" aria-hidden="true">
            <defs>
              <linearGradient id="edge" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#ff5470" />
                <stop offset="100%" stopColor="#7c6cff" />
              </linearGradient>
            </defs>
            {NODES.slice(0, -1).map((_, i) => (
              <line
                key={i}
                x1="160"
                y1={70 + i * 100}
                x2="160"
                y2={170 + i * 100}
                stroke="url(#edge)"
                strokeWidth="2"
                strokeDasharray="6 6"
              >
                <animate attributeName="stroke-dashoffset" from="24" to="0" dur="1.1s" repeatCount="indefinite" />
              </line>
            ))}
            {NODES.map((svc, i) => {
              const isCulprit = svc === top.service;
              return (
                <g key={svc}>
                  <circle
                    cx="160"
                    cy={70 + i * 100}
                    r={isCulprit ? 34 : 26}
                    fill={isCulprit ? "rgba(255,84,112,0.16)" : "rgba(124,108,255,0.12)"}
                    stroke={isCulprit ? "#ff5470" : "#7c6cff"}
                    strokeWidth={isCulprit ? 2.5 : 1.5}
                  >
                    {isCulprit && (
                      <animate attributeName="r" values="34;39;34" dur="1.8s" repeatCount="indefinite" />
                    )}
                  </circle>
                  <text
                    x="160"
                    y={74 + i * 100}
                    textAnchor="middle"
                    fontFamily="var(--font-mono)"
                    fontSize="12"
                    fill={isCulprit ? "#ff7d92" : "#b9bce0"}
                  >
                    {svc}
                  </text>
                </g>
              );
            })}
          </svg>
        </motion.div>
      </div>
    </header>
  );
}
