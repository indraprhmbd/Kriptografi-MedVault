/**
 * MedVault REST API Client.
 *
 * Typed HTTP interface connecting the Electron desktop frontend
 * to the local FastAPI backend service running on 127.0.0.1:8000.
 */

import axios from "axios";

export const API_BASE_URL = "http://127.0.0.1:8000/api/v1";

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
});

// Request interceptor for bearer authentication and observability
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem("medvault_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  console.log(`[MedVault API Request] ${config.method?.toUpperCase()} ${config.url}`);
  return config;
});

// Response interceptor for consistent error handling, 401 logout, and observability
apiClient.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    if (error.response?.status === 401) {
      const reason =
        error.response?.data?.detail ||
        "Sesi dibatalkan atau kunci fisik USB Dongle telah dicabut.";
      console.warn("[MedVault Auth] Sesi tidak valid atau telah berakhir:", reason);
      localStorage.removeItem("medvault_token");
      localStorage.removeItem("medvault_user");
      window.dispatchEvent(
        new CustomEvent("medvault:unauthorized", { detail: { reason } })
      );
    }

    console.error(`[MedVault API Error] ${error.config?.url}:`, error.response?.status, error.response?.data || error.message);
    let message = "Terjadi kesalahan koneksi ke server MedVault";
    if (error.response?.data?.detail) {
      message = error.response.data.detail;
    } else if (error.message) {
      message = error.message;
    }
    return Promise.reject(new Error(message));
  }
);

// ============================================================================
// Types
// ============================================================================

export interface UserSession {
  id: number;
  username: string;
  role: string;
  dongle_hash_prefix?: string;
  token?: string;
}

export interface DongleStatusReport {
  detected: boolean;
  mountpoint: string | null;
  token_hash: string | null;
  token_hash_prefix: string | null;
  message: string;
}

export interface DongleDriveInfo {
  device: string;
  mountpoint: string;
  label: string;
  is_removable: boolean;
  has_key: boolean;
  token_hash: string | null;
  token_hash_prefix: string | null;
  free_gb: number;
  total_gb: number;
}

export interface DongleProvisionResponse {
  status: string;
  mountpoint: string;
  key_file: string;
  token_hash: string;
  token_hash_prefix: string;
  message: string;
}

export interface FingerprintStatusReport {
  available: boolean;
  message: string;
}

export interface MfaLoginFactors {
  password: boolean;
  dongle: boolean;
  biometric: boolean;
  face_score: number;
}

export interface MfaLoginResponse {
  authenticated: boolean;
  access_token?: string;
  token_type?: string;
  user?: UserSession;
  factors: MfaLoginFactors;
  message: string;
}

export interface PatientItem {
  id: number;
  medical_record_number: string;
  birth_date: string;
  created_at: string;
  decrypted: {
    nik: string;
    full_name: string;
    address: string;
  };
  raw_sqlite_ciphertext: {
    nik_encrypted: string;
    full_name_encrypted: string;
    address_encrypted: string;
  };
}

export interface MedicalRecordItem {
  id: number;
  patient_id: number;
  doctor_id: number;
  diagnosis: string;
  prescription: string;
  visit_status: string;
  lab_file_path?: string | null;
  lab_original_filename?: string | null;
  stego_image_path?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CryptoInspectionData {
  record_id: number;
  patient_id: number;
  diagnosis_inspection: {
    plaintext: string;
    step1_vigenere: string;
    step2_aes128_b64: string;
    verified_roundtrip: string;
  };
  prescription_inspection: {
    plaintext: string;
    step1_vigenere: string;
    step2_aes128_b64: string;
    verified_roundtrip: string;
  };
  raw_sqlite_diagnosis_base64: string;
  raw_sqlite_prescription_base64: string;
}

export interface LabPreviewData {
  status: string;
  original_filename: string;
  encrypted_file_path: string;
  file_size_bytes: number;
  nonce_hex: string;
  first_128_hex_bytes: string;
  note: string;
}

export interface StegoMetrics {
  output_path: string;
  embedded_bytes: number;
  embedded_bits: number;
  total_capacity_bits: number;
  capacity_usage_pct: number;
  psnr: number;
  mse: number;
}

export interface AuditBlockItem {
  block_index: number;
  timestamp: string;
  actor_username: string;
  action_type: string;
  target_record_id: number | null;
  change_summary: string;
  previous_hash: string;
  current_hash: string;
}

export interface IntegrityReport {
  is_valid: boolean;
  total_blocks: number;
  broken_at_index: number | null;
  error_detail: string | null;
}

// ============================================================================
// Service APIs
// ============================================================================

export const authApi = {
  checkHealth: async () => {
    const res = await apiClient.get("/health");
    return res.data;
  },
  getDongleStatus: async (): Promise<DongleStatusReport> => {
    const res = await apiClient.get<DongleStatusReport>("/auth/dongle-status");
    return res.data;
  },
  listDongleDrives: async (): Promise<{ drives: DongleDriveInfo[] }> => {
    const res = await apiClient.get<{ drives: DongleDriveInfo[] }>("/auth/dongle-drives");
    return res.data;
  },
  provisionDongle: async (
    target_mountpoint: string,
    overwrite: boolean = false
  ): Promise<DongleProvisionResponse> => {
    const res = await apiClient.post<DongleProvisionResponse>("/auth/dongle-provision", {
      target_mountpoint,
      overwrite,
    });
    return res.data;
  },
  getFingerprintStatus: async (): Promise<FingerprintStatusReport> => {
    const res = await apiClient.get<FingerprintStatusReport>("/auth/fingerprint-status");
    return res.data;
  },
  registerDoctor: async (formData: FormData) => {
    const res = await apiClient.post("/auth/register", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return res.data;
  },
  loginDoctor: async (formData: FormData): Promise<MfaLoginResponse> => {
    const res = await apiClient.post<MfaLoginResponse>("/auth/login", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return res.data;
  },
};

export const patientApi = {
  listPatients: async (): Promise<{ total: number; patients: PatientItem[] }> => {
    const res = await apiClient.get("/patients");
    return res.data;
  },
  createPatient: async (data: {
    medical_record_number: string;
    nik: string;
    full_name: string;
    address: string;
    birth_date: string;
  }) => {
    const res = await apiClient.post("/patients", data);
    return res.data;
  },
  getPatient: async (id: number) => {
    const res = await apiClient.get(`/patients/${id}`);
    return res.data;
  },
};

export const recordApi = {
  createRecord: async (data: {
    patient_id: number;
    doctor_id: number;
    actor_username: string;
    diagnosis: string;
    prescription: string;
    visit_status?: string;
  }) => {
    const res = await apiClient.post("/records", data);
    return res.data;
  },
  listByPatient: async (patientId: number): Promise<{ total: number; records: MedicalRecordItem[] }> => {
    const res = await apiClient.get(`/records/patient/${patientId}`);
    return res.data;
  },
  inspectRecord: async (recordId: number): Promise<{ inspection: CryptoInspectionData }> => {
    const res = await apiClient.get(`/records/${recordId}/inspect`);
    return res.data;
  },
};

export const vaultApi = {
  uploadLabFile: async (recordId: number, actorUsername: string, file: File) => {
    const formData = new FormData();
    formData.append("actor_username", actorUsername);
    formData.append("file", file);

    const res = await apiClient.post(`/vault/${recordId}/upload`, formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return res.data;
  },
  previewLabFile: async (recordId: number): Promise<LabPreviewData> => {
    const res = await apiClient.get(`/vault/${recordId}/preview`);
    return res.data;
  },
  downloadLabFile: async (recordId: number): Promise<{ blob: Blob; filename: string }> => {
    const res = await apiClient.get(`/vault/${recordId}/download`, {
      responseType: "blob",
    });
    const disposition = res.headers["content-disposition"] || "";
    let filename = `hasil_lab_rekam_${recordId}.pdf`;
    const match = disposition.match(/filename="?([^"]+)"?/);
    if (match && match[1]) {
      filename = match[1];
    }
    return { blob: res.data, filename };
  },
  getDownloadUrl: (recordId: number) => `${API_BASE_URL}/vault/${recordId}/download`,
};

export const stegoApi = {
  embedSecret: async (recordId: number, actorUsername: string, secretText: string, imageFile: File): Promise<{ metrics: StegoMetrics; stego_filename: string }> => {
    const formData = new FormData();
    formData.append("actor_username", actorUsername);
    formData.append("secret_text", secretText);
    formData.append("image_file", imageFile);

    const res = await apiClient.post(`/stego/${recordId}/embed`, formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return res.data;
  },
  extractSecret: async (stegoImageFile: File): Promise<{ extracted_text: string }> => {
    const formData = new FormData();
    formData.append("stego_image", stegoImageFile);

    const res = await apiClient.post("/stego/extract", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
    return res.data;
  },
  downloadStegoScan: async (recordId: number): Promise<{ blob: Blob; filename: string }> => {
    const res = await apiClient.get(`/stego/${recordId}/download`, {
      responseType: "blob",
    });
    return { blob: res.data, filename: `stego_scan_rekam_${recordId}.png` };
  },
  getDownloadUrl: (recordId: number) => `${API_BASE_URL}/stego/${recordId}/download`,
};

export const auditApi = {
  getBlocks: async (): Promise<{ total_blocks: number; integrity_report: IntegrityReport; blocks: AuditBlockItem[] }> => {
    const res = await apiClient.get("/audit/blocks");
    return res.data;
  },
  verifyChain: async (): Promise<{ verification: IntegrityReport; status_label: string }> => {
    const res = await apiClient.get("/audit/verify");
    return res.data;
  },
};

export const systemApi = {
  getLogs: async (lines = 100): Promise<{ status: string; total_lines: number; lines: string[] }> => {
    const res = await apiClient.get(`/system/logs?lines=${lines}`);
    return res.data;
  },
};
