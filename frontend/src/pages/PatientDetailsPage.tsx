import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  deleteImage,
  deletePatient,
  getImageAnalysis,
  getPatient,
  getPatientImages,
  runImageAnalysis,
  updatePatient,
  uploadPatientImage,
  type AnalysisResult,
  type CreatePatientPayload,
  type DentalImage,
  type Patient,
  type User,
} from "../api/client";
import { AnalysisResults } from "../components/AnalysisResults";
import { AnalysisDetailsModal } from "../components/AnalysisDetailsModal";
import { AppShell } from "../components/AppShell";
import { ErrorBanner } from "../components/ErrorBanner";
import { ProtectedImage } from "../components/ProtectedImage";
import {
  formatISODateForDisplay,
  formatRussianDateInput,
  isValidDisplayDate,
  normalizeDisplayDate,
} from "../utils/date";

type PatientDetailsPageProps = {
  token: string;
  user: User | null;
  patientID: number;
  onLogout: () => void;
  navigateToPatients: () => void;
  navigateToPatient: (id: number) => void;
};

const emptyPatientForm: CreatePatientPayload = {
  full_name: "",
  birth_date: "",
  phone: "",
  comment: "",
};

function imageStatusLabel(status: string): string {
  if (status === "analyzed") {
    return "проанализирован";
  }

  if (status === "uploaded") {
    return "загружен";
  }

  return status;
}

export function PatientDetailsPage({
  token,
  user,
  patientID,
  onLogout,
  navigateToPatients,
}: PatientDetailsPageProps) {
  const [patient, setPatient] = useState<Patient | null>(null);
  const [patientForm, setPatientForm] = useState<CreatePatientPayload>(emptyPatientForm);
  const [editingPatient, setEditingPatient] = useState(false);
  const [images, setImages] = useState<DentalImage[]>([]);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [resultsByImage, setResultsByImage] = useState<Record<number, AnalysisResult[]>>({});
  const [visibleResultsByImage, setVisibleResultsByImage] = useState<Record<number, boolean>>({});
  const [resultMessagesByImage, setResultMessagesByImage] = useState<Record<number, string>>({});
  const [emptyResultsTextByImage, setEmptyResultsTextByImage] = useState<Record<number, string>>({});
  const [activeResultImageID, setActiveResultImageID] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyImageID, setBusyImageID] = useState<number | null>(null);
  const [deletingImageID, setDeletingImageID] = useState<number | null>(null);
  const [uploading, setUploading] = useState(false);
  const [savingPatient, setSavingPatient] = useState(false);
  const [deletingPatient, setDeletingPatient] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const title = useMemo(
    () => (patient ? patient.full_name : "Карточка пациента"),
    [patient],
  );
  const activeResultImage = useMemo(
    () => images.find((image) => image.id === activeResultImageID) || null,
    [activeResultImageID, images],
  );

  function fillPatientForm(nextPatient: Patient) {
    setPatientForm({
      full_name: nextPatient.full_name,
      birth_date: formatISODateForDisplay(nextPatient.birth_date),
      phone: nextPatient.phone || "",
      comment: nextPatient.comment || "",
    });
  }

  async function loadPatientData() {
    setLoading(true);
    setError("");

    try {
      const [nextPatient, nextImages] = await Promise.all([
        getPatient(token, patientID),
        getPatientImages(token, patientID),
      ]);
      setPatient(nextPatient);
      fillPatientForm(nextPatient);
      setImages(nextImages);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось загрузить пациента");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadPatientData();
  }, [patientID, token]);

  async function handleSavePatient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setNotice("");

    if (!isValidDisplayDate(patientForm.birth_date)) {
      setError("Введите дату рождения в формате дд.мм.гггг");
      return;
    }

    const payload = {
      ...patientForm,
      birth_date: normalizeDisplayDate(patientForm.birth_date),
    };

    setSavingPatient(true);

    try {
      const nextPatient = await updatePatient(token, patientID, payload);
      setPatient(nextPatient);
      fillPatientForm(nextPatient);
      setEditingPatient(false);
      setNotice("Карточка пациента обновлена");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось обновить пациента");
    } finally {
      setSavingPatient(false);
    }
  }

  async function handleDeletePatient() {
    if (!patient) {
      return;
    }

    const confirmed = window.confirm(
      `Удалить пациента "${patient.full_name}"? Все снимки и результаты анализа тоже будут удалены.`,
    );
    if (!confirmed) {
      return;
    }

    setDeletingPatient(true);
    setError("");

    try {
      await deletePatient(token, patient.id);
      navigateToPatients();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось удалить пациента");
    } finally {
      setDeletingPatient(false);
    }
  }

  async function handleUpload() {
    if (!selectedFile) {
      setError("Сначала выберите снимок jpg, jpeg или png");
      return;
    }

    setUploading(true);
    setError("");
    setNotice("");

    try {
      await uploadPatientImage(token, patientID, selectedFile);
      setSelectedFile(null);
      setNotice("Снимок загружен");
      await loadPatientData();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось загрузить снимок");
    } finally {
      setUploading(false);
    }
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    setSelectedFile(event.target.files?.[0] || null);
  }

  async function handleRunAnalysis(imageID: number) {
    setBusyImageID(imageID);
    setError("");
    setNotice("");

    try {
      const job = await runImageAnalysis(token, imageID);
      setResultsByImage((current) => ({
        ...current,
        [imageID]: job.results,
      }));
      setVisibleResultsByImage((current) => ({
        ...current,
        [imageID]: true,
      }));
      setResultMessagesByImage((current) => ({
        ...current,
        [imageID]:
          job.results.length === 0
            ? "Анализ завершен. Модель не нашла подозрительных зон."
            : `Анализ завершен. Найдено зон: ${job.results.length}.`,
      }));
      setEmptyResultsTextByImage((current) => ({
        ...current,
        [imageID]: "Модель не нашла подозрительных зон.",
      }));
      await loadPatientData();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось запустить анализ");
    } finally {
      setBusyImageID(null);
    }
  }

  async function handleShowResults(imageID: number) {
    setBusyImageID(imageID);
    setError("");
    setVisibleResultsByImage((current) => ({
      ...current,
      [imageID]: true,
    }));
    setResultMessagesByImage((current) => ({
      ...current,
      [imageID]: "Загружаем сохраненные результаты...",
    }));

    try {
      const analysis = await getImageAnalysis(token, imageID);
      setResultsByImage((current) => ({
        ...current,
        [imageID]: analysis.results,
      }));
      setResultMessagesByImage((current) => ({
        ...current,
        [imageID]:
          analysis.results.length === 0
            ? ""
            : `Загружено результатов: ${analysis.results.length}.`,
      }));
      setEmptyResultsTextByImage((current) => ({
        ...current,
        [imageID]: "Для этого снимка пока нет сохраненных результатов. Сначала запустите анализ.",
      }));
      setActiveResultImageID(imageID);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось загрузить результаты");
      setResultMessagesByImage((current) => ({
        ...current,
        [imageID]: "Не удалось загрузить сохраненные результаты.",
      }));
    } finally {
      setBusyImageID(null);
    }
  }

  async function handleDeleteImage(image: DentalImage) {
    const confirmed = window.confirm(
      `Удалить снимок "${image.original_name}"? Результаты анализа для него тоже будут удалены.`,
    );
    if (!confirmed) {
      return;
    }

    setDeletingImageID(image.id);
    setError("");
    setNotice("");

    try {
      await deleteImage(token, image.id);
      setResultsByImage((current) => {
        const next = { ...current };
        delete next[image.id];
        return next;
      });
      setVisibleResultsByImage((current) => {
        const next = { ...current };
        delete next[image.id];
        return next;
      });
      setResultMessagesByImage((current) => {
        const next = { ...current };
        delete next[image.id];
        return next;
      });
      setEmptyResultsTextByImage((current) => {
        const next = { ...current };
        delete next[image.id];
        return next;
      });
      if (activeResultImageID === image.id) {
        setActiveResultImageID(null);
      }

      setNotice("Снимок удален");
      await loadPatientData();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось удалить снимок");
    } finally {
      setDeletingImageID(null);
    }
  }

  return (
    <AppShell
      user={user}
      title={title}
      onHome={navigateToPatients}
      onLogout={onLogout}
      actions={
        <button className="secondary-button" type="button" onClick={navigateToPatients}>
          К списку пациентов
        </button>
      }
    >
      <ErrorBanner message={error} />
      {notice ? <div className="notice-banner">{notice}</div> : null}

      {loading ? <p className="muted">Загружаем карточку пациента...</p> : null}

      {patient ? (
        <section className="patient-layout">
          <aside className="panel patient-summary">
            <div className="panel-heading">
              <h2>Пациент</h2>
            </div>

            {editingPatient ? (
              <form className="form-grid" onSubmit={handleSavePatient}>
                <label>
                  ФИО
                  <input
                    value={patientForm.full_name}
                    onChange={(event) =>
                      setPatientForm({ ...patientForm, full_name: event.target.value })
                    }
                    required
                  />
                </label>
                <label>
                  Дата рождения
                  <input
                    type="text"
                    inputMode="numeric"
                    placeholder="дд.мм.гггг"
                    value={patientForm.birth_date}
                    onChange={(event) =>
                      setPatientForm({
                        ...patientForm,
                        birth_date: formatRussianDateInput(event.target.value),
                      })
                    }
                  />
                </label>
                <label>
                  Телефон
                  <input
                    value={patientForm.phone}
                    onChange={(event) =>
                      setPatientForm({ ...patientForm, phone: event.target.value })
                    }
                  />
                </label>
                <label>
                  Комментарий
                  <textarea
                    rows={4}
                    value={patientForm.comment}
                    onChange={(event) =>
                      setPatientForm({ ...patientForm, comment: event.target.value })
                    }
                  />
                </label>
                <div className="form-actions">
                  <button className="primary-button" type="submit" disabled={savingPatient}>
                    {savingPatient ? "Сохраняем..." : "Сохранить"}
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => {
                      fillPatientForm(patient);
                      setEditingPatient(false);
                    }}
                    disabled={savingPatient}
                  >
                    Отменить
                  </button>
                </div>
              </form>
            ) : (
              <>
                <dl>
                  <div>
                    <dt>ФИО</dt>
                    <dd>{patient.full_name}</dd>
                  </div>
                  <div>
                    <dt>Дата рождения</dt>
                    <dd>{formatISODateForDisplay(patient.birth_date) || "Не указана"}</dd>
                  </div>
                  <div>
                    <dt>Телефон</dt>
                    <dd>{patient.phone || "Не указан"}</dd>
                  </div>
                  <div>
                    <dt>Комментарий</dt>
                    <dd>{patient.comment || "Нет комментария"}</dd>
                  </div>
                </dl>
                <div className="button-row">
                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => setEditingPatient(true)}
                  >
                    Редактировать
                  </button>
                  <button
                    className="danger-button"
                    type="button"
                    onClick={handleDeletePatient}
                    disabled={deletingPatient}
                  >
                    {deletingPatient ? "Удаляем..." : "Удалить"}
                  </button>
                </div>
              </>
            )}
          </aside>

          <section className="panel">
            <div className="panel-heading">
              <div>
                <h2>Снимки</h2>
                <p className="muted">
                  Результат анализа формируется моделью компьютерного зрения и требует проверки врачом.
                </p>
              </div>
            </div>

            <div className="upload-row">
              <input
                type="file"
                accept=".jpg,.jpeg,.png,image/jpeg,image/png"
                onChange={handleFileChange}
              />
              <button
                className="primary-button"
                type="button"
                onClick={handleUpload}
                disabled={uploading}
              >
                {uploading ? "Загружаем..." : "Загрузить снимок"}
              </button>
            </div>

            {images.length === 0 ? (
              <p className="muted">У пациента пока нет загруженных снимков.</p>
            ) : (
              <div className="image-list">
                {images.map((image) => {
                  const imageResults = resultsByImage[image.id] || [];
                  const resultsVisible = visibleResultsByImage[image.id] || false;
                  const resultMessage = resultMessagesByImage[image.id] || "";
                  const emptyResultsText = emptyResultsTextByImage[image.id];
                  const isBusy = busyImageID === image.id;
                  const isDeleting = deletingImageID === image.id;
                  const isAnalyzed = image.status === "analyzed";

                  return (
                    <article className="image-card" key={image.id}>
                      <ProtectedImage
                        token={token}
                        imageID={image.id}
                        alt={image.original_name}
                      />
                      <div className="image-card-body">
                        <div>
                          <strong>{image.original_name}</strong>
                          <span className="status-pill">{imageStatusLabel(image.status)}</span>
                        </div>
                        <div className="button-row">
                          <button
                            className="primary-button"
                            type="button"
                            onClick={() => handleRunAnalysis(image.id)}
                            disabled={isBusy || isDeleting || isAnalyzed}
                          >
                            {isBusy
                              ? "Анализируем..."
                              : isAnalyzed
                                ? "Анализ готов"
                                : "Запустить анализ"}
                          </button>
                          <button
                            className="secondary-button"
                            type="button"
                            onClick={() => handleShowResults(image.id)}
                            disabled={isBusy || isDeleting}
                          >
                            {isBusy ? "Открываем..." : "Показать результат"}
                          </button>
                          <button
                            className="danger-button"
                            type="button"
                            onClick={() => handleDeleteImage(image)}
                            disabled={isBusy || isDeleting}
                          >
                            {isDeleting ? "Удаляем..." : "Удалить снимок"}
                          </button>
                        </div>
                        {resultMessage ? (
                          <p className="result-message">{resultMessage}</p>
                        ) : null}
                        {resultsVisible ? (
                          <AnalysisResults
                            results={imageResults}
                            emptyMessage={emptyResultsText}
                          />
                        ) : null}
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
          </section>
        </section>
      ) : null}

      {activeResultImage ? (
        <AnalysisDetailsModal
          token={token}
          image={activeResultImage}
          results={resultsByImage[activeResultImage.id] || []}
          onClose={() => setActiveResultImageID(null)}
        />
      ) : null}
    </AppShell>
  );
}
