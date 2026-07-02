import { useEffect, useRef, useState } from "react";
import { usePrefersReducedMotion } from "./useReducedMotion";

interface Props {
  value: number;
  decimals?: number;
  durationMs?: number;
  prefix?: string;
  suffix?: string;
  play?: boolean;
}

// AnimatedNumber counts up to a target value with an easing curve. When the
// viewer prefers reduced motion it snaps straight to the final value.
export function AnimatedNumber({
  value,
  decimals = 0,
  durationMs = 1400,
  prefix = "",
  suffix = "",
  play = true,
}: Props) {
  const reduced = usePrefersReducedMotion();
  const [display, setDisplay] = useState(reduced || !play ? value : 0);
  const raf = useRef<number>();

  useEffect(() => {
    if (reduced || !play) {
      setDisplay(value);
      return;
    }
    const start = performance.now();
    const from = 0;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / durationMs);
      const eased = 1 - Math.pow(1 - t, 3);
      setDisplay(from + (value - from) * eased);
      if (t < 1) raf.current = requestAnimationFrame(tick);
    };
    raf.current = requestAnimationFrame(tick);
    return () => {
      if (raf.current) cancelAnimationFrame(raf.current);
    };
  }, [value, durationMs, reduced, play]);

  return (
    <span>
      {prefix}
      {display.toFixed(decimals)}
      {suffix}
    </span>
  );
}
