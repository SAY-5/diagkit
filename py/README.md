# diagkit-rca

Root-cause analyzer for `incident-bundle.json` produced by the `diagkit`
collector. It clusters failure signatures, correlates them with trace error
spans and metric spikes, and prints a ranked, explainable root-cause report.

```
python -m diagkit_rca analyze incident-bundle.json
diagkit collect --out - | python -m diagkit_rca analyze -
```
