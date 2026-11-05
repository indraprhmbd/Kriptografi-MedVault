import { useEffect, useRef, useState } from "react";
import { CameraIcon, CheckIcon, RefreshIcon } from "./Icons";

interface WebcamCaptureProps {
  onCapture: (file: File) => void;
  onClear?: () => void;
  label?: string;
}

export function WebcamCapture({
  onCapture,
  onClear,
  label = "Kamera Webcam Verifikasi Wajah",
}: WebcamCaptureProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [capturedUrl, setCapturedUrl] = useState<string | null>(null);
  const [cameraError, setCameraError] = useState<string | null>(null);
  const [isStarting, setIsStarting] = useState(false);
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [selectedDeviceId, setSelectedDeviceId] = useState<string>("");

  const stopActiveStream = () => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
    setStream(null);
  };

  const startCamera = async (deviceId?: string) => {
    setCameraError(null);
    setIsStarting(true);
    stopActiveStream();

    try {
      const constraints: MediaStreamConstraints = {
        video: deviceId
          ? { deviceId: { exact: deviceId }, width: { ideal: 640 }, height: { ideal: 480 } }
          : { width: { ideal: 640 }, height: { ideal: 480 }, facingMode: "user" },
        audio: false,
      };

      const mediaStream = await navigator.mediaDevices.getUserMedia(constraints);
      streamRef.current = mediaStream;
      setStream(mediaStream);

      if (videoRef.current) {
        videoRef.current.srcObject = mediaStream;
        videoRef.current.play().catch((err) => {
          console.warn("[WebcamCapture] Autoplay dicegah oleh browser:", err);
        });
      }

      // Query available video devices
      try {
        const allDevices = await navigator.mediaDevices.enumerateDevices();
        const videoInputs = allDevices.filter((d) => d.kind === "videoinput");
        setDevices(videoInputs);

        const currentTrack = mediaStream.getVideoTracks()[0];
        const activeDeviceId = currentTrack?.getSettings()?.deviceId;
        if (activeDeviceId) {
          setSelectedDeviceId(activeDeviceId);
        }
      } catch {
        // enumeration is optional
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setCameraError(`Kamera tidak dapat diakses: ${msg}`);
    } finally {
      setIsStarting(false);
    }
  };

  useEffect(() => {
    startCamera();
    return () => {
      stopActiveStream();
      if (capturedUrl) {
        URL.revokeObjectURL(capturedUrl);
      }
    };
  }, []);

  // Ensure videoRef attaches stream whenever stream changes
  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream;
      videoRef.current.play().catch(() => {});
    }
  }, [stream]);

  const handleDeviceChange = (deviceId: string) => {
    setSelectedDeviceId(deviceId);
    if (capturedUrl) {
      URL.revokeObjectURL(capturedUrl);
      setCapturedUrl(null);
      if (onClear) onClear();
    }
    startCamera(deviceId);
  };

  const handleCapture = () => {
    if (!videoRef.current) return;

    const video = videoRef.current;
    const canvas = document.createElement("canvas");
    canvas.width = video.videoWidth || 640;
    canvas.height = video.videoHeight || 480;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    // Draw flipped horizontal to match mirror mode
    ctx.translate(canvas.width, 0);
    ctx.scale(-1, 1);
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        const file = new File([blob], "webcam_snapshot.jpg", { type: "image/jpeg" });
        if (capturedUrl) {
          URL.revokeObjectURL(capturedUrl);
        }
        const previewUrl = URL.createObjectURL(blob);
        setCapturedUrl(previewUrl);
        onCapture(file);
      },
      "image/jpeg",
      0.95
    );
  };

  const handleRetake = () => {
    if (capturedUrl) {
      URL.revokeObjectURL(capturedUrl);
    }
    setCapturedUrl(null);
    if (onClear) {
      onClear();
    }

    if (videoRef.current) {
      if (streamRef.current && streamRef.current.active) {
        videoRef.current.srcObject = streamRef.current;
        videoRef.current.play().catch(() => {});
      } else {
        startCamera(selectedDeviceId || undefined);
      }
    }
  };

  return (
    <div className="bg-white dark:bg-[#1e1f20] border border-[#dadce0] dark:border-[#3c4043] rounded-xl p-3 shadow-xs flex flex-col justify-between">
      {/* Header with Camera Selection */}
      <div className="flex flex-wrap items-center justify-between gap-2 mb-2.5">
        <label className="text-xs font-semibold text-[#202124] dark:text-[#e8eaed] flex items-center gap-1.5">
          <CameraIcon className="w-3.5 h-3.5 text-[#1a73e8] dark:text-[#8ab4f8]" />
          <span>{label}</span>
        </label>

        <div className="flex items-center gap-1.5">
          {devices.length > 1 && (
            <select
              value={selectedDeviceId}
              onChange={(e) => handleDeviceChange(e.target.value)}
              className="bg-white dark:bg-[#131314] border border-[#dadce0] dark:border-[#3c4043] rounded px-2 py-0.5 text-xs text-[#202124] dark:text-[#e8eaed] focus:outline-none focus:border-[#1a73e8]"
            >
              {devices.map((d, idx) => (
                <option key={d.deviceId || idx} value={d.deviceId}>
                  {d.label || `Kamera ${idx + 1}`}
                </option>
              ))}
            </select>
          )}

          <button
            type="button"
            onClick={() => startCamera(selectedDeviceId || undefined)}
            title="Muat ulang kamera"
            className="p-1 rounded border border-[#dadce0] dark:border-[#3c4043] text-[#5f6368] dark:text-[#9aa0a6] hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] transition"
          >
            <RefreshIcon className={`w-3 h-3 ${isStarting ? "animate-spin" : ""}`} />
          </button>

          {capturedUrl && (
            <span className="text-xs px-2 py-0.5 rounded bg-emerald-50 dark:bg-emerald-950/60 text-[#1e8e3e] dark:text-[#81c995] border border-emerald-200 dark:border-emerald-800 font-medium flex items-center gap-1">
              <CheckIcon className="w-3.5 h-3.5" />
              <span>Foto Tersimpan</span>
            </span>
          )}
        </div>
      </div>

      {cameraError ? (
        <div className="p-3 bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900 rounded-md text-rose-700 dark:text-rose-300 text-xs text-center space-y-1.5">
          <p>{cameraError}</p>
          <button
            type="button"
            onClick={() => startCamera(selectedDeviceId || undefined)}
            disabled={isStarting}
            className="px-2.5 py-1 bg-white dark:bg-[#282a2c] border border-rose-300 dark:border-rose-800 text-rose-700 dark:text-rose-300 rounded text-xs font-medium transition"
          >
            Coba Nyalakan Lagi
          </button>
        </div>
      ) : (
        <div className="relative rounded-md overflow-hidden border border-[#dadce0] dark:border-[#3c4043] bg-black aspect-video flex items-center justify-center">
          {/* Permanent Video Element */}
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            className={`w-full h-full object-cover mirror-mode ${capturedUrl ? "hidden" : "block"}`}
          />

          {/* Facial targeting guide */}
          {!capturedUrl && (
            <div className="absolute inset-0 pointer-events-none flex items-center justify-center">
              <div className="w-36 h-48 border-2 border-blue-400/50 rounded-full border-dashed" />
            </div>
          )}

          {/* Captured snapshot display */}
          {capturedUrl && (
            <img
              src={capturedUrl}
              alt="Snapshot Wajah"
              className="w-full h-full object-cover"
            />
          )}

          {/* Action buttons */}
          <div className="absolute bottom-2.5 inset-x-0 flex justify-center gap-2">
            {capturedUrl ? (
              <button
                type="button"
                onClick={handleRetake}
                className="flex items-center gap-1 px-3 py-1.5 bg-white dark:bg-[#1e1f20] text-[#202124] dark:text-[#e8eaed] text-xs font-semibold rounded-md border border-[#dadce0] dark:border-[#3c4043] shadow-md hover:bg-[#f1f3f4] dark:hover:bg-[#282a2c] transition"
              >
                <RefreshIcon className="w-3 h-3" />
                <span>Ambil Ulang</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={handleCapture}
                disabled={isStarting}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-[#1a73e8] hover:bg-[#1557b0] disabled:opacity-50 text-white text-xs font-semibold rounded-md shadow-md transition transform active:scale-95"
              >
                <CameraIcon className="w-3.5 h-3.5" />
                <span>Ambil Foto Snapshot</span>
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
