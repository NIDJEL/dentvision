const API_URL = import.meta.env.VITE_API_URL || "http://localhost:8080";

export type User = {
  id: number;
  email: string;
  full_name: string;
  role: string;
};

export type LoginResponse = {
  token: string;
  user: User;
};

export type Patient = {
  id: number;
  doctor_id?: number;
  full_name: string;
  birth_date?: string;
  phone?: string;
  comment?: string;
  created_at?: string;
};

export type CreatePatientPayload = {
  full_name: string;
  birth_date: string;
  phone: string;
  comment: string;
};

export type DentalImage = {
  id: number;
  patient_id: number;
  file_id: number;
  uploaded_by: number;
  file_path: string;
  original_name: string;
  mime_type: string;
  file_size: number;
  status: string;
  created_at: string;
};

export type AnalysisResult = {
  id: number;
  job_id?: number;
  image_id?: number;
  label: string;
  confidence: number;
  x: number;
  y: number;
  width: number;
  height: number;
  created_at?: string;
};

export type AnalysisJob = {
  id: number;
  image_id: number;
  model_id: number;
  status: string;
  error_message?: string;
  created_at: string;
  finished_at: string;
  results: AnalysisResult[];
};

export type ImageAnalysis = {
  image_id: number;
  results: AnalysisResult[];
};

type RequestOptions = RequestInit & {
  token?: string;
};

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { token, headers, body, ...rest } = options;
  const requestHeaders = new Headers(headers);

  if (token) {
    requestHeaders.set("Authorization", `Bearer ${token}`);
  }

  if (body && !(body instanceof FormData) && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...rest,
    headers: requestHeaders,
    body,
  });

  if (!response.ok) {
    throw new ApiError(response.status, await readErrorMessage(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

async function readErrorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string; detail?: string };
    return data.error || data.detail || `Request failed with status ${response.status}`;
  } catch {
    return `Request failed with status ${response.status}`;
  }
}

export function login(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function getMe(token: string): Promise<User> {
  return request<User>("/me", { token });
}

export async function getPatients(token: string): Promise<Patient[]> {
  const response = await request<{ patients: Patient[] }>("/patients", { token });
  return response.patients;
}

export function createPatient(
  token: string,
  payload: CreatePatientPayload,
): Promise<Patient> {
  return request<Patient>("/patients", {
    method: "POST",
    token,
    body: JSON.stringify(payload),
  });
}

export function getPatient(token: string, patientID: number): Promise<Patient> {
  return request<Patient>(`/patients/${patientID}`, { token });
}

export function uploadPatientImage(
  token: string,
  patientID: number,
  file: File,
): Promise<DentalImage> {
  const form = new FormData();
  form.append("image", file);

  return request<DentalImage>(`/patients/${patientID}/images`, {
    method: "POST",
    token,
    body: form,
  });
}

export async function getPatientImages(
  token: string,
  patientID: number,
): Promise<DentalImage[]> {
  const response = await request<{ images: DentalImage[] }>(
    `/patients/${patientID}/images`,
    { token },
  );
  return response.images;
}

export function runImageAnalysis(token: string, imageID: number): Promise<AnalysisJob> {
  return request<AnalysisJob>(`/images/${imageID}/analysis`, {
    method: "POST",
    token,
  });
}

export function getImageAnalysis(
  token: string,
  imageID: number,
): Promise<ImageAnalysis> {
  return request<ImageAnalysis>(`/images/${imageID}/analysis`, { token });
}

export async function getImageFile(token: string, imageID: number): Promise<Blob> {
  const response = await fetch(`${API_URL}/images/${imageID}/file`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new ApiError(response.status, await readErrorMessage(response));
  }

  return response.blob();
}
