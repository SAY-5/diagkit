import { useEffect } from "react";
import { selfCheck } from "./sim/pipeline";
import { useDiagnosis } from "./useDiagnosis";
import { Hero } from "./components/Hero";

export default function App() {
  useEffect(() => {
    selfCheck();
  }, []);

  const { bundle, ranking } = useDiagnosis(42, "payments-outage");

  return (
    <>
      <Hero top={ranking[0]} services={bundle.services} logs={bundle.logs.length} traces={bundle.traces.length} />
    </>
  );
}
