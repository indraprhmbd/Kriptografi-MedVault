using System;
using System.Diagnostics;
using System.IO;
using System.Windows.Forms;

namespace MedVaultLauncher
{
    static class Program
    {
        [STAThread]
        static void Main(string[] args)
        {
            try
            {
                string baseDir = AppDomain.CurrentDomain.BaseDirectory;
                string venvPythonw = Path.Combine(baseDir, ".venv", "Scripts", "pythonw.exe");
                string venvPython = Path.Combine(baseDir, ".venv", "Scripts", "python.exe");
                string scriptPath = Path.Combine(baseDir, "scripts", "run_app.py");

                string pythonExe = File.Exists(venvPythonw) ? venvPythonw :
                                   File.Exists(venvPython) ? venvPython : "pythonw.exe";

                if (!File.Exists(scriptPath))
                {
                    MessageBox.Show(
                        "Berkas sistem 'scripts/run_app.py' tidak ditemukan pada direktori:\n" + baseDir,
                        "MedVault - Kesalahan Peluncur",
                        MessageBoxButtons.OK,
                        MessageBoxIcon.Error
                    );
                    return;
                }

                ProcessStartInfo psi = new ProcessStartInfo();
                psi.FileName = pythonExe;
                psi.Arguments = "\"" + scriptPath + "\"";
                psi.WorkingDirectory = baseDir;
                psi.UseShellExecute = false;
                psi.CreateNoWindow = true;
                psi.WindowStyle = ProcessWindowStyle.Hidden;

                using (Process proc = Process.Start(psi))
                {
                    if (proc != null)
                    {
                        proc.WaitForExit();
                    }
                }
            }
            catch (Exception ex)
            {
                MessageBox.Show(
                    "Terjadi kesalahan saat memulai MedVault:\n" + ex.Message,
                    "MedVault - Kesalahan Sistem",
                    MessageBoxButtons.OK,
                    MessageBoxIcon.Error
                );
            }
        }
    }
}
