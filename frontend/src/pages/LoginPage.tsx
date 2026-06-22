import { FormEvent, useState } from "react";
import { ApiError, login } from "../api/client";
import { ErrorBanner } from "../components/ErrorBanner";

type LoginPageProps = {
  onLogin: (token: string) => void;
};

export function LoginPage({ onLogin }: LoginPageProps) {
  const [email, setEmail] = useState("doctor@dentvision.com");
  const [password, setPassword] = useState("doctor123");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      const response = await login(email, password);
      onLogin(response.token);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Не удалось войти");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel">
        <div>
          <p className="eyebrow">DentVision</p>
          <h1>Вход врача</h1>
          <p className="muted">Войдите, чтобы вести пациентов и анализировать снимки.</p>
        </div>

        <ErrorBanner message={error} />

        <form className="form-grid" onSubmit={handleSubmit}>
          <label>
            Электронная почта
            <input
              autoComplete="email"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </label>

          <label>
            Пароль
            <input
              autoComplete="current-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>

          <button className="primary-button" type="submit" disabled={submitting}>
            {submitting ? "Входим..." : "Войти"}
          </button>
        </form>
      </section>
    </main>
  );
}
