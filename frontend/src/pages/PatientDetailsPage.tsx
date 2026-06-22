import { ChangeEvent, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  getImageAnalysis,
  getPatient,
  getPatientImages,
  runImageAnalysis,
  uploadPatientImage,
  type AnalysisResult,
  type DentalImage,
  type Patient,
  type User,
} from "../api/client";
import { AnalysisResults } from "../components/AnalysisResults";
import { AppShell } from "../components/AppShell";
import { ErrorBanner } from "../components/ErrorBanner";
import { ProtectedImage } from "../components/ProtectedImage";

type PatientDetailsPageProps = {
  token: string;
  user: User | null;
  patientID: number;
  onLogout: () => void;
  navigateToPatients: () => void;
  navigateToPatient: (id: number) => void;
};

export function PatientDetailsPage({
  token,
  user,
  patientID,
  onLogout,
  navigateToPatients,
}: PatientDetailsPageProps) {
  const [patient, setPatient] = useState<Patient | null>(null);
  const [images, setImages] = useState<DentalImage[]>([]);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [resultsByImage, setResultsByImage] = useState<Record<number, AnalysisResult[]>>({});
  const [loading, setLoading] = useState(true);
  const [busyImageID, setBusyImageID] = useState<number | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");

  const title = useMemo(
    () => (patient ? patient.full_name : "Patient details"),
    [patient],
  );

  async function loadPatientData() {
    setLoading(true);
    setError("");

    try {
      const [nextPatient, nextImages] = await Promise.all([
        getPatient(token, patientID),
        getPatientImages(token, patientID),
      ]);
      setPatient(nextPatient);
      setImages(nextImages);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load patient");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadPatientData();
  }, [patientID, token]);

  async function handleUpload() {
    if (!selectedFile) {
      setError("Choose jpg, jpeg or png image first");
      return;
    }

    setUploading(true);
    setError("");

    try {
      await uploadPatientImage(token, patientID, selectedFile);
      setSelectedFile(null);
      await loadPatientData();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to upload image");
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

    try {
      const job = await runImageAnalysis(token, imageID);
      setResultsByImage((current) => ({
        ...current,
        [imageID]: job.results,
      }));
      await loadPatientData();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to run analysis");
    } finally {
      setBusyImageID(null);
    }
  }

  async function handleShowResults(imageID: number) {
    setBusyImageID(imageID);
    setError("");

    try {
      const analysis = await getImageAnalysis(token, imageID);
      setResultsByImage((current) => ({
        ...current,
        [imageID]: analysis.results,
      }));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load analysis");
    } finally {
      setBusyImageID(null);
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
          Back to patients
        </button>
      }
    >
      <ErrorBanner message={error} />

      {loading ? <p className="muted">Loading patient...</p> : null}

      {patient ? (
        <section className="patient-layout">
          <aside className="panel patient-summary">
            <h2>Patient</h2>
            <dl>
              <div>
                <dt>Full name</dt>
                <dd>{patient.full_name}</dd>
              </div>
              <div>
                <dt>Birth date</dt>
                <dd>{patient.birth_date || "Not specified"}</dd>
              </div>
              <div>
                <dt>Phone</dt>
                <dd>{patient.phone || "Not specified"}</dd>
              </div>
              <div>
                <dt>Comment</dt>
                <dd>{patient.comment || "No comment"}</dd>
              </div>
            </dl>
          </aside>

          <section className="panel">
            <h2>Dental images</h2>
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
                {uploading ? "Uploading..." : "Upload image"}
              </button>
            </div>

            {images.length === 0 ? (
              <p className="muted">No uploaded images yet.</p>
            ) : (
              <div className="image-list">
                {images.map((image) => (
                  <article className="image-card" key={image.id}>
                    <ProtectedImage
                      token={token}
                      imageID={image.id}
                      alt={image.original_name}
                    />
                    <div className="image-card-body">
                      <div>
                        <strong>{image.original_name}</strong>
                        <span className="status-pill">{image.status}</span>
                      </div>
                      <div className="button-row">
                        <button
                          className="primary-button"
                          type="button"
                          onClick={() => handleRunAnalysis(image.id)}
                          disabled={busyImageID === image.id}
                        >
                          {busyImageID === image.id ? "Working..." : "Run analysis"}
                        </button>
                        <button
                          className="secondary-button"
                          type="button"
                          onClick={() => handleShowResults(image.id)}
                          disabled={busyImageID === image.id}
                        >
                          Show result
                        </button>
                      </div>
                      <AnalysisResults results={resultsByImage[image.id] || []} />
                    </div>
                  </article>
                ))}
              </div>
            )}
          </section>
        </section>
      ) : null}
    </AppShell>
  );
}
