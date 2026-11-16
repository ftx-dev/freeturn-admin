const dt = new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" });

export const formatDate = (unix: number) => (unix ? dt.format(new Date(unix * 1000)) : "—");

export function timeAgo(unix: number): string {
  if (!unix) return "никогда";
  const s = Math.max(0, Math.floor(Date.now() / 1000 - unix));
  if (s < 60) return "только что";
  if (s < 3600) return `${Math.floor(s / 60)} мин назад`;
  if (s < 86400) return `${Math.floor(s / 3600)} ч назад`;
  return `${Math.floor(s / 86400)} дн назад`;
}

export const ACTIONS: Record<string, string> = {
  setup: "Создан администратор",
  login: "Вход",
  "server.create": "Добавлен сервер",
  "server.update": "Изменён сервер",
  "server.delete": "Удалён сервер",
  "server.ssh_test": "Проверка SSH",
  "server.trust": "Подтверждён ключ хоста",
  "server.apply": "Установка или настройка",
  "server.stop": "Остановка FreeTurn",
  "server.restart": "Перезапуск FreeTurn",
  "server.uninstall": "Удаление FreeTurn",
  "user.add": "Выдан доступ",
  "user.remove": "Отозван доступ",
  "server.calls": "Изменены звонки сервера",
  "admin.create": "Создана учётка админки",
  "admin.update": "Изменена учётка админки",
  "admin.delete": "Удалена учётка админки",
  "admin.password": "Сменён свой пароль",
};
