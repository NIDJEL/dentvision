import { useEffect, useMemo, useState } from "react";
import { getImageFile, type AnalysisResult, type DentalImage } from "../api/client";

type AnalysisDetailsModalProps = {
  token: string;
  image: DentalImage;
  results: AnalysisResult[];
  onClose: () => void;
};

type ColoredResult = AnalysisResult & {
  color: string;
  description: string;
};

const BOX_COLORS = ["#e23d28", "#007b87", "#7c4dff", "#d18b00", "#2c8f45", "#c23a7a"];

const LABEL_DESCRIPTIONS: Record<string, string> = {
  impacted:
    "Возможная зона ретенции или затрудненного прорезывания. Требуется проверка врачом.",
  caries: "Возможное кариозное изменение твердых тканей. Требуется клиническая проверка.",
  periapical_lesion:
    "Возможное периапикальное изменение рядом с верхушкой корня. Требуется оценка врачом.",
  deep_caries: "Возможная зона глубокого кариозного поражения. Требуется проверка врачом.",
};

export function AnalysisDetailsModal({
  token,
  image,
  results,
  onClose,
}: AnalysisDetailsModalProps) {
  const [imageURL, setImageURL] = useState("");
  const [imageError, setImageError] = useState("");
  const [naturalSize, setNaturalSize] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState(1);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);

  const coloredResults = useMemo<ColoredResult[]>(
    () =>
      results.map((result, index) => ({
        ...result,
        color: BOX_COLORS[index % BOX_COLORS.length],
        description:
          LABEL_DESCRIPTIONS[result.label] ||
          "Подозрительная область, найденная моделью компьютерного зрения. Требуется проверка врачом.",
      })),
    [results],
  );

  useEffect(() => {
    let objectURL = "";
    let cancelled = false;

    setImageURL("");
    setImageError("");
    setNaturalSize({ width: 0, height: 0 });
    setZoom(1);
    setActiveIndex(null);

    getImageFile(token, image.id)
      .then((blob) => {
        if (cancelled) {
          return;
        }

        objectURL = URL.createObjectURL(blob);
        setImageURL(objectURL);
      })
      .catch((err) => {
        if (!cancelled) {
          setImageError(err instanceof Error ? err.message : "Не удалось загрузить снимок");
        }
      });

    return () => {
      cancelled = true;
      if (objectURL) {
        URL.revokeObjectURL(objectURL);
      }
    };
  }, [image.id, token]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const canvasWidth = naturalSize.width > 0 ? naturalSize.width * zoom : 760;
  const canvasHeight = naturalSize.height > 0 ? naturalSize.height * zoom : 460;

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="analysis-modal" role="dialog" aria-modal="true">
        <header className="modal-header">
          <div>
            <h2>Результат анализа</h2>
            <p>{image.original_name}</p>
          </div>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Закрыть">
            X
          </button>
        </header>

        <div className="modal-toolbar">
          <button
            className="secondary-button"
            type="button"
            onClick={() => setZoom((current) => Math.max(0.5, current - 0.25))}
          >
            Уменьшить
          </button>
          <span>{Math.round(zoom * 100)}%</span>
          <button
            className="secondary-button"
            type="button"
            onClick={() => setZoom((current) => Math.min(3, current + 0.25))}
          >
            Увеличить
          </button>
          <button className="secondary-button" type="button" onClick={() => setZoom(1)}>
            Сбросить
          </button>
        </div>

        <div className="analysis-modal-body">
          <div className="analysis-image-viewport">
            {imageError ? <div className="image-placeholder">{imageError}</div> : null}
            {!imageURL && !imageError ? (
              <div className="image-placeholder">Загружаем снимок...</div>
            ) : null}
            {imageURL ? (
              <div
                className="analysis-canvas"
                style={{ width: canvasWidth, height: canvasHeight }}
              >
                <img
                  src={imageURL}
                  alt={image.original_name}
                  onLoad={(event) => {
                    setNaturalSize({
                      width: event.currentTarget.naturalWidth,
                      height: event.currentTarget.naturalHeight,
                    });
                  }}
                />
                {naturalSize.width > 0
                  ? coloredResults.map((result, index) => (
                      <button
                        className={`analysis-box ${activeIndex === index ? "active" : ""}`}
                        key={result.id}
                        style={{
                          borderColor: result.color,
                          color: result.color,
                          left: `${(result.x / naturalSize.width) * 100}%`,
                          top: `${(result.y / naturalSize.height) * 100}%`,
                          width: `${(result.width / naturalSize.width) * 100}%`,
                          height: `${(result.height / naturalSize.height) * 100}%`,
                        }}
                        type="button"
                        onClick={() => setActiveIndex(index)}
                        aria-label={`${result.label} ${Math.round(result.confidence * 100)}%`}
                      >
                        <span style={{ background: result.color }}>{index + 1}</span>
                      </button>
                    ))
                  : null}
              </div>
            ) : null}
          </div>

          <aside className="analysis-side-panel">
            <h3>Найденные зоны</h3>
            <p className="muted">
              Модель выделяет подозрительные зоны. Финальное заключение остается за врачом.
            </p>
            {coloredResults.length === 0 ? (
              <p className="muted">Для этого снимка пока нет сохраненных зон.</p>
            ) : (
              <div className="zone-list">
                {coloredResults.map((result, index) => (
                  <button
                    className={`zone-item ${activeIndex === index ? "active" : ""}`}
                    key={result.id}
                    type="button"
                    onClick={() => setActiveIndex(index)}
                  >
                    <span className="zone-number" style={{ background: result.color }}>
                      {index + 1}
                    </span>
                    <span>
                      <strong>
                        {result.label} - {Math.round(result.confidence * 100)}%
                      </strong>
                      <small>{result.description}</small>
                      <small>
                        x {result.x}, y {result.y}, {result.width}x{result.height}px
                      </small>
                    </span>
                  </button>
                ))}
              </div>
            )}
          </aside>
        </div>
      </section>
    </div>
  );
}
