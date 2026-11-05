import { CryptoInspectionData } from "../services/api";
import { ArrowRightIcon, CheckIcon, EyeIcon, LockIcon, XIcon } from "./Icons";

interface CryptoInspectorModalProps {
  isOpen: boolean;
  onClose: () => void;
  data: CryptoInspectionData | null;
}

export function CryptoInspectorModal({ isOpen, onClose, data }: CryptoInspectorModalProps) {
  if (!isOpen || !data) return null;

  const diag = data.diagnosis_inspection;
  const presc = data.prescription_inspection;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60">
      <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl max-w-4xl w-full max-h-[90vh] flex flex-col shadow-xl overflow-hidden transition-colors">
        {/* Header */}
        <div className="p-3.5 border-b border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between bg-[#f8f9fa] dark:bg-[#18191a]">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded-lg bg-blue-50 dark:bg-blue-950/60 text-[#1a73e8] dark:text-[#8ab4f8]">
              <EyeIcon className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
                Audit Kriptografis Rekam Medis
              </h3>
              <div className="flex items-center gap-1.5 text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
                <span>Alur transformasi: Plaintext</span>
                <ArrowRightIcon className="w-3 h-3" />
                <span>Vigenere</span>
                <ArrowRightIcon className="w-3 h-3" />
                <span>AES-128-GCM</span>
              </div>
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

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3.5">
          {/* Section 1: Diagnosis */}
          <div className="bg-[#f8f9fa] dark:bg-[#18191a] border border-[#dadce0] dark:border-[#3c4043] rounded-md p-3 space-y-2.5">
            <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
              <h4 className="text-xs font-semibold text-[#1a73e8] dark:text-[#8ab4f8] flex items-center gap-1.5">
                <LockIcon className="w-3.5 h-3.5" />
                <span>Super-Enkripsi: Diagnosis Medis</span>
              </h4>
              <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-mono">
                Rekam Medis #{data.record_id}
              </span>
            </div>

            {/* Step 1: Plaintext */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider">
                1. Teks Diagnosis Asli (Plaintext)
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#202124] dark:text-[#e8eaed] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {diag.plaintext}
              </div>
            </div>

            {/* Step 2: Vigenere Intermediate */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider flex items-center justify-between">
                <span>2. Substitusi Vigenere Cipher (Kunci: HIPPOCRATESKEY)</span>
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#f9ab00] dark:text-[#fdd663] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {diag.step1_vigenere}
              </div>
            </div>

            {/* Step 3: AES-128-GCM Base64 */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider flex items-center justify-between">
                <span>3. AES-128-GCM Base64 (Tersimpan di SQLite DB)</span>
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#1a73e8] dark:text-[#8ab4f8] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {diag.step2_aes128_b64}
              </div>
            </div>

            {/* Step 4: Verification */}
            <div className="p-2.5 rounded bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 flex items-center justify-between text-xs text-emerald-800 dark:text-emerald-300">
              <span className="flex items-center gap-1.5 font-semibold">
                <CheckIcon className="w-3.5 h-3.5 text-[#1e8e3e] dark:text-[#81c995]" />
                <span>Round-Trip Valid: Plaintext Terdekripsi Sempurna</span>
              </span>
              <span className="font-mono text-xs">{diag.verified_roundtrip}</span>
            </div>
          </div>

          {/* Section 2: Prescription */}
          <div className="bg-[#f8f9fa] dark:bg-[#18191a] border border-[#dadce0] dark:border-[#3c4043] rounded-md p-3 space-y-2.5">
            <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
              <h4 className="text-xs font-semibold text-[#1a73e8] dark:text-[#8ab4f8] flex items-center gap-1.5">
                <LockIcon className="w-3.5 h-3.5" />
                <span>Super-Enkripsi: Resep Terapi Obat</span>
              </h4>
              <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-mono">
                Rekam Medis #{data.record_id}
              </span>
            </div>

            {/* Step 1: Plaintext */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider">
                1. Teks Resep Asli (Plaintext)
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#202124] dark:text-[#e8eaed] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {presc.plaintext}
              </div>
            </div>

            {/* Step 2: Vigenere Intermediate */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider">
                2. Substitusi Vigenere Cipher
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#f9ab00] dark:text-[#fdd663] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {presc.step1_vigenere}
              </div>
            </div>

            {/* Step 3: AES-128-GCM Base64 */}
            <div className="space-y-1">
              <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] uppercase tracking-wider">
                3. AES-128-GCM Base64
              </div>
              <div className="p-2.5 bg-white dark:bg-[#1e1f20] rounded text-xs font-mono text-[#1a73e8] dark:text-[#8ab4f8] border border-[#dadce0] dark:border-[#3c4043] break-words">
                {presc.step2_aes128_b64}
              </div>
            </div>

            {/* Step 4: Verification */}
            <div className="p-2.5 rounded bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 flex items-center justify-between text-xs text-emerald-800 dark:text-emerald-300">
              <span className="flex items-center gap-1.5 font-semibold">
                <CheckIcon className="w-3.5 h-3.5 text-[#1e8e3e] dark:text-[#81c995]" />
                <span>Round-Trip Valid: Plaintext Terdekripsi Sempurna</span>
              </span>
              <span className="font-mono text-xs">{presc.verified_roundtrip}</span>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="p-3 border-t border-[#dadce0] dark:border-[#3c4043] flex justify-end bg-[#f8f9fa] dark:bg-[#18191a]">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-1.5 bg-[#1a73e8] hover:bg-[#1557b0] text-white text-xs font-semibold rounded-md shadow-xs transition"
          >
            Tutup Inspeksi
          </button>
        </div>
      </div>
    </div>
  );
}
