// MedVault Go dashboard controller.
//
// All cryptography that touches the dongle happens here in the browser. The
// server only ever receives an Ed25519 signature over a canonical transaction
// payload. WebCrypto Ed25519 support is required for the signing flow.

const PKCS8_ED25519_PREFIX = "302e020100300506032b657004220420";

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

    session: null,
    health: null,
    dongle: null,
    dongleNote: "",
    faceStatus: "",
    faceStream: null,
    facePreviewActive: false,

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
    tamperRecordId: 0,

    pending: null,

    get telemetryPretty() {
      if (!this.telemetry) {
        return "Belum ada respons kriptografi. Lakukan satu aksi untuk melihat nilai antara.";
      }
      return JSON.stringify(this.telemetry, null, 2);
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
        return "Hubungkan USB Dongle fisik yang diberikan fasilitas Anda.";
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
      if (!this.dongle) {
        return "USB Dongle belum terdeteksi. Pastikan sudah tertancap, lalu coba lagi.";
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
      return "";
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
      return (this.registerTouched.dongle || Boolean(this.dongle)) && Boolean(this.registerDongleError);
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
      if (!this._faceUnloadBound) {
        this._faceUnloadBound = true;
        window.addEventListener("beforeunload", () => this.stopFacePreview());
      }
      this.restoreRememberedRecord();
      await this.checkHealth();
      await this.restoreSession();

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
      try {
        const response = await fetch("/healthz", { credentials: "same-origin" });
        if (!response.ok) return;
        const env = await response.json();
        this.health = env.data || null;
      } catch (_) {
        // The banner stays empty when the API is unreachable.
      }
    },

    async restoreSession() {
      try {
        const env = await this.api("/api/v1/auth/me");
        const data = env.data || {};
        this.session = data.user || null;
        if (this.session) {
          this.loginForm.username = this.session.username;
        }
      } catch (_) {
        this.session = null;
      }
    },

    notify(message, type = "ok") {
      this.toast = { type, message };
      window.clearTimeout(this._toastTimer);
      this._toastTimer = window.setTimeout(() => {
        this.toast = { type: "ok", message: "" };
      }, 6000);
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
      }
      if (response.status === 401) {
        this.session = null;
      }
      if (!response.ok || envelope.ok === false) {
        throw new Error(envelope.error || `status ${response.status}`);
      }
      return envelope;
    },

    // ---------------------------------------------------------------
    // Dongle and camera
    // ---------------------------------------------------------------

    async loadDongle(event) {
      const file = event.target.files && event.target.files[0];
      if (!file) return;

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
        this.dongleNote =
          "Kunci privat tetap di memori tab ini. Tutup tab untuk menghapusnya.";

        if (this.dongle.username) {
          if (!this.registerForm.username.trim()) {
            this.registerForm.username = this.dongle.username;
          }
          this.loginForm.username = this.dongle.username;
        }
        if (this.dongle.keyId) {
          this.registerForm.dongle_key_id = this.dongle.keyId;
        }
        this.registerTouched.dongle = true;

        const publicHex = await this.derivePublicKeyHex(seed);
        this.registerForm.dongle_pubkey_hex = publicHex;
        this.notify("Berkas dongle siap digunakan");
      });
      event.target.value = "";
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
          this.loginForm.webcam_dhash_hex = hashHex;
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
        this.loginStep = 3;
      }
    },

    async login() {
      await this.run("Masuk", async () => {
        const username = this.loginForm.username.trim();
        if (!username) throw new Error("isi nama pengguna dulu, ya");
        if (!this.loginForm.webcam_dhash_hex) {
          throw new Error("wajah belum dipindai, ambil verifikasi wajah dulu");
        }

        const challengeEnv = await this.api(
          `/api/v1/auth/challenge?username=${encodeURIComponent(username)}`
        );
        const challenge = challengeEnv.data.challenge_hex;
        const signature = await this.signPayload(challenge);

        const env = await this.api("/api/v1/auth/login", {
          method: "POST",
          body: {
            username,
            password: this.loginForm.password,
            challenge_hex: challenge,
            signature_hex: signature,
            webcam_dhash_hex: this.loginForm.webcam_dhash_hex,
          },
        });

        this.session = (env.data && env.data.user) || { username };
        this.loginStep = 1;
        this.notify(env.message || "Sesi aktif. Selamat bekerja.");
        this.navigate("/dashboard");
      });
    },

    async logout() {
      await this.run("Keluar", async () => {
        await this.api("/api/v1/auth/logout", { method: "POST", body: {} });
        this.session = null;
        this.patients = [];
        this.blocks = [];
        this.validation = null;
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

        const challengeEnv = await this.api(
          `/api/v1/auth/challenge?username=${encodeURIComponent(username)}`
        );
        const challenge = challengeEnv.data.challenge_hex;
        const signature = await this.signPayload(challenge);

        const begin = await this.api("/api/v1/auth/webauthn/login/begin", {
          method: "POST",
          body: { username },
        });
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

    async createPatient() {
      await this.run("Simpan pasien", async () => {
        const env = await this.api("/api/v1/patients", {
          method: "POST",
          body: this.patientForm,
        });
        const patient = env.data && env.data.patient;
        this.patientForm = {
          medical_record_number: "",
          nik: "",
          full_name: "",
          address: "",
          birth_date: "",
        };
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
        const env = await this.api("/api/v1/records", {
          method: "POST",
          body: this.recordForm,
        });
        const data = env.data || {};
        if (data.record) {
          this.rememberRecord(data.record.id);
        }
        if (data.proposal) {
          this.pending = {
            token: data.proposal.proposal_token,
            payload: data.proposal.tx_payload,
            action: data.proposal.action_type,
          };
        }
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
        this.pending = null;
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
          this.pending = {
            token: data.proposal.proposal_token,
            payload: data.proposal.tx_payload,
            action: data.proposal.action_type,
          };
        }
        this.vaultInfo = {
          original_name: (data.record || {}).lab_original_name,
          ciphertext_bytes: 0,
          plaintext_sha256: data.plaintext_sha256,
          header_preview_hex: data.ciphertext_preview,
        };
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
          this.pending = {
            token: data.proposal.proposal_token,
            payload: data.proposal.tx_payload,
            action: data.proposal.action_type,
          };
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