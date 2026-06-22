import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  createPatient,
  deletePatient,
  getPatients,
  updatePatient,
  type CreatePatientPayload,
  type Patient,
  type User,
} from "../api/client";
import { AppShell } from "../components/AppShell";
import { ErrorBanner } from "../components/ErrorBanner";
import {
  formatISODateForDisplay,
  formatRussianDateInput,
  isValidDisplayDate,
  normalizeDisplayDate,
} from "../utils/date";

type PatientsPageProps = {
  token: string;
  user: User | null;
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

export function PatientsPage({
  token,
  user,
  onLogout,
  navigateToPatients,
  navigateToPatient,
}: PatientsPageProps) {
  const [patients, setPatients] = useState<Patient[]>([]);
  const [form, setForm] = useState(emptyPatientForm);
  const [editingPatientID, setEditingPatientID] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingPatientID, setDeletingPatientID] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const filteredPatients = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return patients;
    }

    return patients.filter((patient) =>
      [patient.full_name, patient.phone || "", patient.comment || ""]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery),
    );
  }, [patients, query]);

  async function loadPatients() {
    setLoading(true);
    setError("");

    try {
      setPatients(await getPatients(token));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось загрузить пациентов");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadPatients();
  }, [token]);

  function resetForm() {
    setForm(emptyPatientForm);
    setEditingPatientID(null);
  }

  function handleEdit(patient: Patient) {
    setNotice("");
    setError("");
    setEditingPatientID(patient.id);
    setForm({
      full_name: patient.full_name,
      birth_date: formatISODateForDisplay(patient.birth_date),
      phone: patient.phone || "",
      comment: patient.comment || "",
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setNotice("");

    if (!isValidDisplayDate(form.birth_date)) {
      setError("Введите дату рождения в формате дд.мм.гггг");
      return;
    }

    const payload = {
      ...form,
      birth_date: normalizeDisplayDate(form.birth_date),
    };

    setSubmitting(true);

    try {
      if (editingPatientID) {
        await updatePatient(token, editingPatientID, payload);
        setNotice("Карточка пациента обновлена");
      } else {
        await createPatient(token, payload);
        setNotice("Пациент добавлен");
      }

      resetForm();
      await loadPatients();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось сохранить пациента");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(patient: Patient) {
    const confirmed = window.confirm(
      `Удалить пациента "${patient.full_name}"? Все снимки и результаты анализа тоже будут удалены.`,
    );
    if (!confirmed) {
      return;
    }

    setDeletingPatientID(patient.id);
    setError("");
    setNotice("");

    try {
      await deletePatient(token, patient.id);
      if (editingPatientID === patient.id) {
        resetForm();
      }

      setNotice("Пациент удален");
      await loadPatients();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось удалить пациента");
    } finally {
      setDeletingPatientID(null);
    }
  }

  return (
    <AppShell
      user={user}
      title="Пациенты"
      onHome={navigateToPatients}
      onLogout={onLogout}
    >
      <ErrorBanner message={error} />
      {notice ? <div className="notice-banner">{notice}</div> : null}

      <section className="content-grid">
        <form className="panel form-grid" onSubmit={handleSubmit}>
          <div>
            <h2>{editingPatientID ? "Редактировать пациента" : "Новый пациент"}</h2>
            <p className="muted">
              Заполните основные данные, чтобы быстро найти пациента перед анализом снимков.
            </p>
          </div>

          <label>
            ФИО
            <input
              value={form.full_name}
              onChange={(event) => setForm({ ...form, full_name: event.target.value })}
              required
            />
          </label>
          <label>
            Дата рождения
            <input
              type="text"
              inputMode="numeric"
              placeholder="дд.мм.гггг"
              value={form.birth_date}
              onChange={(event) =>
                setForm({
                  ...form,
                  birth_date: formatRussianDateInput(event.target.value),
                })
              }
            />
          </label>
          <label>
            Телефон
            <input
              value={form.phone}
              onChange={(event) => setForm({ ...form, phone: event.target.value })}
            />
          </label>
          <label>
            Комментарий
            <textarea
              rows={4}
              value={form.comment}
              onChange={(event) => setForm({ ...form, comment: event.target.value })}
            />
          </label>

          <div className="form-actions">
            <button className="primary-button" type="submit" disabled={submitting}>
              {submitting
                ? "Сохраняем..."
                : editingPatientID
                  ? "Сохранить"
                  : "Добавить"}
            </button>
            {editingPatientID ? (
              <button
                className="secondary-button"
                type="button"
                onClick={resetForm}
                disabled={submitting}
              >
                Отменить
              </button>
            ) : null}
          </div>
        </form>

        <section className="panel">
          <div className="panel-heading">
            <div>
              <h2>Картотека</h2>
              <p className="muted">
                {patients.length === 0
                  ? "Пациентов пока нет"
                  : `Всего пациентов: ${patients.length}`}
              </p>
            </div>
            <input
              className="search-input"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Поиск по ФИО, телефону или комментарию"
            />
          </div>

          {loading ? <p className="muted">Загружаем пациентов...</p> : null}
          {!loading && patients.length === 0 ? (
            <p className="muted">Добавьте первого пациента через форму слева.</p>
          ) : null}
          {!loading && patients.length > 0 && filteredPatients.length === 0 ? (
            <p className="muted">По этому запросу ничего не найдено.</p>
          ) : null}

          <div className="patient-list">
            {filteredPatients.map((patient) => (
              <article className="patient-row" key={patient.id}>
                <div className="patient-row-main">
                  <strong>{patient.full_name}</strong>
                  <div className="patient-meta">
                    <span>{patient.phone || "Телефон не указан"}</span>
                    <span>
                      {formatISODateForDisplay(patient.birth_date) ||
                        "Дата рождения не указана"}
                    </span>
                    {patient.comment ? <span>{patient.comment}</span> : null}
                  </div>
                </div>
                <div className="patient-actions">
                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => navigateToPatient(patient.id)}
                  >
                    Открыть
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => handleEdit(patient)}
                  >
                    Редактировать
                  </button>
                  <button
                    className="danger-button"
                    type="button"
                    onClick={() => handleDelete(patient)}
                    disabled={deletingPatientID === patient.id}
                  >
                    {deletingPatientID === patient.id ? "Удаляем..." : "Удалить"}
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>
      </section>
    </AppShell>
  );
}
