import { useEffect, useState } from "react";
import {
  authApi,
  UserSession,
} from "./services/api";
import {
  BlockchainIcon,
  FileLockIcon,
  ImageIcon,
  MedVaultLogoIllustration,
  MoonIcon,
  SunIcon,
  TerminalIcon,
  UsersIcon,
} from "./components/Icons";
import { useTheme } from "./components/ThemeContext";
import { useVerbose } from "./components/VerboseContext";
import { AuthPage } from "./pages/AuthPage";
import { PatientsPage } from "./pages/PatientsPage";
import { VaultPage } from "./pages/VaultPage";
import { StegoPage } from "./pages/StegoPage";
import { BlockchainPage } from "./pages/BlockchainPage";

export default function App() {
  const [activeTab, setActiveTab] = useState<number>(1);
  const [currentUser, setCurrentUser] = useState<UserSession | null>(null);
  const [backendOnline, setBackendOnline] = useState<boolean | null>(null);
  const [ejectionNotice, setEjectionNotice] = useState<string | null>(null);
  const { theme, toggleTheme } = useTheme();
  const { isVerbose, toggleVerbose } = useVerbose();

  // Restore session from localStorage if present
  useEffect(() => {
    const token = localStorage.getItem("medvault_token");
    const userStr = localStorage.getItem("medvault_user");
    if (token && userStr) {
      try {
        const parsed = JSON.parse(userStr);
        setCurrentUser(parsed);
        setActiveTab(1);
      } catch {
        localStorage.removeItem("medvault_token");
        localStorage.removeItem("medvault_user");
      }
    }

    const handleUnauthorized = (e: Event) => {
      const customEvent = e as CustomEvent<{ reason?: string }>;
      const reason =
        customEvent.detail?.reason ||
        "Sesi Terkunci: Kunci fisik USB Dongle telah dicabut atau sesi telah berakhir.";
      setCurrentUser(null);
      setEjectionNotice(reason);
    };

    window.addEventListener("medvault:unauthorized", handleUnauthorized as EventListener);
    return () =>
      window.removeEventListener("medvault:unauthorized", handleUnauthorized as EventListener);
  }, []);

  // Check backend health periodically
  useEffect(() => {
    const check = async () => {
      try {
        const res = await authApi.checkHealth();
        setBackendOnline(res.database_connected === true);
      } catch {
        setBackendOnline(false);
      }
    };
    check();
    const interval = setInterval(check, 6000);
    return () => clearInterval(interval);
  }, []);

  // Continuous Hardware Dongle Watchdog while logged in
  useEffect(() => {
    if (!currentUser) return;

    const watchdog = async () => {
      try {
        const status = await authApi.getDongleStatus();
        if (!status.detected) {
          console.warn("[MedVault Watchdog] USB Dongle fisik tidak terdeteksi!");
          localStorage.removeItem("medvault_token");
          localStorage.removeItem("medvault_user");
          setCurrentUser(null);
          setEjectionNotice(
            "Sesi Terkunci: Kunci fisik USB Dongle telah dicabut dari perangkat. Silakan hubungkan kembali USB Dongle dan login ulang."
          );
          return;
        }

        const expectedPrefix = currentUser.dongle_hash_prefix;
        if (
          expectedPrefix &&
          status.token_hash &&
          !status.token_hash.toLowerCase().startsWith(expectedPrefix.toLowerCase())
        ) {
          console.warn("[MedVault Watchdog] USB Dongle yang terpasang tidak cocok!");
          localStorage.removeItem("medvault_token");
          localStorage.removeItem("medvault_user");
          setCurrentUser(null);
          setEjectionNotice(
            "Sesi Terkunci: Kunci fisik USB Dongle yang terpasang tidak cocok dengan akun dokter aktif."
          );
        }
      } catch {
        // Backend momentarily unavailable, watchdog will retry next cycle
      }
    };

    const interval = setInterval(watchdog, 2500);
    return () => clearInterval(interval);
  }, [currentUser]);

  const handleLoginSuccess = (user: UserSession) => {
    setCurrentUser(user);
    setEjectionNotice(null);
    localStorage.setItem("medvault_user", JSON.stringify(user));
    setActiveTab(1);
  };

  const handleLogout = () => {
    localStorage.removeItem("medvault_token");
    localStorage.removeItem("medvault_user");
    setCurrentUser(null);
    setEjectionNotice(null);
    setActiveTab(1);
  };

  const navItems = [
    { id: 1, label: "Rekam Medis Pasien", icon: UsersIcon },
    { id: 2, label: "Berkas Laboratorium", icon: FileLockIcon },
    { id: 3, label: "Citra Radiologi", icon: ImageIcon },
    { id: 4, label: "Buku Log Audit Trail", icon: BlockchainIcon },
  ];

  // =========================================================================
  // VIEW 1: DEDICATED INITIAL LOGIN SCREEN (No Sidebar, Pure Focus)
  // =========================================================================
  if (!currentUser) {
    return (
      <div className="min-h-screen bg-[#f8f9fa] dark:bg-[#131314] text-[#202124] dark:text-[#e8eaed] flex flex-col justify-between p-4 transition-colors duration-150 select-none">
        {/* Top Minimal Utility Header */}
        <header className="max-w-4xl w-full mx-auto flex items-center justify-between pb-3 border-b border-[#dadce0] dark:border-[#3c4043]">
          <div className="flex items-center gap-2.5">
            <MedVaultLogoIllustration className="w-8 h-8" />
            <div>
              <h1 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed] tracking-tight leading-none">
                RS MedVault Healthcare
              </h1>
              <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5 block font-medium">
                Sistem Rekam Medis Elektronik (RME)
              </span>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Backend Connectivity Status */}
            <div className="flex items-center gap-1.5 text-xs text-[#5f6368] dark:text-[#9aa0a6]">
              {backendOnline === true ? (
                <span className="text-[#1e8e3e] dark:text-[#81c995] font-medium flex items-center gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#1e8e3e] dark:bg-[#81c995]" />
                  <span>FastAPI Terhubung</span>
                </span>
              ) : backendOnline === false ? (
                <span className="text-[#d93025] dark:text-[#f28b82] font-medium flex items-center gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#d93025] dark:bg-[#f28b82]" />
                  <span>FastAPI Terputus</span>
                </span>
              ) : (
                <span>Memeriksa server...</span>
              )}
            </div>

            {/* Verbose / Technical Mode Toggle Button */}
            <button
              type="button"
              onClick={toggleVerbose}
              title={
                isVerbose
                  ? "Mode Teknis Aktif (Klik untuk Mode Medis Sederhana)"
                  : "Mode Medis Aktif (Klik untuk Mode Teknis & Audit Kriptografi)"
              }
              className={`p-1.5 rounded-md border text-xs font-semibold flex items-center gap-1.5 transition ${
                isVerbose
                  ? "bg-blue-50 dark:bg-blue-950/60 border-blue-400 dark:border-blue-600 text-blue-700 dark:text-blue-300"
                  : "border-[#dadce0] dark:border-[#3c4043] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed]"
              }`}
            >
              <TerminalIcon className="w-3.5 h-3.5" />
              <span>{isVerbose ? "Mode Teknis" : "Teknis"}</span>
            </button>

            {/* Theme Toggle Button */}
            <button
              type="button"
              onClick={toggleTheme}
              title={theme === "dark" ? "Ganti ke Mode Terang" : "Ganti ke Mode Gelap"}
              className="p-1.5 rounded-md border border-[#dadce0] dark:border-[#3c4043] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed] transition"
            >
              {theme === "dark" ? <SunIcon className="w-3.5 h-3.5" /> : <MoonIcon className="w-3.5 h-3.5" />}
            </button>
          </div>
        </header>

        {/* Dedicated Centered Auth Form */}
        <main className="flex-1 flex items-center justify-center py-6">
          <AuthPage
            onLoginSuccess={handleLoginSuccess}
            ejectionNotice={ejectionNotice}
            onClearEjectionNotice={() => setEjectionNotice(null)}
          />
        </main>

        {/* Minimal Footer */}
        <footer className="text-center text-xs text-[#80868b] dark:text-[#5f6368] pt-2 border-t border-[#dadce0] dark:border-[#3c4043]">
          RS MedVault Healthcare - Sistem Rekam Medis Elektronik Terintegrasi
        </footer>
      </div>
    );
  }

  // =========================================================================
  // VIEW 2: AUTHENTICATED CLINICAL WORKSPACE DASHBOARD (Full Sidebar & Content)
  // =========================================================================
  return (
    <div className="flex h-screen bg-[#f8f9fa] dark:bg-[#131314] text-[#202124] dark:text-[#e8eaed] overflow-hidden select-none transition-colors duration-150">
      {/* Sidebar Navigation */}
      <aside className="w-60 bg-white dark:bg-[#1e1f20] border-r border-[#dadce0] dark:border-[#3c4043] flex flex-col justify-between flex-shrink-0 transition-colors duration-150">
        <div>
          {/* Brand Header */}
          <div className="px-3.5 py-3 border-b border-[#dadce0] dark:border-[#3c4043] flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <MedVaultLogoIllustration className="w-8 h-8" />
              <div>
                <h1 className="text-sm font-semibold text-[#202124] dark:text-[#e8eaed] tracking-tight leading-none">
                  RS MedVault
                </h1>
                <span className="text-xs text-[#5f6368] dark:text-[#9aa0a6] mt-0.5 block font-medium">
                  Instalasi Rawat Jalan & Lab
                </span>
              </div>
            </div>

            <div className="flex items-center gap-1.5">
              {/* Verbose / Technical Mode Toggle Button */}
              <button
                type="button"
                onClick={toggleVerbose}
                title={
                  isVerbose
                    ? "Mode Teknis Aktif (Klik untuk Mode Medis Sederhana)"
                    : "Mode Medis Aktif (Klik untuk Mode Teknis/Audit)"
                }
                className={`p-1 rounded-md border text-xs font-semibold transition ${
                  isVerbose
                    ? "bg-blue-50 dark:bg-blue-950/60 border-blue-400 dark:border-blue-600 text-blue-700 dark:text-blue-300"
                    : "border-[#dadce0] dark:border-[#3c4043] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed]"
                }`}
              >
                <TerminalIcon className="w-3.5 h-3.5" />
              </button>

              {/* Theme Switcher Button */}
              <button
                type="button"
                onClick={toggleTheme}
                title={theme === "dark" ? "Ganti ke Mode Terang" : "Ganti ke Mode Gelap"}
                className="p-1 rounded-md border border-[#dadce0] dark:border-[#3c4043] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed] transition"
              >
                {theme === "dark" ? <SunIcon className="w-3.5 h-3.5" /> : <MoonIcon className="w-3.5 h-3.5" />}
              </button>
            </div>
          </div>

          {/* Clinical Navigation Items */}
          <nav className="p-2 space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeTab === item.id;

              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-xs font-medium transition ${
                    isActive
                      ? "bg-[#e8f0fe] dark:bg-[#1a73e8]/20 text-[#1a73e8] dark:text-[#8ab4f8] font-semibold"
                      : "text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] hover:text-[#202124] dark:hover:text-[#e8eaed]"
                  }`}
                >
                  <Icon className="w-4 h-4 shrink-0" />
                  <span className="truncate">{item.label}</span>
                </button>
              );
            })}
          </nav>
        </div>

        {/* Footer: Doctor Session Info & Logout Button */}
        <div className="p-3 border-t border-[#dadce0] dark:border-[#3c4043] bg-[#f8f9fa] dark:bg-[#18191a] space-y-2">
          <div className="p-2.5 bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-lg space-y-1.5 shadow-xs">
            <div className="flex items-center justify-between">
              <div className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] truncate">
                Dr. {currentUser.username}
              </div>
              <span className="text-xs px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 font-semibold uppercase">
                {currentUser.role}
              </span>
            </div>
            <div className="text-xs text-[#5f6368] dark:text-[#9aa0a6]">
              Stasiun Kerja: WS-01
            </div>
            <button
              type="button"
              onClick={handleLogout}
              className="w-full py-1.5 bg-[#f1f3f4] dark:bg-[#282a2c] hover:bg-red-50 dark:hover:bg-red-950/40 hover:text-red-600 dark:hover:text-red-400 border border-[#dadce0] dark:border-[#3c4043] rounded-md text-xs font-medium text-[#5f6368] dark:text-[#9aa0a6] transition"
            >
              Keluar Sesi
            </button>
          </div>

          <div className="flex items-center justify-between text-xs text-[#5f6368] dark:text-[#9aa0a6] pt-1 border-t border-[#dadce0] dark:border-[#3c4043]">
            <span>FastAPI Server</span>
            {backendOnline === true ? (
              <span className="text-[#1e8e3e] dark:text-[#81c995] font-medium flex items-center gap-1">
                <span className="w-1.5 h-1.5 rounded-full bg-[#1e8e3e] dark:bg-[#81c995]" />
                <span>Terhubung</span>
              </span>
            ) : backendOnline === false ? (
              <span className="text-[#d93025] dark:text-[#f28b82] font-medium flex items-center gap-1">
                <span className="w-1.5 h-1.5 rounded-full bg-[#d93025] dark:bg-[#f28b82]" />
                <span>Terputus</span>
              </span>
            ) : (
              <span>Memeriksa...</span>
            )}
          </div>
        </div>
      </aside>

      {/* Main Content Viewport */}
      <main className="flex-1 overflow-y-auto bg-[#f8f9fa] dark:bg-[#131314] p-3.5 lg:p-4">
        <div className="max-w-7xl mx-auto">
          {activeTab === 1 && <PatientsPage currentUser={currentUser} />}
          {activeTab === 2 && <VaultPage currentUser={currentUser} />}
          {activeTab === 3 && <StegoPage currentUser={currentUser} />}
          {activeTab === 4 && <BlockchainPage />}
        </div>
      </main>
    </div>
  );
}
