import { axisMoney, type EquityPoint } from "../equity";

const width = 720;
const height = 280;
const pad = { l: 58, r: 16, t: 16, b: 32 };

export function EquityChart({ points }: { points: EquityPoint[] }) {
  if (points.length < 2) return null;
  const values = points.map((point) => point.portfolioValue);
  let min = Math.min(...values);
  let max = Math.max(...values);
  if (min === max) {
    min -= 1;
    max += 1;
  }
  const span = max - min;
  const innerW = width - pad.l - pad.r;
  const innerH = height - pad.t - pad.b;
  const x = (index: number) => pad.l + (index / (points.length - 1)) * innerW;
  const y = (value: number) => pad.t + (1 - (value - min) / span) * innerH;
  const line = points
    .map((point, index) => `${index === 0 ? "M" : "L"}${x(index).toFixed(2)},${y(point.portfolioValue).toFixed(2)}`)
    .join(" ");
  const last = points.length - 1;
  const area = `${line} L${x(last).toFixed(2)},${(height - pad.b).toFixed(2)} L${x(0).toFixed(2)},${(height - pad.b).toFixed(2)} Z`;
  const first = points[0];
  const end = points[last];
  if (!first || !end) return null;
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((step) => min + span * step);

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="h-80 w-full font-mono"
      role="img"
      aria-label={`Portfolio value from ${first.date} to ${end.date}`}
    >
      {ticks.map((tick) => (
        <g key={tick}>
          <line
            x1={pad.l}
            x2={width - pad.r}
            y1={y(tick)}
            y2={y(tick)}
            stroke="var(--border)"
            strokeDasharray="3 3"
          />
          <text x={pad.l - 8} y={y(tick) + 3} textAnchor="end" fill="var(--muted-foreground)" fontSize="11">
            {axisMoney(tick)}
          </text>
        </g>
      ))}
      <path d={area} fill="var(--primary)" fillOpacity="0.08" />
      <path d={line} fill="none" stroke="var(--primary)" strokeWidth="2" />
      <text x={x(0)} y={height - 8} fill="var(--muted-foreground)" fontSize="11">
        {first.date.slice(0, 7)}
      </text>
      <text x={x(last)} y={height - 8} textAnchor="end" fill="var(--muted-foreground)" fontSize="11">
        {end.date.slice(0, 7)}
      </text>
    </svg>
  );
}
