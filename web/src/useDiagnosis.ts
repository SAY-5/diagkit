import { useMemo } from "react";
import { diagnose, type Diagnosis } from "./sim/pipeline";

// useDiagnosis runs the full browser pipeline for a seed and scenario and
// memoizes the result so components can share one deterministic diagnosis.
export function useDiagnosis(seed: number, scenario: string): Diagnosis {
  return useMemo(() => diagnose(seed, scenario), [seed, scenario]);
}
