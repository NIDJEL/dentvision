import { useEffect, useMemo, useState } from "react";
import { getMe, type User } from "./api/client";
import { clearStoredToken, getStoredToken, storeToken } from "./auth";
import { LoginPage } from "./pages/LoginPage";
import { PatientDetailsPage } from "./pages/PatientDetailsPage";
import { PatientsPage } from "./pages/PatientsPage";

type Route =
  | { name: "login" }
  | { name: "patients" }
  | { name: "patient"; id: number };

function parseRoute(): Route {
  const match = window.location.pathname.match(/^\/patients\/(\d+)$/);

  if (match) {
    return { name: "patient", id: Number(match[1]) };
  }

  if (window.location.pathname === "/patients") {
    return { name: "patients" };
  }

  return { name: "login" };
}

function pushRoute(path: string): void {
  window.history.pushState({}, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function App() {
  const [route, setRoute] = useState<Route>(() => parseRoute());
  const [token, setToken] = useState(() => getStoredToken());
  const [user, setUser] = useState<User | null>(null);
  const [authChecked, setAuthChecked] = useState(false);

  useEffect(() => {
    const onPopState = () => setRoute(parseRoute());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    if (!token) {
      setUser(null);
      setAuthChecked(true);
      if (route.name !== "login") {
        pushRoute("/");
      }
      return;
    }

    let cancelled = false;
    setAuthChecked(false);

    getMe(token)
      .then((me) => {
        if (cancelled) {
          return;
        }

        setUser(me);
        setAuthChecked(true);

        if (route.name === "login") {
          pushRoute("/patients");
        }
      })
      .catch(() => {
        if (cancelled) {
          return;
        }

        clearStoredToken();
        setToken("");
        setUser(null);
        setAuthChecked(true);
        pushRoute("/");
      });

    return () => {
      cancelled = true;
    };
  }, [route.name, token]);

  const appContext = useMemo(
    () => ({
      token,
      user,
      onLogout: () => {
        clearStoredToken();
        setToken("");
        setUser(null);
        pushRoute("/");
      },
      navigateToPatients: () => pushRoute("/patients"),
      navigateToPatient: (id: number) => pushRoute(`/patients/${id}`),
    }),
    [token, user],
  );

  if (!authChecked) {
    return <div className="app-loading">Загружаем DentVision...</div>;
  }

  if (!token || route.name === "login") {
    return (
      <LoginPage
        onLogin={(nextToken) => {
          storeToken(nextToken);
          setToken(nextToken);
          pushRoute("/patients");
        }}
      />
    );
  }

  if (route.name === "patient") {
    return <PatientDetailsPage {...appContext} patientID={route.id} />;
  }

  return <PatientsPage {...appContext} />;
}
