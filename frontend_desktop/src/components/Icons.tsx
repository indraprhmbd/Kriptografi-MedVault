/**
 * Custom SVG Vector Icons for MedVault Desktop.
 *
 * Handcrafted, solid/filled zero-dependency SVG components replacing external icon libraries.
 */

interface IconProps {
  className?: string;
  size?: number;
}

export function ShieldIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 2L4 5v6.09c0 5.05 3.41 9.76 8 10.91 4.59-1.15 8-5.86 8-10.91V5l-8-3z" />
    </svg>
  );
}

export function UsersIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" />
    </svg>
  );
}

export function FileLockIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm2 14c0 1.1-.9 2-2 2h-4c-1.1 0-2-.9-2-2v-2c0-1.1.9-2 2-2v-1c0-1.1.9-2 2-2s2 .9 2 2v1h.5c.83 0 1.5.67 1.5 1.5V16zm-5-3v-1c0-.55.45-1 1-1s1 .45 1 1v1h-2zm3-7V3.5L18.5 8H14z" />
    </svg>
  );
}

export function ImageIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 9.5c.83 0 1.5-.67 1.5-1.5S9.33 6.5 8.5 6.5 7 7.17 7 8s.67 1.5 1.5 1.5zM5 18l4.5-6 3.5 4.5 2.5-3.01L19 18H5z" />
    </svg>
  );
}

export function BlockchainIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M3 3h7v7H3V3zm11 0h7v7h-7V3zm-11 11h7v7H3v-7zm11 0h7v7h-7v-7zm-3-8h2v2h-2V6zm-8 8h2v2H6v-2zm11 0h2v2h-2v-2zM6 8h2v2H6V8zm6 6h2v2h-2v-2z" />
    </svg>
  );
}

export function KeyIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12.65 10C11.83 7.67 9.61 6 7 6c-3.31 0-6 2.69-6 6s2.69 6 6 6c2.61 0 4.83-1.67 5.65-4H17v4h4v-4h2v-4H12.65zM7 14c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2z" />
    </svg>
  );
}

export function CameraIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M20 5h-3.17L15 3H9L7.17 5H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm-8 13c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.65 0-3 1.35-3 3s1.35 3 3 3 3-1.35 3-3-1.35-3-3-3z" />
    </svg>
  );
}

export function CheckIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z" />
    </svg>
  );
}

export function AlertIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" />
    </svg>
  );
}

export function DownloadIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z" />
    </svg>
  );
}

export function EyeIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z" />
    </svg>
  );
}

export function RefreshIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z" />
    </svg>
  );
}

export function UploadIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M9 16h6v-6h4l-7-7-7 7h4v6zm-4 2h14v2H5v-2z" />
    </svg>
  );
}

export function PlusIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
    </svg>
  );
}

export function XIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12 19 6.41z" />
    </svg>
  );
}

export function LockIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z" />
    </svg>
  );
}

export function FingerprintIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M17.81 4.47c-.08 0-.16-.02-.23-.06C15.66 3.42 14 3 12.01 3c-1.98 0-3.86.47-5.57 1.41-.24.13-.54.04-.68-.2-.13-.24-.04-.55.2-.68C7.82 2.52 9.86 2 12.01 2c2.13 0 3.99.47 5.92 1.5.24.13.33.43.2.67-.08.19-.23.3-.42.3zM3.56 9.72c-.06 0-.13-.02-.19-.05-.24-.13-.33-.44-.2-.68C4.54 6.34 7.97 4.5 12 4.5s7.46 1.84 8.83 4.49c.13.24.04.54-.2.68-.24.13-.55.04-.68-.2C18.72 7.07 15.6 5.5 12 5.5s-6.72 1.57-7.95 3.99c-.09.15-.29.23-.49.23zm9.44 13.28c-.28 0-.5-.22-.5-.5v-4.5c0-1.65-1.35-3-3-3s-3 1.35-3 3v2.5c0 .28-.22.5-.5.5s-.5-.22-.5-.5v-2.5c0-2.21 1.79-4 4-4s4 1.79 4 4v4.5c0 .28-.22.5-.5.5zm4 0c-.28 0-.5-.22-.5-.5V17c0-3.31-2.69-6-6-6s-6 2.69-6 6v3.5c0 .28-.22.5-.5.5s-.5-.22-.5-.5V17c0-3.86 3.14-7 7-7s7 3.14 7 7v5.5c0 .28-.22.5-.5.5z" />
    </svg>
  );
}

export function SunIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58a.996.996 0 0 0-1.41 0 .996.996 0 0 0 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37a.996.996 0 0 0-1.41 0 .996.996 0 0 0 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0a.996.996 0 0 0 0-1.41l-1.06-1.06zm1.06-10.96a.996.996 0 0 0 0-1.41.996.996 0 0 0-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36a.996.996 0 0 0 0-1.41.996.996 0 0 0 0-1.41l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z" />
    </svg>
  );
}

export function MoonIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12.3 2a10 10 0 0 0-1.9 19.8 10 10 0 0 0 11.4-11.4A10 10 0 0 1 12.3 2z" />
    </svg>
  );
}

export function ArrowRightIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8z" />
    </svg>
  );
}

export function ArrowLeftIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z" />
    </svg>
  );
}

export function TerminalIcon({ className = "w-5 h-5", size = 20 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" className={className}>
      <path d="M20 4H4c-1.11 0-2 .89-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V6c0-1.11-.89-2-2-2zm-2 14h-6v-2h6v2zm-8.29-3.71L5.41 10l4.3-4.29 1.41 1.41L8.24 10l2.88 2.88-1.41 1.41z" />
    </svg>
  );
}

export function MedVaultLogoIllustration({ className = "w-8 h-8" }: { className?: string }) {
  return (
    <svg viewBox="0 0 40 40" fill="none" className={className}>
      <rect width="40" height="40" rx="10" className="fill-blue-600 dark:fill-blue-500" />
      <path
        d="M20 9L29 13.5V20C29 25.5 25.2 30.6 20 32C14.8 30.6 11 25.5 11 20V13.5L20 9Z"
        fill="white"
        fillOpacity="0.2"
        stroke="white"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="M20 14V26M14 20H26"
        stroke="white"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function EmptyStateIllustration({ className = "w-32 h-32" }: { className?: string }) {
  return (
    <svg viewBox="0 0 120 120" fill="none" className={className}>
      <rect x="25" y="30" width="70" height="65" rx="8" className="fill-slate-100 dark:fill-[#282a2c] stroke-slate-300 dark:stroke-[#3c4043]" strokeWidth="2" />
      <rect x="35" y="20" width="50" height="15" rx="4" className="fill-slate-200 dark:fill-[#3c4043]" />
      <line x1="40" y1="50" x2="80" y2="50" className="stroke-slate-300 dark:stroke-slate-600" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="40" y1="62" x2="70" y2="62" className="stroke-slate-300 dark:stroke-slate-600" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="40" y1="74" x2="60" y2="74" className="stroke-slate-300 dark:stroke-slate-600" strokeWidth="2.5" strokeLinecap="round" />
      <circle cx="85" cy="85" r="14" className="fill-blue-600 dark:fill-blue-500" />
      <path d="M85 79V91M79 85H91" stroke="white" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function VaultIllustration({ className = "w-24 h-24" }: { className?: string }) {
  return (
    <svg viewBox="0 0 96 96" fill="none" className={className}>
      <rect x="12" y="16" width="72" height="64" rx="8" className="fill-slate-100 dark:fill-[#282a2c] stroke-slate-300 dark:stroke-[#3c4043]" strokeWidth="2" />
      <circle cx="48" cy="48" r="20" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" strokeDasharray="3 3" />
      <circle cx="48" cy="48" r="10" className="fill-blue-600/10 dark:fill-blue-400/20 stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" />
      <line x1="48" y1="38" x2="48" y2="32" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="48" y1="58" x2="48" y2="64" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="38" y1="48" x2="32" y2="48" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <line x1="58" y1="48" x2="64" y2="48" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

export function StegoIllustration({ className = "w-24 h-24" }: { className?: string }) {
  return (
    <svg viewBox="0 0 96 96" fill="none" className={className}>
      <rect x="16" y="16" width="64" height="64" rx="8" className="fill-slate-100 dark:fill-[#282a2c] stroke-slate-300 dark:stroke-[#3c4043]" strokeWidth="2" />
      <circle cx="36" cy="36" r="6" className="fill-blue-600 dark:fill-blue-400" />
      <path d="M22 66L42 46L54 58L68 44L74 50V72C74 76.4 70.4 80 66 80H30C25.6 80 22 76.4 22 72V66Z" className="fill-slate-200 dark:fill-[#3c4043]" />
      <circle cx="68" cy="28" r="12" className="fill-emerald-600 dark:fill-emerald-500" />
      <path d="M68 22V34M62 28H74" stroke="white" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function BlockchainIllustration({ className = "w-24 h-24" }: { className?: string }) {
  return (
    <svg viewBox="0 0 96 96" fill="none" className={className}>
      <rect x="10" y="32" width="22" height="26" rx="4" className="fill-slate-100 dark:fill-[#282a2c] stroke-slate-300 dark:stroke-[#3c4043]" strokeWidth="2" />
      <rect x="37" y="32" width="22" height="26" rx="4" className="fill-blue-50 dark:fill-blue-950/40 stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" />
      <rect x="64" y="32" width="22" height="26" rx="4" className="fill-slate-100 dark:fill-[#282a2c] stroke-slate-300 dark:stroke-[#3c4043]" strokeWidth="2" />
      <line x1="32" y1="45" x2="37" y2="45" className="stroke-slate-400 dark:stroke-slate-500" strokeWidth="2" strokeLinecap="round" />
      <line x1="59" y1="45" x2="64" y2="45" className="stroke-slate-400 dark:stroke-slate-500" strokeWidth="2" strokeLinecap="round" />
      <circle cx="48" cy="45" r="4" className="fill-blue-600 dark:fill-blue-400" />
    </svg>
  );
}

export function MedicalSecurityHeroIllustration({ className = "w-28 h-28" }: { className?: string }) {
  return (
    <svg viewBox="0 0 128 128" fill="none" className={className}>
      {/* Soft Ambient Background Glow */}
      <circle cx="64" cy="64" r="54" className="fill-blue-50/70 dark:fill-blue-950/30" />
      
      {/* Outer Protective Security Shield */}
      <path
        d="M64 18L98 33V66C98 87.2 83.5 106.8 64 112C44.5 106.8 30 87.2 30 66V33L64 18Z"
        className="fill-blue-600 dark:fill-blue-500 stroke-blue-700 dark:stroke-blue-400"
        strokeWidth="2.5"
        strokeLinejoin="round"
      />

      {/* Inner Transparent Accent Layer */}
      <path
        d="M64 26L90 38V65C90 82.5 78.8 98.7 64 103.5C49.2 98.7 38 82.5 38 65V38L64 26Z"
        fill="white"
        fillOpacity="0.16"
      />

      {/* Prominent Medical Cross Emblem */}
      <path
        d="M64 42V68M51 55H77"
        stroke="white"
        strokeWidth="4.5"
        strokeLinecap="round"
      />

      {/* Vital Heartbeat / EKG Pulse Line */}
      <path
        d="M44 80H53L57 74L63 87L69 77L73 80H84"
        stroke="white"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeOpacity="0.9"
      />

      {/* Floating Hardware Key & Cryptographic Lock Badge */}
      <g transform="translate(78, 76)">
        <circle cx="16" cy="16" r="15" className="fill-white dark:fill-[#1e1f20] stroke-slate-200 dark:stroke-[#3c4043]" strokeWidth="2" />
        <rect x="10" y="14" width="12" height="9" rx="2" className="fill-blue-600 dark:fill-blue-400" />
        <path d="M13 14V11A3 3 0 0 1 19 11V14" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="1.8" strokeLinecap="round" />
        <circle cx="16" cy="18.5" r="1" fill="white" />
      </g>
    </svg>
  );
}

export function DongleEjectedIllustration({ className = "w-6 h-6" }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" fill="none" className={className}>
      <circle cx="16" cy="16" r="14" className="fill-red-50 dark:fill-rose-950/50 stroke-red-200 dark:stroke-rose-900" strokeWidth="1.5" />
      <rect x="10" y="8" width="12" height="10" rx="2" className="fill-red-600 dark:fill-rose-400" />
      <rect x="13" y="18" width="6" height="5" rx="1" className="fill-slate-400 dark:fill-slate-500" />
      <line x1="8" y1="24" x2="24" y2="8" className="stroke-red-600 dark:stroke-rose-400" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

export function CloudflareSecurityShieldIllustration({ className = "w-24 h-24" }: { className?: string }) {
  return (
    <svg viewBox="0 0 120 120" fill="none" className={className}>
      {/* Ambient Radial Waves */}
      <circle cx="60" cy="60" r="50" className="stroke-blue-100 dark:stroke-blue-950/60" strokeWidth="1.5" strokeDasharray="4 4" />
      <circle cx="60" cy="60" r="42" className="fill-blue-50/60 dark:fill-blue-950/20 stroke-blue-200/80 dark:stroke-blue-900/40" strokeWidth="1.5" />
      
      {/* Outer Geometric Hex-Shield */}
      <path
        d="M60 18L92 32V60C92 79.5 78.4 97.4 60 102C41.6 97.4 28 79.5 28 60V32L60 18Z"
        className="fill-blue-600 dark:fill-blue-500"
      />

      {/* Layered Facet Shading */}
      <path
        d="M60 22L88 34V60C88 77 76.2 92.5 60 97.5V22Z"
        fill="white"
        fillOpacity="0.12"
      />

      {/* Inner Medical Cross Emblem */}
      <path
        d="M60 40V72M44 56H76"
        stroke="white"
        strokeWidth="4.5"
        strokeLinecap="round"
      />

      {/* Micro Status Nodes */}
      <circle cx="60" cy="32" r="2.5" fill="white" fillOpacity="0.8" />
      <circle cx="80" cy="46" r="2.5" fill="white" fillOpacity="0.8" />
      <circle cx="40" cy="46" r="2.5" fill="white" fillOpacity="0.8" />
      <circle cx="60" cy="88" r="2.5" fill="white" fillOpacity="0.8" />
    </svg>
  );
}

export function HardwareKeyScanIllustration({ className = "w-24 h-24", detected = false }: { className?: string; detected?: boolean }) {
  return (
    <svg viewBox="0 0 120 120" fill="none" className={className}>
      {/* Outer Pulse Circles */}
      <circle
        cx="60"
        cy="60"
        r="52"
        className={detected ? "stroke-emerald-200 dark:stroke-emerald-950/60" : "stroke-blue-100 dark:stroke-blue-950/60"}
        strokeWidth="1.5"
        strokeDasharray="3 3"
      />
      <circle
        cx="60"
        cy="60"
        r="44"
        className={detected ? "fill-emerald-50/50 dark:fill-emerald-950/20" : "fill-blue-50/50 dark:fill-blue-950/20"}
      />

      {/* Hardware Key Body */}
      <g transform="translate(42, 32)">
        {/* USB Connector Plug */}
        <rect x="11" y="4" width="14" height="12" rx="2" className="fill-slate-300 dark:fill-slate-600 stroke-slate-400 dark:stroke-slate-500" strokeWidth="1.5" />
        <rect x="14" y="7" width="2.5" height="4" rx="0.5" className="fill-slate-400 dark:fill-slate-400" />
        <rect x="19.5" y="7" width="2.5" height="4" rx="0.5" className="fill-slate-400 dark:fill-slate-400" />

        {/* Security Token Handle */}
        <path
          d="M6 14H30C32.2 14 34 15.8 34 18V44C34 47.3 31.3 50 28 50H8C4.7 50 2 47.3 2 44V18C2 15.8 3.8 14 6 14Z"
          className={detected ? "fill-emerald-600 dark:fill-emerald-500" : "fill-blue-600 dark:fill-blue-500"}
        />

        {/* Central Physical Sensor Contact */}
        <circle cx="18" cy="32" r="7" className="fill-white/20 stroke-white" strokeWidth="1.5" />
        <circle cx="18" cy="32" r="3.5" fill="white" />

        {/* Keyhole Loop at Bottom */}
        <circle cx="18" cy="44" r="2.5" className="fill-white/80" />
      </g>

      {/* Status Ring Indicator */}
      {detected && (
        <circle cx="60" cy="98" r="4" className="fill-emerald-500 animate-pulse" />
      )}
    </svg>
  );
}

export function BiometricReticleIllustration({ className = "w-24 h-24" }: { className?: string }) {
  return (
    <svg viewBox="0 0 120 120" fill="none" className={className}>
      {/* Concentric Scanner Halo */}
      <circle cx="60" cy="60" r="50" className="stroke-blue-200/60 dark:stroke-blue-900/40" strokeWidth="1" strokeDasharray="6 6" />
      <circle cx="60" cy="60" r="40" className="fill-blue-50/50 dark:fill-blue-950/20 stroke-blue-500/50" strokeWidth="1.5" />

      {/* Target Corner Reticles */}
      <path d="M42 34H34V42" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M78 34H86V42" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M34 78V86H42" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M86 78V86H78" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2.5" strokeLinecap="round" />

      {/* Fingerprint Ridge Center */}
      <g transform="translate(42, 42)">
        <path d="M18 10C14.7 10 12 12.7 12 16C12 21 10 24 8 26" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" strokeLinecap="round" />
        <path d="M22 13C24.2 13 26 14.8 26 17C26 23 23 27 21 30" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" strokeLinecap="round" />
        <path d="M18 17C16.9 17 16 17.9 16 19C16 23 15 25 14 27" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" strokeLinecap="round" />
        <path d="M20 20C20.6 20 21 20.4 21 21C21 24 20 26 19 28" className="stroke-blue-600 dark:stroke-blue-400" strokeWidth="2" strokeLinecap="round" />
        <circle cx="18" cy="24" r="1.5" className="fill-blue-600 dark:fill-blue-400" />
      </g>
    </svg>
  );
}
