import "./Footer.css";

const REPO = "https://github.com/SAY-5/diagkit";

export function Footer() {
  return (
    <footer className="footer" aria-labelledby="footer-title">
      <div className="wrap footer-grid">
        <div>
          <h2 id="footer-title" className="footer-title">
            What this is
          </h2>
          <p className="footer-copy">
            diagkit runs a seeded, simulated distributed system, so the entire pipeline is
            reproducible with no real cluster. Same seed, same scenario, same answer, every time. The
            numbers you see here are computed live in your browser by a faithful port of the same
            collector, fingerprinter, and ranking logic that runs on the backend.
          </p>
        </div>
        <div className="footer-meta mono">
          <div className="footer-row">
            <span className="footer-key">topology</span>
            <span>gateway to orders to payments to db</span>
          </div>
          <div className="footer-row">
            <span className="footer-key">signals</span>
            <span>signature density, metric spike, propagation</span>
          </div>
          <div className="footer-row">
            <span className="footer-key">source</span>
            <a href={REPO} target="_blank" rel="noreferrer noopener">
              github.com/SAY-5/diagkit
            </a>
          </div>
        </div>
      </div>
      <div className="wrap footer-base mono">
        <span>diagkit incident root-cause console</span>
        <a href={REPO} target="_blank" rel="noreferrer noopener" className="footer-cta">
          view the repository
        </a>
      </div>
    </footer>
  );
}
