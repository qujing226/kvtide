type KpiCardProps = {
  label: string;
  value: string;
  unit?: string;
  detail: string;
};

export function KpiCard({ label, value, unit, detail }: KpiCardProps) {
  return (
    <article className="kpi-card panel">
      <span className="kpi-label">{label}</span>
      <div className="kpi-value">
        {value}
        {unit ? <small>{unit}</small> : null}
      </div>
      <span className="kpi-detail">{detail}</span>
    </article>
  );
}
