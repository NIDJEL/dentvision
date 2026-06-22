import { FormEvent, useEffect, useState } from "react";
import {
  ApiError,
  createPatient,
  getPatients,
  type CreatePatientPayload,
  type Patient,
  type User,
} from "../api/client";
import { AppShell } from "../components/AppShell";
import { ErrorBanner } from "../components/ErrorBanner";

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
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function loadPatients() {
    setLoading(true);
    setError("");

    try {
      setPatients(await getPatients(token));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load patients");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadPatients();
  }, [token]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");

    try {
      await createPatient(token, form);
      setForm(emptyPatientForm);
      await loadPatients();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create patient");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AppShell
      user={user}
      title="Patients"
      onHome={navigateToPatients}
      onLogout={onLogout}
    >
      <ErrorBanner message={error} />

      <section className="content-grid">
        <form className="panel form-grid" onSubmit={handleSubmit}>
          <h2>New patient</h2>
          <label>
            Full name
            <input
              value={form.full_name}
              onChange={(event) => setForm({ ...form, full_name: event.target.value })}
              required
            />
          </label>
          <label>
            Birth date
            <input
              type="date"
              value={form.birth_date}
              onChange={(event) => setForm({ ...form, birth_date: event.target.value })}
            />
          </label>
          <label>
            Phone
            <input
              value={form.phone}
              onChange={(event) => setForm({ ...form, phone: event.target.value })}
            />
          </label>
          <label>
            Comment
            <textarea
              rows={4}
              value={form.comment}
              onChange={(event) => setForm({ ...form, comment: event.target.value })}
            />
          </label>
          <button className="primary-button" type="submit" disabled={submitting}>
            {submitting ? "Creating..." : "Create patient"}
          </button>
        </form>

        <section className="panel">
          <h2>Patient list</h2>
          {loading ? <p className="muted">Loading patients...</p> : null}
          {!loading && patients.length === 0 ? (
            <p className="muted">No patients yet.</p>
          ) : null}
          <div className="patient-list">
            {patients.map((patient) => (
              <article className="patient-row" key={patient.id}>
                <div>
                  <strong>{patient.full_name}</strong>
                  <span>{patient.phone || "No phone"}</span>
                </div>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => navigateToPatient(patient.id)}
                >
                  Open
                </button>
              </article>
            ))}
          </div>
        </section>
      </section>
    </AppShell>
  );
}
