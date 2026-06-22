import type { AnalysisResult } from "../api/client";

type AnalysisResultsProps = {
  results: AnalysisResult[];
  emptyMessage?: string;
};

export function AnalysisResults({
  results,
  emptyMessage = "Для этого снимка пока нет сохраненных результатов. Сначала запустите анализ.",
}: AnalysisResultsProps) {
  if (results.length === 0) {
    return <p className="muted">{emptyMessage}</p>;
  }

  return (
    <div className="results-table-wrap">
      <table className="results-table">
        <thead>
          <tr>
            <th>Метка</th>
            <th>Уверенность</th>
            <th>X</th>
            <th>Y</th>
            <th>Ширина</th>
            <th>Высота</th>
          </tr>
        </thead>
        <tbody>
          {results.map((result) => (
            <tr key={result.id}>
              <td>{result.label}</td>
              <td>{Math.round(result.confidence * 100)}%</td>
              <td>{result.x}</td>
              <td>{result.y}</td>
              <td>{result.width}</td>
              <td>{result.height}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
