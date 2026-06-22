import type { AnalysisResult } from "../api/client";

type AnalysisResultsProps = {
  results: AnalysisResult[];
};

export function AnalysisResults({ results }: AnalysisResultsProps) {
  if (results.length === 0) {
    return <p className="muted">No saved analysis results yet.</p>;
  }

  return (
    <div className="results-table-wrap">
      <table className="results-table">
        <thead>
          <tr>
            <th>Label</th>
            <th>Confidence</th>
            <th>X</th>
            <th>Y</th>
            <th>Width</th>
            <th>Height</th>
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
