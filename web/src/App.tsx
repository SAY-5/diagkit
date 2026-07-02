import { useEffect } from "react";
import { selfCheck } from "./sim/pipeline";

export default function App() {
  useEffect(() => {
    selfCheck();
  }, []);

  return (
    <main className="wrap">
      <section className="section">
        <p className="eyebrow">diagkit</p>
        <h1>incident root-cause console</h1>
        <p className="mono">pipeline online. see the console for the seeded self-check.</p>
      </section>
    </main>
  );
}
