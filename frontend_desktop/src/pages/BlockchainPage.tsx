import { useEffect, useState } from "react";
import {
  auditApi,
  AuditBlockItem,
  IntegrityReport,
} from "../services/api";
import {
  AlertIcon,
  BlockchainIcon,
  BlockchainIllustration,
  CheckIcon,
  LockIcon,
  RefreshIcon,
  ShieldIcon,
} from "../components/Icons";
import { useVerbose } from "../components/VerboseContext";

export function BlockchainPage() {
  const { isVerbose } = useVerbose();
  const [blocks, setBlocks] = useState<AuditBlockItem[]>([]);
  const [integrity, setIntegrity] = useState<IntegrityReport | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isVerifying, setIsVerifying] = useState(false);
  const [selectedBlock, setSelectedBlock] = useState<AuditBlockItem | null>(null);

  const fetchLedger = async () => {
    setIsLoading(true);
    try {
      const res = await auditApi.getBlocks();
      setBlocks(res.blocks);
      setIntegrity(res.integrity_report);
      if (res.blocks.length > 0 && !selectedBlock) {
        setSelectedBlock(res.blocks[0]);
      }
    } catch {
      // Handled silently
    } finally {
      setIsLoading(false);
    }
  };

  const handleVerify = async () => {
    setIsVerifying(true);
    try {
      const res = await auditApi.verifyChain();
      setIntegrity(res.verification);
    } catch {
      // Handled silently
    } finally {
      setIsVerifying(false);
    }
  };

  useEffect(() => {
    fetchLedger();
  }, []);

  const getActionBadgeClass = (action: string) => {
    switch (action) {
      case "GENESIS":
        return "bg-purple-50 dark:bg-purple-950/30 text-purple-700 dark:text-purple-400 border border-purple-200 dark:border-purple-800";
      case "CREATE_RECORD":
        return "bg-emerald-50 dark:bg-emerald-950/30 text-emerald-700 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800";
      case "UPDATE_RECORD":
        return "bg-blue-50 dark:bg-blue-950/30 text-blue-700 dark:text-blue-400 border border-blue-200 dark:border-blue-800";
      case "UPLOAD_LAB":
        return "bg-amber-50 dark:bg-amber-950/30 text-amber-700 dark:text-amber-400 border border-amber-200 dark:border-amber-800";
      case "EMBED_STEGO":
        return "bg-teal-50 dark:bg-teal-950/30 text-teal-700 dark:text-teal-400 border border-teal-200 dark:border-teal-800";
      default:
        return "bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700";
    }
  };

  return (
    <div className="space-y-3.5">
      {/* Header Operational Bar */}
      <div className="p-3.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl flex flex-col md:flex-row items-start md:items-center justify-between gap-3 shadow-xs">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400 flex items-center justify-center border border-blue-100 dark:border-blue-900/50">
            <BlockchainIcon className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed]">
              Buku Log Audit Kepatuhan Akses (Audit Trail)
            </h2>
            <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5">
              Pencatatan riwayat transaksi dan jejak audit aktivitas rekam medis
            </p>
          </div>
        </div>

        {/* Verification Trigger Button & Status Badge */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={handleVerify}
            disabled={isVerifying}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-white text-xs font-semibold rounded-md shadow-xs transition"
          >
            <ShieldIcon className={`w-3.5 h-3.5 ${isVerifying ? "animate-spin" : ""}`} />
            <span>{isVerifying ? "Memverifikasi..." : "Verifikasi Rantai Integritas"}</span>
          </button>

          {integrity && (
            <div
              className={`text-xs px-2.5 py-1 rounded-md font-semibold border flex items-center gap-1.5 ${
                integrity.is_valid
                  ? "bg-emerald-50 dark:bg-emerald-950/30 text-emerald-800 dark:text-emerald-300 border-emerald-200 dark:border-emerald-800"
                  : "bg-red-50 dark:bg-rose-950/30 text-red-800 dark:text-rose-300 border-red-200 dark:border-rose-800"
              }`}
            >
              {integrity.is_valid ? (
                <>
                  <CheckIcon className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
                  <span>Rantai Valid ({integrity.total_blocks} Blok)</span>
                </>
              ) : (
                <>
                  <AlertIcon className="w-3.5 h-3.5 text-red-600 dark:text-rose-400" />
                  <span>Manipulasi Terdeteksi (Blok #{integrity.broken_at_index})</span>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Main Grid: Sequential Chain & Detailed Block Card */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-3.5">
        {/* Left: Sequential Blocks List (5 cols) */}
        <div className="lg:col-span-5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col">
          <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2">
              <BlockchainIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              <span>Rantai Blok Audit ({blocks.length})</span>
            </h3>
            <button
              onClick={fetchLedger}
              className="p-1 text-[#5f6368] dark:text-[#9aa0a6] hover:text-[#202124] dark:hover:text-white rounded hover:bg-slate-100 dark:hover:bg-[#282a2c] transition"
              title="Muat ulang ledger"
            >
              <RefreshIcon className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`} />
            </button>
          </div>

          <div className="space-y-2 max-h-[560px] overflow-y-auto pr-1">
            {blocks.map((b) => {
              const isSelected = selectedBlock?.block_index === b.block_index;
              return (
                <div
                  key={b.block_index}
                  onClick={() => setSelectedBlock(b)}
                  className={`p-2.5 rounded-lg border cursor-pointer transition ${
                    isSelected
                      ? "bg-blue-50 dark:bg-blue-950/30 border-blue-500 dark:border-blue-500 shadow-xs"
                      : "bg-[#f8f9fa] dark:bg-[#131314] border-[#dadce0] dark:border-[#3c4043] hover:border-slate-400 dark:hover:border-slate-500"
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs font-bold text-[#202124] dark:text-[#e8eaed] font-mono">
                      Blok #{b.block_index}
                    </span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded font-semibold ${getActionBadgeClass(
                        b.action_type
                      )}`}
                    >
                      {b.action_type}
                    </span>
                  </div>

                  <p className="text-xs text-[#5f6368] dark:text-[#9aa0a6] font-medium truncate">
                    {b.change_summary}
                  </p>

                  <div className="mt-1.5 pt-1.5 border-t border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between text-xs text-[#5f6368] dark:text-[#9aa0a6] font-mono">
                    <span>Aktor: {b.actor_username}</span>
                    <span className="text-blue-600 dark:text-blue-400">
                      Hash: {b.current_hash.substring(0, 10)}...
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Selected Block Deep Inspector (7 cols) */}
        <div className="lg:col-span-7 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3.5 space-y-3 shadow-xs flex flex-col">
          <div className="flex items-center justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-2">
            <h3 className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-2">
              <LockIcon className="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
              <span>Detail Segel Hash Blok</span>
            </h3>
            {selectedBlock && (
              <span className="text-xs font-mono font-bold text-blue-600 dark:text-blue-400">
                Index #{selectedBlock.block_index}
              </span>
            )}
          </div>

          {selectedBlock ? (
            <div className="space-y-3">
              {/* Block Metadata */}
              <div className="p-3 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-2 text-xs">
                <div className="flex justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
                  <span className="text-[#5f6368] dark:text-[#9aa0a6]">Tipe Aksi:</span>
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-semibold ${getActionBadgeClass(
                      selectedBlock.action_type
                    )}`}
                  >
                    {selectedBlock.action_type}
                  </span>
                </div>
                <div className="flex justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
                  <span className="text-[#5f6368] dark:text-[#9aa0a6]">Aktor:</span>
                  <span className="font-semibold text-[#202124] dark:text-[#e8eaed]">
                    {selectedBlock.actor_username}
                  </span>
                </div>
                <div className="flex justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
                  <span className="text-[#5f6368] dark:text-[#9aa0a6]">Waktu UTC:</span>
                  <span className="font-mono text-[#202124] dark:text-[#e8eaed]">
                    {selectedBlock.timestamp}
                  </span>
                </div>
                {selectedBlock.target_record_id !== null && (
                  <div className="flex justify-between border-b border-[#dadce0] dark:border-[#3c4043] pb-1.5">
                    <span className="text-[#5f6368] dark:text-[#9aa0a6]">ID Rekam Medis:</span>
                    <span className="text-blue-600 dark:text-blue-400 font-semibold font-mono">
                      #{selectedBlock.target_record_id}
                    </span>
                  </div>
                )}
                <div>
                  <span className="text-[#5f6368] dark:text-[#9aa0a6] block mb-1">
                    Ringkasan Perubahan:
                  </span>
                  <div className="p-2 bg-white dark:bg-[#1e1f20] rounded-md border border-[#dadce0] dark:border-[#3c4043] text-[#202124] dark:text-[#e8eaed] font-mono text-xs">
                    {selectedBlock.change_summary}
                  </div>
                </div>
              </div>

              {/* Hash Chain Links */}
              <div className="space-y-2">
                <div className="space-y-1">
                  <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6] flex justify-between">
                    <span>Previous Hash (Blok #{selectedBlock.block_index - 1}):</span>
                    {selectedBlock.block_index === 0 && (
                      <span className="text-purple-600 dark:text-purple-400 text-xs">
                        64 Karakter Nol (Genesis)
                      </span>
                    )}
                  </div>
                  <div className="p-2.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md font-mono text-xs text-slate-700 dark:text-amber-300 break-all select-all leading-tight">
                    {selectedBlock.previous_hash}
                  </div>
                </div>

                <div className="space-y-1">
                  <div className="text-xs font-semibold text-[#5f6368] dark:text-[#9aa0a6]">
                    Current Hash:
                  </div>
                  <div className="p-2.5 bg-[#f8f9fa] dark:bg-[#131314] border border-blue-200 dark:border-blue-900/60 rounded-md font-mono text-xs text-blue-600 dark:text-blue-400 break-all select-all leading-tight">
                    {selectedBlock.current_hash}
                  </div>
                </div>
              </div>

              {/* Formula Note (Visible in Technical Audit Mode) */}
              {isVerbose && (
                <div className="p-2.5 bg-[#f8f9fa] dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs space-y-1">
                  <p className="font-semibold text-blue-600 dark:text-blue-400">
                    Formula Hash Deterministik:
                  </p>
                  <p className="font-mono text-xs text-[#5f6368] dark:text-[#9aa0a6] break-all leading-relaxed">
                    SHA-256(canonical_json(block_index, timestamp, actor, action_type, target_record_id, change_summary, previous_hash))
                  </p>
                </div>
              )}
            </div>
          ) : (
            <div className="py-16 text-center text-xs text-[#5f6368] dark:text-[#9aa0a6] flex flex-col items-center">
              <BlockchainIllustration className="w-16 h-16 mb-2 opacity-60" />
              <span>Pilih blok di sebelah kiri untuk melihat detail hash.</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
