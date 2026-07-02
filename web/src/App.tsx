import { useEffect, useState } from "react";
import { selfCheck } from "./sim/pipeline";
import { useDiagnosis } from "./useDiagnosis";
import { Hero } from "./components/Hero";
import { Topology } from "./components/Topology";
import { Signatures } from "./components/Signatures";
import { RootCause } from "./components/RootCause";
import { Footer } from "./components/Footer";

export default function App() {
  useEffect(() => {
    selfCheck();
  }, []);

  // The hero always shows the canonical default incident so the opening frame
  // is stable; the interactive sections below share live seed/scenario state.
  const hero = useDiagnosis(42, "payments-outage");

  const [seed, setSeed] = useState(42);
  const [scenario, setScenario] = useState("payments-outage");
  const { bundle, ranking } = useDiagnosis(seed, scenario);

  return (
    <>
      <a href="#rootcause" className="skip-link">
        Skip to the root-cause ranking
      </a>
      <Hero
        top={hero.ranking[0]}
        services={hero.bundle.services}
        logs={hero.bundle.logs.length}
        traces={hero.bundle.traces.length}
      />
      <main id="main">
        <Topology bundle={bundle} ranking={ranking} culprit={ranking[0].service} />
        <Signatures bundle={bundle} />
        <RootCause
          ranking={ranking}
          scenario={scenario}
          seed={seed}
          onScenario={setScenario}
          onSeed={setSeed}
        />
      </main>
      <Footer />
    </>
  );
}
