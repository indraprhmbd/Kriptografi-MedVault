import { useEffect, useState } from "react";
import { authApi, DongleDriveInfo, DongleStatusReport } from "../services/api";
import {
  AlertIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  BiometricReticleIllustration,
  CheckIcon,
  CloudflareSecurityShieldIllustration,
  HardwareKeyScanIllustration,
  KeyIcon,
  RefreshIcon,
  ShieldIcon,
  UsersIcon,
  XIcon,
} from "./Icons";
import { WebcamCapture } from "./WebcamCapture";

interface RegisterDoctorModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (newUsername: string) => void;
  dongleStatus?: DongleStatusReport | null;
}

export function RegisterDoctorModal({
  isOpen,
  onClose,
  onSuccess,
}: RegisterDoctorModalProps) {
  // Multi-step Wizard: 1 = Kredensial, 2 = Kunci USB, 3 = Biometrik
  const [step, setStep] = useState<1 | 2 | 3>(1);

  // Step 1: Kredensial
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  // Step 2: Kunci USB Dongle & Auto-Provisioning
  const [drives, setDrives] = useState<DongleDriveInfo[]>([]);
  const [isLoadingDrives, setIsLoadingDrives] = useState(false);
  const [selectedMountpoint, setSelectedMountpoint] = useState<string | null>(null);
  const [selectedHash, setSelectedHash] = useState<string>("");
  const [isProvisioning, setIsProvisioning] = useState<string | null>(null); // mountpoint being provisioned
  const [provisionSuccessMsg, setProvisionSuccessMsg] = useState<string | null>(null);

  // Step 3: Biometrik Wajah
  const [faceFile, setFaceFile] = useState<File | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Feedback notifications
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Reset wizard on modal open
  useEffect(() => {
    if (isOpen) {
      setStep(1);
      setUsername("");
      setPassword("");
      setConfirmPassword("");
      setSelectedMountpoint(null);
      setSelectedHash("");
      setFaceFile(null);
      setErrorMessage(null);
      setSuccessMessage(null);
      setProvisionSuccessMsg(null);
      loadDrives();
    }
  }, [isOpen]);

  const loadDrives = async () => {
    setIsLoadingDrives(true);
    try {
      const res = await authApi.listDongleDrives();
      setDrives(res.drives);

      // Auto-select first drive that already contains auth.key if nothing selected yet
      if (!selectedMountpoint) {
        const withKey = res.drives.find((d) => d.has_key && d.token_hash);
        if (withKey && withKey.token_hash) {
          setSelectedMountpoint(withKey.mountpoint);
          setSelectedHash(withKey.token_hash);
        }
      }
    } catch {
      // Handled silently
    } finally {
      setIsLoadingDrives(false);
    }
  };

  if (!isOpen) return null;

  // Step 1 -> Step 2 validation
  const handleProceedToStep2 = (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);

    const cleanUser = username.trim();
    if (!cleanUser) {
      setErrorMessage("Silakan masukkan username dokter");
      return;
    }
    if (cleanUser.length < 3) {
      setErrorMessage("Username dokter minimal 3 karakter");
      return;
    }
    if (!password) {
      setErrorMessage("Silakan masukkan kata sandi akun");
      return;
    }
    if (password.length < 6) {
      setErrorMessage("Kata sandi dokter minimal 6 karakter");
      return;
    }
    if (password !== confirmPassword) {
      setErrorMessage("Konfirmasi kata sandi tidak cocok dengan kata sandi");
      return;
    }

    setStep(2);
    loadDrives();
  };

  // Auto-provision a blank USB drive
  const handleProvisionDrive = async (mountpoint: string) => {
    setIsProvisioning(mountpoint);
    setErrorMessage(null);
    setProvisionSuccessMsg(null);

    try {
      const res = await authApi.provisionDongle(mountpoint, false);
      setSelectedMountpoint(res.mountpoint);
      setSelectedHash(res.token_hash);
      setProvisionSuccessMsg(
        `Kunci auth.key berhasil diinisialisasi pada ${res.mountpoint} dan diamankan sebagai file sistem tersembunyi.`
      );
      // Refresh list to update badge
      await loadDrives();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setErrorMessage(`Gagal menginisialisasi USB Dongle: ${msg}`);
    } finally {
      setIsProvisioning(null);
    }
  };

  // Select an existing key
  const handleSelectDriveKey = (drive: DongleDriveInfo) => {
    if (drive.token_hash) {
      setSelectedMountpoint(drive.mountpoint);
      setSelectedHash(drive.token_hash);
      setProvisionSuccessMsg(null);
      setErrorMessage(null);
    }
  };

  // Step 2 -> Step 3 validation
  const handleProceedToStep3 = () => {
    setErrorMessage(null);
    if (!selectedHash) {
      setErrorMessage(
        "Pilih flash drive yang memiliki kunci auth.key atau klik 'Inisialisasi Kunci Baru' pada salah satu drive."
      );
      return;
    }
    setStep(3);
  };

  // Step 3 -> Final submission
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setSuccessMessage(null);

    if (!faceFile) {
      setErrorMessage("Silakan ambil foto referensi biometrik wajah dokter terlebih dahulu");
      return;
    }

    setIsSubmitting(true);
    try {
      const formData = new FormData();
      formData.append("username", username.trim());
      formData.append("password", password);
      formData.append("dongle_token_hash_hex", selectedHash);
      formData.append("face_image", faceFile);

      const res = await authApi.registerDoctor(formData);
      const registeredName = username.trim();
      setSuccessMessage(res.message || `Akun Dr. ${registeredName} berhasil didaftarkan!`);

      setTimeout(() => {
        onSuccess(registeredName);
        onClose();
      }, 1500);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setErrorMessage(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
      <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl max-w-xl w-full max-h-[92vh] flex flex-col shadow-2xl overflow-hidden transition-colors">
        {/* Header with Title and Close Button */}
        <div className="px-5 py-3.5 border-b border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between bg-[#f8f9fa] dark:bg-[#18191a]">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded-lg bg-blue-50 dark:bg-blue-950/60 text-[#1a73e8] dark:text-[#8ab4f8]">
              <UsersIcon className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
                Registrasi Dokter Baru
              </h3>
              <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                Pendaftaran 3-faktor: Kredensial, Kunci USB, dan Biometrik
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-md text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed] transition"
          >
            <XIcon className="w-4 h-4" />
          </button>
        </div>

        {/* Step Indicator Progress Bar */}
        <div className="px-5 py-2.5 bg-[#f1f3f4] dark:bg-[#131314] border-b border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between">
          <div className="flex items-center gap-1.5 text-xs font-semibold">
            <span
              className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold ${
                step === 1
                  ? "bg-[#1a73e8] text-white"
                  : step > 1
                  ? "bg-emerald-600 text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              1
            </span>
            <span className={step === 1 ? "text-[#1a73e8] dark:text-[#8ab4f8]" : "text-[#5f6368] dark:text-[#9aa0a6]"}>
              Kredensial
            </span>
          </div>

          <div className="w-6 h-px bg-slate-300 dark:bg-slate-700" />

          <div className="flex items-center gap-1.5 text-xs font-semibold">
            <span
              className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold ${
                step === 2
                  ? "bg-[#1a73e8] text-white"
                  : step > 2
                  ? "bg-emerald-600 text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              2
            </span>
            <span className={step === 2 ? "text-[#1a73e8] dark:text-[#8ab4f8]" : "text-[#5f6368] dark:text-[#9aa0a6]"}>
              Kunci USB
            </span>
          </div>

          <div className="w-6 h-px bg-slate-300 dark:bg-slate-700" />

          <div className="flex items-center gap-1.5 text-xs font-semibold">
            <span
              className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold ${
                step === 3
                  ? "bg-[#1a73e8] text-white"
                  : "bg-slate-200 dark:bg-slate-700 text-[#5f6368] dark:text-[#9aa0a6]"
              }`}
            >
              3
            </span>
            <span className={step === 3 ? "text-[#1a73e8] dark:text-[#8ab4f8]" : "text-[#5f6368] dark:text-[#9aa0a6]"}>
              Biometrik
            </span>
          </div>
        </div>

        {/* Modal Scrollable Body */}
        <div className="flex-1 overflow-y-auto p-5 space-y-4">
          {/* Feedback Alerts */}
          {errorMessage && (
            <div className="p-3 bg-red-50 dark:bg-rose-950/40 border border-red-200 dark:border-rose-900 rounded-lg text-xs text-red-700 dark:text-rose-300 flex items-start gap-2">
              <AlertIcon className="w-4 h-4 text-red-600 dark:text-rose-400 shrink-0 mt-0.5" />
              <span>{errorMessage}</span>
            </div>
          )}

          {successMessage && (
            <div className="p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 rounded-lg text-xs text-emerald-700 dark:text-emerald-300 flex items-center gap-2">
              <CheckIcon className="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
              <span>{successMessage}</span>
            </div>
          )}

          {/* =============================================================== */}
          {/* STEP 1: KREDENSIAL DOKTER                                       */}
          {/* =============================================================== */}
          {step === 1 && (
            <form onSubmit={handleProceedToStep2} className="space-y-4">
              <div className="flex items-center gap-3 p-3 bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043]">
                <CloudflareSecurityShieldIllustration className="w-12 h-12 shrink-0" />
                <div>
                  <h4 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                    Faktor 1: Kredensial Akun Dokter
                  </h4>
                  <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                    Kata sandi dienkripsi dengan proteksi berlapis berstandar keamanan rumah sakit.
                  </p>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] mb-1">
                  Username Dokter
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="Masukkan username dokter..."
                  className="w-full px-3 py-2 bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-sm text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-[#1a73e8] transition"
                  required
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] mb-1">
                    Kata Sandi
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Minimal 6 karakter..."
                    className="w-full px-3 py-2 bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-sm text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-[#1a73e8] transition"
                    required
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] mb-1">
                    Konfirmasi Kata Sandi
                  </label>
                  <input
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Ulangi kata sandi..."
                    className="w-full px-3 py-2 bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-sm text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-[#1a73e8] transition"
                    required
                  />
                </div>
              </div>

              <div className="pt-2 flex justify-end gap-2 border-t border-[#dadce0] dark:border-[#3c4043]">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2 bg-[#f1f3f4] dark:bg-[#282a2c] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#e8eaed] dark:hover:bg-[#3c4043] text-xs font-medium rounded-md transition"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-[#1a73e8] hover:bg-[#1557b0] text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center gap-1.5"
                >
                  <span>Lanjut ke Kunci Fisik USB</span>
                  <ArrowRightIcon className="w-4 h-4" />
                </button>
              </div>
            </form>
          )}

          {/* =============================================================== */}
          {/* STEP 2: KUNCI FISIK USB & AUTO-PROVISIONING                     */}
          {/* =============================================================== */}
          {step === 2 && (
            <div className="space-y-4">
              <div className="flex items-center gap-3 p-3 bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043]">
                <HardwareKeyScanIllustration
                  className="w-12 h-12 shrink-0"
                  detected={Boolean(selectedHash)}
                />
                <div>
                  <h4 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                    Faktor 2: Kunci Fisik USB Dongle (Possession Factor)
                  </h4>
                  <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                    Pilih USB yang sudah memiliki kunci atau buat kunci baru secara otomatis.
                  </p>
                </div>
              </div>

              {provisionSuccessMsg && (
                <div className="p-3 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 rounded-lg text-xs text-emerald-800 dark:text-emerald-300 flex items-start gap-2">
                  <CheckIcon className="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0 mt-0.5" />
                  <span>{provisionSuccessMsg}</span>
                </div>
              )}

              {/* Drive scanning header with refresh */}
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6]">
                  Daftar Drive / Partisi Penyimpanan
                </span>
                <button
                  type="button"
                  onClick={loadDrives}
                  disabled={isLoadingDrives}
                  className="flex items-center gap-1 text-xs text-[#1a73e8] dark:text-[#8ab4f8] hover:underline font-medium"
                >
                  <RefreshIcon className={`w-3.5 h-3.5 ${isLoadingDrives ? "animate-spin" : ""}`} />
                  <span>Pindai Ulang Drive</span>
                </button>
              </div>

              {/* Drives List */}
              <div className="space-y-2.5 max-h-56 overflow-y-auto pr-1">
                {isLoadingDrives ? (
                  <div className="p-6 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                    Memindai partisi USB terpasang...
                  </div>
                ) : drives.length === 0 ? (
                  <div className="p-6 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043]">
                    Tidak ada drive USB yang terdeteksi. Hubungkan USB flash drive Anda ke komputer.
                  </div>
                ) : (
                  drives.map((d) => {
                    const isSelected = selectedMountpoint === d.mountpoint;
                    const isCurrentlyProvisioning = isProvisioning === d.mountpoint;

                    return (
                      <div
                        key={d.mountpoint}
                        className={`p-3 rounded-lg border transition ${
                          isSelected
                            ? "bg-blue-50 dark:bg-blue-950/30 border-[#1a73e8] shadow-xs"
                            : "bg-[#f8f9fa] dark:bg-[#131314] border-[#dadce0] dark:border-[#3c4043]"
                        }`}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <div className="flex items-center gap-2.5 min-w-0">
                            <div
                              className={`p-1.5 rounded-md ${
                                d.has_key
                                  ? "bg-emerald-100 dark:bg-emerald-950/60 text-emerald-700 dark:text-emerald-400"
                                  : "bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-300"
                              }`}
                            >
                              <KeyIcon className="w-4 h-4" />
                            </div>
                            <div className="min-w-0">
                              <div className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] truncate">
                                {d.label}
                              </div>
                              <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] truncate font-mono">
                                Mount: {d.mountpoint}
                              </div>
                            </div>
                          </div>

                          {/* Drive Key Status & Action Button */}
                          <div className="flex items-center gap-2 shrink-0">
                            {d.has_key ? (
                              <div className="flex items-center gap-1.5">
                                <span className="text-xs px-2 py-0.5 rounded font-medium bg-emerald-100 dark:bg-emerald-950/60 text-emerald-800 dark:text-emerald-300">
                                  Kunci Siap
                                </span>
                                <button
                                  type="button"
                                  onClick={() => handleSelectDriveKey(d)}
                                  className={`px-2.5 py-1 text-xs font-medium rounded-md border transition ${
                                    isSelected
                                      ? "bg-[#1a73e8] text-white border-[#1a73e8]"
                                      : "bg-white dark:bg-[#282a2c] text-[#1a73e8] dark:text-[#8ab4f8] border-[#dadce0] dark:border-[#3c4043] hover:border-[#1a73e8]"
                                  }`}
                                >
                                  {isSelected ? "Terpilih" : "Pilih Kunci"}
                                </button>
                              </div>
                            ) : (
                              <button
                                type="button"
                                onClick={() => handleProvisionDrive(d.mountpoint)}
                                disabled={isCurrentlyProvisioning}
                                className="px-3 py-1 bg-amber-600 hover:bg-amber-700 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center gap-1"
                              >
                                {isCurrentlyProvisioning ? (
                                  <>
                                    <RefreshIcon className="w-3 h-3 animate-spin" />
                                    <span>Menginisialisasi...</span>
                                  </>
                                ) : (
                                  <span>Inisialisasi Kunci Baru</span>
                                )}
                              </button>
                            )}
                          </div>
                        </div>

                        {/* Hash Preview if key exists */}
                        {d.has_key && d.token_hash_prefix && (
                          <div className="mt-2 pt-1.5 border-t border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between text-xs text-[#5f6368] dark:text-[#9aa0a6] font-mono">
                            <span>Hash SHA-256: {d.token_hash_prefix}</span>
                            <span className="text-emerald-600 dark:text-emerald-400 font-medium font-sans">
                              Hidden System File
                            </span>
                          </div>
                        )}
                      </div>
                    );
                  })
                )}
              </div>

              {/* Selected Key Summary */}
              {selectedHash && (
                <div className="p-3 bg-emerald-50 dark:bg-emerald-950/30 border border-emerald-200 dark:border-emerald-800 rounded-lg flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <CheckIcon className="w-4 h-4 text-emerald-600 dark:text-emerald-400 shrink-0" />
                    <span className="text-emerald-800 dark:text-emerald-300 font-medium">
                      Kunci Terpilih: {selectedMountpoint} (Hash: {selectedHash.substring(0, 16)}...)
                    </span>
                  </div>
                  <span className="text-emerald-700 dark:text-emerald-400 font-semibold uppercase">
                    Aktif
                  </span>
                </div>
              )}

              <div className="pt-2 flex justify-between border-t border-[#dadce0] dark:border-[#3c4043]">
                <button
                  type="button"
                  onClick={() => setStep(1)}
                  className="px-4 py-2 bg-[#f1f3f4] dark:bg-[#282a2c] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#e8eaed] dark:hover:bg-[#3c4043] text-xs font-medium rounded-md transition flex items-center gap-1.5"
                >
                  <ArrowLeftIcon className="w-4 h-4" />
                  <span>Kembali ke Kredensial</span>
                </button>
                <button
                  type="button"
                  onClick={handleProceedToStep3}
                  disabled={!selectedHash}
                  className="px-4 py-2 bg-[#1a73e8] hover:bg-[#1557b0] disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center gap-1.5"
                >
                  <span>Lanjut ke Biometrik Wajah</span>
                  <ArrowRightIcon className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}

          {/* =============================================================== */}
          {/* STEP 3: ENROLMENT BIOMETRIK WAJAH                               */}
          {/* =============================================================== */}
          {step === 3 && (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="flex items-center gap-3 p-3 bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043]">
                <BiometricReticleIllustration className="w-12 h-12 shrink-0" />
                <div>
                  <h4 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                    Faktor 3: Perekaman Biometrik Wajah (Inherence Factor)
                  </h4>
                  <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                    Posisikan wajah dokter di tengah kamera untuk pengambilan foto referensi biometrik.
                  </p>
                </div>
              </div>

              {/* Webcam Component */}
              <div>
                <WebcamCapture
                  onCapture={(file) => setFaceFile(file)}
                  onClear={() => setFaceFile(null)}
                  label="Kamera Referensi Biometrik Wajah"
                />
              </div>

              {/* Account Summary Confirmation Card */}
              <div className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-1.5 text-xs">
                <div className="font-semibold text-[#202124] dark:text-[#e8eaed] mb-1">
                  Ringkasan Akun Dokter:
                </div>
                <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                  <span>Username:</span>
                  <span className="font-semibold text-[#202124] dark:text-[#e8eaed]">
                    {username}
                  </span>
                </div>
                <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                  <span>Kunci USB Dongle:</span>
                  <span className="font-mono text-[#202124] dark:text-[#e8eaed]">
                    {selectedMountpoint} ({selectedHash.substring(0, 16)}...)
                  </span>
                </div>
                <div className="flex justify-between text-[#5f6368] dark:text-[#9aa0a6]">
                  <span>Foto Biometrik:</span>
                  <span className={faceFile ? "text-emerald-600 dark:text-emerald-400 font-semibold" : "text-amber-600 font-semibold"}>
                    {faceFile ? "Foto Wajah Siap Didaftarkan" : "Belum Mengambil Foto"}
                  </span>
                </div>
              </div>

              <div className="pt-2 flex justify-between border-t border-[#dadce0] dark:border-[#3c4043]">
                <button
                  type="button"
                  onClick={() => setStep(2)}
                  className="px-4 py-2 bg-[#f1f3f4] dark:bg-[#282a2c] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#e8eaed] dark:hover:bg-[#3c4043] text-xs font-medium rounded-md transition flex items-center gap-1.5"
                >
                  <ArrowLeftIcon className="w-4 h-4" />
                  <span>Kembali ke Kunci USB</span>
                </button>
                <button
                  type="submit"
                  disabled={!faceFile || isSubmitting}
                  className="px-5 py-2 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-xs transition flex items-center gap-1.5"
                >
                  <ShieldIcon className="w-3.5 h-3.5" />
                  <span>{isSubmitting ? "Mendaftarkan..." : "Selesaikan Pendaftaran Dokter"}</span>
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
