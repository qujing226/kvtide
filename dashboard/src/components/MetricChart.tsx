import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { TimeSeriesPoint } from "../metrics/series";

export type ChartSeries = {
  name: string;
  color: string;
  data: TimeSeriesPoint[];
};

type MetricChartProps = {
  title: string;
  unit: string;
  series: ChartSeries[];
  decimals?: number;
};

type ChartRow = Record<string, number> & { timestamp: number };

function mergeSeries(series: ChartSeries[]): ChartRow[] {
  const rows = new Map<number, ChartRow>();
  for (const entry of series) {
    for (const point of entry.data) {
      const row = rows.get(point.timestamp) ?? { timestamp: point.timestamp };
      row[entry.name] = point.value;
      rows.set(point.timestamp, row);
    }
  }
  return [...rows.values()].sort((left, right) => left.timestamp - right.timestamp);
}

function formatTime(timestamp: number): string {
  return new Intl.DateTimeFormat("en", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(timestamp);
}

export function MetricChart({
  title,
  unit,
  series,
  decimals = 1,
}: MetricChartProps) {
  const data = mergeSeries(series);

  return (
    <article className="metric-panel panel">
      <header className="panel-header">
        <span className="panel-title">{title}</span>
        <div className="chart-legend">
          {series.map((entry) => (
            <span key={entry.name}>
              <i style={{ backgroundColor: entry.color }} />
              {entry.name}
            </span>
          ))}
          <span className="panel-unit">{unit}</span>
        </div>
      </header>
      <div className="metric-chart" aria-label={`${title} chart`}>
        {data.length === 0 ? (
          <div className="empty-chart">Waiting for metric samples</div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 16, right: 18, bottom: 4, left: 0 }}>
              <CartesianGrid vertical={false} stroke="#e9edf3" />
              <XAxis
                axisLine={{ stroke: "#ccd4df" }}
                dataKey="timestamp"
                domain={["dataMin", "dataMax"]}
                minTickGap={58}
                tick={{ fill: "#8a95a6", fontSize: 10 }}
                tickFormatter={formatTime}
                tickLine={false}
                type="number"
              />
              <YAxis
                axisLine={false}
                tick={{ fill: "#8a95a6", fontSize: 10 }}
                tickFormatter={(value: number) => value.toFixed(decimals)}
                tickLine={false}
                width={48}
              />
              <Tooltip
                contentStyle={{
                  border: "1px solid #dfe5ed",
                  borderRadius: 8,
                  boxShadow: "0 12px 28px rgba(31, 42, 58, 0.1)",
                  fontSize: 11,
                }}
                labelFormatter={(value) => formatTime(Number(value))}
                formatter={(value) => [Number(value).toFixed(decimals), unit]}
              />
              {series.map((entry) => (
                <Line
                  connectNulls
                  dataKey={entry.name}
                  dot={false}
                  isAnimationActive={false}
                  key={entry.name}
                  stroke={entry.color}
                  strokeWidth={2}
                  type="monotone"
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
    </article>
  );
}
