import { useEffect, useState } from "react";
import {
  CryptoInspectionData,
  MedicalRecordItem,
  patientApi,
  PatientItem,
  recordApi,
  UserSession,
} from "../services/api";
import {
  AlertIcon,
  CheckIcon,
  EmptyStateIllustration,
  EyeIcon,
  LockIcon,
  PlusIcon,
  RefreshIcon,
  UsersIcon,
} from "../components/Icons";
import { CryptoInspectorModal } from "../components/CryptoInspectorModal";
import { useVerbose } from "../components/VerboseContext";

interface PatientsPageProps {
  currentUser: UserSession | null;
}

export function PatientsPage({ currentUser }: PatientsPageProps) {
  const { isVerbose } = useVerbose();
  const [patients, setPatients] = useState<PatientItem[]>([]);
  const [selectedPatient, setSelectedPatient] = useState<PatientItem | null>(null);
  const [records, setRecords] = useState<MedicalRecordItem[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // Add Patient Form State
  const [isAddingPatient, setIsAddingPatient] = useState(false);
  const [mrn, setMrn] = useState("");
  const [nik, setNik] = useState("");
  const [fullName, setFullName] = useState("");
  const [address, setAddress] = useState("");
  const [birthDate, setBirthDate] = useState("1990-01-01");

  // Add Record Form State
  const [isAddingRecord, setIsAddingRecord] = useState(false);
  const [diagnosis, setDiagnosis] = useState("");
  const [prescription, setPrescription] = useState("");
  const [visitStatus, setVisitStatus] = useState("OPEN");

  // Inspection Modal State
  const [inspectionData, setInspectionData] = useState<CryptoInspectionData | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const [feedback, setFeedback] = useState<{ type: "success" | "error"; msg: string } | null>(
    null
  );

  const fetchPatients = async () => {
    setIsLoading(true);
    try {
      const res = await patientApi.listPatients();
      setPatients(res.patients);
      if (res.patients.length > 0 && !selectedPatient) {
        setSelectedPatient(res.patients[0]);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    } finally {
      setIsLoading(false);
    }
  };

  const fetchRecords = async (patientId: number) => {
    try {
      const res = await recordApi.listByPatient(patientId);
      setRecords(res.records);
    } catch {
      setRecords([]);
    }
  };

  useEffect(() => {
    fetchPatients();
  }, []);

  useEffect(() => {
    if (selectedPatient) {
      fetchRecords(selectedPatient.id);
    }
  }, [selectedPatient]);

  const handleCreatePatient = async (e: React.FormEvent) => {
    e.preventDefault();
    setFeedback(null);
    try {
      await patientApi.createPatient({
        medical_record_number: mrn.trim(),
        nik: nik.trim(),
        full_name: fullName.trim(),
        address: address.trim(),
        birth_date: birthDate,
      });
      setFeedback({
        type: "success",
        msg: "Pasien baru berhasil dienkripsi AES-128 dan disimpan ke database.",
      });
      setIsAddingPatient(false);
      setMrn("");
      setNik("");
      setFullName("");
      setAddress("");
      fetchPatients();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    }
  };

  const handleCreateRecord = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedPatient || !currentUser) return;
    setFeedback(null);
    try {
      await recordApi.createRecord({
        patient_id: selectedPatient.id,
        doctor_id: currentUser.id,
        actor_username: currentUser.username,
        diagnosis: diagnosis.trim(),
        prescription: prescription.trim(),
        visit_status: visitStatus,
      });
      setFeedback({
        type: "success",
        msg: "Rekam medis berhasil disuper-enkripsi (Vigenere + AES-128) dan tercatat di blockchain.",
      });
      setIsAddingRecord(false);
      setDiagnosis("");
      setPrescription("");
      fetchRecords(selectedPatient.id);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    }
  };

  const handleInspect = async (recordId: number) => {
    try {
      const res = await recordApi.inspectRecord(recordId);
      setInspectionData(res.inspection);
      setIsModalOpen(true);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setFeedback({ type: "error", msg });
    }
  };

  return (
    <div className="space-y-3.5">
      {/* Header Bar - Clean Enterprise Surface */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-4 py-3 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl shadow-xs">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400 flex items-center justify-center border border-blue-100 dark:border-blue-900/50">
            <UsersIcon className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
              Registrasi Pasien & Rekam Medis Elektronik (RME)
            </h2>
            <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
              Pencatatan klinis dan riwayat konsultasi pasien rawat jalan
            </p>
          </div>
        </div>

        {/* Technical Audit Badge shown in Verbose Mode */}
        {isVerbose && (
          <div className="flex items-center gap-2 bg-blue-50 dark:bg-blue-950/40 px-3 py-1.5 rounded-lg border border-blue-200 dark:border-blue-800 text-xs text-blue-700 dark:text-blue-300">
            <span className="w-1.5 h-1.5 rounded-full bg-blue-600 dark:bg-blue-400" />
            <span className="font-semibold">Inspeksi Enkripsi Kolom SQLite Aktif</span>
          </div>
        )}
      </div>

      {feedback && (
        <div
          className={`p-2.5 rounded-lg text-xs flex items-center gap-2 border ${
            feedback.type === "success"
              ? "bg-emerald-50 dark:bg-emerald-950/30 border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-300"
              : "bg-red-50 dark:bg-rose-950/30 border-red-200 dark:border-rose-800 text-red-800 dark:text-rose-300"
          }`}
        >
          {feedback.type === "success" ? (
            <CheckIcon className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : (
            <AlertIcon className="w-3.5 h-3.5 text-red-600 dark:text-rose-400 shrink-0" />
          )}
          <span>{feedback.msg}</span>
        </div>
      )}

      {/* Main Split Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-3.5">
        {/* Left: Patient List (5 cols) */}
        <div className="lg:col-span-5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col">
          <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
              Daftar Pasien ({patients.length})
            </h3>
            <div className="flex items-center gap-1.5">
              <button
                onClick={fetchPatients}
                className="p-1 rounded text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#282a2c] transition"
                title="Segarkan data"
              >
                <RefreshIcon className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`} />
              </button>
              <button
                onClick={() => setIsAddingPatient(!isAddingPatient)}
                className="flex items-center gap-1 px-2.5 py-1.5 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-white text-xs font-semibold rounded-md transition shadow-xs"
              >
                <PlusIcon className="w-3 h-3" />
                <span>Tambah Pasien</span>
              </button>
            </div>
          </div>

          {/* Add Patient Form */}
          {isAddingPatient && (
            <form
              onSubmit={handleCreatePatient}
              className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-2.5"
            >
              <div className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                Pendaftaran Pasien Baru
              </div>
              <input
                type="text"
                placeholder="No. Rekam Medis (RM-2026-0005)"
                value={mrn}
                onChange={(e) => setMrn(e.target.value)}
                className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500"
                required
              />
              <input
                type="text"
                placeholder="NIK 16 Digit"
                value={nik}
                onChange={(e) => setNik(e.target.value)}
                className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500"
                required
              />
              <input
                type="text"
                placeholder="Nama Lengkap Pasien"
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500"
                required
              />
              <textarea
                placeholder="Alamat Domisili"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500 h-14 resize-none"
                required
              />
              <div>
                <label className="text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] block mb-1">
                  Tanggal Lahir
                </label>
                <input
                  type="date"
                  value={birthDate}
                  onChange={(e) => setBirthDate(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] focus:outline-none focus:border-blue-500"
                  required
                />
              </div>
              <div className="flex justify-end gap-1.5 pt-1">
                <button
                  type="button"
                  onClick={() => setIsAddingPatient(false)}
                  className="px-2.5 py-1.5 border border-[#dadce0] dark:border-[#3c4043] text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] hover:bg-slate-100 dark:hover:bg-[#282a2c] rounded-md transition"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-xs font-semibold text-white rounded-md transition"
                >
                  Simpan Pasien
                </button>
              </div>
            </form>
          )}

          {/* Patient Card Items */}
          <div className="space-y-2 max-h-[520px] overflow-y-auto pr-1">
            {patients.length === 0 ? (
              <div className="py-10 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] flex flex-col items-center">
                <EmptyStateIllustration className="w-16 h-16 mb-2 opacity-50" />
                <span>Belum ada data pasien terdaftar.</span>
              </div>
            ) : (
              patients.map((p) => {
                const isSelected = selectedPatient?.id === p.id;
                return (
                  <div
                    key={p.id}
                    onClick={() => setSelectedPatient(p)}
                    className={`p-2.5 rounded-lg border cursor-pointer transition ${
                      isSelected
                        ? "bg-blue-50 dark:bg-blue-950/30 border-blue-500 dark:border-blue-500 shadow-xs"
                        : "bg-[#f8f9fa] dark:bg-[#131314] border-[#dadce0] dark:border-[#3c4043] hover:border-slate-400 dark:hover:border-slate-500"
                    }`}
                  >
                    <div className="flex items-center justify-between mb-0.5">
                      <span className="text-xs font-bold text-blue-600 dark:text-blue-400 font-mono">
                        {p.medical_record_number}
                      </span>
                      <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                        {p.birth_date}
                      </span>
                    </div>

                    <div className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
                      {p.decrypted.full_name}
                    </div>
                    <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
                      NIK: {p.decrypted.nik}
                    </div>

                    {/* Mode Inspeksi: Raw Ciphertext Preview (Mode Teknis) */}
                    {isVerbose && (
                      <div className="mt-2 pt-1.5 border-t border-[#dadce0] dark:border-[#3c4043] text-xs space-y-1">
                        <div
                          className="text-slate-600 dark:text-slate-400 font-mono truncate bg-white dark:bg-[#1e1f20] px-1.5 py-0.5 rounded border border-[#dadce0] dark:border-[#3c4043]"
                          title={p.raw_sqlite_ciphertext.nik_encrypted}
                        >
                          <span className="font-semibold text-blue-600 dark:text-blue-400">NIK:</span>{" "}
                          {p.raw_sqlite_ciphertext.nik_encrypted}
                        </div>
                        <div
                          className="text-slate-600 dark:text-slate-400 font-mono truncate bg-white dark:bg-[#1e1f20] px-1.5 py-0.5 rounded border border-[#dadce0] dark:border-[#3c4043]"
                          title={p.raw_sqlite_ciphertext.full_name_encrypted}
                        >
                          <span className="font-semibold text-blue-600 dark:text-blue-400">Nama:</span>{" "}
                          {p.raw_sqlite_ciphertext.full_name_encrypted}
                        </div>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Right: Selected Patient Records (7 cols) */}
        <div className="lg:col-span-7 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col">
          {selectedPatient ? (
            <>
              {/* Patient Banner */}
              <div className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg flex items-center justify-between gap-3">
                <div>
                  <span className="text-xs text-blue-600 dark:text-blue-400 font-bold uppercase tracking-wider">
                    Pasien Terpilih
                  </span>
                  <h3 className="text-base font-semibold text-[#202124] dark:text-[#e8eaed]">
                    {selectedPatient.decrypted.full_name}
                  </h3>
                  <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                    No. RM: {selectedPatient.medical_record_number} &bull; Alamat: {selectedPatient.decrypted.address}
                  </p>
                </div>
                <button
                  onClick={() => setIsAddingRecord(!isAddingRecord)}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-white text-xs font-semibold rounded-md transition shadow-xs shrink-0"
                >
                  <PlusIcon className="w-3.5 h-3.5" />
                  <span>Tambah Rekam Medis</span>
                </button>
              </div>

              {/* Add Record Form */}
              {isAddingRecord && (
                <form
                  onSubmit={handleCreateRecord}
                  className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-2.5"
                >
                  <h4 className="text-xs font-bold text-[#202124] dark:text-[#e8eaed] flex items-center gap-1.5">
                    <LockIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
                    <span>Catat Catatan Medis (CPPT)</span>
                  </h4>
                  <div>
                    <label className="text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] block mb-1">
                      Diagnosis Medis
                    </label>
                    <textarea
                      value={diagnosis}
                      onChange={(e) => setDiagnosis(e.target.value)}
                      placeholder="Contoh: Hipertensi Primer Derajat II - TD 150/95 mmHg"
                      className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500 h-16 resize-none"
                      required
                    />
                  </div>
                  <div>
                    <label className="text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] block mb-1">
                      Resep Obat & Terapi
                    </label>
                    <textarea
                      value={prescription}
                      onChange={(e) => setPrescription(e.target.value)}
                      placeholder="Contoh: Amlodipine 1x10mg pagi, Captopril 2x25mg jika perlu"
                      className="w-full px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs text-[#202124] dark:text-[#e8eaed] placeholder-slate-400 focus:outline-none focus:border-blue-500 h-14 resize-none"
                      required
                    />
                  </div>
                  <div className="flex items-center justify-between pt-1">
                    <select
                      value={visitStatus}
                      onChange={(e) => setVisitStatus(e.target.value)}
                      className="px-2.5 py-1.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs font-medium text-[#202124] dark:text-[#e8eaed] focus:outline-none"
                    >
                      <option value="OPEN">Status: RAWAT JALAN (OPEN)</option>
                      <option value="UNDER_TREATMENT">Status: DALAM TINDAKAN</option>
                      <option value="CLOSED">Status: SELESAI (CLOSED)</option>
                    </select>
                    <div className="flex gap-1.5">
                      <button
                        type="button"
                        onClick={() => setIsAddingRecord(false)}
                        className="px-2.5 py-1.5 border border-[#dadce0] dark:border-[#3c4043] text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] hover:bg-slate-100 dark:hover:bg-[#282a2c] rounded-md transition"
                      >
                        Batal
                      </button>
                      <button
                        type="submit"
                        className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-xs font-semibold text-white rounded-md transition shadow-xs"
                      >
                        Simpan Rekam Medis
                      </button>
                    </div>
                  </div>
                </form>
              )}

              {/* Records List */}
              <div className="space-y-2">
                <div className="text-xs font-bold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider">
                  Catatan Perkembangan Pasien Terintegrasi (CPPT) ({records.length})
                </div>

                {records.length === 0 ? (
                  <div className="py-10 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] bg-[#f8f9fa] dark:bg-[#131314] rounded-lg border border-[#dadce0] dark:border-[#3c4043] flex flex-col items-center">
                    <EmptyStateIllustration className="w-14 h-14 mb-2 opacity-40" />
                    <span>Belum ada rekam medis untuk pasien ini.</span>
                  </div>
                ) : (
                  records.map((rec) => (
                    <div
                      key={rec.id}
                      className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-2"
                    >
                      <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
                        <span className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed]">
                          Catatan Medis #{rec.id} &bull; Status: {rec.visit_status}
                        </span>
                        <button
                          onClick={() => handleInspect(rec.id)}
                          className="flex items-center gap-1.5 px-2.5 py-1 bg-white dark:bg-[#282a2c] border border-[#dadce0] dark:border-[#3c4043] hover:border-blue-500 text-blue-600 dark:text-blue-400 text-xs font-medium rounded-md transition shadow-xs"
                        >
                          <EyeIcon className="w-3.5 h-3.5" />
                          <span>Audit Kriptografis</span>
                        </button>
                      </div>

                      <div className="space-y-1">
                        <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-medium">Diagnosis Klinis:</div>
                        <div className="text-xs text-[#202124] dark:text-[#e8eaed] bg-white dark:bg-[#1e1f20] p-2 rounded-md border border-[#dadce0] dark:border-[#3c4043]">
                          {rec.diagnosis}
                        </div>
                      </div>

                      <div className="space-y-1">
                        <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-medium">Resep & Terapi:</div>
                        <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] bg-white dark:bg-[#1e1f20] p-2 rounded-md border border-[#dadce0] dark:border-[#3c4043]">
                          {rec.prescription}
                        </div>
                      </div>

                      <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6] pt-1 flex justify-between border-t border-[#dadce0] dark:border-[#3c4043]">
                        <span>Waktu Pencatatan: {rec.created_at}</span>
                        {rec.lab_original_filename && (
                          <span className="text-emerald-700 dark:text-emerald-400 font-medium">
                            Berkas Lab: {rec.lab_original_filename}
                          </span>
                        )}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </>
          ) : (
            <div className="py-16 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] flex flex-col items-center">
              <EmptyStateIllustration className="w-16 h-16 mb-2 opacity-50" />
              <span>Pilih pasien di sebelah kiri untuk melihat rekam medis.</span>
            </div>
          )}
        </div>
      </div>

      {/* Crypto Inspector Modal */}
      <CryptoInspectorModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        data={inspectionData}
      />
    </div>
  );
}