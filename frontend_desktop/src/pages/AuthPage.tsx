import { useEffect, useState } from "react";
import {
  authApi,
  DongleDriveInfo,
  DongleStatusReport,
  FingerprintStatusReport,
  UserSession,
} from "../services/api";
import {
  AlertIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  CameraIcon,
  CheckIcon,
  CloudflareSecurityShieldIllustration,
  DongleEjectedIllustration,
  FingerprintIcon,
  HardwareKeyScanIllustration,
  KeyIcon,
  RefreshIcon,
  ShieldIcon,
  UsersIcon,
  XIcon,
} from "../components/Icons";
import { WebcamCapture } from "../components/WebcamCapture";
import { RegisterDoctorModal } from "../components/RegisterDoctorModal";

interface AuthPageProps {
  onLoginSuccess: (user: UserSession) => void;
  ejectionNotice?: string | null;
  onClearEjectionNotice?: () => void;
}

export function AuthPage({
  onLoginSuccess,
  ejectionNotice,
  onClearEjectionNotice,
}: AuthPageProps) {
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [biometricMode, setBiometricMode] = useState<"fingerprint" | "face">("fingerprint");
  const [webcamFile, setWebcamFile] = useState<File | null>(null);
  const [dongleStatus, setDongleStatus] = useState<DongleStatusReport | null>(null);
  const [candidateDrives, setCandidateDrives] = useState<DongleDriveInfo[]>([]);
  const [selectedDongleMountpoint, setSelectedDongleMountpoint] = useState<string | null>(null);
  const [fingerprintStatus, setFingerprintStatus] = useState<FingerprintStatusReport | null>(null);
  const [isCheckingHardware, setIsCheckingHardware] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isRegisterOpen, setIsRegisterOpen] = useState(false);

  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // If ejected mid-session, reset back to step 1
  useEffect(() => {
    if (ejectionNotice) {
      setStep(1);
    }
  }, [ejectionNotice]);

  const checkHardwareStatus = async () => {
    setIsCheckingHardware(true);
    try {
      const [dongleRes, fpRes, drivesRes] = await Promise.all([
        authApi.getDongleStatus(),
        authApi.getFingerprintStatus().catch(() => ({
          available: false,
          message: "Sensor tidak terdeteksi",
        })),
        authApi.listDongleDrives().catch(() => ({ drives: [] })),
      ]);
      setDongleStatus(dongleRes);
      setFingerprintStatus(fpRes);

      const drives = drivesRes.drives || [];
      setCandidateDrives(drives);

      const drivesWithKey = drives.filter((d) => d.has_key);
      if (drivesWithKey.length > 0) {
        setSelectedDongleMountpoint((prev) => {
          if (prev && drivesWithKey.some((d) => d.mountpoint === prev)) {
            return prev;
          }
          return drivesWithKey[0].mountpoint;
        });
      }
    } catch {
      // Backend may be initializing
    } finally {
      setIsCheckingHardware(false);
    }
  };

  useEffect(() => {
    checkHardwareStatus();
    const interval = setInterval(checkHardwareStatus, 3000);
    return () => clearInterval(interval);
  }, []);

  const drivesWithKey = candidateDrives.filter((d) => d.has_key);
  const hasDetectedKey = drivesWithKey.length > 0 || Boolean(dongleStatus?.detected);

  const handleProceedToDongle = (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    if (!username.trim() || !password.trim()) {
      setErrorMessage("Silakan masukkan username dan kata sandi dokter");
      return;
    }
    setStep(2);
  };

  const handleProceedToBiometrics = () => {
    setErrorMessage(null);
    if (!hasDetectedKey) {
      setErrorMessage("Kunci fisik USB Dongle belum terdeteksi. Hubungkan flash drive Anda.");
      return;
    }
    setStep(3);
  };

  const handleFinalSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setErrorMessage(null);
    setSuccessMessage(null);

    if (!username.trim() || !password.trim()) {
      setErrorMessage("Silakan lengkapi username dan kata sandi");
      setStep(1);
      return;
    }
    if (!hasDetectedKey) {
      setErrorMessage("Kunci fisik USB Dongle tidak terdeteksi");
      setStep(2);
      return;
    }
    if (biometricMode === "face" && !webcamFile) {
      setErrorMessage("Ambil foto verifikasi wajah via kamera terlebih dahulu");
      return;
    }

    setIsSubmitting(true);
    try {
      const formData = new FormData();
      formData.append("username", username.trim());
      formData.append("password", password);
      formData.append("biometric_mode", biometricMode);
      if (selectedDongleMountpoint) {
        formData.append("dongle_mountpoint", selectedDongleMountpoint);
      }
      if (biometricMode === "face" && webcamFile) {
        formData.append("webcam_snapshot", webcamFile);
      }

      const res = await authApi.loginDoctor(formData);

      if (res.authenticated && res.user) {
        if (res.access_token) {
          localStorage.setItem("medvault_token", res.access_token);
        }
        setSuccessMessage(res.message);
        setTimeout(() => {
          onLoginSuccess(res.user!);
        }, 700);
      } else {
        setErrorMessage(res.message);
        // If password was wrong, return to step 1
        if (res.factors && !res.factors.password) {
          setStep(1);
        } else if (res.factors && !res.factors.dongle) {
          setStep(2);
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setErrorMessage(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="w-full max-w-lg mx-auto space-y-4">
      {/* Mid-Session Ejection Notice Banner */}
      {ejectionNotice && (
        <div className="bg-red-50 dark:bg-rose-950/40 border border-red-200 dark:border-rose-900 rounded-xl p-3.5 flex items-start justify-between gap-3 shadow-xs">
          <div className="flex items-start gap-3">
            <DongleEjectedIllustration className="w-6 h-6 shrink-0 mt-0.5" />
            <div>
              <h4 className="text-xs font-semibold text-red-800 dark:text-rose-200">
                Sesi Akses Terkunci
              </h4>
              <p className="text-xs text-red-700 dark:text-rose-300 mt-0.5 leading-relaxed">
                {ejectionNotice}
              </p>
            </div>
          </div>
          {onClearEjectionNotice && (
            <button
              type="button"
              onClick={onClearEjectionNotice}
              className="text-red-500 hover:text-red-700 dark:text-rose-400 dark:hover:text-rose-200 p-1 rounded-md transition"
            >
              <XIcon className="w-4 h-4" />
            </button>
          )}
        </div>
      )}

      {/* Main Centered Card */}
      <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl shadow-xs overflow-hidden transition-colors">
        {/* Step Progress Header */}
        <div className="px-5 py-3 border-b border-[#dadce0] dark:border-[#3c4043] bg-[#f8f9fa] dark:bg-[#18191a] flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div
              className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-semibold transition ${
                step === 1
                  ? "bg-blue-600 text-white"
                  : step > 1
                  ? "bg-emerald-600 text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
              }`}
            >
              {step > 1 ? <CheckIcon className="w-3.5 h-3.5" /> : "1"}
            </div>
            <span
              className={`text-xs font-medium ${
                step === 1 ? "text-blue-600 dark:text-blue-400 font-semibold" : "text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              Kredensial
            </span>

            <div className="w-4 h-px bg-slate-300 dark:bg-slate-700" />

            <div
              className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-semibold transition ${
                step === 2
                  ? "bg-blue-600 text-white"
                  : step > 2
                  ? "bg-emerald-600 text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
              }`}
            >
              {step > 2 ? <CheckIcon className="w-3.5 h-3.5" /> : "2"}
            </div>
            <span
              className={`text-xs font-medium ${
                step === 2 ? "text-blue-600 dark:text-blue-400 font-semibold" : "text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              Kunci USB
            </span>

            <div className="w-4 h-px bg-slate-300 dark:bg-slate-700" />

            <div
              className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-semibold transition ${
                step === 3
                  ? "bg-blue-600 text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
              }`}
            >
              3
            </div>
            <span
              className={`text-xs font-medium ${
                step === 3 ? "text-blue-600 dark:text-blue-400 font-semibold" : "text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              Biometrik
            </span>
          </div>

          <button
            type="button"
            onClick={() => setIsRegisterOpen(true)}
            className="text-xs text-[#1a73e8] dark:text-[#8ab4f8] hover:underline font-medium flex items-center gap-1 transition"
          >
            <UsersIcon className="w-3.5 h-3.5" />
            <span>Daftar Akun</span>
          </button>
        </div>

        {/* Form Body */}
        <div className="p-6">
          {errorMessage && (
            <div className="mb-4 p-3 bg-red-50 dark:bg-rose-950/40 border border-red-200 dark:border-rose-900 rounded-lg text-xs text-red-700 dark:text-rose-300 flex items-start gap-2">
              <AlertIcon className="w-4 h-4 text-red-600 dark:text-rose-400 shrink-0 mt-0.5" />
              <span>{errorMessage}</span>
            </div>
          )}
          {successMessage && (
            <div className="mb-4 p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 rounded-lg text-xs text-emerald-700 dark:text-emerald-300 flex items-start gap-2">
              <CheckIcon className="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0 mt-0.5" />
              <span>{successMessage}</span>
            </div>
          )}

          {/* ============================================================= */}
          {/* STEP 1: KREDENSIAL DOKTER                                     */}
          {/* ============================================================= */}
          {step === 1 && (
            <form onSubmit={handleProceedToDongle} className="space-y-4">
              <div className="text-center space-y-1 mb-4">
                <CloudflareSecurityShieldIllustration className="w-20 h-20 mx-auto" />
                <h3 className="text-base font-semibold text-[#202124] dark:text-[#e8eaed]">
                  Autentikasi Akun Dokter
                </h3>
                <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                  Masukkan identitas tenaga medis yang terdaftar di rumah sakit
                </p>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="block text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] mb-1">
                    Username Dokter
                  </label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="Masukkan username dokter..."
                    className="w-full px-3.5 py-2.5 bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg text-sm text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-[#1a73e8] transition"
                    autoFocus
                    required
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] mb-1">
                    Kata Sandi
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Masukkan kata sandi akun..."
                    className="w-full px-3.5 py-2.5 bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg text-sm text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-[#1a73e8] transition"
                    required
                  />
                </div>
              </div>

              <button
                type="submit"
                className="w-full mt-4 py-2.5 bg-[#1a73e8] hover:bg-[#1557b0] text-white text-sm font-semibold rounded-lg shadow-xs transition flex items-center justify-center gap-2"
              >
                <span>Lanjutkan ke Kunci Fisik</span>
                <ArrowRightIcon className="w-4 h-4" />
              </button>
            </form>
          )}

          {/* ============================================================= */}
          {/* STEP 2: KUNCI FISIK USB DONGLE                                */}
          {/* ============================================================= */}
          {step === 2 && (
            <div className="space-y-4">
              <div className="text-center space-y-1 mb-3">
                <HardwareKeyScanIllustration
                  className="w-20 h-20 mx-auto"
                  detected={hasDetectedKey}
                />
                <h3 className="text-base font-semibold text-[#202124] dark:text-[#e8eaed]">
                  Pemeriksaan Kunci USB Fisik
                </h3>
                <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                  Pastikan flash drive USB Dongle tenaga medis terpasang di perangkat
                </p>
              </div>

              {/* Status Header & Refresh */}
              <div className="flex items-center justify-between text-xs text-[#5f6368] dark:text-[#9aa0a6] px-1">
                <span className="font-semibold text-[#202124] dark:text-[#e8eaed]">
                  {drivesWithKey.length > 1
                    ? `Terdeteksi ${drivesWithKey.length} Kunci USB (Pilih salah satu)`
                    : drivesWithKey.length === 1
                    ? "Kunci USB Terpilih"
                    : "Status Kunci USB"}
                </span>
                <button
                  type="button"
                  onClick={checkHardwareStatus}
                  className="flex items-center gap-1 text-xs text-[#1a73e8] dark:text-[#8ab4f8] hover:underline font-medium"
                  title="Pindai ulang sambungan drive"
                >
                  <RefreshIcon className={`w-3.5 h-3.5 ${isCheckingHardware ? "animate-spin text-blue-600" : ""}`} />
                  <span>Pindai Ulang</span>
                </button>
              </div>

              {/* List of Detected USB Keys or Waiting State */}
              {drivesWithKey.length === 0 ? (
                <div
                  className={`p-3.5 rounded-lg border transition ${
                    dongleStatus?.detected
                      ? "bg-emerald-50 dark:bg-emerald-950/30 border-emerald-200 dark:border-emerald-800"
                      : "bg-slate-50 dark:bg-[#131314] border-[#dadce0] dark:border-[#3c4043]"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2.5">
                      <div
                        className={`p-1.5 rounded-md ${
                          dongleStatus?.detected
                            ? "bg-emerald-600 text-white"
                            : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
                        }`}
                      >
                        <KeyIcon className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                          {dongleStatus?.detected ? "Kunci USB Terhubung" : "Menunggu Sambungan USB"}
                        </div>
                        <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                          {dongleStatus?.detected
                            ? `Partisi drive: ${dongleStatus.mountpoint}`
                            : "Hubungkan flash drive auth.key dokter"}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="space-y-2 max-h-56 overflow-y-auto pr-0.5">
                  {drivesWithKey.map((d) => {
                    const isSelected = selectedDongleMountpoint === d.mountpoint;
                    return (
                      <div
                        key={d.mountpoint}
                        onClick={() => setSelectedDongleMountpoint(d.mountpoint)}
                        className={`p-3 rounded-lg border cursor-pointer transition flex items-center justify-between ${
                          isSelected
                            ? "bg-blue-50/80 dark:bg-blue-950/40 border-blue-500 dark:border-blue-500 shadow-xs ring-1 ring-blue-500"
                            : "bg-white dark:bg-[#1e1f20] border-[#dadce0] dark:border-[#3c4043] hover:border-blue-300 dark:hover:border-blue-700"
                        }`}
                      >
                        <div className="flex items-center gap-2.5">
                          <div
                            className={`p-1.5 rounded-md ${
                              isSelected
                                ? "bg-blue-600 text-white"
                                : "bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300"
                            }`}
                          >
                            <KeyIcon className="w-4 h-4" />
                          </div>
                          <div>
                            <div className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-1.5">
                              <span>{d.label}</span>
                              {d.is_removable && (
                                <span className="px-1.5 py-0.2 rounded text-xs bg-emerald-100 dark:bg-emerald-950/60 text-emerald-700 dark:text-emerald-300 font-medium">
                                  Flash Drive
                                </span>
                              )}
                            </div>
                            <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5 font-mono">
                              {d.mountpoint}
                              {d.token_hash_prefix && (
                                <span className="ml-2 text-slate-500">
                                  (Key: {d.token_hash_prefix})
                                </span>
                              )}
                            </div>
                          </div>
                        </div>

                        <div className="shrink-0 pl-2">
                          <div
                            className={`w-5 h-5 rounded-full flex items-center justify-center border transition ${
                              isSelected
                                ? "border-blue-600 bg-blue-600 text-white"
                                : "border-slate-300 dark:border-slate-600"
                            }`}
                          >
                            {isSelected && <CheckIcon className="w-3.5 h-3.5" />}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}

              <div className="flex items-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setStep(1)}
                  className="flex-1 py-2.5 border border-[#dadce0] dark:border-[#3c4043] text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] hover:bg-slate-50 dark:hover:bg-[#282a2c] rounded-lg transition flex items-center justify-center gap-1.5"
                >
                  <ArrowLeftIcon className="w-4 h-4" />
                  <span>Kembali</span>
                </button>
                <button
                  type="button"
                  onClick={handleProceedToBiometrics}
                  disabled={!hasDetectedKey}
                  className="flex-1 py-2.5 bg-[#1a73e8] hover:bg-[#1557b0] disabled:opacity-40 text-white text-xs font-semibold rounded-lg shadow-xs transition flex items-center justify-center gap-1.5"
                >
                  <span>Lanjut ke Biometrik</span>
                  <ArrowRightIcon className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}

          {/* ============================================================= */}
          {/* STEP 3: VERIFIKASI BIOMETRIK                                  */}
          {/* ============================================================= */}
          {step === 3 && (
            <div className="space-y-4">
              <div className="text-center space-y-1 mb-2">
                <h3 className="text-base font-semibold text-[#202124] dark:text-[#e8eaed]">
                  Konfirmasi Biometrik
                </h3>
                <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                  Pilih metode biometrik untuk otorisasi akhir akses rekam medis
                </p>
              </div>

              {/* Biometric Mode Tab Switcher */}
              <div className="flex rounded-lg bg-[#f1f3f4] dark:bg-[#131314] p-1 border border-[#dadce0] dark:border-[#3c4043]">
                <button
                  type="button"
                  onClick={() => setBiometricMode("fingerprint")}
                  className={`flex-1 py-2 px-3 rounded-md text-xs font-semibold flex items-center justify-center gap-1.5 transition ${
                    biometricMode === "fingerprint"
                      ? "bg-white dark:bg-[#1e1f20] text-[#1a73e8] dark:text-[#8ab4f8] shadow-xs"
                      : "text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-[#e8eaed]"
                  }`}
                >
                  <FingerprintIcon className="w-4 h-4" />
                  <span>Sidik Jari (Windows Hello)</span>
                </button>
                <button
                  type="button"
                  onClick={() => setBiometricMode("face")}
                  className={`flex-1 py-2 px-3 rounded-md text-xs font-semibold flex items-center justify-center gap-1.5 transition ${
                    biometricMode === "face"
                      ? "bg-white dark:bg-[#1e1f20] text-[#1a73e8] dark:text-[#8ab4f8] shadow-xs"
                      : "text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-[#e8eaed]"
                  }`}
                >
                  <CameraIcon className="w-4 h-4" />
                  <span>Kamera Wajah</span>
                </button>
              </div>

              {/* Biometric Viewports */}
              {biometricMode === "fingerprint" ? (
                <div className="p-5 bg-slate-50 dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg flex flex-col items-center justify-center text-center space-y-3">
                  <div className="w-16 h-16 rounded-full bg-blue-50 dark:bg-blue-950/50 border border-blue-200 dark:border-blue-900 flex items-center justify-center text-blue-600 dark:text-blue-400">
                    <FingerprintIcon className="w-8 h-8" />
                  </div>
                  <div>
                    <h4 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
                      Sensor Sidik Jari Siap
                    </h4>
                    <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                      {fingerprintStatus?.message || "Sensor Windows Hello siap memindai jari dokter"}
                    </p>
                  </div>
                </div>
              ) : (
                <WebcamCapture
                  onCapture={(file) => setWebcamFile(file)}
                  onClear={() => setWebcamFile(null)}
                  label="Kamera Pengenal Wajah"
                />
              )}

              <div className="flex items-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setStep(2)}
                  className="py-2.5 px-4 border border-[#dadce0] dark:border-[#3c4043] text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] hover:bg-slate-50 dark:hover:bg-[#282a2c] rounded-lg transition flex items-center gap-1.5"
                >
                  <ArrowLeftIcon className="w-4 h-4" />
                  <span>Kunci USB</span>
                </button>
                <button
                  type="button"
                  onClick={() => handleFinalSubmit()}
                  disabled={isSubmitting || (biometricMode === "face" && !webcamFile)}
                  className="flex-1 py-2.5 bg-[#1a73e8] hover:bg-[#1557b0] disabled:opacity-40 text-white text-xs font-semibold rounded-lg shadow-xs transition flex items-center justify-center gap-2"
                >
                  <ShieldIcon className="w-4 h-4" />
                  <span>
                    {isSubmitting
                      ? "Memverifikasi..."
                      : biometricMode === "fingerprint"
                      ? "Pindai Jari & Buka Rekam Medis"
                      : "Verifikasi Wajah & Buka Rekam Medis"}
                  </span>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Doctor Registration Modal */}
      <RegisterDoctorModal
        isOpen={isRegisterOpen}
        onClose={() => setIsRegisterOpen(false)}
        onSuccess={(newUsername: string) => {
          setUsername(newUsername);
          setStep(1);
          checkHardwareStatus();
          setSuccessMessage(`Akun Dr. ${newUsername} berhasil didaftarkan! Silakan masukkan kata sandi untuk login.`);
        }}
        dongleStatus={dongleStatus}
      />
    </div>
  );
}
