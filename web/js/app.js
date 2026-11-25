// MedVault Go dashboard controller.
//
// All cryptography that touches the dongle happens here in the browser. The
// server only ever receives an Ed25519 signature over a canonical transaction
// payload. WebCrypto Ed25519 support is required for the signing flow.

const PKCS8_ED25519_PREFIX = "302e020100300506032b657004220420";
const MAX_VAULT_FILE_BYTES = 64 * 1024 * 1024;
const PENDING_STORAGE_KEY = "medvault.pending";
const PATIENT_DRAFT_KEY = "medvault.patientDraft";

function hexToBytes(hex) {
  const clean = (hex || "").trim();
  if (clean.length % 2 !== 0) {
    throw new Error("hex string has an odd length");
  }
  const out = new Uint8Array(clean.length / 2);
  for (let i = 0; i < out.length; i += 1) {
    out[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16);
  }
  return out;
}

function bytesToHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function base64urlToHex(value) {
  let s = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
  while (s.length % 4 !== 0) s += "=";
  const raw = atob(s);
  let hex = "";
  for (let i = 0; i < raw.length; i += 1) {
    hex += raw.charCodeAt(i).toString(16).padStart(2, "0");
  }
  return hex;
}

function credentialToJSON(credential) {
  if (typeof credential.toJSON === "function") {
    return credential.toJSON();
  }
  const response = credential.response;
  const json = {
    id: credential.id,
    rawId: bytesToHex(credential.rawId),
    type: credential.type,
    response: {},
  };
  if (response.clientDataJSON) {
    json.response.clientDataJSON = bytesToHex(response.clientDataJSON);
  }
  if (response.attestationObject) {
    json.response.attestationObject = bytesToHex(response.attestationObject);
  }
  if (response.authenticatorData) {
    json.response.authenticatorData = bytesToHex(response.authenticatorData);
  }
  if (response.signature) {
    json.response.signature = bytesToHex(response.signature);
  }
  if (response.userHandle) {
    json.response.userHandle = bytesToHex(response.userHandle);
  }
  return json;
}

function readWebAuthnOptions(payload) {
  if (!payload || !payload.publicKey) {
    throw new Error("server did not return publicKey options");
  }
  return payload.publicKey;
}

const DONGLE_IDB_NAME = "medvault_hardware";
const DONGLE_IDB_STORE = "dongle_handle";

function openDongleDB() {
  return new Promise((resolve) => {
    if (typeof window === "undefined" || !window.indexedDB) {
      resolve(null);
      return;
    }
    try {
      const req = window.indexedDB.open(DONGLE_IDB_NAME, 1);
      req.onupgradeneeded = (e) => {
        const db = e.target.result;
        if (!db.objectStoreNames.contains(DONGLE_IDB_STORE)) {
          db.createObjectStore(DONGLE_IDB_STORE);
        }
      };
      req.onsuccess = (e) => resolve(e.target.result);
      req.onerror = () => resolve(null);
    } catch (_) {
      resolve(null);
    }
  });
}

async function idbSaveDongleHandle(handle) {
  if (!handle) return;
  try {
    const db = await openDongleDB();
    if (!db) return;
    const tx = db.transaction(DONGLE_IDB_STORE, "readwrite");
    tx.objectStore(DONGLE_IDB_STORE).put(handle, "active_handle");
  } catch (_) {}
}

async function idbGetDongleHandle() {
  try {
    const db = await openDongleDB();
    if (!db) return null;
    return new Promise((resolve) => {
      try {
        const tx = db.transaction(DONGLE_IDB_STORE, "readonly");
        const req = tx.objectStore(DONGLE_IDB_STORE).get("active_handle");
        req.onsuccess = (e) => resolve(e.target.result || null);
        req.onerror = () => resolve(null);
      } catch (_) {
        resolve(null);
      }
    });
  } catch (_) {
    return null;
  }
}

async function idbClearDongleHandle() {
  try {
    const db = await openDongleDB();
    if (!db) return;
    const tx = db.transaction(DONGLE_IDB_STORE, "readwrite");
    tx.objectStore(DONGLE_IDB_STORE).delete("active_handle");
  } catch (_) {}
}

window.medvault = function medvault() {
  return {
    tabs: [
      { id: "dashboard", label: "Dashboard", path: "/dashboard" },
      { id: "patients", label: "Pasien", path: "/patients" },
      { id: "records", label: "Rekam Medis", path: "/records" },
      { id: "vault", label: "Brankas File", path: "/vault" },
      { id: "stego", label: "Steganografi", path: "/steganography" },
      { id: "audit", label: "Audit Ledger", path: "/audit" },
      { id: "telemetry", label: "Telemetry", path: "/telemetry" },
      { id: "security", label: "Keamanan", path: "/security" },
    ],
    tab: "gate",
    route: "gate",
    loginStep: 1,
    registerStep: 1,
    showPassword: false,
    showRegisterPassword: false,
    registerPasswordConfirm: "",
    registerTouched: {
      username: false,
      full_name: false,
      password: false,
      password_confirm: false,
      dongle: false,
      face: false,
    },

    busy: false,
    toast: { type: "ok", message: "" },
    telemetry: null,
    telemetryOpen: false,
    // The Glass-Box page is a separate document, so the last trace is mirrored
    // into sessionStorage and restored on load instead of being invented.
    telemetrySource: null,
    telemetryAt: "",

    session: null,
    sessionInfo: null,
    // Keeps live identity hidden until /auth/me resolves, so Figma demo labels
    // never flash on the sidebar during a full page navigation.
    sessionReady: false,
    health: null,
    healthState: "unknown",
    dongle: null,
    dongleNote: "",
    dongleFileHandle: null,
    dongleConnected: false,
    dongleSupported: typeof window !== "undefined" && typeof window.showOpenFilePicker === "function",
    dongleEjected: false,
    dongleEjectedAt: "",
    dongleLastChecked: "",
    _dongleWatchdogTimer: null,
    // Registration step 2: "" (not chosen), "new" (generate a .medkey in the
    // browser) or "existing" (read a .medkey issued earlier).
    dongleMode: "",
    newDongle: {
      generating: false,
      issuedAt: "",
      publicKeyHex: "",
      savedAs: "",
      saveMethod: "",
      saved: false,
      confirmed: false,
    },
    faceStatus: "",
    faceStream: null,
    facePreviewActive: false,
    // Login step 3 uses its own preview because the gate page has no
    // registration wizard. The captured flag is the single source of truth
    // for the Selesaikan login button.
    loginFaceStream: null,
    loginFacePreviewActive: false,
    loginFaceCaptured: false,

    registerForm: {
      username: "",
      full_name: "",
      password: "",
      dongle_key_id: "",
      dongle_pubkey_hex: "",
      webcam_dhash_hex: "",
    },
    loginForm: {
      username: "",
      password: "",
      webcam_dhash_hex: "",
    },

    patients: [],
    patientView: "decrypted",
    patientForm: {
      medical_record_number: "",
      nik: "",
      full_name: "",
      address: "",
      birth_date: "",
    },
    patientTouched: {
      medical_record_number: false,
      nik: false,
      full_name: false,
      birth_date: false,
    },

    get patientNikValid() {
      return /^[0-9]{16}$/.test(String(this.patientForm.nik || "").trim());
    },

    get patientBirthDateValid() {
      const value = String(this.patientForm.birth_date || "").trim();
      if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
      const parsed = new Date(`${value}T00:00:00Z`);
      return !Number.isNaN(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value;
    },

    recordForm: {
      patient_id: 0,
      diagnosis: "",
      prescription: "",
      visit_status: "OPEN",
    },
    inspectId: 0,
    inspection: null,

    vaultRecordId: 0,
    vaultFile: null,
    vaultInfo: null,

    stegoRecordId: 0,
    stegoFile: null,
    stegoResult: null,
    extractFile: null,
    extracted: null,

    blocks: [],
    validation: null,
    validationCheckedAt: "",
    tamperRecordId: 0,

    pending: null,

    get telemetryPretty() {
      if (!this.telemetry) {
        return "Belum ada respons kriptografi. Lakukan satu aksi untuk melihat nilai antara.";
      }
      return JSON.stringify(this.telemetry, null, 2);
    },

    get telemetryDurationLabel() {
      const micros = this.telemetry && this.telemetry.duration_us;
      if (typeof micros !== "number") {
        return "-";
      }
      if (micros < 1000) {
        return micros + " us";
      }
      return (micros / 1000).toFixed(2) + " ms";
    },

    get validationCheckedLabel() {
      if (!this.validationCheckedAt) {
        return "Belum diperiksa";
      }
      return new Date(this.validationCheckedAt).toLocaleString("id-ID", {
        dateStyle: "medium",
        timeStyle: "short",
      });
    },

    get healthLabel() {
      if (this.healthState === "down") return "API tidak terjangkau";
      if (this.healthState === "ok") return "API terhubung";
      return "Memeriksa API...";
    },

    get healthBadgeClass() {
      if (this.healthState === "down") return "mv-badge-danger";
      if (this.healthState === "ok") return "mv-badge-success";
      return "mv-badge-teal";
    },

    get currentTabLabel() {
      const current = this.tabs.find((tab) => tab.id === this.route);
      return current ? current.label : "Autentikasi";
    },

    // Presentation-only mood for the mascot. Authentication still flows through
    // the existing loginStep, dongle, and webcam state.
    get authMood() {
      if (this.toast.type === "error" && this.toast.message) return "error";
      if (this.busy) return "idle";
      if (
        (this.loginStep === 2 && this.dongle) ||
        (this.loginStep === 3 && this.loginForm.webcam_dhash_hex)
      ) {
        return "success";
      }
      return "idle";
    },

    get registerMood() {
      if (this.toast.type === "error" && this.toast.message) return "error";
      if (this.busy) return "idle";
      if (this.registerStep === 4) return "success";
      if (this.registerStep === 2 && this.registerDongleValid) return "success";
      if (this.registerStep === 3 && this.registerBiometricValid) return "success";
      return "idle";
    },

    get registerStepSubtitle() {
      if (this.registerStep === 2) {
        return "Siapkan USB Dongle: buat kunci baru atau pakai berkas .medkey yang sudah ada.";
      }
      if (this.registerStep === 3) {
        return "Ambil verifikasi wajah supaya akun terikat pada perangkat ini.";
      }
      if (this.registerStep === 4) {
        return "Akun dokter Anda sudah siap digunakan.";
      }
      return "Mulai dari data akun yang akan Anda pakai setiap hari.";
    },

    get registerUsernameError() {
      const value = this.registerForm.username.trim();
      if (!value) return "Nama pengguna wajib diisi ya.";
      if (value.length < 3) return "Nama pengguna minimal 3 karakter ya.";
      if (/\s/.test(value)) return "Nama pengguna tidak boleh memakai spasi. Gunakan garis bawah atau titik.";
      if (!/^[A-Za-z0-9._-]+$/.test(value)) {
        return "Gunakan huruf, angka, titik, garis bawah, atau tanda hubung.";
      }
      return "";
    },

    get registerFullNameError() {
      const value = this.registerForm.full_name.trim();
      if (!value) return "Nama lengkap wajib diisi ya.";
      if (value.length < 2) return "Nama lengkap minimal 2 karakter ya.";
      return "";
    },

    get registerPasswordError() {
      if (!this.registerForm.password) return "Kata sandi wajib diisi ya.";
      if (this.registerForm.password.length < 12) return "Kata sandi minimal 12 karakter ya.";
      return "";
    },

    get registerPasswordConfirmError() {
      if (!this.registerPasswordConfirm) {
        return "Ulangi kata sandi supaya tidak ada salah ketik.";
      }
      if (this.registerPasswordConfirm !== this.registerForm.password) {
        return "Kata sandi belum sama. Periksa lagi ya.";
      }
      return "";
    },

    get registerPasswordScore() {
      const value = this.registerForm.password;
      if (!value) return 0;
      let score = 0;
      if (value.length >= 12) score += 1;
      if (value.length >= 16) score += 1;
      if (/[a-z]/.test(value) && /[A-Z]/.test(value)) score += 1;
      if (/\d/.test(value) || /[^A-Za-z0-9]/.test(value)) score += 1;
      return Math.min(4, Math.max(1, score));
    },

    get registerPasswordStrengthLabel() {
      const labels = ["", "Lemah", "Cukup", "Kuat", "Sangat kuat"];
      return labels[this.registerPasswordScore] || "";
    },

    get registerDongleUsernameMismatch() {
      return Boolean(
        this.dongle &&
          this.dongle.username &&
          this.registerForm.username.trim() &&
          this.dongle.username !== this.registerForm.username.trim()
      );
    },

    get registerDongleError() {
      if (!this.dongleMode) {
        return "Pilih dulu: buat dongle baru atau pakai dongle yang sudah ada.";
      }
      if (!this.dongle) {
        return this.dongleMode === "new"
          ? "Kunci dongle belum dibuat. Tekan Buat ulang kunci, ya."
          : "Berkas .medkey belum dipilih. Pilih berkas dari USB Anda.";
      }
      if (!this.registerForm.dongle_key_id.trim()) {
        return "ID USB Dongle belum terbaca. Pilih ulang berkas .medkey, ya.";
      }
      if (!/^[0-9a-fA-F]{64}$/.test(this.registerForm.dongle_pubkey_hex.trim())) {
        return "Kunci publik USB Dongle belum siap. Pilih ulang berkas .medkey, ya.";
      }
      if (this.registerDongleUsernameMismatch) {
        return "Nama pengguna di USB Dongle berbeda dengan akun ini. Kembali ke langkah data akun atau gunakan dongle yang sesuai.";
      }
      if (this.dongleMode === "new" && !this.newDongle.saved) {
        return "Simpan berkas .medkey ke USB dulu sebelum melanjutkan.";
      }
      if (this.dongleMode === "new" && !this.newDongle.confirmed) {
        return "Centang konfirmasi bahwa berkas sudah tersimpan di USB.";
      }
      return "";
    },

    get registerDongleCardTitle() {
      if (this.dongleMode === "new") {
        if (this.newDongle.generating) return "Sedang membuat kunci...";
        if (!this.dongle) return "Kunci belum dibuat";
        if (!this.newDongle.saved) return "Kunci dongle sudah dibuat";
        return "Berkas dongle tersimpan";
      }
      if (this.dongleMode === "existing") {
        return this.dongle ? "Berkas dongle terbaca" : "Belum ada berkas dipilih";
      }
      return "Pilih cara menyiapkan dongle";
    },

    get registerDongleCardText() {
      if (this.dongleMode === "new") {
        if (this.newDongle.generating) return "Pasangan kunci dibuat di browser ini. Mohon tunggu sebentar.";
        if (!this.dongle) return "Pembuatan kunci belum berhasil. Tekan Buat ulang kunci untuk mencoba lagi.";
        if (!this.newDongle.saved) {
          return "Colok USB Anda, lalu tekan Simpan ke USB dan pilih drive USB sebagai lokasi penyimpanan.";
        }
        return "Tersimpan sebagai " + (this.newDongle.savedAs || this.newDongleFileName) +
          (this.newDongle.saveMethod === "download"
            ? ". Pindahkan berkas ini dari folder Unduhan ke USB, lalu hapus salinannya dari komputer."
            : ".");
      }
      if (this.dongleMode === "existing") {
        return this.dongle
          ? "Dongle " + this.registerForm.dongle_key_id + " siap digunakan."
          : "Colok USB Anda, lalu pilih berkas .medkey dari drive USB.";
      }
      return "Belum punya berkas .medkey? Pilih Buat dongle baru, kunci dibuat otomatis oleh sistem.";
    },

    get newDongleFileName() {
      const name = this.registerForm.username.trim() || "dongle";
      return name + ".medkey";
    },

    get newDongleFingerprint() {
      const hex = this.newDongle.publicKeyHex;
      return hex ? hex.slice(0, 8) + "..." + hex.slice(-8) : "";
    },

    get registerDongleValid() {
      return !this.registerDongleError;
    },

    get registerBiometricError() {
      if (!this.registerForm.webcam_dhash_hex) {
        return "Wajah belum dipindai. Nyalakan kamera, lalu ambil verifikasi wajah.";
      }
      if (!/^[0-9a-fA-F]{16}$/.test(this.registerForm.webcam_dhash_hex)) {
        return "Hasil pemindaian wajah belum lengkap. Coba ambil ulang, ya.";
      }
      return "";
    },

    get registerBiometricValid() {
      return !this.registerBiometricError;
    },

    get registerAccountValid() {
      return !(
        this.registerUsernameError ||
        this.registerFullNameError ||
        this.registerPasswordError ||
        this.registerPasswordConfirmError
      );
    },

    get registerStepValid() {
      if (this.registerStep === 1) return this.registerAccountValid;
      if (this.registerStep === 2) return this.registerDongleValid;
      if (this.registerStep === 3) return this.registerBiometricValid;
      return true;
    },

    get showRegisterUsernameError() {
      return (this.registerTouched.username || this.registerForm.username.length > 0) &&
        Boolean(this.registerUsernameError);
    },

    get showRegisterFullNameError() {
      return (this.registerTouched.full_name || this.registerForm.full_name.length > 0) &&
        Boolean(this.registerFullNameError);
    },

    get showRegisterPasswordError() {
      return (this.registerTouched.password || this.registerForm.password.length > 0) &&
        Boolean(this.registerPasswordError);
    },

    get showRegisterPasswordConfirmError() {
      return (this.registerTouched.password_confirm || this.registerPasswordConfirm.length > 0) &&
        Boolean(this.registerPasswordConfirmError);
    },

    get showRegisterDongleError() {
      return (
        (this.registerTouched.dongle ||
          (Boolean(this.dongle) && this.dongleMode !== "new")) &&
        Boolean(this.registerDongleError)
      );
    },

    get showRegisterBiometricError() {
      return (this.registerTouched.face || Boolean(this.registerForm.webcam_dhash_hex)) &&
        Boolean(this.registerBiometricError);
    },

    // Friendly copy for the authentication gate. Raw server errors remain in the
    // Glass-Box telemetry payload so the technical detail stays available.
    get authFeedback() {
      if (this.toast.type !== "error" || !this.toast.message) return "";
      const message = String(this.toast.message);
      if (/already enrolled|already exists|already registered|sudah terdaftar/i.test(message)) {
        return "Nama pengguna atau ID dongle sudah terdaftar. Gunakan data lain, ya.";
      }
      if (/at least 12|minimal 12/i.test(message)) {
        return "Kata sandi minimal 12 karakter, ya.";
      }
      if (/password|kata sandi|wrong/i.test(message)) {
        return "Kata sandi belum cocok. Coba periksa lagi, ya.";
      }
      if (/are required|is required|must be provided|wajib/i.test(message)) {
        return "Lengkapi semua data dulu, ya.";
      }
      if (/signature|tanda tangan/i.test(message)) {
        return "Tanda tangan USB Dongle belum cocok. Pastikan dongle ini milik akun Anda.";
      }
      if (/challenge|sesi verifikasi/i.test(message)) {
        return "Sesi verifikasi sudah berakhir. Ulangi dari langkah pertama, ya.";
      }
      if (/pengguna.*dongle|username.*dongle|dongle.*username/i.test(message)) {
        return "Nama pengguna di USB Dongle tidak cocok dengan akun ini. Periksa lagi, ya.";
      }
      if (/tidak valid|invalid/i.test(message) && /dongle|medkey|private_key_seed|seed/i.test(message)) {
        return "Berkas USB Dongle tidak bisa dibaca. Pastikan berkas .medkey berasal dari fasilitas Anda.";
      }
      if (/browser|chrome|edge/i.test(message)) {
        return "Browser ini belum mendukung fitur keamanan tersebut. Gunakan Chrome atau Edge terbaru.";
      }
      if (/permission|notallowed|denied|izin/i.test(message)) {
        return "Akses kamera belum diizinkan. Izinkan kamera di browser, lalu coba lagi.";
      }
      if (/belum mendaftarkan windows hello/i.test(message)) {
        return "Akun ini belum mendaftarkan Windows Hello. Masuk dengan verifikasi wajah dulu, ya.";
      }
      if (/webauthn|windows hello|passkey/i.test(message)) {
        return "Verifikasi Windows Hello belum selesai. Coba lagi, ya.";
      }
      if (/dongle|medkey/i.test(message)) {
        return "USB Dongle belum terdeteksi. Pastikan sudah tertancap, lalu coba lagi.";
      }
      if (/kamera|wajah|dhash|biometrik|face/i.test(message)) {
        return "Wajah belum berhasil dipindai. Coba ambil ulang dengan cahaya yang cukup.";
      }
      if (/username|akun|user/i.test(message)) {
        return "Nama pengguna belum cocok. Periksa lagi, ya.";
      }
      if (/registrasi|register|daftar/i.test(message)) {
        return "Pendaftaran belum selesai. Periksa kembali data Anda, ya.";
      }
      return "Ada langkah yang belum selesai. Coba lagi, ya.";
    },

    async init() {
      this.route = document.body.dataset.route || "gate";
      this.tab = this.route;
      this.setupAssetFallbacks();
      this.restoreTelemetry();
      if (!this._faceUnloadBound) {
        this._faceUnloadBound = true;
        window.addEventListener("beforeunload", () => {
          this.stopFacePreview();
          this.stopLoginFacePreview();
          this.stopDongleWatchdog();
        });
      }
      this.restoreRememberedRecord();
      this.restorePatientDraft();
      ["medical_record_number", "nik", "full_name", "address", "birth_date"].forEach((field) => {
        this.$watch(`patientForm.${field}`, () => this.savePatientDraft());
      });
      try {
        await Promise.all([this.checkHealth(), this.restoreSession()]);
      } finally {
        this.sessionReady = true;
      }
      if (this.session) this.restorePending();

      if (this.session && this.route === "gate") {
        window.location.replace("/dashboard");
        return;
      }
      if (!this.session && this.route !== "gate") {
        window.location.replace("/login");
        return;
      }
      await this.loadRouteData();
    },

    // Each module is a real page, so navigation triggers a full document load.
    navigate(path) {
      if (!this.session && !["/", "/login", "/register"].includes(path)) {
        this.notify("Silakan masuk terlebih dahulu", "error");
        return;
      }
      const current = window.location.pathname.replace(/\/+$/, "") || "/";
      if (current === path) {
        this.loadRouteData();
        return;
      }
      window.location.assign(path);
    },

    // Hide a failed mascot image and flag its wrapper so the text fallback can
    // appear. Images may finish loading before Alpine initialises, so both
    // cases are checked.
    setupAssetFallbacks() {
      const drop = (img) => {
        if (img.parentElement) {
          img.parentElement.classList.add("is-asset-missing");
        }
        img.remove();
      };
      document.querySelectorAll("img[data-mv-asset]").forEach((img) => {
        if (img.complete && img.naturalWidth === 0) {
          drop(img);
          return;
        }
        img.addEventListener("error", () => drop(img), { once: true });
      });
    },

    async loadRouteData() {
      if (!this.session) return;
      try {
        if (["dashboard", "patients", "records"].includes(this.route)) {
          await this.loadPatients();
        }
        if (["dashboard", "audit"].includes(this.route)) {
          await this.loadAudit();
        }
      } catch (error) {
        this.notify(`Muat data: ${error.message || error}`, "error");
      }
    },

    async checkHealth() {
      this.healthState = "unknown";
      this.health = null;
      try {
        const response = await fetch("/healthz", { credentials: "same-origin" });
        if (!response.ok) throw new Error(`health status ${response.status}`);
        const env = await response.json();
        if (env.ok === false) throw new Error("health envelope reported failure");
        this.health = env.data || null;
        this.healthState = "ok";
      } catch (_) {
        this.health = null;
        this.healthState = "down";
      }
    },

    async restoreSession() {
      try {
        const env = await this.api("/api/v1/auth/me");
        const data = env.data || {};
        this.session = data.user || null;
        this.sessionInfo = data.session || null;
        if (this.session) {
          this.loginForm.username = this.session.username;
          if (!this.dongleFileHandle) {
            const handle = await idbGetDongleHandle();
            if (handle) {
              this.dongleFileHandle = handle;
            }
          }
          this.startDongleWatchdog();
        }
      } catch (_) {
        this.session = null;
        this.sessionInfo = null;
        this.stopDongleWatchdog();
      }
    },

    // The Glass-Box page reads the last trace from sessionStorage because each
    // navigation is a full document load. Only real API responses are stored.
    restoreTelemetry() {
      try {
        const raw = sessionStorage.getItem("medvault.telemetry");
        if (!raw) return;
        const saved = JSON.parse(raw);
        if (saved && saved.trace) {
          this.telemetry = saved.trace;
          this.telemetrySource = saved.source || null;
          this.telemetryAt = saved.captured_at || "";
        }
      } catch (_) {
        // Ignore an unreadable cache entry.
      }
    },

    notify(message, type = "ok") {
      this.toast = { type, message };
      window.clearTimeout(this._toastTimer);
      this._toastTimer = window.setTimeout(() => {
        this.toast = { type: "ok", message: "" };
      }, 6000);
    },

    // Copies the exact _telemetry object rendered by the Glass-Box Inspector.
    async copyTelemetry() {
      try {
        await navigator.clipboard.writeText(this.telemetryPretty);
        this.notify("JSON telemetry disalin");
      } catch (_) {
        this.notify("Gagal menyalin JSON telemetry", "error");
      }
    },

    async run(label, fn) {
      this.busy = true;
      try {
        return await fn();
      } catch (error) {
        this.notify(`${label}: ${error.message || error}`, "error");
        return null;
      } finally {
        this.busy = false;
      }
    },

    isPngFile(file) {
      if (!file) return false;
      const name = String(file.name || "");
      return /\.png$/i.test(name) || String(file.type || "") === "image/png";
    },

    setVaultFile(file) {
      if (!file) return;
      if (file.size > MAX_VAULT_FILE_BYTES) {
        this.vaultFile = null;
        this.notify("Berkas melebihi batas 64 MB", "error");
        return;
      }
      this.vaultFile = file;
    },

    setStegoFile(file, target) {
      if (!file) return;
      if (!this.isPngFile(file)) {
        this.notify("Berkas harus berformat PNG", "error");
        return;
      }
      if (target === "extract") {
        this.extractFile = file;
      } else {
        this.stegoFile = file;
      }
    },

    onDragOver(event) {
      if (event && typeof event.preventDefault === "function") event.preventDefault();
      if (event && event.dataTransfer) event.dataTransfer.dropEffect = "copy";
      if (event && event.currentTarget) event.currentTarget.classList.add("is-dragover");
    },

    onDragLeave(event) {
      if (event && typeof event.preventDefault === "function") event.preventDefault();
      if (event && event.currentTarget) event.currentTarget.classList.remove("is-dragover");
    },

    onDrop(event, target) {
      if (event && typeof event.preventDefault === "function") event.preventDefault();
      if (event && event.currentTarget) event.currentTarget.classList.remove("is-dragover");
      const files = event && event.dataTransfer ? event.dataTransfer.files : null;
      const file = files && files[0];
      if (!file) return;
      if (target === "vault") {
        this.setVaultFile(file);
      } else {
        this.setStegoFile(file, target);
      }
    },

    async setPatientView(view) {
      await this.run("Muat pasien", async () => {
        this.patientView = view;
        await this.loadPatients();
      });
    },

    async api(path, options = {}) {
      const request = {
        method: options.method || "GET",
        credentials: "same-origin",
        headers: {},
      };
      if (options.form) {
        request.body = options.form;
      } else if (options.body !== undefined) {
        request.headers["Content-Type"] = "application/json";
        request.body = JSON.stringify(options.body);
      }

      const response = await fetch(path, request);
      const text = await response.text();
      let envelope;
      try {
        envelope = text ? JSON.parse(text) : {};
      } catch (_) {
        throw new Error(`respons bukan JSON (${response.status})`);
      }

      if (envelope._telemetry) {
        this.telemetry = envelope._telemetry;
        this.telemetrySource = { method: request.method, path };
        this.telemetryAt = new Date().toISOString();
        try {
          sessionStorage.setItem(
            "medvault.telemetry",
            JSON.stringify({
              trace: this.telemetry,
              source: this.telemetrySource,
              captured_at: this.telemetryAt,
            })
          );
        } catch (_) {
          // Storage may be unavailable in private mode; the in-memory copy
          // still works for the current document.
        }
      }
      if (response.status === 401) {
        this.session = null;
        this.sessionInfo = null;
        if (this.route !== "gate" && !this._authRedirectPending) {
          this._authRedirectPending = true;
          window.location.replace("/login");
        }
      }
      if (!response.ok || envelope.ok === false) {
        throw new Error(envelope.error || `status ${response.status}`);
      }
      return envelope;
    },

    // ---------------------------------------------------------------
    // Dongle and camera
    // ---------------------------------------------------------------

    // ---------------------------------------------------------------
    // New dongle (registration step 2, "Buat dongle baru")
    // ---------------------------------------------------------------

    freshNewDongle() {
      return {
        generating: false,
        issuedAt: "",
        publicKeyHex: "",
        savedAs: "",
        saveMethod: "",
        saved: false,
        confirmed: false,
      };
    },

    resetDongleSelection() {
      this.dongle = null;
      this.dongleNote = "";
      this.registerForm.dongle_key_id = "";
      this.registerForm.dongle_pubkey_hex = "";
      this.registerTouched.dongle = false;
      this.newDongle = this.freshNewDongle();
    },

    chooseDongleMode(mode) {
      if (this.busy || this.newDongle.generating) return;
      if (mode === "new") {
        // Keep a key that was already generated for this username.
        if (this.dongleMode === "new" && this.dongle) return;
        this.dongleMode = "new";
        this.resetDongleSelection();
        void this.generateDongle();
        return;
      }
      this.dongleMode = "existing";
      this.resetDongleSelection();
      // Must run synchronously inside the click so the browser allows it.
      if (this.$refs.registerDongleInput) this.$refs.registerDongleInput.click();
    },

    regenerateDongle() {
      if (this.busy || this.newDongle.generating) return;
      if (
        this.newDongle.saved &&
        !window.confirm(
          "Kunci baru akan membuat berkas .medkey yang lama tidak berlaku untuk pendaftaran ini. Lanjutkan?"
        )
      ) {
        return;
      }
      this.resetDongleSelection();
      void this.generateDongle();
    },

    // Generates an Ed25519 pair in the browser. Only the public key is ever
    // sent to the server; the private seed lives in this tab and in the file
    // the user saves to the USB drive.
    async generateDongle() {
      const username = this.registerForm.username.trim();
      if (!username) {
        this.notify("Isi nama pengguna di langkah 1 dulu, ya", "error");
        return;
      }
      this.newDongle = { ...this.freshNewDongle(), generating: true };

      try {
        await this.run("Buat dongle", async () => {
          if (!window.crypto || !crypto.subtle) {
            throw new Error("browser tidak mendukung WebCrypto, buka lewat https atau localhost");
          }
          let pair;
          try {
            pair = await crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]);
          } catch (_) {
            throw new Error("browser belum mendukung Ed25519, gunakan Chrome atau Edge terbaru");
          }

          const pkcs8Hex = bytesToHex(await crypto.subtle.exportKey("pkcs8", pair.privateKey));
          if (!pkcs8Hex.startsWith(PKCS8_ED25519_PREFIX)) {
            throw new Error("format kunci dari browser tidak dikenali");
          }
          const seedHex = pkcs8Hex.slice(PKCS8_ED25519_PREFIX.length);
          const publicHex = bytesToHex(await crypto.subtle.exportKey("raw", pair.publicKey));
          if (!/^[0-9a-f]{64}$/.test(seedHex) || !/^[0-9a-f]{64}$/.test(publicHex)) {
            throw new Error("panjang kunci tidak sesuai");
          }
          if ((await this.derivePublicKeyHex(seedHex)) !== publicHex) {
            throw new Error("kunci publik tidak cocok dengan kunci privat");
          }

          const suffix = bytesToHex(crypto.getRandomValues(new Uint8Array(2)));
          const keyId =
            "DONGLE-" + new Date().getFullYear() + "-" + username.toUpperCase() + "-" + suffix.toUpperCase();

          this.dongle = { username, keyId, seedHex };
          this.dongleNote =
            "Kunci privat tetap di memori tab ini. Tutup tab untuk menghapusnya.";
          this.loginForm.username = username;
          this.registerForm.dongle_key_id = keyId;
          this.registerForm.dongle_pubkey_hex = publicHex;
          this.newDongle = {
            ...this.freshNewDongle(),
            issuedAt: new Date().toISOString(),
            publicKeyHex: publicHex,
          };
          this.notify("Kunci dongle baru sudah dibuat. Simpan ke USB ya.");
        });
      } finally {
        this.newDongle.generating = false;
      }
    },

    // Same JSON shape as cmd/medkeygen (internal/crypto MedKey), so the file
    // also works through "Pakai dongle yang sudah ada" and the login page.
    buildMedKeyText() {
      if (!this.dongle || !this.newDongle.publicKeyHex) {
        throw new Error("kunci belum dibuat");
      }
      return (
        JSON.stringify(
          {
            version: 1,
            key_id: this.dongle.keyId,
            username: this.dongle.username,
            algorithm: "Ed25519",
            public_key_hex: this.newDongle.publicKeyHex,
            private_key_seed_hex: this.dongle.seedHex,
            issued_at: this.newDongle.issuedAt,
          },
          null,
          2
        ) + "\n"
      );
    },

    async saveNewDongle() {
      if (this.busy || this.newDongle.generating) return;
      // showSaveFilePicker needs a fresh click, so it is called before any await.
      await this.run("Simpan dongle", async () => {
        const text = this.buildMedKeyText();
        const fileName = this.newDongleFileName;

        if (typeof window.showSaveFilePicker === "function") {
          let handle;
          try {
            handle = await window.showSaveFilePicker({
              suggestedName: fileName,
              types: [
                {
                  description: "Kunci USB Dongle MedVault",
                  accept: { "application/json": [".medkey"] },
                },
              ],
            });
          } catch (error) {
            if (error && error.name === "AbortError") {
              this.notify("Penyimpanan dibatalkan. Tekan Simpan ke USB untuk mencoba lagi.", "error");
              return;
            }
            throw error;
          }
          const writable = await handle.createWritable();
          await writable.write(text);
          await writable.close();

          // Read it back so we know the USB really holds a valid key.
          const written = JSON.parse(await (await handle.getFile()).text());
          if (
            written.private_key_seed_hex !== this.dongle.seedHex ||
            written.public_key_hex !== this.newDongle.publicKeyHex
          ) {
            throw new Error("berkas yang tersimpan tidak cocok, coba simpan ulang");
          }
          this.newDongle.savedAs = handle.name;
          this.newDongle.saveMethod = "picker";
          this.dongleFileHandle = handle;
          this.dongleConnected = true;
          this.dongleEjected = false;
          await idbSaveDongleHandle(handle);
        } else {
          const url = URL.createObjectURL(new Blob([text], { type: "application/json" }));
          const link = document.createElement("a");
          link.href = url;
          link.download = fileName;
          document.body.appendChild(link);
          link.click();
          link.remove();
          window.setTimeout(() => URL.revokeObjectURL(url), 1000);
          this.newDongle.savedAs = fileName;
          this.newDongle.saveMethod = "download";
        }

        this.newDongle.saved = true;
        this.notify("Berkas dongle tersimpan. Centang konfirmasi untuk melanjutkan.");
      });
    },


    async processDongleFile(file, handle = null) {
      await this.run("Baca dongle", async () => {
        const text = await file.text();
        const key = JSON.parse(text);
        const seed = String(key.private_key_seed_hex || "").trim();
        if (!/^[0-9a-fA-F]{64}$/.test(seed)) {
          throw new Error("berkas dongle tidak valid, pastikan berasal dari fasilitas Anda");
        }

        this.dongle = {
          username: key.username || "",
          keyId: key.key_id || "",
          seedHex: seed,
        };
        this.dongleFileHandle = handle;
        this.dongleConnected = Boolean(handle);
        this.dongleEjected = false;
        if (handle) {
          await idbSaveDongleHandle(handle);
        }
        this.dongleNote = handle
          ? "USB Dongle fisik terhubung dan dipantau secara real-time."
          : "Kunci privat tetap di memori tab ini. Tutup tab untuk menghapusnya.";

        if (this.dongle.username) {
          if (!this.registerForm.username.trim()) {
            this.registerForm.username = this.dongle.username;
          }
          if (!this.loginForm.username.trim()) {
            this.loginForm.username = this.dongle.username;
          }
        }
        if (this.dongle.keyId) {
          this.registerForm.dongle_key_id = this.dongle.keyId;
        }
        this.registerTouched.dongle = true;

        const publicHex = await this.derivePublicKeyHex(seed);
        const declaredPublic = String(key.public_key_hex || "").trim().toLowerCase();
        if (declaredPublic && declaredPublic !== publicHex.toLowerCase()) {
          throw new Error(
            "kunci publik pada berkas dongle tidak cocok dengan kunci privatnya"
          );
        }
        this.registerForm.dongle_pubkey_hex = publicHex;
        this.notify("Berkas dongle siap digunakan");
      });
    },

    async pickDongle() {
      if (typeof window.showOpenFilePicker === "function") {
        try {
          const [handle] = await window.showOpenFilePicker({
            multiple: false,
            types: [
              {
                description: "Kunci USB Dongle MedVault (*.medkey)",
                accept: { "application/json": [".medkey", ".json"] },
              },
            ],
          });
          if (!handle) return;
          const file = await handle.getFile();
          await this.processDongleFile(file, handle);
        } catch (error) {
          if (error && error.name === "AbortError") return;
          this.notify("Gagal membuka USB Dongle: " + (error.message || error), "error");
        }
      } else {
        const input = document.querySelector("input[data-mv-dongle-input]") ||
                      (this.$refs && (this.$refs.dongleInput || this.$refs.registerDongleInput));
        if (input) input.click();
      }
    },

    async loadDongle(event) {
      const file = event.target.files && event.target.files[0];
      if (!file) return;
      await this.processDongleFile(file, null);
      event.target.value = "";
    },

    async verifyDonglePresence() {
      if (!this.dongleFileHandle) {
        return !this.dongleSupported;
      }
      try {
        if (typeof this.dongleFileHandle.queryPermission === "function") {
          const perm = await this.dongleFileHandle.queryPermission({ mode: "read" });
          if (perm !== "granted") {
            this.dongleConnected = false;
            return false;
          }
        }
        const file = await this.dongleFileHandle.getFile();
        const slice = file.slice(0, 1);
        await slice.arrayBuffer();
        this.dongleConnected = true;
        this.dongleLastChecked = new Date().toISOString();
        return true;
      } catch (_) {
        this.dongleConnected = false;
        return false;
      }
    },

    startDongleWatchdog() {
      this.stopDongleWatchdog();
      if (!this.dongleSupported) return;

      void this.verifyDonglePresence();

      this._dongleWatchdogTimer = window.setInterval(async () => {
        if (!this.session) {
          this.stopDongleWatchdog();
          return;
        }
        if (this.dongleFileHandle) {
          const isAlive = await this.verifyDonglePresence();
          if (!isAlive && this.session && !this.dongleEjected) {
            await this.handleDongleEjection();
          }
        }
      }, 3000);
    },

    stopDongleWatchdog() {
      if (this._dongleWatchdogTimer) {
        window.clearInterval(this._dongleWatchdogTimer);
        this._dongleWatchdogTimer = null;
      }
    },

    async handleDongleEjection() {
      this.dongleEjected = true;
      this.dongleEjectedAt = new Date().toLocaleTimeString("id-ID");
      this.dongleConnected = false;

      if (this.dongle) {
        this.dongle.seedHex = "";
        this.dongle = null;
      }
      this.clearPending();
      await idbClearDongleHandle();

      try {
        await fetch("/api/v1/auth/logout", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "same-origin",
        });
      } catch (_) {}

      this.session = null;
      this.sessionInfo = null;
      this.patients = [];
      this.blocks = [];
      this.stopDongleWatchdog();
      this.notify("Peringatan: USB Dongle dicabut! Sesi kerja dikunci.", "error");
    },

    acknowledgeDongleEjection() {
      this.dongleEjected = false;
      this.dongleFileHandle = null;
      this.navigate("/login");
    },

    // Signing keys stay non-extractable. Public key derivation is the only
    // operation that needs an extractable import, so it asks for one explicitly.
    async importSeed(seedHex, extractable = false) {
      const der = hexToBytes(PKCS8_ED25519_PREFIX + seedHex);
      try {
        return await crypto.subtle.importKey(
          "pkcs8",
          der,
          { name: "Ed25519" },
          extractable,
          ["sign"]
        );
      } catch (_) {
        throw new Error(
          "browser ini belum mendukung keamanan USB Dongle, gunakan Chrome atau Edge terbaru"
        );
      }
    },

    async derivePublicKeyHex(seedHex) {
      const key = await this.importSeed(seedHex, true);
      const jwk = await crypto.subtle.exportKey("jwk", key);
      return base64urlToHex(jwk.x);
    },

    async signPayload(payload) {
      if (!this.dongle) {
        throw new Error("USB Dongle belum terdeteksi, muat berkas .medkey dulu");
      }
      if (this.dongleFileHandle) {
        const isPresent = await this.verifyDonglePresence();
        if (!isPresent) {
          await this.handleDongleEjection();
          throw new Error("USB Dongle tidak terdeteksi pada drive komputer, operasi dibatalkan");
        }
      }
      const key = await this.importSeed(this.dongle.seedHex);
      const signature = await crypto.subtle.sign(
        { name: "Ed25519" },
        key,
        new TextEncoder().encode(payload)
      );
      return bytesToHex(signature);
    },

    async startFacePreview() {
      if (this.facePreviewActive) return;

      await this.run("Nyalakan kamera", async () => {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
          throw new Error("browser ini belum menyediakan akses kamera, gunakan Chrome atau Edge terbaru");
        }

        const stream = await navigator.mediaDevices.getUserMedia({
          video: { width: 640, height: 480, facingMode: "user" },
          audio: false,
        });

        if (this.registerStep !== 3) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }

        const video = document.getElementById("register-face-video");
        if (!video) {
          stream.getTracks().forEach((track) => track.stop());
          throw new Error("pratinjau kamera tidak tersedia, muat ulang halaman lalu coba lagi");
        }

        video.srcObject = stream;
        video.muted = true;
        video.playsInline = true;
        await video.play();

        this.faceStream = stream;
        this.facePreviewActive = true;
        this.registerTouched.face = true;
      });
    },

    stopFacePreview() {
      if (this.faceStream) {
        this.faceStream.getTracks().forEach((track) => track.stop());
      }
      this.faceStream = null;
      this.facePreviewActive = false;
      const video = document.getElementById("register-face-video");
      if (video) video.srcObject = null;
    },

    async captureFace() {
      await this.run("Ambil kamera", async () => {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
          throw new Error("browser ini belum menyediakan akses kamera, gunakan Chrome atau Edge terbaru");
        }

        const reusable = this.facePreviewActive && this.faceStream;
        const stream =
          reusable ||
          (await navigator.mediaDevices.getUserMedia({
            video: { width: 640, height: 480, facingMode: "user" },
            audio: false,
          }));

        try {
          const preview = document.getElementById("register-face-video");
          const video = reusable && preview ? preview : document.createElement("video");
          video.muted = true;
          video.playsInline = true;
          if (!reusable) video.srcObject = stream;
          await video.play();
          await new Promise((resolve) => window.setTimeout(resolve, 350));

          const canvas = document.createElement("canvas");
          canvas.width = 9;
          canvas.height = 8;
          const ctx = canvas.getContext("2d", { willReadFrequently: true });
          ctx.drawImage(video, 0, 0, 9, 8);

          const hashHex = this.dhashFromCanvas(ctx);
          this.registerForm.webcam_dhash_hex = hashHex;
          this.registerTouched.face = true;
          this.faceStatus = `dHash ${hashHex}`;
          this.notify("Wajah berhasil dipindai");
        } finally {
          if (!reusable) {
            stream.getTracks().forEach((track) => track.stop());
          }
        }
      });
    },

    async startLoginFacePreview() {
      if (this.loginFacePreviewActive) return;

      await this.run("Nyalakan kamera", async () => {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
          throw new Error("browser ini belum menyediakan akses kamera, gunakan Chrome atau Edge terbaru");
        }

        const stream = await navigator.mediaDevices.getUserMedia({
          video: { width: 640, height: 480, facingMode: "user" },
          audio: false,
        });

        if (this.loginStep !== 3) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }

        const video = document.getElementById("login-face-video");
        if (!video) {
          stream.getTracks().forEach((track) => track.stop());
          throw new Error("pratinjau kamera tidak tersedia, muat ulang halaman lalu coba lagi");
        }

        video.srcObject = stream;
        video.muted = true;
        video.playsInline = true;
        await video.play();

        this.loginFaceStream = stream;
        this.loginFacePreviewActive = true;
        this.clearLoginFaceCapture();
      });
    },

    stopLoginFacePreview() {
      if (this.loginFaceStream) {
        this.loginFaceStream.getTracks().forEach((track) => track.stop());
      }
      this.loginFaceStream = null;
      this.loginFacePreviewActive = false;
      const video = document.getElementById("login-face-video");
      if (video) video.srcObject = null;
    },

    clearLoginFaceCapture() {
      this.loginForm.webcam_dhash_hex = "";
      this.loginFaceCaptured = false;
    },

    async captureLoginFace() {
      await this.run("Ambil kamera", async () => {
        this.clearLoginFaceCapture();

        if (!this.loginFacePreviewActive || !this.loginFaceStream) {
          throw new Error("nyalakan kamera dulu, lalu ambil verifikasi wajah");
        }

        const video = document.getElementById("login-face-video");
        if (!video) {
          throw new Error("pratinjau kamera tidak tersedia, muat ulang halaman lalu coba lagi");
        }

        for (let i = 0; i < 20 && video.readyState < 2; i += 1) {
          await new Promise((resolve) => window.setTimeout(resolve, 100));
        }
        if (video.readyState < 2) {
          throw new Error("kamera belum siap, tunggu sebentar lalu coba lagi");
        }

        // Let the exposure settle so a warming up sensor is not sampled.
        await new Promise((resolve) => window.setTimeout(resolve, 350));

        const canvas = document.createElement("canvas");
        canvas.width = 9;
        canvas.height = 8;
        const ctx = canvas.getContext("2d", { willReadFrequently: true });
        ctx.drawImage(video, 0, 0, 9, 8);

        const hashHex = this.dhashFromCanvas(ctx);
        this.loginForm.webcam_dhash_hex = hashHex;
        this.loginFaceCaptured = true;
        this.faceStatus = `dHash ${hashHex}`;
        this.notify("Wajah berhasil dipindai");
      });
    },

    backFromLoginFace() {
      this.stopLoginFacePreview();
      this.clearLoginFaceCapture();
      this.loginStep = 2;
    },

    dhashFromCanvas(ctx) {
      const { data } = ctx.getImageData(0, 0, 9, 8);
      const luminance = [];
      for (let i = 0; i < 9 * 8; i += 1) {
        const offset = i * 4;
        luminance.push(
          0.299 * data[offset] +
            0.587 * data[offset + 1] +
            0.114 * data[offset + 2]
        );
      }

      let bits = "";
      for (let y = 0; y < 8; y += 1) {
        for (let x = 0; x < 8; x += 1) {
          const left = luminance[y * 9 + x];
          const right = luminance[y * 9 + x + 1];
          bits += left > right ? "1" : "0";
        }
      }

      let hex = "";
      for (let i = 0; i < 64; i += 4) {
        hex += parseInt(bits.slice(i, i + 4), 2).toString(16);
      }
      return hex;
    },

    // ---------------------------------------------------------------
    // Authentication
    // ---------------------------------------------------------------

    nextRegisterStep() {
      if (this.registerStep === 1) {
        this.registerTouched.username = true;
        this.registerTouched.full_name = true;
        this.registerTouched.password = true;
        this.registerTouched.password_confirm = true;
        if (!this.registerAccountValid) {
          this.notify("Periksa data akun dulu, ya", "error");
          return;
        }
        this.registerStep = 2;
        if (this.dongleMode === "new") {
          const username = this.registerForm.username.trim();
          if (this.dongle && this.dongle.username !== username) {
            this.resetDongleSelection();
          }
          if (!this.dongle) void this.generateDongle();
        }
        return;
      }

      if (this.registerStep === 2) {
        this.registerTouched.dongle = true;
        if (!this.registerDongleValid) {
          this.notify(this.registerDongleError, "error");
          return;
        }
        this.registerStep = 3;
        void this.startFacePreview();
      }
    },

    prevRegisterStep() {
      if (this.registerStep === 3) {
        this.stopFacePreview();
      }
      if (this.registerStep > 1) {
        this.registerStep -= 1;
      }
    },

    async submitRegistration() {
      await this.run("Registrasi", async () => {
        this.registerTouched.face = true;
        if (!this.registerAccountValid) {
          this.registerStep = 1;
          throw new Error("periksa data akun dulu, ya");
        }
        if (!this.registerDongleValid) {
          this.registerStep = 2;
          throw new Error(this.registerDongleError);
        }
        if (!this.registerBiometricValid) {
          throw new Error(this.registerBiometricError);
        }
        if (!this.registerForm.dongle_pubkey_hex && this.dongle) {
          this.registerForm.dongle_pubkey_hex = await this.derivePublicKeyHex(
            this.dongle.seedHex
          );
        }

        const payload = {
          username: this.registerForm.username.trim(),
          full_name: this.registerForm.full_name.trim(),
          password: this.registerForm.password,
          dongle_key_id: this.registerForm.dongle_key_id.trim(),
          dongle_pubkey_hex: this.registerForm.dongle_pubkey_hex.trim().toLowerCase(),
          webcam_dhash_hex: this.registerForm.webcam_dhash_hex.trim().toLowerCase(),
        };

        const env = await this.api("/api/v1/auth/register", {
          method: "POST",
          body: payload,
        });

        this.loginForm.username = payload.username;
        this.registerStep = 4;
        this.stopFacePreview();
        this.registerForm.password = "";
        this.registerPasswordConfirm = "";
        this.notify(env.message || "Akun dokter berhasil dibuat");
      });
    },

    nextLoginStep() {
      if (this.loginStep === 1) {
        if (!this.loginForm.username.trim() || !this.loginForm.password) {
          this.notify("Isi nama pengguna dan kata sandi dulu, ya", "error");
          return;
        }
        this.loginStep = 2;
        return;
      }
      if (this.loginStep === 2) {
        if (!this.dongle) {
          this.notify("USB Dongle belum terdeteksi. Pastikan sudah tertancap, lalu coba lagi", "error");
          return;
        }
        if (this.dongle.username && this.dongle.username !== this.loginForm.username.trim()) {
          this.notify("Nama pengguna di USB Dongle tidak cocok dengan akun ini", "error");
          return;
        }
        this.clearLoginFaceCapture();
        this.loginStep = 3;
      }
    },

    async login() {
      await this.run("Masuk", async () => {
        const username = this.loginForm.username.trim();
        if (!username) throw new Error("isi nama pengguna dulu, ya");
        if (!this.loginFaceCaptured || !this.loginForm.webcam_dhash_hex) {
          throw new Error("wajah belum dipindai, ambil verifikasi wajah dulu");
        }

        const challengeEnv = await this.api(
          `/api/v1/auth/challenge?username=${encodeURIComponent(username)}`
        );
        const challenge = challengeEnv.data.challenge_hex;
        const signature = await this.signPayload(challenge);

        let env;
        try {
          env = await this.api("/api/v1/auth/login", {
            method: "POST",
            body: {
              username,
              password: this.loginForm.password,
              challenge_hex: challenge,
              signature_hex: signature,
              webcam_dhash_hex: this.loginForm.webcam_dhash_hex,
            },
          });
        } catch (error) {
          // A rejected face must not leave the previous reading marked as
          // ready. The user has to capture a fresh frame before retrying.
          this.clearLoginFaceCapture();
          throw error;
        }

        this.stopLoginFacePreview();
        this.session = (env.data && env.data.user) || { username };
        this.sessionInfo = (env.data && env.data.session) || null;
        this.loginStep = 1;
        this.startDongleWatchdog();
        this.notify(env.message || "Sesi aktif. Selamat bekerja.");
        this.navigate("/dashboard");
      });
    },

    async logout() {
      await this.run("Keluar", async () => {
        this.stopDongleWatchdog();
        this.dongleFileHandle = null;
        this.dongleConnected = false;
        this.dongleEjected = false;
        await idbClearDongleHandle();

        await this.api("/api/v1/auth/logout", { method: "POST", body: {} });
        this.session = null;
        this.sessionInfo = null;
        this.patients = [];
        this.blocks = [];
        this.validation = null;
        this.validationCheckedAt = "";
        this.telemetry = null;
        this.telemetrySource = null;
        this.telemetryAt = "";
        this.clearPending();
        this.clearPatientDraft();
        try {
          sessionStorage.removeItem("medvault.telemetry");
        } catch (_) {
          // Ignore storage failures on logout.
        }
        this.dongle = null;
        this.dongleNote = "";
        this.loginStep = 1;
        this.loginForm.password = "";
        this.loginForm.webcam_dhash_hex = "";
        this.notify("Sesi diakhiri");
        this.navigate("/login");
      });
    },

    async webauthnRegister() {
      await this.run("WebAuthn", async () => {
        const begin = await this.api("/api/v1/auth/webauthn/register/begin", {
          method: "POST",
          body: {},
        });
        const options = readWebAuthnOptions(begin.data.options);
        const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON
          ? PublicKeyCredential.parseCreationOptionsFromJSON(options)
          : options;
        const credential = await navigator.credentials.create({ publicKey });

        await this.api("/api/v1/auth/webauthn/register/finish", {
          method: "POST",
          body: {
            ceremony_token: begin.data.ceremony_token,
            response: credentialToJSON(credential),
          },
        });
        this.notify("Windows Hello terdaftar untuk akun ini");
      });
    },

    async webauthnLogin() {
      await this.run("WebAuthn", async () => {
        const username = this.loginForm.username.trim();
        if (!username) throw new Error("isi nama pengguna dulu, ya");

        // Ask for the assertion options first so an account without an
        // enrolled credential does not consume a dongle challenge.
        let begin;
        try {
          begin = await this.api("/api/v1/auth/webauthn/login/begin", {
            method: "POST",
            body: { username },
          });
        } catch (error) {
          if (/no Windows Hello credential|belum mendaftarkan/i.test(error.message || "")) {
            throw new Error("akun ini belum mendaftarkan Windows Hello, masuk dengan verifikasi wajah dulu");
          }
          throw error;
        }

        const challengeEnv = await this.api(
          `/api/v1/auth/challenge?username=${encodeURIComponent(username)}`
        );
        const challenge = challengeEnv.data.challenge_hex;
        const signature = await this.signPayload(challenge);

        const options = readWebAuthnOptions(begin.data.options);
        const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON
          ? PublicKeyCredential.parseRequestOptionsFromJSON(options)
          : options;
        const credential = await navigator.credentials.get({ publicKey });

        const env = await this.api("/api/v1/auth/webauthn/login/finish", {
          method: "POST",
          body: {
            ceremony_token: begin.data.ceremony_token,
            response: credentialToJSON(credential),
            challenge_hex: challenge,
            signature_hex: signature,
          },
        });

        this.session = (env.data && env.data.user) || { username };
        this.sessionInfo = (env.data && env.data.session) || null;
        this.stopLoginFacePreview();
        this.clearLoginFaceCapture();
        this.notify("Masuk dengan Windows Hello dan dongle");
        this.navigate("/dashboard");
      });
    },

    // ---------------------------------------------------------------
    // Patients and records
    // ---------------------------------------------------------------

    async loadPatients() {
      const env = await this.api(`/api/v1/patients?view=${this.patientView}`);
      const data = env.data || {};
      this.patients = data.patients || [];
    },

    // The last record id is kept locally so the vault, steganography, and audit
    // pages can prefill their forms after a full page navigation.
    rememberRecord(id) {
      this.inspectId = id;
      this.tamperRecordId = id;
      this.vaultRecordId = id;
      this.stegoRecordId = id;
      try {
        window.localStorage.setItem("medvault.recordId", String(id));
      } catch (_) {
        // Storage can be disabled; the in-memory value still applies.
      }
    },

    restoreRememberedRecord() {
      let stored = 0;
      try {
        stored = Number(window.localStorage.getItem("medvault.recordId") || 0);
      } catch (_) {
        stored = 0;
      }
      if (stored > 0) {
        this.inspectId = stored;
        this.tamperRecordId = stored;
        this.vaultRecordId = stored;
        this.stegoRecordId = stored;
      }
    },

    // The enrolment form is a full document load per module. Keep a draft in
    // sessionStorage so an expired session or an accidental refresh does not
    // throw away everything the doctor already typed.
    savePatientDraft() {
      const hasContent = Object.values(this.patientForm).some(
        (value) => String(value || "").trim() !== ""
      );
      try {
        if (!hasContent) {
          sessionStorage.removeItem(PATIENT_DRAFT_KEY);
          return;
        }
        sessionStorage.setItem(PATIENT_DRAFT_KEY, JSON.stringify(this.patientForm));
      } catch (_) {
        // Storage may be disabled; the in-memory form still applies.
      }
    },

    clearPatientDraft() {
      try {
        sessionStorage.removeItem(PATIENT_DRAFT_KEY);
      } catch (_) {
        // Ignore storage failures.
      }
    },

    restorePatientDraft() {
      try {
        const raw = sessionStorage.getItem(PATIENT_DRAFT_KEY);
        if (!raw) return;
        const saved = JSON.parse(raw);
        if (!saved || typeof saved !== "object") return;
        this.patientForm = {
          medical_record_number: String(saved.medical_record_number || ""),
          nik: String(saved.nik || ""),
          full_name: String(saved.full_name || ""),
          address: String(saved.address || ""),
          birth_date: String(saved.birth_date || ""),
        };
      } catch (_) {
        this.clearPatientDraft();
      }
    },

    patientApiError(error) {
      const message = String((error && error.message) || error || "");
      if (/already exists|already enrolled|sudah terdaftar|unique/i.test(message)) {
        return new Error("Nomor rekam medis sudah terdaftar. Gunakan nomor lain.");
      }
      if (/nik|16 digits/i.test(message)) {
        return new Error("NIK harus tepat 16 digit angka tanpa spasi.");
      }
      if (/birth_date|YYYY-MM-DD/i.test(message)) {
        return new Error("Tanggal lahir harus berformat YYYY-MM-DD.");
      }
      if (/are required|is required/i.test(message)) {
        return new Error("Lengkapi nomor rekam medis, NIK, nama lengkap, dan tanggal lahir.");
      }
      return error;
    },

    // Focuses the enrolment form without relying on a DOM global inside an
    // Alpine expression, so a missing element cannot break the page.
    focusPatientMrn() {
      const input = document.getElementById("patient-mrn");
      if (!input) return;
      input.focus();
      if (typeof input.scrollIntoView === "function") {
        input.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    },

    async createPatient() {
      const form = this.patientForm || {};
      const medicalRecordNumber = String(form.medical_record_number || "").trim();
      const nik = String(form.nik || "").trim();
      const fullName = String(form.full_name || "").trim();
      const address = String(form.address || "").trim();
      const birthDate = String(form.birth_date || "").trim();

      this.patientTouched.medical_record_number = true;
      this.patientTouched.nik = true;
      this.patientTouched.full_name = true;
      this.patientTouched.birth_date = true;

      if (!medicalRecordNumber || !nik || !fullName || !birthDate) {
        this.notify("Lengkapi nomor rekam medis, NIK, nama lengkap, dan tanggal lahir", "error");
        return;
      }
      if (!/^[0-9]{16}$/.test(nik)) {
        this.notify("NIK harus 16 digit angka tanpa spasi atau tanda baca", "error");
        return;
      }
      if (!/^\d{4}-\d{2}-\d{2}$/.test(birthDate)) {
        this.notify("Tanggal lahir harus berformat YYYY-MM-DD", "error");
        return;
      }

      await this.run("Simpan pasien", async () => {
        let env;
        try {
          env = await this.api("/api/v1/patients", {
            method: "POST",
            body: {
              medical_record_number: medicalRecordNumber,
              nik,
              full_name: fullName,
              address,
              birth_date: birthDate,
            },
          });
        } catch (error) {
          throw this.patientApiError(error);
        }
        const patient = env.data && env.data.patient;
        this.patientForm = {
          medical_record_number: "",
          nik: "",
          full_name: "",
          address: "",
          birth_date: "",
        };
        this.patientTouched = {
          medical_record_number: false,
          nik: false,
          full_name: false,
          birth_date: false,
        };
        this.clearPatientDraft();
        await this.loadPatients();
        if (patient) {
          this.recordForm.patient_id = patient.id;
        }
        this.notify(env.message || "Pasien tersimpan");
      });
    },

    async createRecord() {
      await this.run("Rekam medis", async () => {
        if (!this.recordForm.patient_id) {
          throw new Error("pilih pasien terlebih dahulu");
        }
        if (!String(this.recordForm.diagnosis || "").trim()) {
          throw new Error("isi diagnosis terlebih dahulu");
        }
        const env = await this.api("/api/v1/records", {
          method: "POST",
          body: this.recordForm,
        });
        const data = env.data || {};
        if (data.record) {
          this.rememberRecord(data.record.id);
        }
        if (data.proposal) {
          this.rememberPending(data.proposal);
        }
        this.recordForm = {
          patient_id: this.recordForm.patient_id,
          diagnosis: "",
          prescription: "",
          visit_status: this.recordForm.visit_status,
        };
        this.notify(env.message || "Rekam medis terenkripsi");
      });
    },

    async inspectRecord() {
      await this.run("Inspeksi", async () => {
        if (!this.inspectId) throw new Error("isi ID rekam medis");
        const env = await this.api(`/api/v1/records/${this.inspectId}/inspect`);
        this.inspection = env.data;
      });
    },

    // Every module navigation is a full document load, so the signature step
    // must survive a refresh. Mirror the proposal into sessionStorage and
    // restore it once the session is known to be valid.
    rememberPending(proposal) {
      this.pending = {
        token: proposal.proposal_token,
        payload: proposal.tx_payload,
        action: proposal.action_type,
        expires_at: proposal.expires_at || "",
      };
      try {
        sessionStorage.setItem(PENDING_STORAGE_KEY, JSON.stringify(this.pending));
      } catch (_) {
        // Storage may be disabled; the in-memory copy still applies.
      }
    },

    clearPending() {
      this.pending = null;
      try {
        sessionStorage.removeItem(PENDING_STORAGE_KEY);
      } catch (_) {
        // Ignore storage failures.
      }
    },

    restorePending() {
      try {
        const raw = sessionStorage.getItem(PENDING_STORAGE_KEY);
        if (!raw) return;
        const saved = JSON.parse(raw);
        if (
          !saved ||
          typeof saved.token !== "string" ||
          typeof saved.payload !== "string" ||
          !saved.token ||
          !saved.payload
        ) {
          this.clearPending();
          return;
        }
        if (saved.expires_at && Date.parse(saved.expires_at) <= Date.now()) {
          this.clearPending();
          return;
        }
        this.pending = {
          token: saved.token,
          payload: saved.payload,
          action: saved.action || "",
          expires_at: saved.expires_at || "",
        };
      } catch (_) {
        this.clearPending();
      }
    },

    async sealPending() {
      await this.run("Segel blok", async () => {
        if (!this.pending) throw new Error("tidak ada proposal aktif");
        const signature = await this.signPayload(this.pending.payload);
        const env = await this.api("/api/v1/records/seal", {
          method: "POST",
          body: {
            proposal_token: this.pending.token,
            signature_hex: signature,
          },
        });
        const block = env.data && env.data.block;
        this.clearPending();
        await this.loadAudit();
        this.notify(
          block
            ? `Blok ${block.block_index} tersegel dan diverifikasi validator`
            : "Blok tersegel"
        );
      });
    },

    // ---------------------------------------------------------------
    // Lab vault
    // ---------------------------------------------------------------

    async uploadLab() {
      await this.run("Segel berkas", async () => {
        if (!this.vaultRecordId) throw new Error("isi ID rekam medis");
        if (!this.vaultFile) throw new Error("pilih berkas");

        const form = new FormData();
        form.append("file", this.vaultFile);

        const env = await this.api(
          `/api/v1/vault/${this.vaultRecordId}/upload`,
          { method: "POST", form }
        );
        const data = env.data || {};
        if (data.proposal) {
          this.rememberPending(data.proposal);
        }
        this.vaultInfo = {
          original_name: (data.record || {}).lab_original_name,
          plaintext_bytes: data.plaintext_bytes,
          plaintext_sha256: data.plaintext_sha256,
          header_preview_hex: data.ciphertext_preview,
        };
        this.vaultFile = null;
        if (this.$refs.vaultFileInput) this.$refs.vaultFileInput.value = "";
        this.notify(env.message || "Berkas tersegel");
      });
    },

    async inspectVault() {
      await this.run("Metadata brankas", async () => {
        if (!this.vaultRecordId) throw new Error("isi ID rekam medis");
        const env = await this.api(
          `/api/v1/vault/${this.vaultRecordId}/inspect`
        );
        this.vaultInfo = env.data;
      });
    },

    vaultDownloadUrl() {
      return `/api/v1/vault/${this.vaultRecordId}/download`;
    },

    // ---------------------------------------------------------------
    // Steganography
    // ---------------------------------------------------------------

    async embedStego() {
      await this.run("Sisipkan watermark", async () => {
        if (!this.stegoRecordId) throw new Error("isi ID rekam medis");
        if (!this.stegoFile) throw new Error("pilih berkas PNG");

        const form = new FormData();
        form.append("image", this.stegoFile);

        const env = await this.api(
          `/api/v1/stego/${this.stegoRecordId}/embed`,
          { method: "POST", form }
        );
        const data = env.data || {};
        this.stegoResult = data;
        if (data.proposal) {
          this.rememberPending(data.proposal);
        }
        this.notify(env.message || "Watermark tersisip");
      });
    },

    async extractStego() {
      await this.run("Ekstraksi", async () => {
        if (!this.extractFile) throw new Error("pilih berkas PNG");
        const form = new FormData();
        form.append("image", this.extractFile);

        const env = await this.api("/api/v1/stego/extract", {
          method: "POST",
          form,
        });
        this.extracted = env.data;
        this.notify(env.message || "Watermark terverifikasi");
      });
    },

    stegoImageUrl() {
      return `/api/v1/stego/${this.stegoRecordId}/image?t=${Date.now()}`;
    },

    // ---------------------------------------------------------------
    // Audit chain
    // ---------------------------------------------------------------

    async loadAudit() {
      const env = await this.api("/api/v1/audit/blocks");
      const data = env.data || {};
      this.blocks = data.blocks || [];
      this.validation = data.validation || null;
      this.validationCheckedAt = new Date().toISOString();
    },

    async simulateTamper() {
      await this.run("Simulasi tamper", async () => {
        if (!this.tamperRecordId) throw new Error("isi ID rekam medis");
        await this.api("/api/v1/audit/simulate-tamper", {
          method: "POST",
          body: { record_id: this.tamperRecordId },
        });
        await this.loadAudit();
        this.notify("Ciphertext dirusak, periksa status rantai");
      });
    },

    async restoreTamper() {
      await this.run("Pulihkan", async () => {
        if (!this.tamperRecordId) throw new Error("isi ID rekam medis");
        await this.api("/api/v1/audit/restore-tamper", {
          method: "POST",
          body: { record_id: this.tamperRecordId },
        });
        await this.loadAudit();
        this.notify("Ciphertext asli dipulihkan");
      });
    },
  };
};