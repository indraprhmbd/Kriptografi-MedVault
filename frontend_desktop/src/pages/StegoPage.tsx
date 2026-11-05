import { useEffect, useState } from "react";
import {
  MedicalRecordItem,
  patientApi,
  PatientItem,
  recordApi,
  stegoApi,
  StegoMetrics,
  UserSession,
} from "../services/api";
import {
  AlertIcon,
  CheckIcon,
  DownloadIcon,
  EyeIcon,
  ImageIcon,
  LockIcon,
  StegoIllustration,
  UploadIcon,
} from "../components/Icons";
import { useVerbose } from "../components/VerboseContext";

interface StegoPageProps {
  currentUser: UserSession | null;
}

export function StegoPage({ currentUser }: StegoPageProps) {
  const { isVerbose } = useVerbose();
  const [activeTab, setActiveTab] = useState<"embed" | "extract">("embed");

  // Embed Tab State
  const [patients, setPatients] = useState<PatientItem[]>([]);
  const [selectedPatientId, setSelectedPatientId] = useState<number | null>(null);
  const [records, setRecords] = useState<MedicalRecordItem[]>([]);
  const [selectedRecordId, setSelectedRecordId] = useState<number | null>(null);
  const [secretText, setSecretText] = useState("");
  const [inputImageFile, setInputImageFile] = useState<File | null>(null);
  const [imagePreviewUrl, setImagePreviewUrl] = useState<string | null>(null);
  const [isEmbedding, setIsEmbedding] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [embedResult, setEmbedResult] = useState<{ metrics: StegoMetrics; stego_filename: string } | null>(
    null
  );

  // Extract Tab State
  const [stegoUploadFile, setStegoUploadFile] = useState<File | null>(null);
  const [isExtracting, setIsExtracting] = useState(false);
  const [extractedSecret, setExtractedSecret] = useState<string | null>(null);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; msg: string } | null>(
    null
  );

  useEffect(() => {
    patientApi.listPatients().then((res) => {
      setPatients(res.patients);
      if (res.patients.length > 0) {
        setSelectedPatientId(res.patients[0].id);
      }
    });
  }, []);

  useEffect(() => {
    if (selectedPatientId) {
      recordApi.listByPatient(selectedPatientId).then((res) => {
        setRecords(res.records);
        if (res.records.length > 0) {
          setSelectedRecordId(res.records[0].id);
          const currentPatient = patients.find((p) => p.id === selectedPatientId);
          setSecretText(
            `Pasien: ${currentPatient?.decrypted.full_name || ""} | Diagnosis: ${res.records[0].diagnosis} | Resep: ${res.records[0].prescription}`
          );
        } else {
          setSelectedRecordId(null);
          setSecretText("");
        }
      });
    }
  }, [selectedPatientId, patients]);

  const handleImageSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setInputImageFile(file);
      setImagePreviewUrl(URL.createObjectURL(file));
      setEmbedResult(null);
    }
  };

  const handleEmbed = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedRecordId || !inputImageFile || !secretText.trim() || !currentUser) return;
    setFeedback(null);
    setIsEmbedding(true);

    try {
      const res = await stegoApi.embedSecret(
        selectedRecordId,
        currentUser.username,
        secretText.trim(),
        inputImageFile
      );
      setEmbedResult(res);
      setFeedback({
        type: "success",
        msg: "Ringkasan medis berhasil disematkan ke dalam citra radiologi secara lossless.",
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsEmbedding(false);
    }
  };

  const handleDownloadStego = async () => {
    if (!selectedRecordId) return;
    setFeedback(null);
    setIsDownloading(true);
    try {
      const { blob, filename } = await stegoApi.downloadStegoScan(selectedRecordId);
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
        msg: `Citra stego ${filename} berhasil diunduh.`,
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsDownloading(false);
    }
  };

  const handleExtract = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!stegoUploadFile) return;
    setFeedback(null);
    setIsExtracting(true);
    setExtractedSecret(null);

    try {
      const res = await stegoApi.extractSecret(stegoUploadFile);
      setExtractedSecret(res.extracted_text);
      setFeedback({
        type: "success",
        msg: "Metadata rekam medis berhasil diekstraksi dan diverifikasi secara utuh.",
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsExtracting(false);
    }
  };

  return (
    <div className="space-y-3.5">
      {/* Header Operational Bar */}
      <div className="p-3.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl flex flex-col md:flex-row items-start md:items-center justify-between gap-3 shadow-xs">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400 flex items-center justify-center border border-blue-100 dark:border-blue-900/50">
            <ImageIcon className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
              Sertifikasi Digital & Watermarking Citra Radiologi
            </h2>
            <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
              Penyisipan dan verifikasi ringkasan rekam medis pada citra radiologi secara lossless
            </p>
          </div>
        </div>

        {/* Tab Segmented Control */}
        <div className="flex bg-[#f1f3f4] dark:bg-[#131314] p-0.5 rounded-lg border border-[#dadce0] dark:border-[#3c4043] shrink-0">
          <button
            onClick={() => setActiveTab("embed")}
            className={`px-3 py-1.5 rounded-md text-xs font-semibold transition ${
              activeTab === "embed"
                ? "bg-white dark:bg-[#1e1f20] text-blue-600 dark:text-blue-400 shadow-xs"
                : "text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-[#e8eaed]"
            }`}
          >
            Sematkan Metadata (Embed)
          </button>
          <button
            onClick={() => setActiveTab("extract")}
            className={`px-3 py-1.5 rounded-md text-xs font-semibold transition ${
              activeTab === "extract"
                ? "bg-white dark:bg-[#1e1f20] text-blue-600 dark:text-blue-400 shadow-xs"
                : "text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-[#e8eaed]"
            }`}
          >
            Verifikasi & Ekstraksi (Extract)
          </button>
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

      {/* Tab 1: Embed Mode */}
      {activeTab === "embed" ? (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-3.5">
          {/* Left: Input Form (6 cols) */}
          <div className="lg:col-span-6 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2 border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
              <UploadIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              <span>Input Citra Radiologi & Metadata Medis</span>
            </h3>

            <form onSubmit={handleEmbed} className="space-y-3">
              {/* Patient & Record Selection */}
              <div className="grid grid-cols-2 gap-2.5">
                <div>
                  <label className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-semibold block mb-1">
                    Pasien
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
                <div>
                  <label className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-semibold block mb-1">
                    Rekam Medis
                  </label>
                  <select
                    value={selectedRecordId || ""}
                    onChange={(e) => setSelectedRecordId(Number(e.target.value))}
                    className="w-full px-2.5 py-1.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] focus:outline-none focus:border-blue-500"
                    disabled={records.length === 0}
                  >
                    {records.map((r) => (
                      <option key={r.id} value={r.id}>
                        #{r.id} - {r.diagnosis.substring(0, 20)}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Upload PNG scan */}
              <div>
                <label className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-semibold block mb-1">
                  Citra Radiologi Host (Format PNG Lossless)
                </label>
                <div className="border border-[#dadce0] dark:border-[#3c4043] bg-[#f8f9fa] dark:bg-[#131314] rounded-md p-2.5">
                  <input
                    type="file"
                    accept=".png"
                    onChange={handleImageSelect}
                    className="text-xs text-[#5f6368] dark:text-[#9aa0a6] file:mr-2.5 file:py-1 file:px-2.5 file:rounded file:border-0 file:text-xs file:font-semibold file:bg-blue-50 dark:file:bg-blue-950/40 file:text-blue-600 dark:file:text-blue-400 hover:file:bg-blue-100 cursor-pointer"
                    required
                  />
                </div>
              </div>

              {/* Secret message payload */}
              <div>
                <label className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-semibold block mb-1">
                  Ringkasan Catatan Rahasia yang Disematkan
                </label>
                <textarea
                  value={secretText}
                  onChange={(e) => setSecretText(e.target.value)}
                  placeholder="Masukkan ringkasan diagnosis rahasia..."
                  className="w-full px-2.5 py-1.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500 h-20 resize-none font-mono"
                  required
                />
              </div>

              <button
                type="submit"
                disabled={!selectedRecordId || !inputImageFile || isEmbedding}
                className="w-full py-2 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center justify-center gap-1.5"
              >
                <LockIcon className="w-3.5 h-3.5" />
                <span>{isEmbedding ? "Menyematkan..." : "Sematkan Metadata ke Citra Radiologi"}</span>
              </button>
            </form>
          </div>

          {/* Right: Fidelity Metrics & Stego Result (6 cols) */}
          <div className="lg:col-span-6 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col justify-between">
            <div>
              <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2 border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
                <EyeIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
                <span>Pratinjau Citra & Metrik Kualitas Lossless</span>
              </h3>

              {imagePreviewUrl ? (
                <div className="space-y-3 mt-3">
                  <div className="grid grid-cols-2 gap-2.5">
                    <div className="space-y-1">
                      <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-medium">Citra Asli:</div>
                      <div className="aspect-square rounded-lg overflow-hidden border border-[#dadce0] dark:border-[#3c4043] bg-slate-100 dark:bg-black">
                        <img src={imagePreviewUrl} alt="Original Scan" className="w-full h-full object-cover" />
                      </div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-medium">
                        {embedResult ? "Citra Stego (LSB):" : "Hasil Proses:"}
                      </div>
                      <div className="aspect-square rounded-lg overflow-hidden border border-[#dadce0] dark:border-[#3c4043] bg-slate-100 dark:bg-black flex items-center justify-center">
                        {embedResult ? (
                          <img src={imagePreviewUrl} alt="Stego Scan" className="w-full h-full object-cover" />
                        ) : (
                          <span className="text-slate-400 dark:text-slate-600 text-xs">Belum diproses</span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Quality Metrics */}
                  {embedResult && (
                    <div className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-1.5 text-xs font-mono">
                      <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                        <span className="font-sans">Ukuran Payload:</span>
                        <span className="text-[#202124] dark:text-[#e8eaed]">
                          {embedResult.metrics.embedded_bytes} bytes ({embedResult.metrics.embedded_bits} bits)
                        </span>
                      </div>
                      <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                        <span className="font-sans">Kapasitas Citra:</span>
                        <span className="text-[#202124] dark:text-[#e8eaed]">
                          {embedResult.metrics.total_capacity_bits} bits
                        </span>
                      </div>
                      <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                        <span className="font-sans">Penggunaan Kapasitas:</span>
                        <span className="font-semibold text-blue-600 dark:text-blue-400">
                          {embedResult.metrics.capacity_usage_pct}%
                        </span>
                      </div>
                      <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                        <span className="font-sans">Nilai PSNR:</span>
                        <span className="text-emerald-700 dark:text-emerald-400 font-bold">
                          {embedResult.metrics.psnr} dB
                        </span>
                      </div>
                      <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                        <span className="font-sans">Nilai MSE:</span>
                        <span className="text-emerald-700 dark:text-emerald-400">
                          {embedResult.metrics.mse}
                        </span>
                      </div>

                      <button
                        type="button"
                        onClick={handleDownloadStego}
                        disabled={isDownloading}
                        className="mt-2 w-full py-2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-xs font-semibold rounded-md transition flex items-center justify-center gap-1.5 shadow-xs font-sans"
                      >
                        <DownloadIcon className="w-3.5 h-3.5" />
                        <span>{isDownloading ? "Mengunduh..." : "Unduh Citra Radiologi Terverifikasi (PNG)"}</span>
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <div className="py-12 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] flex flex-col items-center">
                  <StegoIllustration className="w-16 h-16 mb-2 opacity-60" />
                  <span>Pilih file citra PNG di sebelah kiri untuk melihat pratinjau.</span>
                </div>
              )}
            </div>
          </div>
        </div>
      ) : (
        /* Tab 2: Extract Mode */
        <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-4 max-w-xl mx-auto space-y-3 shadow-xs">
          <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2 border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
            <EyeIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
            <span>Ekstraksi Metadata dari Citra Radiologi PNG</span>
          </h3>

          <form onSubmit={handleExtract} className="space-y-3">
            <div className="border border-[#dadce0] dark:border-[#3c4043] bg-[#f8f9fa] dark:bg-[#131314] rounded-lg p-3">
              <label className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-semibold block mb-1.5">
                Pilih File Citra Stego (.PNG)
              </label>
              <input
                type="file"
                accept=".png"
                onChange={(e) => setStegoUploadFile(e.target.files?.[0] || null)}
                className="text-xs text-[#5f6368] dark:text-[#9aa0a6] file:mr-2.5 file:py-1 file:px-2.5 file:rounded-md file:border-0 file:text-xs file:font-semibold file:bg-blue-50 dark:file:bg-blue-950/40 file:text-blue-600 dark:file:text-blue-400 hover:file:bg-blue-100 cursor-pointer"
                required
              />
            </div>

            <button
              type="submit"
              disabled={!stegoUploadFile || isExtracting}
              className="w-full py-2 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center justify-center gap-1.5"
            >
              <EyeIcon className="w-3.5 h-3.5" />
              <span>{isExtracting ? "Mengekstraksi..." : "Verifikasi & Ekstraksi Metadata"}</span>
            </button>
          </form>

          {extractedSecret && (
            <div className="p-3 bg-emerald-50 dark:bg-emerald-950/20 border border-emerald-200 dark:border-emerald-800 rounded-lg space-y-1.5 mt-3">
              <div className="text-xs font-semibold text-emerald-800 dark:text-emerald-400 flex items-center gap-1.5">
                <CheckIcon className="w-3.5 h-3.5" />
                <span>Metadata Medis Berhasil Ditemukan:</span>
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded-md text-xs font-mono text-[#202124] dark:text-[#e8eaed] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {extractedSecret}
              </div>
              <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                {isVerbose ? (
                  <span>
                    Delimiter penutup terverifikasi: <code className="text-blue-600 dark:text-blue-400 font-mono">$$MEDVAULT_EOF$$</code>
                  </span>
                ) : (
                  <span className="text-emerald-700 dark:text-emerald-400 font-medium">
                    Integritas citra terverifikasi utuh
                  </span>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
