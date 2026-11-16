// Тема: тёмная по умолчанию, выбор запоминается в браузере (только удобство, не настройка).
export type Theme = "dark" | "light";

export function loadTheme(): Theme {
  try {
    return localStorage.getItem("fta-theme") === "light" ? "light" : "dark";
  } catch {
    return "dark";
  }
}

export function applyTheme(t: Theme) {
  document.documentElement.dataset.theme = t;
  try { localStorage.setItem("fta-theme", t); } catch { /* приватный режим */ }
}
