import type { TopDriverJSON } from "../types";

interface Props {
  drivers: TopDriverJSON[];
}

export function TopDriversTable({ drivers }: Props) {
  if (drivers.length === 0) {
    return <div className="loading">Нет данных по драйверам.</div>;
  }
  return (
    <table className="drivers-table">
      <thead>
        <tr>
          <th>Домен</th>
          <th className="num">Вклад</th>
          <th className="num">Доля</th>
          <th>Направление</th>
        </tr>
      </thead>
      <tbody>
        {drivers.map((d) => (
          <tr key={d.domain}>
            <td>{d.domain_display}</td>
            <td className="num">
              {d.contribution >= 0 ? "+" : ""}
              {d.contribution.toFixed(4)}
            </td>
            <td className="num">{(d.share * 100).toFixed(1)}%</td>
            <td>
              <span className={`direction-chip ${d.direction}`}>
                {d.direction.replace("_", "-")}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
