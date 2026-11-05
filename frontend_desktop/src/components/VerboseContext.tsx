import React, { createContext, useContext, useEffect, useState } from "react";

interface VerboseContextType {
  isVerbose: boolean;
  toggleVerbose: () => void;
  setVerbose: (val: boolean) => void;
}

const VerboseContext = createContext<VerboseContextType | undefined>(undefined);

const STORAGE_KEY = "medvault_verbose";

export function VerboseProvider({ children }: { children: React.ReactNode }) {
  const [isVerbose, setIsVerboseState] = useState<boolean>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === "true";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, String(isVerbose));
    } catch {
      // Storage unavailable or disabled
    }
  }, [isVerbose]);

  const toggleVerbose = () => setIsVerboseState((prev) => !prev);
  const setVerbose = (val: boolean) => setIsVerboseState(val);

  return (
    <VerboseContext.Provider value={{ isVerbose, toggleVerbose, setVerbose }}>
      {children}
    </VerboseContext.Provider>
  );
}

export function useVerbose(): VerboseContextType {
  const ctx = useContext(VerboseContext);
  if (!ctx) {
    throw new Error("useVerbose must be used within a VerboseProvider");
  }
  return ctx;
}
