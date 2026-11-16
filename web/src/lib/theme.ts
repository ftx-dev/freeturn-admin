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
  // Иконка вкладки - под тему: светлая в светлой, тёмная в тёмной.
  document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.setAttribute("href", t === "light" ? "/favicon-light.png" : "/favicon.png");
  try { localStorage.setItem("fta-theme", t); } catch { /* приватный режим */ }
}
