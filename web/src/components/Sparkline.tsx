interface Props {
  values: number[];
  color: string;
  height?: number;
  width?: number;
  highlightFrom?: number; // index where the spike window begins
  label: string;
}

// Sparkline draws a compact time series and shades the portion of the window
// where the incident spike ramps in.
export function Sparkline({ values, color, height = 46, width = 180, highlightFrom, label }: Props) {
  if (values.length === 0) return null;
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  const stepX = width / (values.length - 1 || 1);

  const points = values.map((v, i) => {
    const x = i * stepX;
    const y = height - ((v - min) / span) * (height - 6) - 3;
    return [x, y] as const;
  });

  const path = points.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${path} L${width},${height} L0,${height} Z`;
  const hlX = highlightFrom != null ? highlightFrom * stepX : null;

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      width={width}
      height={height}
      role="img"
      aria-label={label}
      className="sparkline"
      preserveAspectRatio="none"
    >
      {hlX != null && (
        <rect x={hlX} y={0} width={width - hlX} height={height} fill={color} opacity={0.08} />
      )}
      <path d={area} fill={color} opacity={0.12} />
      <path d={path} fill="none" stroke={color} strokeWidth={1.8} strokeLinejoin="round" />
      <circle cx={points[points.length - 1][0]} cy={points[points.length - 1][1]} r={2.6} fill={color} />
    </svg>
  );
}
