import { useEffect, useState } from "react";
import {
  authApi,
  DongleStatusReport,
  LabPreviewData,
  MedicalRecordItem,
  patientApi,
  PatientItem,
  recordApi,
  UserSession,
  vaultApi,
} from "../services/api";
import {
  AlertIcon,
  CheckIcon,
  DownloadIcon,
  FileLockIcon,
  KeyIcon,
  LockIcon,
  UploadIcon,
  VaultIllustration,
} from "../components/Icons";
import { useVerbose } from "../components/VerboseContext";

interface VaultPageProps {
  currentUser: UserSession | null;
}

export function VaultPage({ currentUser }: VaultPageProps) {
  const { isVerbose } = useVerbose();
  const [patients, setPatients] = useState<PatientItem[]>([]);
  const [selectedPatientId, setSelectedPatientId] = useState<number | null>(null);
  const [records, setRecords] = useState<MedicalRecordItem[]>([]);
  const [selectedRecordId, setSelectedRecordId] = useState<number | null>(null);

  const [dongleStatus, setDongleStatus] = useState<DongleStatusReport | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [previewData, setPreviewData] = useState<LabPreviewData | null>(null);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; msg: string } | null>(
    null
  );

  const loadPatients = async () => {
    try {
      const res = await patientApi.listPatients();
      setPatients(res.patients);
      if (res.patients.length > 0) {
        setSelectedPatientId(res.patients[0].id);
      }
    } catch {
      // Handled silently
    }
  };

  const checkDongle = async () => {
    try {
      const res = await authApi.getDongleStatus();
      setDongleStatus(res);
    } catch {
      // Handled silently
    }
  };

  useEffect(() => {
    loadPatients();
    checkDongle();
  }, []);

  useEffect(() => {
    if (selectedPatientId) {
      recordApi.listByPatient(selectedPatientId).then((res) => {
        setRecords(res.records);
        if (res.records.length > 0) {
          setSelectedRecordId(res.records[0].id);
        } else {
          setSelectedRecordId(null);
        }
      });
    }
  }, [selectedPatientId]);

  useEffect(() => {
    if (selectedRecordId) {
      vaultApi
        .previewLabFile(selectedRecordId)
        .then((data) => setPreviewData(data))
        .catch(() => setPreviewData(null));
    } else {
      setPreviewData(null);
    }
  }, [selectedRecordId]);

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedRecordId || !selectedFile || !currentUser) return;
    setFeedback(null);
    setIsUploading(true);

    try {
      const res = await vaultApi.uploadLabFile(
        selectedRecordId,
        currentUser.username,
        selectedFile
      );
      setFeedback({ type: "success", msg: res.message });
      setSelectedFile(null);
      const preview = await vaultApi.previewLabFile(selectedRecordId);
      setPreviewData(preview);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsUploading(false);
    }
  };

  const handleDownload = async () => {
    if (!selectedRecordId) return;
    if (!dongleStatus?.detected) {
      setFeedback({
        type: "error",
        msg: "Akses Ditolak: USB Dongle fisik (auth.key) harus terpasang untuk mendekripsi dokumen lab.",
      });
      return;
    }

    setFeedback(null);
    setIsDownloading(true);
    try {
      const { blob, filename } = await vaultApi.downloadLabFile(selectedRecordId);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      setFeedback({
        type: "success",
        msg: `Berkas ${filename} berhasil didekripsi dengan ChaCha20 dan diunduh.`,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <div className="space-y-3.5">
      {/* Header Operational Bar */}
      <div className="p-3.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl flex flex-col md:flex-row items-start md:items-center justify-between gap-3 shadow-xs">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400 flex items-center justify-center border border-blue-100 dark:border-blue-900/50">
            <FileLockIcon className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
              Arsip Berkas Laboratorium & Patologi Klinik
            </h2>
            <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
              Penyimpanan dan verifikasi berkas hasil pemeriksaan laboratorium
            </p>
          </div>
        </div>

        {/* USB Dongle Status Pill */}
        <div className="flex items-center gap-2 p-1.5 px-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg shrink-0">
          <div
            className={`p-1 rounded-md ${
              dongleStatus?.detected
                ? "bg-emerald-100 dark:bg-emerald-950/50 text-emerald-700 dark:text-emerald-400"
                : "bg-red-100 dark:bg-rose-950/50 text-red-700 dark:text-rose-400"
            }`}
          >
            <KeyIcon className="w-3.5 h-3.5" />
          </div>
          <div className="text-xs leading-tight">
            <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6] block font-medium">
              Kunci Dekripsi USB
            </span>
            <span className="font-semibold text-[#202124] dark:text-[#e8eaed]">
              {dongleStatus?.detected ? "Dongle Terpasang" : "Dongle Belum Terdeteksi"}
            </span>
          </div>
        </div>
      </div>

      {feedback && (
        <div
          className={`p-2.5 px-3 rounded-lg text-xs flex items-center gap-2 border ${
            feedback.type === "success"
              ? "bg-emerald-50 dark:bg-emerald-950/30 border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300"
              : "bg-red-50 dark:bg-rose-950/30 border-red-200 dark:border-rose-800 text-red-800 dark:text-rose-300"
          }`}
        >
          {feedback.type === "success" ? (
            <CheckIcon className="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : (
            <AlertIcon className="w-4 h-4 text-red-600 dark:text-rose-400 shrink-0" />
          )}
          <span>{feedback.msg}</span>
        </div>
      )}

      {/* Selectors Bar */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5">
        <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3 shadow-xs">
          <label className="block text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] mb-1.5">
            Pilih Pasien Terdaftar
          </label>
          <select
            value={selectedPatientId || ""}
            onChange={(e) => setSelectedPatientId(Number(e.target.value))}
            className="w-full px-2.5 py-1.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] focus:outline-none focus:border-blue-500"
          >
            {patients.map((p) => (
              <option key={p.id} value={p.id}>
                {p.medical_record_number} - {p.decrypted.full_name}
              </option>
            ))}
          </select>
        </div>

        <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3 shadow-xs">
          <label className="block text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] mb-1.5">
            Pilih Rekam Medis Pasien
          </label>
          <select
            value={selectedRecordId || ""}
            onChange={(e) => setSelectedRecordId(Number(e.target.value))}
            className="w-full px-2.5 py-1.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] focus:outline-none focus:border-blue-500"
            disabled={records.length === 0}
          >
            {records.length === 0 ? (
              <option value="">(Pasien ini belum memiliki rekam medis)</option>
            ) : (
              records.map((r) => (
                <option key={r.id} value={r.id}>
                  Catatan #{r.id} - {r.diagnosis.substring(0, 45)}
                </option>
              ))
            )}
          </select>
        </div>
      </div>

      {/* Upload and Vault Details */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-3.5">
        {/* Left: Upload Form (5 cols) */}
        <div className="lg:col-span-5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col justify-between">
          <div className="space-y-3">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2 border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
              <UploadIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              <span>Unggah Berkas Patologi / Hasil Lab</span>
            </h3>

            <form onSubmit={handleUpload} className="space-y-3">
              <div className="border border-dashed border-[#dadce0] dark:border-[#3c4043] hover:border-blue-500 dark:hover:border-blue-400 rounded-lg p-4 text-center bg-[#f8f9fa] dark:bg-[#131314] transition cursor-pointer">
                <input
                  type="file"
                  id="lab-file"
                  onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                  accept=".pdf,.png,.jpg,.jpeg,.docx"
                  className="hidden"
                />
                <label htmlFor="lab-file" className="cursor-pointer space-y-1.5 block">
                  <FileLockIcon className="w-6 h-6 text-blue-600 dark:text-blue-400 mx-auto" />
                  <div className="text-xs text-[#202124] dark:text-[#e8eaed]">
                    {selectedFile ? (
                      <span className="font-semibold text-blue-600 dark:text-blue-400">
                        {selectedFile.name}
                      </span>
                    ) : (
                      <span>Pilih berkas PDF atau gambar hasil lab</span>
                    )}
                  </div>
                  <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                    Maksimal 25MB (Format: PDF, PNG, JPG)
                  </div>
                </label>
              </div>

              <button
                type="submit"
                disabled={!selectedRecordId || !selectedFile || isUploading}
                className="w-full py-2 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center justify-center gap-1.5"
              >
                <LockIcon className="w-3.5 h-3.5" />
                <span>{isUploading ? "Mengenkripsi..." : "Enkripsi & Simpan Dokumen Lab"}</span>
              </button>
            </form>
          </div>
        </div>

        {/* Right: Encrypted Vault Inspection & Decrypted Download (7 cols) */}
        <div className="lg:col-span-7 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col">
          <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2">
              <FileLockIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              <span>Inspeksi Arsip Berkas Terenkripsi di Disk</span>
            </h3>
          </div>

          {previewData ? (
            <div className="space-y-3">
              <div className="p-3 bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043] space-y-1.5 text-xs">
                <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                  <span>Nama Berkas Asli:</span>
                  <span className="font-semibold text-[#202124] dark:text-[#e8eaed]">
                    {previewData.original_filename}
                  </span>
                </div>
                <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                  <span>Ukuran Payload:</span>
                  <span className="font-mono text-[#202124] dark:text-[#e8eaed]">
                    {previewData.file_size_bytes} bytes
                  </span>
                </div>
                {isVerbose && (
                  <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                    <span>12-Byte Nonce:</span>
                    <span className="text-blue-600 dark:text-blue-400 font-mono">
                      {previewData.nonce_hex}
                    </span>
                  </div>
                )}
                <div className="text-[#5f6368] dark:text-[#9aa0a6] truncate pt-1 border-t border-[#dadce0] dark:border-[#3c4043]">
                  <span>Status Enkripsi: </span>
                  <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                    Terenkripsi Aman
                  </span>
                </div>
              </div>

              {/* Hex Dump for Technical Inspection Mode */}
              {isVerbose && (
                <div className="space-y-1">
                  <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] flex justify-between">
                    <span>Pratinjau Heksadesimal Biner Terenkripsi (128 Byte Pertama)</span>
                    <span className="text-xs text-slate-500 font-mono">Mode Teknis</span>
                  </div>
                  <div className="p-2.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg font-mono text-xs text-slate-700 dark:text-amber-300 break-all select-all leading-relaxed">
                    {previewData.first_128_hex_bytes}
                  </div>
                </div>
              )}

              {/* Download & Decrypt Button */}
              <button
                type="button"
                onClick={handleDownload}
                disabled={isDownloading}
                className="w-full py-2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center justify-center gap-1.5"
              >
                <DownloadIcon className="w-3.5 h-3.5" />
                <span>
                  {isDownloading
                    ? "Mendekripsi Dokumen..."
                    : "Dekripsi & Unduh Dokumen Laboratorium"}
                </span>
              </button>
            </div>
          ) : (
            <div className="py-12 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] flex flex-col items-center">
              <VaultIllustration className="w-16 h-16 mb-2 opacity-60" />
              <span>Belum ada berkas laboratorium untuk rekam medis ini.</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
