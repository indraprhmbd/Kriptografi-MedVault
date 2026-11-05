/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        google: {
          blue: "#1a73e8",
          blueHover: "#1557b0",
          blueDark: "#8ab4f8",
          green: "#1e8e3e",
          greenDark: "#81c995",
          red: "#d93025",
          redDark: "#f28b82",
          yellow: "#f9ab00",
          yellowDark: "#fdd663",
          bgLight: "#f8f9fa",
          bgDark: "#131314",
          surfaceLight: "#ffffff",
          surfaceDark: "#1e1f20",
          surfaceElevatedLight: "#f1f3f4",
          surfaceElevatedDark: "#282a2c",
          borderLight: "#dadce0",
          borderDark: "#3c4043",
          textPrimaryLight: "#202124",
          textPrimaryDark: "#e8eaed",
          textSecondaryLight: "#5f6368",
          textSecondaryDark: "#9aa0a6",
        },
      },
    },
  },
  plugins: [],
};
