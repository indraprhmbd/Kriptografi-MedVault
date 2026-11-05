import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { ThemeProvider } from "./components/ThemeContext";
import { VerboseProvider } from "./components/VerboseContext";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      <VerboseProvider>
        <App />
      </VerboseProvider>
    </ThemeProvider>
  </React.StrictMode>
);
