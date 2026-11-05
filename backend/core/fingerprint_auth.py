"""Hardware Fingerprint Authentication Module via Windows Hello.

Integrates native Windows Hello UserConsentVerifier API to perform
hardware-backed biometric fingerprint verification.
"""

from pathlib import Path
import subprocess
from typing import Optional, Tuple


class FingerprintAuthenticator:
    """Manages Windows Hello fingerprint sensor checks and interactive verification."""

    def __init__(self, mock_mode: Optional[bool] = None) -> None:
        """Initialize fingerprint authenticator.

        Args:
            mock_mode: If True, simulates hardware verification for non-interactive automated testing.
                       If None or False, invokes native Windows Hello UserConsentVerifier.
        """
        self.mock_mode: bool = bool(mock_mode)

    def is_available(self) -> Tuple[bool, str]:
        """Check whether Windows Hello biometric fingerprint verification is configured and available."""
        if self.mock_mode:
            return True, "Sensor sidik jari Windows Hello siap digunakan (Mode Simulasi)"

        ps_script = """
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { 
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' 
}
[Windows.Security.Credentials.UI.UserConsentVerifier,Windows.Security.Credentials.UI,ContentType=WindowsRuntime] | Out-Null
$op = [Windows.Security.Credentials.UI.UserConsentVerifier]::CheckAvailabilityAsync()
$task = $asTaskGeneric[0].MakeGenericMethod([Windows.Security.Credentials.UI.UserConsentVerifierAvailability]).Invoke($null, @($op))
$task.Wait()
Write-Output $task.Result
"""
        try:
            cmd = [
                "powershell.exe",
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                ps_script,
            ]
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=8)
            output = res.stdout.strip()

            if output == "Available":
                return True, "Sensor sidik jari Windows Hello terdeteksi dan siap digunakan"
            elif output == "DeviceNotPresent":
                return False, "Perangkat sensor sidik jari tidak ditemukan pada sistem ini"
            elif output == "NotConfiguredForUser":
                return False, "Sidik jari Windows Hello belum didaftarkan pada akun pengguna Windows ini"
            elif output == "DisabledByPolicy":
                return False, "Windows Hello dinonaktifkan oleh kebijakan sistem (Group Policy)"
            else:
                return False, f"Status Windows Hello: {output or 'Tidak tersedia'}"
        except subprocess.TimeoutExpired:
            return False, "Pemeriksaan sensor sidik jari Windows Hello melebihi batas waktu (timeout)"
        except Exception as exc:
            return False, f"Gagal memeriksa status Windows Hello: {str(exc)}"

    def verify_user(
        self, prompt_message: str = "Verifikasi Biometrik Sidik Jari MedVault"
    ) -> Tuple[bool, str]:
        """Trigger native Windows Hello biometric verification prompt.

        Returns:
            (is_verified, status_message)
        """
        if self.mock_mode:
            return True, "Verifikasi sidik jari berhasil (Mode Simulasi)"

        # Escape double quotes in prompt
        escaped_prompt = prompt_message.replace('"', '`"')

        ps_script = f"""
$source = @"
using System;
using System.Diagnostics;
using System.Runtime.InteropServices;

public class WinTopmost {{
    [DllImport("user32.dll", SetLastError = true)]
    public static extern bool SetWindowPos(IntPtr hWnd, IntPtr hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);
    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")]
    public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);

    public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    public static void ElevateBroker() {{
        IntPtr HWND_TOPMOST = new IntPtr(-1);
        uint SWP_FLAGS = 0x0001 | 0x0002 | 0x0040;
        Process[] procs = Process.GetProcessesByName("CredentialUIBroker");
        if (procs.Length == 0) return;
        EnumWindows((hWnd, lParam) => {{
            uint pid;
            GetWindowThreadProcessId(hWnd, out pid);
            foreach (var p in procs) {{
                if (p.Id == pid) {{
                    SetWindowPos(hWnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_FLAGS);
                    BringWindowToTop(hWnd);
                    SetForegroundWindow(hWnd);
                }}
            }}
            return true;
        }}, IntPtr.Zero);
    }}
}}
"@
Add-Type -TypeDefinition $source
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {{ 
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' 
}}
[Windows.Security.Credentials.UI.UserConsentVerifier,Windows.Security.Credentials.UI,ContentType=WindowsRuntime] | Out-Null
$op = [Windows.Security.Credentials.UI.UserConsentVerifier]::RequestVerificationAsync("{escaped_prompt}")
$task = $asTaskGeneric[0].MakeGenericMethod([Windows.Security.Credentials.UI.UserConsentVerificationResult]).Invoke($null, @($op))

# Elevate Windows Hello dialog to topmost so it is visible above fullscreen windows
for ($i = 0; $i -lt 30; $i++) {{
    if ($task.IsCompleted) {{ break }}
    [WinTopmost]::ElevateBroker()
    Start-Sleep -Milliseconds 100
}}

$task.Wait()
Write-Output $task.Result
"""
        try:
            cmd = [
                "powershell.exe",
                "-NoProfile",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                ps_script,
            ]
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
            output = res.stdout.strip()

            if output == "Verified":
                return True, "Verifikasi sidik jari Windows Hello berhasil"
            elif output == "Canceled":
                return False, "Verifikasi sidik jari dibatalkan oleh pengguna"
            elif output == "RetriesExhausted":
                return False, "Verifikasi sidik jari gagal: percobaan melebihi batas"
            elif output == "DeviceBusy":
                return False, "Sensor sidik jari sedang sibuk digunakan aplikasi lain"
            elif output == "DeviceNotPresent":
                return False, "Sensor sidik jari tidak ditemukan pada sistem"
            else:
                return False, f"Verifikasi sidik jari gagal (Status: {output or 'Tidak diketahui'})"
        except subprocess.TimeoutExpired:
            return False, "Verifikasi sidik jari dibatalkan karena melebihi batas waktu 30 detik"
        except Exception as exc:
            return False, f"Kesalahan saat menjalankan verifikasi Windows Hello: {str(exc)}"
